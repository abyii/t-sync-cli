package cmd

import (
	"fmt"
	"os"

	"t-sync-cli/config"

	"github.com/spf13/cobra"
)

var (
	cfgDirOverride string
	repoConfig     *config.Config
	currentDir     string
)

var RootCmd = &cobra.Command{
	Use:   "tsync",
	Short: "T-Sync: An interactive, secure, incremental backup CLI",
	Long: `T-Sync is a secure, incremental, and highly optimized backup format 
that shards files into compressed and encrypted parts. This CLI provides 
Git-like ergonomics for backing up and restoring directories.`,
}

func init() {
	var err error
	currentDir, err = os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
		os.Exit(1)
	}

	RootCmd.PersistentFlags().StringVar(&cfgDirOverride, "repo-path", "", "Path to the repository root directory (defaults to current directory)")
}

// GetRepoPath returns the directory being operated on (either current or overridden)
func GetRepoPath() string {
	if cfgDirOverride != "" {
		return cfgDirOverride
	}
	return currentDir
}

// LoadRepoConfig attempts to load the config for the current repository path.
// Exits the program with an error if loading fails.
func LoadRepoConfig() *config.Config {
	if repoConfig != nil {
		return repoConfig
	}

	path := GetRepoPath()
	cfg, err := config.LoadConfig(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	repoConfig = cfg
	return repoConfig
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
