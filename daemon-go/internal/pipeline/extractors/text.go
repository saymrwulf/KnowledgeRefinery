package extractors

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true,
	".html": true, ".htm": true, ".rtf": true,
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

// TextExtractor handles plain text, markdown, HTML, and RTF files.
type TextExtractor struct{}

func (e *TextExtractor) Name() string     { return "text" }
func (e *TextExtractor) Priority() int    { return 10 }

func (e *TextExtractor) CanHandle(asset storage.FileAsset) bool {
	ext := strings.ToLower(filepath.Ext(asset.Filename))
	return textExtensions[ext]
}

func (e *TextExtractor) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	data, err := os.ReadFile(asset.Path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	ext := strings.ToLower(filepath.Ext(asset.Filename))

	switch ext {
	case ".html", ".htm":
		text = stripHTML(text)
	case ".rtf":
		text = stripRTF(text)
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

func stripHTML(html string) string {
	text := htmlTagRE.ReplaceAllString(html, " ")
	// Unescape common HTML entities
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	return text
}

var rtfControlRE = regexp.MustCompile(`\\[a-z]+\d*\s?`)

func stripRTF(rtf string) string {
	text := rtfControlRE.ReplaceAllString(rtf, " ")
	text = strings.ReplaceAll(text, "{", "")
	text = strings.ReplaceAll(text, "}", "")
	return text
}
