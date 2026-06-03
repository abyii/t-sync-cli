package cmd

import (
	"context"
	"fmt"
	"os"

	"t-sync-cli/config"

	"github.com/abyii/t-sync-sdk-go/tsync"
	"github.com/spf13/cobra"
)

var (
	backupRemote           string
	backupLabel            string
	backupConcurrency      int
	backupKeyID            string
	singleVersionMode      bool
	backupCompressionLevel int
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a new backup version of the current repository",
	Long: `Backup files in the repository directory to the specified remote store.
Only modifications relative to the latest FULL version are backed up (emitted as a DELTA),
unless a FULL version is preferred based on the SDK's heuristics or --single-version is set.`,
	Run: func(cmd *cobra.Command, args []string) {
		repoPath := GetRepoPath()
		cfg := LoadRepoConfig()

		// 1. Resolve remote storage
		destStore, err := ResolveRemoteStorage(backupRemote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// 2. Set up local folder source
		localStore, err := tsync.NewLocalStorage(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing local source store: %v\n", err)
			os.Exit(1)
		}
		srcFolder := tsync.NewFolderSource(localStore, "")

		// 3. Set up ignore filter
		ignoreMatcher := config.NewIgnoreMatcher(repoPath)
		filterFunc := func(path string) (bool, error) {
			return !ignoreMatcher.Matches(path), nil
		}

		// 4. Load public keys from config
		if len(cfg.PublicKeys) == 0 {
			fmt.Fprintln(os.Stderr, "Error: No public keys registered in repository configuration. Run 'tsync keygen' first.")
			os.Exit(1)
		}

		publicKeysBytes := make(map[string][]byte)
		for id, hexStr := range cfg.PublicKeys {
			b, err := HexDecodeKey(hexStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Public key '%s' has invalid hex encoding: %v\n", id, err)
				os.Exit(1)
			}
			publicKeysBytes[id] = b
		}

		// Determine key ID to use
		keyID := backupKeyID
		if keyID == "" {
			// Check env fallback
			keyID = os.Getenv("TSYNC_DEFAULT_KEY_ID")
			if keyID == "" {
				keyID = cfg.DefaultKeyID
			}
		}

		if keyID == "" {
			// Use first key in map
			for id := range publicKeysBytes {
				keyID = id
				break
			}
		}

		if _, ok := publicKeysBytes[keyID]; !ok {
			fmt.Fprintf(os.Stderr, "Error: Selected encryption key ID '%s' not found in config public_keys.\n", keyID)
			os.Exit(1)
		}

		// 5. Initialize T-Sync Client and options
		client := tsync.NewClient(destStore)

		concurrency := backupConcurrency
		if concurrency <= 0 {
			// default: 4 for local destination, 8 for remote object storage
			concurrency = 4
			targetRemote := backupRemote
			if targetRemote == "" {
				targetRemote = cfg.DefaultRemote
			}
			if rc, ok := cfg.Remotes[targetRemote]; ok && rc.Provider != "local" {
				concurrency = 8
			}
		}

		var compLevel *int
		if backupCompressionLevel != -2 {
			if backupCompressionLevel < -1 || backupCompressionLevel > 9 {
				fmt.Fprintln(os.Stderr, "Error: compression level must be between -1 and 9")
				os.Exit(1)
			}
			compLevel = &backupCompressionLevel
		} else {
			compLevel = cfg.CompressionLevel
		}

		ctx := context.Background()
		opts := tsync.BackupOptions{
			Label:             backupLabel,
			SingleVersionMode: singleVersionMode,
			FilterFunc:        filterFunc,
			Concurrency:       concurrency,
			KeyID:             keyID,
			PublicKeys:        publicKeysBytes,
			CompressionLevel:  compLevel,
			OnProgress:        MakeProgressCallback("BACKUP"),
		}

		fmt.Printf("Starting backup to remote storage... (concurrency=%d, key_id=%s)\n", concurrency, keyID)
		version, err := client.Backup(ctx, srcFolder, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: Backup failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n✔ Backup completed successfully!")
		fmt.Printf("Version ID:   %d\n", version.SnowflakeId)
		fmt.Printf("Label:        %s\n", version.Label)
		fmt.Printf("Kind:         %v\n", version.Kind)
		fmt.Printf("Timestamp:    %s\n", version.BackupTimestamp.AsTime().Local().Format("2006-01-02 15:04:05"))
	},
}

func init() {
	backupCmd.Flags().StringVarP(&backupLabel, "message", "m", "", "Label/message for the backup version")
	backupCmd.Flags().StringVarP(&backupRemote, "remote", "r", "", "Remote store name to backup to (defaults to config default)")
	backupCmd.Flags().IntVar(&backupConcurrency, "concurrency", 0, "Number of concurrent file uploads")
	backupCmd.Flags().StringVar(&backupKeyID, "key-id", "", "Public key ID to encrypt the zip password (defaults to config default)")
	backupCmd.Flags().BoolVar(&singleVersionMode, "single-version", false, "Discard history and replace remote store with this single version")
	backupCmd.Flags().IntVar(&backupCompressionLevel, "compression-level", -2, "Compression level (0 for Store, 1-9 for Deflate)")

	RootCmd.AddCommand(backupCmd)
}
