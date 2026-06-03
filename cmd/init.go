package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"t-sync-cli/config"

	"github.com/spf13/cobra"
)

var (
	forceInit            bool
	initCompressionLevel int
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new T-Sync repository config for this folder",
	Long:  `Creates a new repository configuration file in ~/.tsync/ mapped to the current absolute path.`,
	Run: func(cmd *cobra.Command, args []string) {
		repoPath := GetRepoPath()
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving absolute path: %v\n", err)
			os.Exit(1)
		}

		configPath, err := config.GetConfigPath(absPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting config path: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(configPath); err == nil && !forceInit {
			fmt.Printf("Reinitialized existing T-Sync repository configuration in %s\n", configPath)
			return
		}

		// Initialize default config
		cfg := &config.Config{
			Path:          absPath,
			DefaultRemote: "origin",
			DefaultKeyID:  "default",
			PublicKeys:    make(map[string]string),
			Remotes:       make(map[string]config.RemoteConfig),
		}

		if initCompressionLevel != -2 {
			if initCompressionLevel < -1 || initCompressionLevel > 9 {
				fmt.Fprintln(os.Stderr, "Error: compression level must be between -1 and 9")
				os.Exit(1)
			}
			cfg.CompressionLevel = &initCompressionLevel
		}

		// Check if a default key exists in keys folder
		keysDir, err := config.GetKeysDir()
		if err == nil {
			defaultKeyPath := filepath.Join(keysDir, "default.key")
			if _, statErr := os.Stat(defaultKeyPath); statErr == nil {
				// We can't easily read public key from private key file without doing crypto math.
				// However, if the user ran keygen first, it would have registered it.
				// For now, if default.key exists, we register its placeholder or leave it to be loaded.
				// Actually, we can just let it exist; the user will set up their keys.
			}
		}

		err = config.SaveConfig(absPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving repository config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Initialized empty T-Sync repository in %s\n", configPath)
		fmt.Printf("To add a backup destination, run:\n")
		fmt.Printf("  tsync remote add origin local --path /path/to/backup-vault\n")
	},
}

func init() {
	initCmd.Flags().BoolVar(&forceInit, "force", false, "Force reinitialization of the config file")
	initCmd.Flags().IntVar(&initCompressionLevel, "compression-level", -2, "Compression level (0 for Store, 1-9 for Deflate)")
	RootCmd.AddCommand(initCmd)
}
