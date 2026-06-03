package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"t-sync-cli/config"

	"github.com/spf13/cobra"
)

var (
	forceImportKey bool
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage local Curve25519 keys and map them to repositories",
	Long:  `List, show, register, import, and delete Curve25519 keypairs used for backup encryption and decryption.`,
	Run: func(cmd *cobra.Command, args []string) {
		runKeysList()
	},
}

var keysShowCmd = &cobra.Command{
	Use:   "show <key-id>",
	Short: "Show details for a specific key",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		keysDir, err := config.GetKeysDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		keyPath := filepath.Join(keysDir, keyID+".key")
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Key '%s' not found.\n", keyID)
			os.Exit(1)
		}

		privBytes, err := config.LoadPrivateKey(keyID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading private key: %v\n", err)
			os.Exit(1)
		}

		pubBytes, err := config.DerivePublicKey(privBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deriving public key: %v\n", err)
			os.Exit(1)
		}

		pubHex := hex.EncodeToString(pubBytes)

		fmt.Printf("Key ID:          %s\n", keyID)
		fmt.Printf("Private Key Path: %s\n", keyPath)
		fmt.Printf("Public Key Hex:   %s\n", pubHex)

		repoPath := GetRepoPath()
		if cfg, err := config.LoadConfig(repoPath); err == nil {
			status := "Not registered in current repo config"
			if cfg.DefaultKeyID == keyID {
				status = "Registered as DEFAULT key in current repo config"
			} else if registeredPub, exists := cfg.PublicKeys[keyID]; exists {
				if registeredPub == pubHex {
					status = "Registered in current repo config"
				} else {
					status = "Registered in current repo config but with a DIFFERENT public key mismatch!"
				}
			}
			fmt.Printf("Repo Status:      %s\n", status)
		}
	},
}

var keysRegisterCmd = &cobra.Command{
	Use:   "register <key-id>",
	Short: "Register an existing key in the current repository's configuration",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		repoPath := GetRepoPath()
		cfg, err := config.LoadConfig(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Current directory is not an initialized T-Sync repository: %v\n", err)
			os.Exit(1)
		}

		privBytes, err := config.LoadPrivateKey(keyID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not load key '%s': %v\n", keyID, err)
			os.Exit(1)
		}

		pubBytes, err := config.DerivePublicKey(privBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deriving public key: %v\n", err)
			os.Exit(1)
		}

		pubHex := hex.EncodeToString(pubBytes)
		cfg.PublicKeys[keyID] = pubHex
		if cfg.DefaultKeyID == "" || cfg.DefaultKeyID == "default" {
			cfg.DefaultKeyID = keyID
		}

		err = config.SaveConfig(repoPath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving repository config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✔ Successfully registered key '%s' (Public Key: %s) in repo config\n", keyID, pubHex)
	},
}

var keysImportCmd = &cobra.Command{
	Use:   "import <key-id> <private-key-hex>",
	Short: "Import a private key from hex string",
	Long: `Import an existing Curve25519 private key from its 32-byte hex-encoded representation.
Saves the key as ~/.tsync/keys/<key-id>.key and automatically registers the derived public key 
in the current repository configuration (if initialized).`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		privHex := strings.TrimSpace(args[1])

		privBytes, err := hex.DecodeString(privHex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid hex-encoded private key: %v\n", err)
			os.Exit(1)
		}

		if len(privBytes) != 32 {
			fmt.Fprintf(os.Stderr, "Error: Invalid private key length. Expected 32 bytes (64 hex characters), got %d bytes\n", len(privBytes))
			os.Exit(1)
		}

		keysDir, err := config.GetKeysDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		keyPath := filepath.Join(keysDir, keyID+".key")
		if _, err := os.Stat(keyPath); err == nil && !forceImportKey {
			fmt.Fprintf(os.Stderr, "Error: Key file already exists at %s. Use --force to overwrite.\n", keyPath)
			os.Exit(1)
		}

		err = config.SavePrivateKey(keyID, privBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving private key: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✔ Saved private key to: %s\n", keyPath)

		pubBytes, err := config.DerivePublicKey(privBytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deriving public key: %v\n", err)
			os.Exit(1)
		}

		pubHex := hex.EncodeToString(pubBytes)
		fmt.Printf("Derived Public Key (Hex): %s\n", pubHex)

		repoPath := GetRepoPath()
		cfg, err := config.LoadConfig(repoPath)
		if err == nil {
			cfg.PublicKeys[keyID] = pubHex
			if cfg.DefaultKeyID == "" || cfg.DefaultKeyID == "default" {
				cfg.DefaultKeyID = keyID
			}
			err = config.SaveConfig(repoPath, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to register public key in repo config: %v\n", err)
			} else {
				fmt.Printf("✔ Automatically registered public key in repository config for %s\n", repoPath)
			}
		}
	},
}

