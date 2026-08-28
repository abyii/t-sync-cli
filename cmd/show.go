package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	tsyncv2 "github.com/abyii/t-sync-sdk-go/v2/gen/go/com/github/abyii/tsync/v2"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var showRemote string

var showCmd = &cobra.Command{
	Use:   "show <version-id> <file-path>",
	Short: "Show details for a specific file in a backup version",
	Long:  `Retrieve and print metadata and encryption records for a specific file path within a backup version.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		versionID, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid version ID '%s': %v\n", args[0], err)
			os.Exit(1)
		}

		filePath := args[1]

		destStore, err := ResolveRemoteStorage(showRemote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		pbBytes, err := destStore.Read(ctx, ".tsync")
		if err != nil {
			if IsNotExistError(err) {
				fmt.Fprintln(os.Stderr, "No backups found in remote store. Run 'tsync backup' first.")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Error reading store metadata: %v\n", err)
			os.Exit(1)
		}

		var metadata tsyncv2.BackupMetadata
		if err := proto.Unmarshal(pbBytes, &metadata); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing metadata: %v\n", err)
			os.Exit(1)
		}

		resolvedMap, err := ResolveVersionMap(&metadata, versionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving version %d: %v\n", versionID, err)
			os.Exit(1)
		}

		fileKey, ok := resolvedMap[filePath]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: File '%s' not found in version %d.\n", filePath, versionID)
			os.Exit(1)
		}

		record, ok := metadata.Files[fileKey]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: File record for key '%s' not found in metadata.\n", fileKey)
			os.Exit(1)
		}

		// Print detailed file record info
		fmt.Printf("File Path:       %s\n", filePath)
		fmt.Printf("File Key (Blob): %s\n", fileKey)
		fmt.Printf("CRC32:           0x%08x\n", record.Crc32)
		fmt.Printf("Size (Raw):      %s (%d bytes)\n", FormatBytes(record.UncompressedSize), record.UncompressedSize)
		fmt.Printf("Size (Comp):     %s (%d bytes)\n", FormatBytes(record.CompressedSize), record.CompressedSize)
		
		ratio := 0.0
		if record.UncompressedSize > 0 {
			ratio = 100.0 - (float64(record.CompressedSize) / float64(record.UncompressedSize) * 100.0)
		}
		fmt.Printf("Savings:         %.2f%%\n", ratio)
		fmt.Printf("Last Modified:   %s\n", record.LastModified.AsTime().Local().Format("2006-01-02 15:04:05"))

		keyID := "(none)"
		if record.KeyId != nil {
			keyID = *record.KeyId
		}
		fmt.Printf("Encryption Key:  %s\n", keyID)
		
		ephKeyDisp := hex.EncodeToString(record.EphemeralPublicKey)
		if len(ephKeyDisp) > 30 {
			ephKeyDisp = ephKeyDisp[:30] + "..."
		}
		fmt.Printf("Ephemeral Pub:   %s\n", ephKeyDisp)
		
		encPassDisp := hex.EncodeToString(record.EncryptedZipPassword)
		if len(encPassDisp) > 30 {
			encPassDisp = encPassDisp[:30] + "..."
		}
		fmt.Printf("Encrypted Pass:  %s\n", encPassDisp)
	},
}

func init() {
	showCmd.Flags().StringVarP(&showRemote, "remote", "r", "", "Remote store name to read from")
	RootCmd.AddCommand(showCmd)
}
