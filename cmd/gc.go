package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/abyii/t-sync-sdk-go/v2/tsync"
	"github.com/spf13/cobra"
)

var gcRemote string

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Run garbage collection on remote storage",
	Long:  `Scan the remote storage and remove any orphaned sharded file-part objects that are no longer referenced by any backup version in the metadata.`,
	Run: func(cmd *cobra.Command, args []string) {
		destStore, err := ResolveRemoteStorage(gcRemote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		client := tsync.NewClient(destStore)

		fmt.Println("Scanning remote storage for orphaned files...")
		err = client.GC(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Garbage collection failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✔ Garbage collection completed successfully. All orphaned file parts have been removed.")
	},
}

func init() {
	gcCmd.Flags().StringVarP(&gcRemote, "remote", "r", "", "Remote store name to run GC on")
	RootCmd.AddCommand(gcCmd)
}
