package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"t-sync-cli/config"

	"github.com/abyii/t-sync-sdk-go/storage_clients"
	_ "github.com/abyii/t-sync-sdk-go/storage_clients/http"
	_ "github.com/abyii/t-sync-sdk-go/storage_clients/oci"
	_ "github.com/abyii/t-sync-sdk-go/storage_clients/s3"
	"github.com/abyii/t-sync-sdk-go/tsync"
)

var StorageOverride tsync.Storage

// ResolveRemoteStorage returns a T-Sync Storage interface for the specified remote name (or default remote).
func ResolveRemoteStorage(remoteName string) (tsync.Storage, error) {
	if StorageOverride != nil {
		return StorageOverride, nil
	}

	cfg := LoadRepoConfig()

	if remoteName == "" {
		remoteName = cfg.DefaultRemote
	}

	if remoteName == "" {
		return nil, fmt.Errorf("no default remote configured. Configure a remote using 'tsync remote add' or specify one with -r/--remote")
	}

	rc, exists := cfg.Remotes[remoteName]
	if !exists {
		return nil, fmt.Errorf("remote '%s' not found in configuration", remoteName)
	}

	return InitStorage(rc)
}

// InitStorage instantiates the corresponding SDK storage provider.
func InitStorage(rc config.RemoteConfig) (tsync.Storage, error) {
	switch rc.Provider {
	case "local":
		return tsync.NewLocalStorage(rc.Path)

	case "s3":
		auth := rc.AuthType
		// Ergonomic check: load from AWS env variables if auth_type is empty
		if auth == "" {
			accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
			secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
			sessionToken := os.Getenv("AWS_SESSION_TOKEN")
			if accessKey != "" && secretKey != "" {
				if sessionToken != "" {
					auth = fmt.Sprintf("S3_ACCESS_KEYS[%s:%s:%s]", accessKey, secretKey, sessionToken)
				} else {
					auth = fmt.Sprintf("S3_ACCESS_KEYS[%s:%s]", accessKey, secretKey)
				}
			}
		}

		if auth == "" {
			return nil, fmt.Errorf("S3 auth_type not configured and AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY are not set in environment")
		}

		if rc.Region != "" {
			os.Setenv("AWS_REGION", rc.Region)
			os.Setenv("AWS_DEFAULT_REGION", rc.Region)
		}

		client, err := storage_clients.GetClient("s3", auth, "")
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS S3 client: %w", err)
		}

		return tsync.NewObjectStorage(client, rc.Bucket, rc.Prefix), nil

	case "oci":
		if rc.AuthType == "" {
			return nil, fmt.Errorf("OCI auth_type is required in configuration (e.g. OCI_CONFIG_FILE, RESOURCE_PRINCIPAL)")
		}

		client, err := storage_clients.GetClient("oci", rc.AuthType, rc.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize OCI Object Storage client: %w", err)
		}

		return tsync.NewObjectStorage(client, rc.Bucket, rc.Prefix), nil

	case "http":
		client, err := storage_clients.GetClient("http", rc.AuthType, rc.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize HTTP storage client: %w", err)
		}

		return tsync.NewObjectStorage(client, "", rc.Prefix), nil

	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", rc.Provider)
	}
}

// MakeProgressCallback returns a snappy progress-printing function.
func MakeProgressCallback(opName string) func(done, total int, path string) {
	return func(done, total int, path string) {
		pct := float64(done) / float64(total) * 100

		// Truncate path for terminal presentation
		displayPath := path
		if len(displayPath) > 45 {
			displayPath = "..." + displayPath[len(displayPath)-42:]
		}

		// Snappy, clean progress print
		fmt.Printf("\r\033[K[%s] %5.1f%% (%d/%d) %s", opName, pct, done, total, displayPath)
		if done == total {
			fmt.Println()
		}
	}
}

// FormatBytes formats byte sizes nicely (B, KB, MB, GB)
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// HexDecodeKey decodes a hex public/private key string
func HexDecodeKey(h string) ([]byte, error) {
	h = strings.TrimSpace(h)
	return hex.DecodeString(h)
}
