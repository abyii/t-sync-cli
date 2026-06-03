package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/abyii/t-sync-sdk-go/tsync"
	tsyncv1 "github.com/abyii/t-sync-sdk-go/gen/go/com/github/abyii/tsync/v1"
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
			if os.IsNotExist(err) || (err != nil && (err.Error() == "failed to read store metadata: file does not exist" || os.IsNotExist(err))) {
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

		fmt.Printf("History of Remote Store [%s]:\n", sm.StoreLabel())
		fmt.Printf("Last updated: %s\n\n", sm.LastUpdated().Local().Format("2006-01-02 15:04:05"))

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "VERSION ID\tTIMESTAMP\tKIND\tFILES\tSIZE\tCOMPRESSED\tLABEL")
		
		// For calculating statistics, we need to access the underlying proto metadata
		// The StoreMetadata type doesn't export the direct proto struct directly except through package?
		// Wait! The StoreMetadata has an unexported 'metadata' field.
		// Let's check how we can get versions and files.
		// Wait, sm.Versions() returns []*tsyncv1.Version.
		// Let's check if the sdk has public methods on StoreMetadata or if we can read the raw metadata ourselves.
		// In client.go, client.ListVersions(ctx) unmarshals and returns []*tsyncv1.Version.
		// Wait, if we can read the raw metadata bytes ourselves:
		// Yes, we can read `.tsync` and unmarshal it directly! That gives us full access to the proto struct.
		// Let's do that! It is very easy and 100% safe.
		
		pbBytes, err := destStore.Read(ctx, ".tsync")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading metadata: %v\n", err)
			os.Exit(1)
		}
		
		var rawMeta tsyncv1.BackupMetadata
		// Wait! Let's check the import path of proto: it's "google.golang.org/protobuf/proto"
		// Let's check if there's any helper function. We can unmarshal it using proto.Unmarshal
		// We've imported google.golang.org/protobuf/proto in client.go. We can do the same.
		// Let's write the unmarshal logic.
		
		// Import proto is needed in this file if we unmarshal.
		
		// Let's write the tabwriter log formatting.
		
		// Wait, we can define a small helper function to unmarshal inside log.go:
		
		// We'll import "google.golang.org/protobuf/proto" in this file.
		
		// Let's check if the proto.Unmarshal matches. Yes.
		
		// Let's do it!
		
		// Wait, let's write it in Go:
		
		// we'll get the raw meta:
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
			
			kindStr := "FULL"
			if v.Kind == tsyncv1.VersionKind_VERSION_KIND_DELTA {
				kindStr = fmt.Sprintf("DELTA (->%d)", v.ParentId)
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

func getVersionStats(metadata *tsyncv1.BackupMetadata, v *tsyncv1.Version) (int, int64, int64, error) {
	resolvedMap, err := tsync.ResolveVersionMap(metadata, v.SnowflakeId)
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
