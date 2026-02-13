package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oho/knowledge-refinery-daemon/internal/config"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

func setupScannerTest(t *testing.T) (*Scanner, *storage.Database, string) {
	t.Helper()
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Initialize(); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Pipeline: config.PipelineConfig{
			MaxFileSizeBytes: 100 * 1024 * 1024,
		},
	}
	scanner := NewScanner(db, cfg)
	dir := t.TempDir()
	return scanner, db, dir
}

func TestScanDirectoryNewFiles(t *testing.T) {
	scanner, _, dir := setupScannerTest(t)

	// Create test files
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("Hello world"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\nSome content"), 0644)

	stats, err := scanner.ScanDirectory(dir)
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	if stats.New != 2 {
		t.Errorf("expected 2 new, got %d", stats.New)
	}
	if stats.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", stats.Errors)
	}
}

func TestScanDirectoryUnchanged(t *testing.T) {
	scanner, _, dir := setupScannerTest(t)

	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("Hello world"), 0644)

	// First scan
	stats1, _ := scanner.ScanDirectory(dir)
	if stats1.New != 1 {
		t.Fatalf("expected 1 new, got %d", stats1.New)
	}

	// Second scan — same file
	stats2, _ := scanner.ScanDirectory(dir)
	if stats2.Unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", stats2.Unchanged)
	}
	if stats2.New != 0 {
		t.Errorf("expected 0 new on second scan, got %d", stats2.New)
	}
}

func TestScanDirectorySkipsHiddenFiles(t *testing.T) {
	scanner, _, dir := setupScannerTest(t)

	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)
	os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("hello"), 0644)

	stats, _ := scanner.ScanDirectory(dir)
	if stats.New != 1 {
		t.Errorf("expected 1 new (hidden file skipped), got %d", stats.New)
	}
}

func TestScanDirectorySkipsEmptyFiles(t *testing.T) {
	scanner, _, dir := setupScannerTest(t)

	os.WriteFile(filepath.Join(dir, "empty.txt"), []byte(""), 0644)

	stats, _ := scanner.ScanDirectory(dir)
	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped (empty), got %d", stats.Skipped)
	}
}

func TestScanDirectoryNotADirectory(t *testing.T) {
	scanner, _, _ := setupScannerTest(t)

	_, err := scanner.ScanDirectory("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestComputeAssetIDDeterministic(t *testing.T) {
	id1 := ComputeAssetID("/path/to/file.txt", 1234567890, 42)
	id2 := ComputeAssetID("/path/to/file.txt", 1234567890, 42)
	if id1 != id2 {
		t.Errorf("asset IDs should be deterministic: %s != %s", id1, id2)
	}
	if len(id1) != 32 {
		t.Errorf("asset ID should be 32 chars, got %d", len(id1))
	}
}

func TestComputeAssetIDDiffers(t *testing.T) {
	id1 := ComputeAssetID("/path/a.txt", 100, 42)
	id2 := ComputeAssetID("/path/b.txt", 100, 42)
	if id1 == id2 {
		t.Error("different paths should produce different IDs")
	}
}

func TestComputeContentHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	hash, err := ComputeContentHash(path)
	if err != nil {
		t.Fatalf("ComputeContentHash: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char SHA256 hex, got %d chars", len(hash))
	}
}

func TestGuessMimeType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
		isNil    bool
	}{
		{"file.md", "text/markdown", false},
		{"file.epub", "application/epub+zip", false},
		{"file.dcm", "application/dicom", false},
		{"file.rtf", "application/rtf", false},
		{"file", "", true},
	}

	for _, tt := range tests {
		mt := GuessMimeType(tt.path)
		if tt.isNil {
			if mt != nil {
				t.Errorf("GuessMimeType(%q) = %v, expected nil", tt.path, *mt)
			}
		} else {
			if mt == nil {
				t.Errorf("GuessMimeType(%q) = nil, expected %q", tt.path, tt.expected)
			} else if *mt != tt.expected {
				t.Errorf("GuessMimeType(%q) = %q, expected %q", tt.path, *mt, tt.expected)
			}
		}
	}
}

func TestScanStatsAdd(t *testing.T) {
	a := ScanStats{New: 1, Updated: 2, Unchanged: 3, Skipped: 4, Errors: 5}
	b := ScanStats{New: 10, Updated: 20, Unchanged: 30, Skipped: 40, Errors: 50}
	a.Add(b)
	if a.New != 11 || a.Updated != 22 || a.Unchanged != 33 || a.Skipped != 44 || a.Errors != 55 {
		t.Errorf("unexpected stats after Add: %+v", a)
	}
}
