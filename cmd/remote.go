package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"t-sync-cli/config"

	"github.com/spf13/cobra"
)

var (
	remotePath      string
	remoteBucket    string
	remotePrefix    string
	remoteRegion    string
	remoteEndpoint  string
	remoteNamespace string
	remoteAuthType  string
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote backup store mapping",
	Long:  `List, add, or remove remote stores mapped to this repository.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default action is to list remotes
		runRemoteList()
	},
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <name> <provider>",
	Short: "Add a new remote backup store mapping",
	Long: `Add a remote store mapping.

Providers and Examples:

1. local: Local filesystem directory.
   Required flags: --path
   Example:
     tsync remote add backup local --path /var/backups/my-repo

2. s3: AWS S3 or compatible bucket.
   Required flags: --bucket
   Optional flags: --prefix, --region, --endpoint, --auth-type
   Example (using static access keys):
     tsync remote add origin s3 --bucket my-bucket --prefix backup/ --region us-west-2 --auth-type "S3_ACCESS_KEYS[MYACCESSKEY:MYSECRETKEY]"
   Example (using environment fallback credentials):
     tsync remote add origin s3 --bucket my-bucket --region us-east-1

3. oci: Oracle Cloud Infrastructure Object Storage.
   Required flags: --bucket, --namespace
   Optional flags: --prefix, --auth-type
   Example (using user profile from ~/.oci/config):
     tsync remote add origin oci --bucket my-bucket --namespace my-tenancy-ns --prefix project/ --auth-type "OCI_CONFIG_FILE[DEFAULT]"
   Example (using instance or resource principals):
     tsync remote add origin oci --bucket my-bucket --namespace my-tenancy-ns --auth-type "RESOURCE_PRINCIPAL"

4. http: Generic HTTP/CDN endpoint or S3-compatible REST API.
   Required flags: --endpoint
   Optional flags: --prefix, --auth-type
   Example (with authorization headers):
     tsync remote add cdn http --endpoint https://cdn.example.com --prefix vault/ --auth-type "HEADER[Authorization: Bearer my-api-token, X-Custom: val]"`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		provider := args[1]

		cfg := LoadRepoConfig()

		if _, exists := cfg.Remotes[name]; exists {
			fmt.Fprintf(os.Stderr, "Error: Remote '%s' already exists.\n", name)
			os.Exit(1)
		}

		remote := config.RemoteConfig{
			Name:     name,
			Provider: provider,
		}

		switch provider {
		case "local":
			if remotePath == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required for local provider")
				os.Exit(1)
			}
			remote.Path = remotePath

		case "s3":
			if remoteBucket == "" {
				fmt.Fprintln(os.Stderr, "Error: --bucket is required for s3 provider")
				os.Exit(1)
			}
			remote.Bucket = remoteBucket
			remote.Prefix = remotePrefix
			remote.Region = remoteRegion
			remote.Endpoint = remoteEndpoint
			remote.AuthType = remoteAuthType

		case "oci":
			if remoteBucket == "" || remoteNamespace == "" {
				fmt.Fprintln(os.Stderr, "Error: --bucket and --namespace are required for oci provider")
				os.Exit(1)
			}
			remote.Bucket = remoteBucket
			remote.Prefix = remotePrefix
			remote.Namespace = remoteNamespace
			remote.AuthType = remoteAuthType

		case "http":
			if remoteEndpoint == "" {
				fmt.Fprintln(os.Stderr, "Error: --endpoint is required for http provider")
				os.Exit(1)
			}
			remote.Endpoint = remoteEndpoint
			remote.Prefix = remotePrefix
			remote.AuthType = remoteAuthType

		default:
			fmt.Fprintf(os.Stderr, "Error: Unsupported provider '%s'\n", provider)
			os.Exit(1)
		}

		cfg.Remotes[name] = remote
		if cfg.DefaultRemote == "" {
			cfg.DefaultRemote = name
		}

		err := config.SaveConfig(GetRepoPath(), cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving remote: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✔ Added remote '%s' (%s)\n", name, provider)
	},
}

var remoteRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a remote backup store mapping",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := LoadRepoConfig()

		if _, exists := cfg.Remotes[name]; !exists {
			fmt.Fprintf(os.Stderr, "Error: Remote '%s' not found.\n", name)
			os.Exit(1)
		}

		delete(cfg.Remotes, name)
		if cfg.DefaultRemote == name {
			// Select another remote as default if available
			cfg.DefaultRemote = ""
			for rName := range cfg.Remotes {
				cfg.DefaultRemote = rName
				break
			}
		}

		err := config.SaveConfig(GetRepoPath(), cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✔ Removed remote '%s'\n", name)
	},
}

var remoteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all mapped remotes",
	Run: func(cmd *cobra.Command, args []string) {
		runRemoteList()
	},
}

func runRemoteList() {
	cfg := LoadRepoConfig()
	if len(cfg.Remotes) == 0 {
		fmt.Println("No remotes configured.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROVIDER\tLOCATION")
	for name, r := range cfg.Remotes {
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
		
		suffix := ""
		if name == cfg.DefaultRemote {
			suffix = " (default)"
		}
		fmt.Fprintf(w, "%s%s\t%s\t%s\n", name, suffix, r.Provider, loc)
	}
	w.Flush()
}

func init() {
	remoteAddCmd.Flags().StringVar(&remotePath, "path", "", "Local filesystem directory path (for 'local')")
	remoteAddCmd.Flags().StringVar(&remoteBucket, "bucket", "", "Object storage bucket name (for 's3', 'oci')")
	remoteAddCmd.Flags().StringVar(&remotePrefix, "prefix", "", "Path prefix inside the bucket (for 's3', 'oci', 'http')")
	remoteAddCmd.Flags().StringVar(&remoteRegion, "region", "", "AWS region (for 's3')")
	remoteAddCmd.Flags().StringVar(&remoteEndpoint, "endpoint", "", "Custom endpoint URL (for 's3', 'http')")
	remoteAddCmd.Flags().StringVar(&remoteNamespace, "namespace", "", "OCI Object Storage Namespace (for 'oci')")
	remoteAddCmd.Flags().StringVar(&remoteAuthType, "auth-type", "", "Authentication helper parameter")

	remoteCmd.AddCommand(remoteAddCmd)
	remoteCmd.AddCommand(remoteRemoveCmd)
	remoteCmd.AddCommand(remoteListCmd)
	RootCmd.AddCommand(remoteCmd)
}
