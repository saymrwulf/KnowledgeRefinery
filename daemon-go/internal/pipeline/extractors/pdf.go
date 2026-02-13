package extractors

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// PDFExtractor uses pdftotext (poppler-utils) to extract text from PDFs.
type PDFExtractor struct{}

func (e *PDFExtractor) Name() string     { return "pdf" }
func (e *PDFExtractor) Priority() int    { return 20 }

func (e *PDFExtractor) CanHandle(asset storage.FileAsset) bool {
	return strings.ToLower(filepath.Ext(asset.Filename)) == ".pdf"
}

func (e *PDFExtractor) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	text, err := extractWithPdftotext(asset.Path)
	if err != nil {
		slog.Warn("pdftotext failed, trying textutil fallback", "error", err)
		text, err = extractWithTextutil(asset.Path)
		if err != nil {
			return nil, fmt.Errorf("pdf extraction failed: %w", err)
		}
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}

	anchor := storage.EvidenceAnchor{AssetID: asset.ID}
	atom := storage.NewContentAtom(
		ComputeAtomID(asset.ID, storage.AtomText, 0),
		asset.ID, storage.AtomText, 0, anchor.ToJSON(),
	)
	atom.PayloadText = &text
	return []storage.ContentAtom{atom}, nil
}

func extractWithPdftotext(path string) (string, error) {
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func extractWithTextutil(path string) (string, error) {
	cmd := exec.Command("textutil", "-convert", "txt", "-stdout", path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
