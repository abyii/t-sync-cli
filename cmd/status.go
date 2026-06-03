package cmd

import (
	"fmt"
	"os"

	"t-sync-cli/config"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current repository status",
	Long:  `Show the mapping configuration, default key ID, and list of remotes for the current folder.`,
	Run: func(cmd *cobra.Command, args []string) {
		repoPath := GetRepoPath()
		cfgPath, err := config.GetConfigPath(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := config.LoadConfig(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("On branch/directory: %s\n", cfg.Path)
		fmt.Printf("Config file:         %s\n\n", cfgPath)

		if cfg.DefaultRemote != "" {
			fmt.Printf("Default Remote: %s\n", cfg.DefaultRemote)
		} else {
			fmt.Println("Default Remote: (none configured)")
		}

		if cfg.DefaultKeyID != "" {
			fmt.Printf("Default Key ID: %s\n", cfg.DefaultKeyID)
		} else {
			fmt.Println("Default Key ID: (none configured)")
		}

		if cfg.CompressionLevel != nil {
			fmt.Printf("Compression Level:   %d\n", *cfg.CompressionLevel)
		} else {
			fmt.Println("Compression Level:   (default/not set)")
		}

		fmt.Println("\nPublic Keys:")
		if len(cfg.PublicKeys) == 0 {
			fmt.Println("  (none registered)")
		} else {
			for id, pub := range cfg.PublicKeys {
				prefix := " "
				if id == cfg.DefaultKeyID {
					prefix = "*" // mark default key
				}
				// truncate public key for cleaner view
				dispPub := pub
				if len(pub) > 16 {
					dispPub = pub[:16] + "..."
				}
				fmt.Printf("  %s %s: %s\n", prefix, id, dispPub)
			}
		}

		fmt.Println("\nRemotes:")
		if len(cfg.Remotes) == 0 {
			fmt.Println("  (none configured)")
		} else {
			for rName, r := range cfg.Remotes {
				prefix := " "
				if rName == cfg.DefaultRemote {
					prefix = "*"
				}
				
				loc := ""
				switch r.Provider {
				case "local":
					loc = r.Path
				case "s3":
					loc = fmt.Sprintf("s3://%s/%s", r.Bucket, r.Prefix)
				case "oci":
					loc = fmt.Sprintf("oci://%s/%s/%s", r.Namespace, r.Bucket, r.Prefix)
				case "http":
					loc = fmt.Sprintf("http://%s/%s", r.Endpoint, r.Prefix)
				}
				fmt.Printf("  %s %s [%s]: %s\n", prefix, rName, r.Provider, loc)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
