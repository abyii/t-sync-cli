package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/abyii/t-sync-sdk-go/tsync"
	"t-sync-cli/cmd"
)



// ---------------------------------------------------------
// Thread-safe Mock Memory Storage for Testing
// ---------------------------------------------------------

type MemStorage struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		files: make(map[string][]byte),
	}
}

func (m *MemStorage) Exists(ctx context.Context, path string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.files[path]
	return ok, nil
}

func (m *MemStorage) Read(ctx context.Context, path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (m *MemStorage) Write(ctx context.Context, path string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.files[path] = cp
	return nil
}

func (m *MemStorage) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, path)
	return nil
}

func (m *MemStorage) List(ctx context.Context, prefix string) ([]tsync.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []tsync.FileInfo
	for name, data := range m.files {
		if strings.HasPrefix(name, prefix) {
			list = append(list, tsync.FileInfo{
				Name:         name,
				Size:         int64(len(data)),
				LastModified: time.Now(),
			})
		}
	}
	return list, nil
}

func (m *MemStorage) OpenReader(ctx context.Context, path string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

type memWriter struct {
	buf bytes.Buffer
	cb  func([]byte) error
}

func (w *memWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *memWriter) Close() error {
	return w.cb(w.buf.Bytes())
}

func (m *MemStorage) OpenWriter(ctx context.Context, path string) (io.WriteCloser, error) {
	return &memWriter{
		cb: func(data []byte) error {
			return m.Write(ctx, path, data)
		},
	}, nil
}

func (m *MemStorage) Size(ctx context.Context, path string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	if !ok {
		return 0, os.ErrNotExist
	}
	return int64(len(data)), nil
}

func (m *MemStorage) ReadRange(ctx context.Context, path string, offset, length int64) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}

	if offset > int64(len(data)) {
		offset = int64(len(data))
	}

	end := int64(len(data))
	if length >= 0 && offset+length < end {
		end = offset + length
	}

	slice := data[offset:end]
	return io.NopCloser(bytes.NewReader(slice)), nil
}

// ---------------------------------------------------------
// Helper: generate random files content
// ---------------------------------------------------------

func randomBytes(size int) []byte {
	b := make([]byte, size)
	_, _ = rand.Read(b)
	return b
}

func generateRandomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

