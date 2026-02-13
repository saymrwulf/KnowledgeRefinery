package extractors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

func TestComputeAtomIDDeterministic(t *testing.T) {
	id1 := ComputeAtomID("asset1", storage.AtomText, 0)
	id2 := ComputeAtomID("asset1", storage.AtomText, 0)
	if id1 != id2 {
		t.Errorf("atom IDs should be deterministic: %s != %s", id1, id2)
	}
	if len(id1) != 32 {
		t.Errorf("atom ID should be 32 chars, got %d", len(id1))
	}
}

func TestComputeAtomIDDiffers(t *testing.T) {
	id1 := ComputeAtomID("asset1", storage.AtomText, 0)
	id2 := ComputeAtomID("asset1", storage.AtomText, 1)
	if id1 == id2 {
		t.Error("different sequence indices should produce different IDs")
	}
}

func TestRegistryPriority(t *testing.T) {
	reg := CreateDefaultRegistry()

	// Create a text file asset
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	asset := storage.NewFileAsset("test-id", path, "test.txt")
	mt := "text/plain"
	asset.MimeType = &mt

	atoms, err := reg.Extract(asset)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if atoms[0].PayloadText == nil || *atoms[0].PayloadText != "hello world" {
		t.Errorf("unexpected atom text")
	}
}

func TestRegistryNoExtractor(t *testing.T) {
	reg := NewRegistry() // empty registry

	asset := storage.NewFileAsset("test-id", "/fake/file.xyz", "file.xyz")
	_, err := reg.Extract(asset)
	if err == nil {
		t.Error("expected error for unhandled file type")
	}
}

func TestTextExtractorCanHandle(t *testing.T) {
	ext := &TextExtractor{}
	tests := []struct {
		filename string
		expected bool
	}{
		{"file.txt", true},
		{"file.md", true},
		{"file.html", true},
		{"file.htm", true},
		{"file.rtf", true},
		{"file.markdown", true},
		{"file.pdf", false},
		{"file.jpg", false},
	}
	for _, tt := range tests {
		asset := storage.NewFileAsset("id", "/path/"+tt.filename, tt.filename)
		if got := ext.CanHandle(asset); got != tt.expected {
			t.Errorf("CanHandle(%q) = %v, expected %v", tt.filename, got, tt.expected)
		}
	}
}

func TestTextExtractorPlainText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	os.WriteFile(path, []byte("  Hello World  "), 0644)

	ext := &TextExtractor{}
	asset := storage.NewFileAsset("id", path, "hello.txt")

	atoms, err := ext.Extract(asset)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	if *atoms[0].PayloadText != "Hello World" {
		t.Errorf("expected trimmed text, got %q", *atoms[0].PayloadText)
	}
}

func TestTextExtractorHTML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "page.html")
	os.WriteFile(path, []byte("<html><body><p>Hello &amp; World</p></body></html>"), 0644)

	ext := &TextExtractor{}
	asset := storage.NewFileAsset("id", path, "page.html")

	atoms, err := ext.Extract(asset)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("expected 1 atom, got %d", len(atoms))
	}
	text := *atoms[0].PayloadText
	if text == "" {
		t.Error("expected non-empty text")
	}
	// Should have stripped tags and unescaped &amp;
	if contains(text, "<p>") {
		t.Error("HTML tags should be stripped")
	}
}

func TestTextExtractorEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	os.WriteFile(path, []byte("   "), 0644)

	ext := &TextExtractor{}
	asset := storage.NewFileAsset("id", path, "empty.txt")

	atoms, err := ext.Extract(asset)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(atoms) != 0 {
		t.Errorf("expected 0 atoms for whitespace-only file, got %d", len(atoms))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestEPUBExtractorCanHandle(t *testing.T) {
	ext := &EPUBExtractor{}
	asset := storage.NewFileAsset("id", "/path/book.epub", "book.epub")
	if !ext.CanHandle(asset) {
		t.Error("should handle .epub files")
	}
	asset2 := storage.NewFileAsset("id", "/path/doc.pdf", "doc.pdf")
	if ext.CanHandle(asset2) {
		t.Error("should not handle .pdf files")
	}
}

func TestPDFExtractorCanHandle(t *testing.T) {
	ext := &PDFExtractor{}
	asset := storage.NewFileAsset("id", "/path/doc.pdf", "doc.pdf")
	if !ext.CanHandle(asset) {
		t.Error("should handle .pdf files")
	}
	asset2 := storage.NewFileAsset("id", "/path/doc.txt", "doc.txt")
	if ext.CanHandle(asset2) {
		t.Error("should not handle .txt files")
	}
}

func TestImageExtractorCanHandle(t *testing.T) {
	ext := &ImageExtractor{}
	for _, fn := range []string{"img.jpg", "img.jpeg", "img.png", "img.webp", "img.heic", "img.tiff"} {
		asset := storage.NewFileAsset("id", "/path/"+fn, fn)
		if !ext.CanHandle(asset) {
			t.Errorf("should handle %s", fn)
		}
	}
	asset := storage.NewFileAsset("id", "/path/doc.txt", "doc.txt")
	if ext.CanHandle(asset) {
		t.Error("should not handle .txt files")
	}
}

func TestArchiveExtractorCanHandle(t *testing.T) {
	ext := &ArchiveExtractor{}
	for _, fn := range []string{"archive.zip", "archive.tar", "archive.gz"} {
		asset := storage.NewFileAsset("id", "/path/"+fn, fn)
		if !ext.CanHandle(asset) {
			t.Errorf("should handle %s", fn)
		}
	}
}

func TestDICOMExtractorCanHandle(t *testing.T) {
	ext := &DICOMExtractor{}
	asset := storage.NewFileAsset("id", "/path/scan.dcm", "scan.dcm")
	if !ext.CanHandle(asset) {
		t.Error("should handle .dcm files")
	}
	asset2 := storage.NewFileAsset("id", "/path/scan.dicom", "scan.dicom")
	if !ext.CanHandle(asset2) {
		t.Error("should handle .dicom files")
	}
}

func TestTikaFallbackCanHandle(t *testing.T) {
	ext := &TikaFallbackExtractor{}
	// Should handle everything
	asset := storage.NewFileAsset("id", "/path/anything.xyz", "anything.xyz")
	if !ext.CanHandle(asset) {
		t.Error("tika fallback should handle any file")
	}
}

func TestDefaultRegistryOrder(t *testing.T) {
	reg := CreateDefaultRegistry()
	// Verify we have all extractors
	if len(reg.extractors) != 7 {
		t.Errorf("expected 7 extractors, got %d", len(reg.extractors))
	}
	// Verify priority ordering (highest first)
	for i := 1; i < len(reg.extractors); i++ {
		if reg.extractors[i].Priority() > reg.extractors[i-1].Priority() {
			t.Errorf("extractors not sorted by priority: %s(%d) > %s(%d)",
				reg.extractors[i].Name(), reg.extractors[i].Priority(),
				reg.extractors[i-1].Name(), reg.extractors[i-1].Priority())
		}
	}
}
