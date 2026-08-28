package cmd

import (
	"fmt"
	"os"

	"t-sync-cli/config"

	"github.com/spf13/cobra"
)

var (
	CLIVersion = "v2.0.11"
	SDKVersion = "v2.0.11"
)

// GetVersionString returns formatted CLI and SDK version.
func GetVersionString() string {
	return fmt.Sprintf("tsync CLI %s (SDK %s)", CLIVersion, SDKVersion)
}

var (
	cfgDirOverride string
	repoConfig     *config.Config
	currentDir     string
)

var RootCmd = &cobra.Command{
	Use:   "tsync",
	Short: "Git like CLI for tsync - a fast, secure, incremental, backup format",
	Long:  "Git like CLI for tsync - a fast, secure, incremental, backup format",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(GetVersionString())
		fmt.Println()
		cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print CLI and SDK version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(GetVersionString())
	},
}

func init() {
	var err error
	currentDir, err = os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
		os.Exit(1)
	}

	RootCmd.Version = GetVersionString()
	RootCmd.SetVersionTemplate("{{.Version}}\n")
	RootCmd.Flags().BoolP("version", "v", false, "Show version information for tsync")

	RootCmd.PersistentFlags().StringVar(&cfgDirOverride, "repo-path", "", "Path to the repository root directory (defaults to current directory)")

	RootCmd.AddCommand(versionCmd)
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
