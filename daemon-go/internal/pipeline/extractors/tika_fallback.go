package extractors

import (
	"log/slog"
	"os"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// TikaFallbackExtractor is a last-resort extractor using macOS textutil or raw text.
type TikaFallbackExtractor struct{}

func (e *TikaFallbackExtractor) Name() string     { return "tika_fallback" }
func (e *TikaFallbackExtractor) Priority() int    { return 1 }

func (e *TikaFallbackExtractor) CanHandle(asset storage.FileAsset) bool {
	return true // fallback handles everything
}

func (e *TikaFallbackExtractor) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	// Try textutil first (macOS built-in)
	text, err := extractWithTextutil(asset.Path)
	if err != nil || strings.TrimSpace(text) == "" {
		slog.Debug("textutil failed, trying raw read", "file", asset.Filename)
		// Try raw text read
		data, err := os.ReadFile(asset.Path)
		if err != nil {
			return nil, err
		}
		text = string(data)
		// Check if it looks like text (not binary)
		if !isLikelyText(text) {
			return nil, nil
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

// isLikelyText checks if content looks like text (low ratio of null/control bytes).
func isLikelyText(s string) bool {
	if len(s) == 0 {
		return false
	}
	controlCount := 0
	checkLen := len(s)
	if checkLen > 1024 {
		checkLen = 1024
	}
	for i := 0; i < checkLen; i++ {
		b := s[i]
		if b == 0 || (b < 32 && b != '\n' && b != '\r' && b != '\t') {
			controlCount++
		}
	}
	return float64(controlCount)/float64(checkLen) < 0.1
}
