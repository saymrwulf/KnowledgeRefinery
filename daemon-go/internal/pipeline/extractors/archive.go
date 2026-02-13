package extractors

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

var archiveExtensions = map[string]bool{
	".zip": true, ".tar": true, ".gz": true, ".xz": true,
	".7z": true, ".rar": true, ".iso": true,
}

const (
	maxArchiveFiles    = 10000
	maxArchiveTotalMB  = 500
	maxArchiveFileMB   = 50
	maxArchiveDepth    = 3
)

// ArchiveExtractor handles ZIP and TAR archives with security checks.
type ArchiveExtractor struct{}

func (e *ArchiveExtractor) Name() string     { return "archive" }
func (e *ArchiveExtractor) Priority() int    { return 5 }

func (e *ArchiveExtractor) CanHandle(asset storage.FileAsset) bool {
	ext := strings.ToLower(filepath.Ext(asset.Filename))
	return archiveExtensions[ext]
}

func (e *ArchiveExtractor) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	ext := strings.ToLower(filepath.Ext(asset.Filename))
	switch ext {
	case ".zip":
		return extractZip(asset, 0)
	case ".tar":
		return extractTar(asset, false, 0)
	case ".gz":
		return extractTar(asset, true, 0)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", ext)
	}
}

func extractZip(asset storage.FileAsset, depth int) ([]storage.ContentAtom, error) {
	if depth >= maxArchiveDepth {
		return nil, fmt.Errorf("archive recursion depth exceeded")
	}

	r, err := zip.OpenReader(asset.Path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if len(r.File) > maxArchiveFiles {
		return nil, fmt.Errorf("archive bomb: too many files (%d)", len(r.File))
	}

	var atoms []storage.ContentAtom
	var totalSize int64
	seqIdx := 0

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		// Zip-slip prevention
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			slog.Warn("Skipping suspicious archive member", "name", f.Name)
			continue
		}

		if f.UncompressedSize64 > maxArchiveFileMB*1024*1024 {
			continue
		}
		totalSize += int64(f.UncompressedSize64)
		if totalSize > maxArchiveTotalMB*1024*1024 {
			return atoms, fmt.Errorf("archive bomb: total size exceeded")
		}

		ext := strings.ToLower(filepath.Ext(f.Name))
		if !isTextLike(ext) {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxArchiveFileMB*1024*1024))
		rc.Close()
		if err != nil {
			continue
		}

		text := strings.TrimSpace(string(data))
		if text == "" {
			continue
		}

		chain := f.Name
		anchor := storage.EvidenceAnchor{AssetID: asset.ID, ArchiveChain: &chain}
		atom := storage.NewContentAtom(
			ComputeAtomID(asset.ID, storage.AtomText, seqIdx),
			asset.ID, storage.AtomText, seqIdx, anchor.ToJSON(),
		)
		atom.PayloadText = &text
		atoms = append(atoms, atom)
		seqIdx++
	}

	return atoms, nil
}

func extractTar(asset storage.FileAsset, isGzip bool, depth int) ([]storage.ContentAtom, error) {
	if depth >= maxArchiveDepth {
		return nil, fmt.Errorf("archive recursion depth exceeded")
	}

	f, err := os.Open(asset.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = f
	if isGzip {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}

	tr := tar.NewReader(reader)
	var atoms []storage.ContentAtom
	var totalSize int64
	fileCount := 0
	seqIdx := 0

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		fileCount++
		if fileCount > maxArchiveFiles {
			return atoms, fmt.Errorf("archive bomb: too many files")
		}

		// Zip-slip prevention
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}

		if header.Size > maxArchiveFileMB*1024*1024 {
			continue
		}
		totalSize += header.Size
		if totalSize > maxArchiveTotalMB*1024*1024 {
			return atoms, fmt.Errorf("archive bomb: total size exceeded")
		}

		ext := strings.ToLower(filepath.Ext(header.Name))
		if !isTextLike(ext) {
			continue
		}

		data, err := io.ReadAll(io.LimitReader(tr, maxArchiveFileMB*1024*1024))
		if err != nil {
			continue
		}

		text := strings.TrimSpace(string(data))
		if text == "" {
			continue
		}

		chain := header.Name
		anchor := storage.EvidenceAnchor{AssetID: asset.ID, ArchiveChain: &chain}
		atom := storage.NewContentAtom(
			ComputeAtomID(asset.ID, storage.AtomText, seqIdx),
			asset.ID, storage.AtomText, seqIdx, anchor.ToJSON(),
		)
		atom.PayloadText = &text
		atoms = append(atoms, atom)
		seqIdx++
	}

	return atoms, nil
}

func isTextLike(ext string) bool {
	textExts := map[string]bool{
		".txt": true, ".md": true, ".markdown": true,
		".html": true, ".htm": true, ".rtf": true,
		".csv": true, ".json": true, ".xml": true, ".yaml": true, ".yml": true,
		".py": true, ".go": true, ".js": true, ".ts": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".rs": true, ".rb": true,
		".sh": true, ".bash": true, ".log": true, ".conf": true, ".cfg": true,
		".ini": true, ".toml": true, ".tex": true, ".bib": true,
	}
	return textExts[ext]
}
