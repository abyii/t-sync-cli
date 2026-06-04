package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"text/tabwriter"

	tsyncv2 "github.com/abyii/t-sync-sdk-go/v2/gen/go/com/github/abyii/tsync/v2"
	"github.com/abyii/t-sync-sdk-go/v2/tsync"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var (
	inspectFile        string
	inspectRemote      string
	inspectVersionRaw  bool
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Low-level read-only operations directly on a .tsync metadata file or store",
	Long:  `Inspect the underlying protobuf metadata (.tsync) either from a local file or a remote backup store.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var inspectSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show high-level summary of the backup store metadata",
	Run: func(cmd *cobra.Command, args []string) {
		meta, err := loadMetadata()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("--- T-Sync Metadata Summary ---")
		fmt.Printf("Schema Version:  %d\n", meta.SchemaVersion)
		if meta.StoreLabel != nil {
			fmt.Printf("Store Label:     %s\n", *meta.StoreLabel)
		} else {
			fmt.Println("Store Label:     (none)")
		}
		if meta.LastUpdated != nil {
			fmt.Printf("Last Updated:    %s\n", meta.LastUpdated.AsTime().Local().Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("Total Versions:  %d\n", len(meta.Versions))
		fmt.Printf("Total TreeNodes: %d\n", len(meta.Trees))
		fmt.Printf("Total Unique Files: %d\n", len(meta.Files))

		fmt.Println("\nRegistered Public Keys in Store:")
		if len(meta.PublicKeys) == 0 {
			fmt.Println("  (none)")
		} else {
			for keyID, pubBytes := range meta.PublicKeys {
				pubHex := hex.EncodeToString(pubBytes)
				fmt.Printf("  - %s: %s\n", keyID, pubHex)
			}
		}
	},
}

var inspectVersionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "List all backup versions with their sizes and metrics",
	Run: func(cmd *cobra.Command, args []string) {
		meta, err := loadMetadata()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(meta.Versions) == 0 {
			fmt.Println("No versions found in metadata.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "VERSION ID\tTIMESTAMP\tROOT HASH\tPRECEDING ID\tFILES\tSIZE\tCOMPRESSED\tLABEL")

		var versionIDs []uint64
		for k := range meta.Versions {
			var id uint64
			fmt.Sscanf(k, "%d", &id)
			versionIDs = append(versionIDs, id)
		}
		
		for i := 0; i < len(versionIDs); i++ {
			for j := i + 1; j < len(versionIDs); j++ {
				if versionIDs[i] > versionIDs[j] {
					versionIDs[i], versionIDs[j] = versionIDs[j], versionIDs[i]
				}
			}
		}

		for _, id := range versionIDs {
			k := fmt.Sprintf("%d", id)
			v := meta.Versions[k]
			
			rootHashStr := truncateKey(v.RootTreeHash)
			
			precedingIDStr := "-"
			if v.PrecedingVersionId != 0 {
				precedingIDStr = fmt.Sprintf("%d", v.PrecedingVersionId)
			}

			fileCount, uncomp, comp, err := getInspectVersionStats(meta, v)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to calculate stats for version %d: %v\n", id, err)
			}

			label := ""
			if v.Label != "" {
				label = v.Label
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
				id,
				v.BackupTimestamp.AsTime().Local().Format("2006-01-02 15:04:05"),
				rootHashStr,
				precedingIDStr,
				fileCount,
				FormatBytes(uncomp),
				FormatBytes(comp),
				label,
			)
		}
		w.Flush()
	},
}

var inspectVersionCmd = &cobra.Command{
	Use:   "version <version-id>",
	Short: "Show details and file mappings for a specific version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var versionID uint64
		_, err := fmt.Sscanf(args[0], "%d", &versionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid version ID '%s': %v\n", args[0], err)
			os.Exit(1)
		}

		meta, err := loadMetadata()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		vKey := fmt.Sprintf("%d", versionID)
		v, exists := meta.Versions[vKey]
		if !exists {
			fmt.Fprintf(os.Stderr, "Error: Version %d not found in metadata\n", versionID)
			os.Exit(1)
		}

		fmt.Printf("--- Version %d Details ---\n", versionID)
		fmt.Printf("Timestamp:    %s\n", v.BackupTimestamp.AsTime().Local().Format("2006-01-02 15:04:05"))
		fmt.Printf("Root Hash:    %s\n", v.RootTreeHash)
		if v.PrecedingVersionId != 0 {
			fmt.Printf("Preceding ID: %d\n", v.PrecedingVersionId)
		}
		if v.Label != "" {
			fmt.Printf("Label:        %s\n", v.Label)
		}

		if inspectVersionRaw {
			fmt.Println("\nRaw Tree Structure:")
			printRawTree(v.RootTreeHash, "  ", meta.Trees)
		} else {
			resolvedMap, err := tsync.ResolveVersionMap(meta, versionID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving version map: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("\nResolved File Map (%d files):\n", len(resolvedMap))
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "FILE PATH\tFILE KEY\tSIZE\tCOMPRESSED\tKEY ID")
			for path, fKey := range resolvedMap {
				sizeStr := "-"
				compStr := "-"
				keyID := "-"
				if rec, exists := meta.Files[fKey]; exists {
					sizeStr = FormatBytes(rec.UncompressedSize)
					compStr = FormatBytes(rec.CompressedSize)
					if rec.KeyId != nil {
						keyID = *rec.KeyId
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", path, fKey, sizeStr, compStr, keyID)
			}
			w.Flush()
		}
	},
}

func printRawTree(treeHash string, indent string, trees map[string]*tsyncv2.TreeNode) {
	node, exists := trees[treeHash]
	if !exists {
		fmt.Printf("%s[MISSING TREE NODE: %s]\n", indent, treeHash)
		return
	}
	for _, entry := range node.Entries {
		switch n := entry.Node.(type) {
		case *tsyncv2.TreeEntry_File:
			if n.File == nil {
				fmt.Printf("%s- %s (nil file)\n", indent, entry.Name)
			} else {
				fmt.Printf("%s- %s (crc32=0x%08x, size=%d)\n", indent, entry.Name, n.File.Crc32, n.File.UncompressedSize)
			}
		case *tsyncv2.TreeEntry_SubtreeHash:
			fmt.Printf("%s+ %s/ (hash=%s)\n", indent, entry.Name, truncateKey(n.SubtreeHash))
			printRawTree(n.SubtreeHash, indent+"  ", trees)
		}
	}
}

var inspectFilesCmd = &cobra.Command{
	Use:   "files",
	Short: "List all unique FileRecords in the metadata",
	Run: func(cmd *cobra.Command, args []string) {
		meta, err := loadMetadata()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if len(meta.Files) == 0 {
			fmt.Println("No file records found in metadata.")
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "FILE KEY\tCRC32 (HEX)\tSIZE\tCOMPRESSED\tKEY ID\tLAST MODIFIED")

		for fKey, rec := range meta.Files {
			keyID := "-"
			if rec.KeyId != nil {
				keyID = *rec.KeyId
			}
			modTime := "-"
			if rec.LastModified != nil {
				modTime = rec.LastModified.AsTime().Local().Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(w, "%s\t%08x\t%s\t%s\t%s\t%s\n",
				fKey,
				rec.Crc32,
				FormatBytes(rec.UncompressedSize),
				FormatBytes(rec.CompressedSize),
				keyID,
				modTime,
			)
		}
		w.Flush()
	},
}

var inspectFileCmd = &cobra.Command{
	Use:   "file <file-key>",
	Short: "Show detailed raw metadata fields for a specific FileRecord",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fileKey := args[0]
		meta, err := loadMetadata()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		rec, exists := meta.Files[fileKey]
		if !exists {
			fmt.Fprintf(os.Stderr, "Error: FileRecord for key '%s' not found.\n", fileKey)
			os.Exit(1)
		}

		fmt.Printf("--- FileRecord Details for '%s' ---\n", fileKey)
		fmt.Printf("CRC32:            %08x (decimal: %d)\n", rec.Crc32, rec.Crc32)
		fmt.Printf("Uncompressed:     %s (%d bytes)\n", FormatBytes(rec.UncompressedSize), rec.UncompressedSize)
		fmt.Printf("Compressed:       %s (%d bytes)\n", FormatBytes(rec.CompressedSize), rec.CompressedSize)
		if rec.LastModified != nil {
			fmt.Printf("Last Modified:    %s\n", rec.LastModified.AsTime().Local().Format("2006-01-02 15:04:05"))
		}
		
		keyID := "(none)"
		if rec.KeyId != nil {
			keyID = *rec.KeyId
		}
		fmt.Printf("Encryption KeyID: %s\n", keyID)
		
		fmt.Printf("Ephemeral PubKey: %s\n", hex.EncodeToString(rec.EphemeralPublicKey))
		fmt.Printf("Encrypted Passwd: %s\n", hex.EncodeToString(rec.EncryptedZipPassword))
	},
}

var inspectTreeCmd = &cobra.Command{
	Use:   "tree <tree-hash>",
	Short: "Show details for a specific content-addressed TreeNode",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		treeHash := args[0]
		meta, err := loadMetadata()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		node, exists := meta.Trees[treeHash]
		if !exists {
			fmt.Fprintf(os.Stderr, "Error: TreeNode for hash '%s' not found.\n", treeHash)
			os.Exit(1)
		}

		fmt.Printf("--- TreeNode Details for '%s' ---\n", treeHash)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ENTRY NAME\tTYPE\tTARGET HASH/METADATA")
		for _, entry := range node.Entries {
			switch n := entry.Node.(type) {
			case *tsyncv2.TreeEntry_File:
				if n.File == nil {
					fmt.Fprintf(w, "%s\tFILE\t(nil)\n", entry.Name)
				} else {
					fmt.Fprintf(w, "%s\tFILE\tcrc32=0x%08x, size=%d\n", entry.Name, n.File.Crc32, n.File.UncompressedSize)
				}
			case *tsyncv2.TreeEntry_SubtreeHash:
				fmt.Fprintf(w, "%s/\tSUBTREE\thash=%s\n", entry.Name, n.SubtreeHash)
			}
		}
		w.Flush()
	},
}

func loadMetadata() (*tsyncv2.BackupMetadata, error) {
	if inspectFile != "" {
		data, err := os.ReadFile(inspectFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("metadata file '%s' does not exist", inspectFile)
			}
			return nil, fmt.Errorf("failed to read metadata file '%s': %w", inspectFile, err)
		}
		var rawMeta tsyncv2.BackupMetadata
		if err := proto.Unmarshal(data, &rawMeta); err != nil {
			return nil, fmt.Errorf("failed to parse metadata protobuf: %w", err)
		}
		return &rawMeta, nil
	}

	destStore, err := ResolveRemoteStorage(inspectRemote)
	if err != nil {
		return nil, fmt.Errorf("could not resolve storage: %w (please specify --file or initialize repository remote)", err)
	}

	ctx := context.Background()
	pbBytes, err := destStore.Read(ctx, ".tsync")
	if err != nil {
		if IsNotExistError(err) {
			return nil, fmt.Errorf("no backups found in remote store. Run 'tsync backup' first.")
		}
		return nil, fmt.Errorf("failed to read store metadata from remote: %w", err)
	}

	var rawMeta tsyncv2.BackupMetadata
	if err := proto.Unmarshal(pbBytes, &rawMeta); err != nil {
		return nil, fmt.Errorf("failed to parse remote metadata: %w", err)
	}
	return &rawMeta, nil
}

func getInspectVersionStats(metadata *tsyncv2.BackupMetadata, v *tsyncv2.Version) (int, int64, int64, error) {
	resolvedMap, err := tsync.ResolveVersionMap(metadata, v.SnowflakeId)
	if err != nil {
		return 0, 0, 0, err
	}
	
	fileCount := len(resolvedMap)
	var uncompressedSize int64
	var compressedSize int64
	
	for _, key := range resolvedMap {
		if rec, exists := metadata.Files[key]; exists {
			uncompressedSize += rec.UncompressedSize
			compressedSize += rec.CompressedSize
		}
	}
	
	return fileCount, uncompressedSize, compressedSize, nil
}

func init() {
	inspectCmd.PersistentFlags().StringVarP(&inspectFile, "file", "f", "", "Path to a local .tsync metadata file")
	inspectCmd.PersistentFlags().StringVarP(&inspectRemote, "remote", "r", "", "Name of the remote store to inspect")

	inspectVersionCmd.Flags().BoolVar(&inspectVersionRaw, "raw", false, "Show raw tree structure instead of resolved path map")

	inspectCmd.AddCommand(inspectSummaryCmd)
	inspectCmd.AddCommand(inspectVersionsCmd)
	inspectCmd.AddCommand(inspectVersionCmd)
	inspectCmd.AddCommand(inspectFilesCmd)
	inspectCmd.AddCommand(inspectFileCmd)
	inspectCmd.AddCommand(inspectTreeCmd)

	RootCmd.AddCommand(inspectCmd)
}
