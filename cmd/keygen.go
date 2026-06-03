package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"t-sync-cli/config"

	"github.com/spf13/cobra"
)

var (
	keyIDFlag string
	forceKey  bool
)

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a new Curve25519 keypair for backup encryption",
	Long: `Generate a new 32-byte Curve25519 keypair. 
The private key is saved securely at ~/.tsync/keys/<key_id>.key,
and the public key is printed and optionally registered in the local repo config.`,
	Run: func(cmd *cobra.Command, args []string) {
		if keyIDFlag == "" {
			fmt.Println("Error: --key-id is required")
			os.Exit(1)
		}

		keysDir, err := config.GetKeysDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		keyPath := filepath.Join(keysDir, keyIDFlag+".key")
		if _, err := os.Stat(keyPath); err == nil && !forceKey {
			fmt.Fprintf(os.Stderr, "Error: Key file already exists at %s. Use --force to overwrite.\n", keyPath)
			os.Exit(1)
		}

		pubKeyHex, err := config.GenerateAndSaveKeypair(keyIDFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating keys: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✔ Generated new keypair with ID: %s\n", keyIDFlag)
		fmt.Printf("✔ Private key saved to: %s\n", keyPath)
		fmt.Printf("Public Key (Hex): %s\n", pubKeyHex)

		// Try to register in local repo config if we are in one
		repoPath := GetRepoPath()
		cfg, err := config.LoadConfig(repoPath)
		if err == nil {
			cfg.PublicKeys[keyIDFlag] = pubKeyHex
			if cfg.DefaultKeyID == "" {
				cfg.DefaultKeyID = keyIDFlag
			}
			err = config.SaveConfig(repoPath, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to register public key in repo config: %v\n", err)
			} else {
				fmt.Printf("✔ Automatically registered public key in repository config for %s\n", repoPath)
			}
		} else {
			fmt.Println("\nTo use this key for encryption, register the public key in your repository config:")
			fmt.Printf("  tsync init (if not initialized yet)\n")
			fmt.Printf("  Or add to config file: ~/.tsync/<mangled_path>.yaml\n")
		}
	},
}

func init() {
	keygenCmd.Flags().StringVar(&keyIDFlag, "key-id", "default", "ID to name the keypair (e.g. prod-key)")
	keygenCmd.Flags().BoolVar(&forceKey, "force", false, "Overwrite the key file if it already exists")
	RootCmd.AddCommand(keygenCmd)
}
