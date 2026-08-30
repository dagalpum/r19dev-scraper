package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "scanner_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	validFile := filepath.Join(tempDir, "MIDA-517.mp4")
	if err := os.WriteFile(validFile, make([]byte, 1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}

	smallFile := filepath.Join(tempDir, "sample.mp4")
	if err := os.WriteFile(smallFile, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	txtFile := filepath.Join(tempDir, "readme.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Extensions:      []string{".mp4"},
		MinSizeMB:       0,
		ExcludePatterns: []string{"*sample*"},
	}

	sc := New(cfg)
	res, err := sc.Scan(tempDir)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(res.Files))
	}
	if res.Files[0].Name != "MIDA-517.mp4" {
		t.Errorf("expected MIDA-517.mp4, got %s", res.Files[0].Name)
	}
}
