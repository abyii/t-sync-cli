package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/abyii/t-sync-sdk-go/v2/tsync"
	tsyncv2 "github.com/abyii/t-sync-sdk-go/v2/gen/go/com/github/abyii/tsync/v2"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var logRemote string

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View backup history logs",
	Long:  `Retrieve and print historical backup versions from the remote metadata.`,
	Run: func(cmd *cobra.Command, args []string) {
		destStore, err := ResolveRemoteStorage(logRemote)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		client := tsync.NewClient(destStore)

		sm, err := client.ReadMetadata(ctx)
		if err != nil {
			// If .tsync file doesn't exist, it means remote is empty/new
			if IsNotExistError(err) {
				fmt.Println("No backups found in remote store. Run 'tsync backup' first.")
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading store metadata: %v\n", err)
			os.Exit(1)
		}

		versions := sm.Versions()
		if len(versions) == 0 {
			fmt.Println("No backup versions found.")
			return
		}

		if label := sm.StoreLabel(); label != "" {
			fmt.Printf("History of Remote Store [%s]:\n", label)
		} else {
			fmt.Println("History of Remote Store:")
		}
		fmt.Printf("Last updated: %s\n\n", sm.LastUpdated().Local().Format("2006-01-02 15:04:05"))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "VERSION ID\tTIMESTAMP\tKIND\tFILES\tSIZE\tCOMPRESSED\tLABEL")
		
		pbBytes, err := destStore.Read(ctx, ".tsync")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading metadata: %v\n", err)
			os.Exit(1)
		}
		
		var rawMeta tsyncv2.BackupMetadata
		err = proto.Unmarshal(pbBytes, &rawMeta)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing metadata: %v\n", err)
			os.Exit(1)
		}

		for _, v := range versions {
			fileCount, uncomp, comp, statsErr := getVersionStats(&rawMeta, v)
			if statsErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to calculate stats for version %d: %v\n", v.SnowflakeId, statsErr)
			}
			
			kindStr := "SNAPSHOT"
			if v.PrecedingVersionId != 0 {
				kindStr = fmt.Sprintf("SNAPSHOT (->%d)", v.PrecedingVersionId)
			}
			
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\t%s\n", 
				v.SnowflakeId, 
				v.BackupTimestamp.AsTime().Local().Format("2006-01-02 15:04:05"), 
				kindStr, 
				fileCount, 
				FormatBytes(uncomp), 
				FormatBytes(comp), 
				v.Label,
			)
		}
		w.Flush()
	},
}

func getVersionStats(metadata *tsyncv2.BackupMetadata, v *tsyncv2.Version) (int, int64, int64, error) {
	resolvedMap, err := ResolveVersionMap(metadata, v.SnowflakeId)
	if err != nil {
		return 0, 0, 0, err
	}
	
	fileCount := len(resolvedMap)
	var uncompressedSize int64
	var compressedSize int64
	
	for _, key := range resolvedMap {
		if rec, exists := metadata.Files[key]; exists {
			uncompressedSize += rec.UncompressedSize
			compressedSize += rec.CompressedSize
		}
	}
	
	return fileCount, uncompressedSize, compressedSize, nil
}

func init() {
	logCmd.Flags().StringVarP(&logRemote, "remote", "r", "", "Remote store name to read logs from")
	RootCmd.AddCommand(logCmd)
}
