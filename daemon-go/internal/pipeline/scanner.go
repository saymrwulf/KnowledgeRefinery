package pipeline

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/config"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// SupportedExtensions lists file types the pipeline can process.
var SupportedExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".html": true, ".htm": true, ".rtf": true,
	".pdf":  true,
	".jpg":  true, ".jpeg": true, ".png": true, ".webp": true,
	".heic": true, ".heif": true, ".tiff": true, ".tif": true,
	".epub": true, ".mobi": true,
	".zip":  true, ".tar": true, ".gz": true, ".xz": true, ".7z": true, ".rar": true, ".iso": true,
	".dcm":  true, ".dicom": true,
}

// ScanStats holds results from a directory scan.
type ScanStats struct {
	New       int `json:"new"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

func (s *ScanStats) Add(other ScanStats) {
	s.New += other.New
	s.Updated += other.Updated
	s.Unchanged += other.Unchanged
	s.Skipped += other.Skipped
	s.Errors += other.Errors
}

func (s ScanStats) ToMap() map[string]int {
	return map[string]int{
		"new":       s.New,
		"updated":   s.Updated,
		"unchanged": s.Unchanged,
		"skipped":   s.Skipped,
		"errors":    s.Errors,
	}
}

// ComputeAssetID generates a deterministic asset ID from path + mtime + size.
func ComputeAssetID(path string, mtimeNs int64, size int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", path, mtimeNs, size)))
	return fmt.Sprintf("%x", h)[:32]
}

// ComputeContentHash computes a streaming SHA-256 of a file's contents.
func ComputeContentHash(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// GuessMimeType guesses the MIME type from a file extension.
func GuessMimeType(path string) *string {
	ext := filepath.Ext(path)
	if ext == "" {
		return nil
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Common types not in Go's registry
		switch strings.ToLower(ext) {
		case ".md", ".markdown":
			mimeType = "text/markdown"
		case ".epub":
			mimeType = "application/epub+zip"
		case ".rtf":
			mimeType = "application/rtf"
		case ".heic", ".heif":
			mimeType = "image/heic"
		case ".webp":
			mimeType = "image/webp"
		case ".dcm", ".dicom":
			mimeType = "application/dicom"
		default:
			return nil
		}
	}
	return &mimeType
}

// Scanner walks directories and maintains the file asset manifest.
type Scanner struct {
	db          *storage.Database
	maxFileSize int64
}

func NewScanner(db *storage.Database, cfg config.Config) *Scanner {
	return &Scanner{
		db:          db,
		maxFileSize: cfg.Pipeline.MaxFileSizeBytes,
	}
}

// ScanDirectory walks a directory tree and upserts FileAssets.
func (s *Scanner) ScanDirectory(root string) (ScanStats, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return ScanStats{}, fmt.Errorf("not a directory: %s", root)
	}

	var stats ScanStats
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			stats.Errors++
			return nil
		}
		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		// Skip hidden files
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if processErr := s.processFile(path, &stats); processErr != nil {
			slog.Error("Error processing file", "path", path, "error", processErr)
			stats.Errors++
		}
		return nil
	})
	return stats, err
}

func (s *Scanner) processFile(path string, stats *ScanStats) error {
	info, err := os.Stat(path)
	if err != nil {
		stats.Skipped++
		return nil
	}
	if info.Size() > s.maxFileSize {
		stats.Skipped++
		return nil
	}
	if info.Size() == 0 {
		stats.Skipped++
		return nil
	}

	absPath, _ := filepath.Abs(path)
	mtimeNs := info.ModTime().UnixNano()
	sizeBytes := info.Size()

	existing, err := s.db.GetFileAssetByPath(absPath)
	if err != nil {
		return err
	}

	if existing != nil {
		// Check if unchanged
		if existing.MtimeNs == mtimeNs && existing.SizeBytes == sizeBytes {
			stats.Unchanged++
			return nil
		}
		// File changed - recompute hash
		hash, err := ComputeContentHash(absPath)
		if err != nil {
			return err
		}
		if existing.ContentHash != nil && *existing.ContentHash == hash {
			stats.Unchanged++
			return nil
		}
		// Genuinely changed
		assetID := ComputeAssetID(absPath, mtimeNs, sizeBytes)
		mimeType := GuessMimeType(absPath)
		asset := storage.NewFileAsset(assetID, absPath, info.Name())
		asset.MimeType = mimeType
		asset.SizeBytes = sizeBytes
		asset.MtimeNs = mtimeNs
		asset.ContentHash = &hash
		if err := s.db.UpsertFileAsset(asset); err != nil {
			return err
		}
		stats.Updated++
		slog.Info("Updated asset", "file", info.Name())
	} else {
		// New file
		hash, err := ComputeContentHash(absPath)
		if err != nil {
			return err
		}
		assetID := ComputeAssetID(absPath, mtimeNs, sizeBytes)
		mimeType := GuessMimeType(absPath)
		asset := storage.NewFileAsset(assetID, absPath, info.Name())
		asset.MimeType = mimeType
		asset.SizeBytes = sizeBytes
		asset.MtimeNs = mtimeNs
		asset.ContentHash = &hash
		if err := s.db.UpsertFileAsset(asset); err != nil {
			return err
		}
		stats.New++
		slog.Debug("New asset", "file", info.Name())
	}
	return nil
}