var keysDeleteCmd = &cobra.Command{
	Use:   "delete <key-id>",
	Short: "Delete a local private key file and unregister it",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyID := args[0]
		keysDir, err := config.GetKeysDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		keyPath := filepath.Join(keysDir, keyID+".key")
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Key file '%s' not found.\n", keyPath)
			os.Exit(1)
		}

		err = os.Remove(keyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting key file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✔ Deleted private key file: %s\n", keyPath)

		repoPath := GetRepoPath()
		if cfg, err := config.LoadConfig(repoPath); err == nil {
			updated := false
			if _, exists := cfg.PublicKeys[keyID]; exists {
				delete(cfg.PublicKeys, keyID)
				updated = true
			}
			if cfg.DefaultKeyID == keyID {
				cfg.DefaultKeyID = ""
				for k := range cfg.PublicKeys {
					cfg.DefaultKeyID = k
					break
				}
				updated = true
			}

			if updated {
				err = config.SaveConfig(repoPath, cfg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to update repository config: %v\n", err)
				} else {
					fmt.Println("✔ Unregistered key from repository configuration")
				}
			}
		}
	},
}

func runKeysList() {
	keysDir, err := config.GetKeysDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	files, err := os.ReadDir(keysDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading keys directory: %v\n", err)
		os.Exit(1)
	}

	var keyFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".key") {
			keyFiles = append(keyFiles, f.Name())
		}
	}

	if len(keyFiles) == 0 {
		fmt.Println("No keys found in ~/.tsync/keys/")
		return
	}

	repoPath := GetRepoPath()
	var repoConfig *config.Config
	if repoPath != "" {
		repoConfig, _ = config.LoadConfig(repoPath)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KEY ID\tPUBLIC KEY (HEX)\tSTATUS")

	for _, name := range keyFiles {
		keyID := strings.TrimSuffix(name, ".key")
		
		privBytes, err := config.LoadPrivateKey(keyID)
		if err != nil {
			fmt.Fprintf(w, "%s\t<error loading key>\t-\n", keyID)
			continue
		}

		pubBytes, err := config.DerivePublicKey(privBytes)
		if err != nil {
			fmt.Fprintf(w, "%s\t<error deriving pubkey>\t-\n", keyID)
			continue
		}

		pubHex := hex.EncodeToString(pubBytes)
		dispPub := pubHex
		if len(pubHex) > 16 {
			dispPub = pubHex[:16] + "..."
		}

		status := "Not registered"
		if repoConfig != nil {
			if repoConfig.DefaultKeyID == keyID {
				status = "Registered (default)"
			} else if _, registered := repoConfig.PublicKeys[keyID]; registered {
				status = "Registered"
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", keyID, dispPub, status)
	}
	w.Flush()
}

func init() {
	keysListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all local keys in ~/.tsync/keys/",
		Run: func(cmd *cobra.Command, args []string) {
			runKeysList()
		},
	}

	keysImportCmd.Flags().BoolVar(&forceImportKey, "force", false, "Overwrite existing key file")

	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysShowCmd)
	keysCmd.AddCommand(keysRegisterCmd)
	keysCmd.AddCommand(keysImportCmd)
	keysCmd.AddCommand(keysDeleteCmd)

	RootCmd.AddCommand(keysCmd)
}
