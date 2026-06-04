package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/abyii/t-sync-sdk-go/v2/tsync"
	"github.com/spf13/cobra"
)

var rmRemote string

var rmCmd = &cobra.Command{
	Use:   "rm <version-id>",
	Short: "Delete a backup version from the remote store",
	Long: `Deletes the specified backup version from the remote store metadata.
Use 'tsync gc' afterwards to reclaim disk space from any orphaned file-part objects.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		versionID, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid version ID '%s': %v\n", args[0], err)
			os.Exit(1)
		}

		destStore, err := ResolveRemoteStorage(rmRemote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		client := tsync.NewClient(destStore)

		fmt.Printf("Deleting version %d from remote store...\n", versionID)
		err = client.DeleteVersion(ctx, versionID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to delete version: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✔ Successfully deleted version %d.\n", versionID)
	},
}

func init() {
	rmCmd.Flags().StringVarP(&rmRemote, "remote", "r", "", "Remote store name")
	RootCmd.AddCommand(rmCmd)
}
