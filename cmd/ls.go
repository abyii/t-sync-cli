package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"t-sync-cli/tui"

	"github.com/abyii/t-sync-sdk-go/v2/tsync"
	tsyncv2 "github.com/abyii/t-sync-sdk-go/v2/gen/go/com/github/abyii/tsync/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var (
	lsRemote      string
	lsInteractive bool
)

var lsCmd = &cobra.Command{
	Use:   "ls <version-id>",
	Short: "List files in a backup version",
	Long: `List all files present in the specified backup version. 
Use --interactive to open a beautiful terminal-based interface to browse, 
search, and view detailed metadata for each file.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		versionID, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid version ID '%s': %v\n", args[0], err)
			os.Exit(1)
		}

		destStore, err := ResolveRemoteStorage(lsRemote)
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

		resolvedMap, err := tsync.ResolveVersionMap(&metadata, versionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving version %d: %v\n", versionID, err)
			os.Exit(1)
		}

		// Convert to UI items list
		var uiItems []tui.FileItem
		for path, fileKey := range resolvedMap {
			rec, exists := metadata.Files[fileKey]
			size := int64(0)
			compSize := int64(0)
			crc := uint32(0)
			modified := ""
			keyID := ""
			if exists {
				size = rec.UncompressedSize
				compSize = rec.CompressedSize
				crc = rec.Crc32
				modified = rec.LastModified.AsTime().Local().Format("2006-01-02 15:04:05")
				if rec.KeyId != nil {
					keyID = *rec.KeyId
				}
			}
			uiItems = append(uiItems, tui.FileItem{
				Path:     path,
				Size:     size,
				CompSize: compSize,
				CRC32:    crc,
				Modified: modified,
				Key:      fileKey,
				KeyID:    keyID,
			})
		}

		if lsInteractive {
			// Run TUI
			m := tui.NewBrowserModel(versionID, uiItems)
			p := tea.NewProgram(m)
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI browser: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Normal table output
			if len(uiItems) == 0 {
				fmt.Println("No files in this version.")
				return
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "FILE PATH\tSIZE\tCRC32\tMODIFIED")
			for _, item := range uiItems {
				fmt.Fprintf(w, "%s\t%s\t0x%08x\t%s\n", item.Path, FormatBytes(item.Size), item.CRC32, item.Modified)
			}
			w.Flush()
		}
	},
}

func init() {
	lsCmd.Flags().StringVarP(&lsRemote, "remote", "r", "", "Remote store name to read from")
	lsCmd.Flags().BoolVarP(&lsInteractive, "interactive", "i", false, "Browse files in a beautiful interactive TUI")
	RootCmd.AddCommand(lsCmd)
}