func TestCLI_EndToEnd(t *testing.T) {
	// 1. Set up sandboxed home / user profile
	tempHome := t.TempDir()
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("HOME", tempHome)

	// Create temporary local repository directory
	repoDir := t.TempDir()

	// 2. Setup mock remote storage
	memStore := NewMemStorage()
	cmd.StorageOverride = memStore
	defer func() {
		cmd.StorageOverride = nil
	}()

	// Helper function to execute commands and capture stdout/stderr from fmt.Printf and Cobra
	runCmd := func(args ...string) (string, error) {
		cmd.RootCmd.SetArgs(args)
		
		// Redirect standard stdout/stderr
		oldStdout := os.Stdout
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stdout = w
		os.Stderr = w
		
		var buf bytes.Buffer
		cmd.RootCmd.SetOut(&buf)
		cmd.RootCmd.SetErr(&buf)
		
		err := cmd.RootCmd.Execute()
		
		// Restore and close write pipe
		w.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		
		var pipeBuf bytes.Buffer
		_, _ = io.Copy(&pipeBuf, r)
		
		return buf.String() + pipeBuf.String(), err
	}

	// 3. E2E Scenario Steps

	// Step 1: Init repository
	t.Log("Initializing repository...")
	out, err := runCmd("init", "--repo-path", repoDir, "--compression-level", "9")
	if err != nil {
		t.Fatalf("init failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Initialized empty T-Sync repository") {
		t.Errorf("Unexpected init output: %s", out)
	}

	// Step 2: Keygen
	t.Log("Generating keypair...")
	out, err = runCmd("keygen", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("keygen failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Generated new keypair with ID: default") {
		t.Errorf("Unexpected keygen output: %s", out)
	}

	// Step 3: Remote Add
	t.Log("Adding remote...")
	out, err = runCmd("remote", "add", "origin", "local", "--path", "dummy-remote-path", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("remote add failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Added remote 'origin'") {
		t.Errorf("Unexpected remote add output: %s", out)
	}

	// Step 4: Status check
	t.Log("Checking status...")
	out, err = runCmd("status", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("status failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Default Key ID: default") || !strings.Contains(out, "Compression Level:   9") {
		t.Errorf("Unexpected status output: %s", out)
	}

	// Step 5: Add some files to repoDir
	file1Path := filepath.Join(repoDir, "file1.txt")
	file2Path := filepath.Join(repoDir, "file2.txt")
	file3Path := filepath.Join(repoDir, "file3.txt")
	file4Path := filepath.Join(repoDir, "file4.txt")
	if err := os.WriteFile(file1Path, []byte("Hello, T-Sync!"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte("Another test file content."), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}
	if err := os.WriteFile(file3Path, []byte("Third file content."), 0644); err != nil {
		t.Fatalf("Failed to write file3: %v", err)
	}
	if err := os.WriteFile(file4Path, []byte("Fourth file content."), 0644); err != nil {
		t.Fatalf("Failed to write file4: %v", err)
	}

	// Step 6: Backup (Full) with flag override
	t.Log("Performing first backup (Full)...")
	out, err = runCmd("backup", "--message", "First E2E Backup", "--repo-path", repoDir, "--compression-level", "5")
	if err != nil {
		t.Fatalf("backup failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Backup completed successfully!") {
		t.Errorf("Unexpected backup output: %s", out)
	}

	// Extract the Version ID from output
	var version1ID uint64
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if strings.Contains(l, "Version ID:") {
			parts := strings.Fields(l)
			if len(parts) >= 3 {
				_, _ = fmt.Sscanf(parts[2], "%d", &version1ID)
			}
		}
	}
	if version1ID == 0 {
		t.Fatalf("Failed to extract version ID from backup output: %s", out)
	}
	t.Logf("First backup version ID: %d", version1ID)

	// Step 7: Log command
	t.Log("Checking log...")
	out, err = runCmd("log", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("log failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "First E2E Backup") || !strings.Contains(out, fmt.Sprintf("%d", version1ID)) {
		t.Errorf("Unexpected log output: %s", out)
	}

	// Step 8: Keys commands (list, show)
	t.Log("Testing keys commands...")
	out, err = runCmd("keys", "list", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("keys list failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("Unexpected keys list output: %s", out)
	}

	out, err = runCmd("keys", "show", "default", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("keys show failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Key ID:          default") {
		t.Errorf("Unexpected keys show output: %s", out)
	}

	// Step 9: Inspect commands (summary, versions, version, files)
	t.Log("Testing inspect commands...")
	out, err = runCmd("inspect", "summary", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("inspect summary failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Total Versions:  1") {
		t.Errorf("Unexpected inspect summary output: %s", out)
	}

	out, err = runCmd("inspect", "versions", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("inspect versions failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, fmt.Sprintf("%d", version1ID)) {
		t.Errorf("Unexpected inspect versions output: %s", out)
	}

	out, err = runCmd("inspect", "version", fmt.Sprintf("%d", version1ID), "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("inspect version failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "file1.txt") || !strings.Contains(out, "file2.txt") {
		t.Errorf("Unexpected inspect version output: %s", out)
	}

	out, err = runCmd("inspect", "files", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("inspect files failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "FILE KEY") {
		t.Errorf("Unexpected inspect files output: %s", out)
	}

	// Step 10: Modify files and perform delta backup
	t.Log("Modifying file1.txt for Delta Backup...")
	if err := os.WriteFile(file1Path, []byte("Hello, T-Sync Delta!"), 0644); err != nil {
		t.Fatalf("Failed to modify file1: %v", err)
	}

	out, err = runCmd("backup", "--message", "Second E2E Backup (Delta)", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("delta backup failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Backup completed successfully!") {
		t.Errorf("Unexpected delta backup output: %s", out)
	}

	var version2ID uint64
	lines = strings.Split(out, "\n")
	for _, l := range lines {
		if strings.Contains(l, "Version ID:") {
			parts := strings.Fields(l)
			if len(parts) >= 3 {
				_, _ = fmt.Sscanf(parts[2], "%d", &version2ID)
			}
		}
	}
	if version2ID == 0 {
		t.Fatalf("Failed to extract delta version ID from backup output: %s", out)
	}
	t.Logf("Second backup version ID (Delta): %d", version2ID)

	// Step 11: Verify delta structure using inspect
	out, err = runCmd("inspect", "version", fmt.Sprintf("%d", version2ID), "--raw", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("inspect raw delta version failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Delta Changes:") || !strings.Contains(out, "Delta Deleted:") {
		t.Errorf("Unexpected raw inspect output: %s", out)
	}

	// Step 12: Restore
	t.Log("Restoring version 1 to a separate folder...")
	restoreDir := t.TempDir()
	out, err = runCmd("restore", fmt.Sprintf("%d", version1ID), "--dir", restoreDir, "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("restore failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Restoration completed successfully!") {
		t.Errorf("Unexpected restore output: %s", out)
	}

	// Verify restored files content
	restored1 := filepath.Join(restoreDir, "file1.txt")
	restored2 := filepath.Join(restoreDir, "file2.txt")
	b1, err := os.ReadFile(restored1)
	if err != nil {
		t.Fatalf("Failed to read restored file1: %v", err)
	}
	if string(b1) != "Hello, T-Sync!" {
		t.Errorf("Expected 'Hello, T-Sync!', got: '%s'", string(b1))
	}
	b2, err := os.ReadFile(restored2)
	if err != nil {
		t.Fatalf("Failed to read restored file2: %v", err)
	}
	if string(b2) != "Another test file content." {
		t.Errorf("Expected 'Another test file content.', got: '%s'", string(b2))
	}

	// Step 12b: Restore to zip without specifying name (optional flag argument)
	t.Log("Restoring version 1 to zip (default filename)...")
	out, err = runCmd("restore", fmt.Sprintf("%d", version1ID), "--zip", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("restore to zip failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Restoration completed successfully!") {
		t.Errorf("Unexpected restore output: %s", out)
	}
	
	defaultZipPath := fmt.Sprintf("restore_%d.zip", version1ID)
	if _, err := os.Stat(defaultZipPath); os.IsNotExist(err) {
		t.Errorf("Expected default zip file to be created at: %s", defaultZipPath)
	} else {
		os.Remove(defaultZipPath)
	}

	// Step 13: Delete key
	t.Log("Testing keys delete...")
	out, err = runCmd("keys", "delete", "default", "--repo-path", repoDir)
	if err != nil {
		t.Fatalf("keys delete failed: %v, output: %s", err, out)
	}
	if !strings.Contains(out, "Deleted private key file") {
		t.Errorf("Unexpected keys delete output: %s", out)
	}
}

