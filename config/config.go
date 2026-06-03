package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/nacl/box"
	"gopkg.in/yaml.v3"
)

type RemoteConfig struct {
	Name      string            `yaml:"name"`
	Provider  string            `yaml:"provider"`            // "local", "s3", "oci", "http"
	Path      string            `yaml:"path,omitempty"`      // for local
	Bucket    string            `yaml:"bucket,omitempty"`    // for s3/oci
	Prefix    string            `yaml:"prefix,omitempty"`    // for s3/oci/http
	Region    string            `yaml:"region,omitempty"`    // for s3
	Endpoint  string            `yaml:"endpoint,omitempty"`  // for s3 (custom endpoint) / http base URL
	Namespace string            `yaml:"namespace,omitempty"` // for oci
	AuthType  string            `yaml:"auth_type,omitempty"` // for s3 (keys), oci (principal), http (headers)
}

type Config struct {
	Path           string                  `yaml:"path"`
	DefaultRemote  string                  `yaml:"default_remote"`
	DefaultKeyID   string                  `yaml:"default_key_id"`
	PrivateKeyPath string                  `yaml:"private_key_path,omitempty"`
	PublicKeys     map[string]string       `yaml:"public_keys"` // keyID -> hex-encoded pubkey
	Remotes        map[string]RemoteConfig `yaml:"remotes"`
}

// GetTSyncDir returns the path to the global ~/.tsync directory
func GetTSyncDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	tsyncDir := filepath.Join(home, ".tsync")
	if err := os.MkdirAll(tsyncDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ~/.tsync directory: %w", err)
	}
	return tsyncDir, nil
}

// GetKeysDir returns the path to ~/.tsync/keys
func GetKeysDir() (string, error) {
	base, err := GetTSyncDir()
	if err != nil {
		return "", err
	}
	keysDir := filepath.Join(base, "keys")
	if err := os.MkdirAll(keysDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create ~/.tsync/keys directory: %w", err)
	}
	return keysDir, nil
}

// ManglePath converts an absolute directory path to a valid filename
func ManglePath(dir string) string {
	dir = filepath.Clean(dir)
	dir = strings.ToLower(dir)
	var sb strings.Builder
	for _, r := range dir {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	name := sb.String()
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	return strings.Trim(name, "_") + ".yaml"
}

// GetConfigPath returns the path to the config file for the given local repository directory
func GetConfigPath(dir string) (string, error) {
	base, err := GetTSyncDir()
	if err != nil {
		return "", err
	}
	mangled := ManglePath(dir)
	return filepath.Join(base, mangled), nil
}

// LoadConfig loads the configuration file for the specified directory
func LoadConfig(dir string) (*Config, error) {
	cfgPath, err := GetConfigPath(dir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("repository not initialized (config not found for path %s). Run 'tsync init'", dir)
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml config: %w", err)
	}

	if cfg.PublicKeys == nil {
		cfg.PublicKeys = make(map[string]string)
	}
	if cfg.Remotes == nil {
		cfg.Remotes = make(map[string]RemoteConfig)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration back to disk
func SaveConfig(dir string, cfg *Config) error {
	cfgPath, err := GetConfigPath(dir)
	if err != nil {
		return err
	}

	cfg.Path = dir // ensure path is set correctly
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to yaml: %w", err)
	}

	return os.WriteFile(cfgPath, data, 0644)
}

// LoadPrivateKey loads the Curve25519 private key for a key ID
func LoadPrivateKey(keyID string) ([]byte, error) {
	// 1. Check environment variable override
	if envKey := os.Getenv("TSYNC_PRIVATE_KEY"); envKey != "" {
		keyBytes, err := hex.DecodeString(strings.TrimSpace(envKey))
		if err == nil && len(keyBytes) == 32 {
			return keyBytes, nil
		}
	}

	// 2. Check private key file in ~/.tsync/keys/
	keysDir, err := GetKeysDir()
	if err != nil {
		return nil, err
	}

	keyPath := filepath.Join(keysDir, keyID+".key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key for ID %s at %s: %w", keyID, keyPath, err)
	}

	keyHex := strings.TrimSpace(string(data))
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex-encoded private key: %w", err)
	}

	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(keyBytes))
	}

	return keyBytes, nil
}

// SavePrivateKey saves the Curve25519 private key for a key ID
func SavePrivateKey(keyID string, privKey []byte) error {
	if len(privKey) != 32 {
		return fmt.Errorf("invalid private key length to save: expected 32 bytes, got %d", len(privKey))
	}

	keysDir, err := GetKeysDir()
	if err != nil {
		return err
	}

	keyPath := filepath.Join(keysDir, keyID+".key")
	keyHex := hex.EncodeToString(privKey)
	return os.WriteFile(keyPath, []byte(keyHex), 0600)
}

// GenerateAndSaveKeypair generates a new Curve25519 key pair, saves private key, returns public key in hex
func GenerateAndSaveKeypair(keyID string) (pubKeyHex string, err error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate Curve25519 keypair: %w", err)
	}

	err = SavePrivateKey(keyID, priv[:])
	if err != nil {
		return "", fmt.Errorf("failed to save private key: %w", err)
	}

	return hex.EncodeToString(pub[:]), nil
}
