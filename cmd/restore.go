package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"t-sync-cli/config"
	"t-sync-cli/tui"

	tsyncv2 "github.com/abyii/t-sync-sdk-go/v2/gen/go/com/github/abyii/tsync/v2"
	"github.com/abyii/t-sync-sdk-go/v2/tsync"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var (
	restoreRemote      string
	restoreDir         string
	restoreConcurrency int
	restorePrivateKey  string
	noOverwrite        bool
	restoreZip         string
	restoreInteractive bool
)

var restoreCmd = &cobra.Command{
	Use:   "restore <version-id>",
	Short: "Restore files from a backup version",
	Long: `Restores all or selected files from a specific backup version.
By default, extracts files into the specified directory (defaults to current dir).
If --zip is specified, reconstructs the files into a single ZIP archive on-the-fly.
Use --interactive to selectively check files from a checklist TUI.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		startTime := time.Now()
		versionID, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid version ID '%s': %v\n", args[0], err)
			os.Exit(1)
		}

		destStore, err := ResolveRemoteStorage(restoreRemote)
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

		// 1. Resolve decryption private key automatically
		var privKey []byte
		if restorePrivateKey != "" {
			// Read from user-specified file
			data, fileErr := os.ReadFile(restorePrivateKey)
			if fileErr != nil {
				fmt.Fprintf(os.Stderr, "Error reading private key file: %v\n", fileErr)
				os.Exit(1)
			}
			privKey, err = HexDecodeKey(string(data))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Private key file has invalid hex: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Auto-detect key ID from the version records
			keyID := ""
			for _, fileKey := range resolvedMap {
				if rec, ok := metadata.Files[fileKey]; ok && rec.KeyId != nil && *rec.KeyId != "" {
					keyID = *rec.KeyId
					break
				}
			}

			if keyID != "" {
				privKey, err = config.LoadPrivateKey(keyID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to automatically load private key for ID '%s': %v\n", keyID, err)
					fmt.Println("Please provide the private key path manually using --private-key")
					os.Exit(1)
				}
				fmt.Printf("✔ Auto-loaded private key for key ID '%s'\n", keyID)
			}
		}

		// 2. Interactive selective restore
		var filesToRestore []string
		if restoreInteractive {
			var uiItems []tui.FileItem
			for path, fileKey := range resolvedMap {
				rec, exists := metadata.Files[fileKey]
				size := int64(0)
				if exists {
					size = rec.UncompressedSize
				}
				uiItems = append(uiItems, tui.FileItem{
					Path: path,
					Size: size,
				})
			}

			m := tui.NewSelectorModel(versionID, uiItems)
			p := tea.NewProgram(m)
			resModel, err := p.Run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI selector: %v\n", err)
				os.Exit(1)
			}

			finalModel := resModel.(tui.SelectorModel)
			if finalModel.Quitting || !finalModel.Submitted {
				fmt.Println("Restoration cancelled.")
				return
			}

			filesToRestore = finalModel.GetSelectedPaths()
			if len(filesToRestore) == 0 {
				fmt.Println("No files selected for restoration.")
				return
			}
			fmt.Printf("Selected %d files to restore.\n", len(filesToRestore))
		}

		// 3. Set up restore options
		client := tsync.NewClient(destStore)
		opts := tsync.RestoreOptions{
			PrivateKey:           privKey,
			Concurrency:          restoreConcurrency,
			NoOverwrite:          noOverwrite,
			FilesToRestore:       filesToRestore,
			OnProgress:           MakeProgressCallback("RESTORE"),
			SkipDecryptionErrors: true,
		}

		if restoreZip == "__DEFAULT__" {
			restoreZip = fmt.Sprintf("restore_%d.zip", versionID)
		}

		if restoreZip != "" {
			// Reconstruct ZIP on-the-fly
			zipFile, err := os.Create(restoreZip)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating output zip file: %v\n", err)
				os.Exit(1)
			}
			defer zipFile.Close()

			opts.ZipWriter = zipFile
			fmt.Printf("Reconstructing backup version to ZIP archive: %s...\n", restoreZip)
		} else {
			// Extract to directory
			outDir := restoreDir
			if outDir == "" {
				outDir = GetRepoPath()
			}
			absOutDir, err := filepath.Abs(outDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving output directory path: %v\n", err)
				os.Exit(1)
			}

			opts.ExtractDir = absOutDir
			fmt.Printf("Extracting backup files to: %s...\n", absOutDir)
		}

		// Run restore
		err = client.Restore(ctx, versionID, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: Restoration failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✔ Restoration completed successfully! (Duration: %v)\n", time.Since(startTime).Round(time.Millisecond))
	},
}

func init() {
	restoreCmd.Flags().StringVarP(&restoreRemote, "remote", "r", "", "Remote store name to read from")
	restoreCmd.Flags().StringVarP(&restoreDir, "dir", "d", "", "Directory to extract files into (defaults to repository directory)")
	restoreCmd.Flags().IntVar(&restoreConcurrency, "concurrency", 10, "Extraction concurrency level")
	restoreCmd.Flags().StringVar(&restorePrivateKey, "private-key", "", "Path to the Curve25519 hex private key file")
	restoreCmd.Flags().BoolVar(&noOverwrite, "no-overwrite", false, "Skip overwriting existing identical files")
	restoreCmd.Flags().StringVar(&restoreZip, "zip", "", "Reconstruct and write directly to a local ZIP file path")
	restoreCmd.Flags().Lookup("zip").NoOptDefVal = "__DEFAULT__"
	restoreCmd.Flags().BoolVarP(&restoreInteractive, "interactive", "i", false, "Interactively check files to restore from checklist TUI")

	RootCmd.AddCommand(restoreCmd)
}
