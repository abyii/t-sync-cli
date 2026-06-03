package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/abyii/t-sync-sdk-go/tsync"
	tsyncv1 "github.com/abyii/t-sync-sdk-go/gen/go/com/github/abyii/tsync/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var diffRemote string

var diffCmd = &cobra.Command{
	Use:   "diff <version-1> <version-2>",
	Short: "Show differences between two backup versions",
	Long:  `List files that were added, modified, or deleted between version-1 and version-2.`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		v1ID, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid version-1 ID '%s': %v\n", args[0], err)
			os.Exit(1)
		}

		v2ID, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid version-2 ID '%s': %v\n", args[1], err)
			os.Exit(1)
		}

		destStore, err := ResolveRemoteStorage(diffRemote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		pbBytes, err := destStore.Read(ctx, ".tsync")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading store metadata: %v\n", err)
			os.Exit(1)
		}

		var metadata tsyncv1.BackupMetadata
		if err := proto.Unmarshal(pbBytes, &metadata); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing metadata: %v\n", err)
			os.Exit(1)
		}

		map1, err := tsync.ResolveVersionMap(&metadata, v1ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving version %d: %v\n", v1ID, err)
			os.Exit(1)
		}

		map2, err := tsync.ResolveVersionMap(&metadata, v2ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving version %d: %v\n", v2ID, err)
			os.Exit(1)
		}

		// Perform comparison
		added := make(map[string]string)
		modified := make(map[string][]string) // path -> [key1, key2]
		deleted := make(map[string]string)

		// Check v2 against v1 to find Added or Modified
		for path, key2 := range map2 {
			key1, exists := map1[path]
			if !exists {
				added[path] = key2
			} else if key1 != key2 {
				modified[path] = []string{key1, key2}
			}
		}

		// Check v1 against v2 to find Deleted
		for path, key1 := range map1 {
			if _, exists := map2[path]; !exists {
				deleted[path] = key1
			}
		}

		// Print diff with ANSI colors
		// Green for Added, Yellow for Modified, Red for Deleted
		hasChanges := false
		
		if len(added) > 0 {
			hasChanges = true
			fmt.Println("\033[32mAdded files:\033[0m")
			for path := range added {
				fmt.Printf("  \033[32m+ %s\033[0m\n", path)
			}
		}

		if len(modified) > 0 {
			if hasChanges {
				fmt.Println()
			}
			hasChanges = true
			fmt.Println("\033[33mModified files:\033[0m")
			for path, keys := range modified {
				fmt.Printf("  \033[33mM %s\033[0m (blob %s -> %s)\n", path, truncateKey(keys[0]), truncateKey(keys[1]))
			}
		}

		if len(deleted) > 0 {
			if hasChanges {
				fmt.Println()
			}
			hasChanges = true
			fmt.Println("\033[31mDeleted files:\033[0m")
			for path := range deleted {
				fmt.Printf("  \033[31m- %s\033[0m\n", path)
			}
		}

		if !hasChanges {
			fmt.Println("No changes found between the two versions.")
		} else {
			fmt.Printf("\nSummary: %d added, %d modified, %d deleted\n", len(added), len(modified), len(deleted))
		}
	},
}

func truncateKey(key string) string {
	if len(key) > 12 {
		return key[:12] + "..."
	}
	return key
}

func init() {
	diffCmd.Flags().StringVarP(&diffRemote, "remote", "r", "", "Remote store name to read from")
	RootCmd.AddCommand(diffCmd)
}
