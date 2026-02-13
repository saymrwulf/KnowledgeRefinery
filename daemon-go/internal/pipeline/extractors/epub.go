package extractors

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// EPUBExtractor handles EPUB e-books via ZIP+OPF parsing.
type EPUBExtractor struct{}

func (e *EPUBExtractor) Name() string     { return "epub" }
func (e *EPUBExtractor) Priority() int    { return 18 }

func (e *EPUBExtractor) CanHandle(asset storage.FileAsset) bool {
	ext := strings.ToLower(filepath.Ext(asset.Filename))
	return ext == ".epub"
}

func (e *EPUBExtractor) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	r, err := zip.OpenReader(asset.Path)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer r.Close()

	// Find container.xml
	opfPath, err := findOPFPath(r)
	if err != nil {
		return nil, err
	}

	// Parse OPF for spine order
	spineItems, err := parseOPF(r, opfPath)
	if err != nil {
		return nil, err
	}

	opfDir := filepath.Dir(opfPath)
	var atoms []storage.ContentAtom
	seqIdx := 0

	for _, item := range spineItems {
		itemPath := item.href
		if opfDir != "." && opfDir != "" {
			itemPath = opfDir + "/" + item.href
		}

		f := findZipFile(r, itemPath)
		if f == nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		text := stripHTML(string(data))
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		chapter := item.id
		anchor := storage.EvidenceAnchor{AssetID: asset.ID, Chapter: &chapter}
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

type spineItem struct {
	id   string
	href string
}

// container.xml structure
type containerXML struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

func findOPFPath(r *zip.ReadCloser) (string, error) {
	f := findZipFile(r, "META-INF/container.xml")
	if f == nil {
		return "", fmt.Errorf("container.xml not found in EPUB")
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var container containerXML
	if err := xml.NewDecoder(rc).Decode(&container); err != nil {
		return "", fmt.Errorf("parse container.xml: %w", err)
	}
	if len(container.Rootfiles) == 0 {
		return "", fmt.Errorf("no rootfile in container.xml")
	}
	return container.Rootfiles[0].FullPath, nil
}

type opfPackage struct {
	Manifest struct {
		Items []struct {
			ID        string `xml:"id,attr"`
			Href      string `xml:"href,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Itemrefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

func parseOPF(r *zip.ReadCloser, opfPath string) ([]spineItem, error) {
	f := findZipFile(r, opfPath)
	if f == nil {
		return nil, fmt.Errorf("OPF file not found: %s", opfPath)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var pkg opfPackage
	if err := xml.NewDecoder(rc).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("parse OPF: %w", err)
	}

	// Build id -> href map
	idToHref := make(map[string]string)
	for _, item := range pkg.Manifest.Items {
		idToHref[item.ID] = item.Href
	}

	var items []spineItem
	for _, ref := range pkg.Spine.Itemrefs {
		href, ok := idToHref[ref.IDRef]
		if ok {
			items = append(items, spineItem{id: ref.IDRef, href: href})
		}
	}
	return items, nil
}

func findZipFile(r *zip.ReadCloser, name string) *zip.File {
	// Normalize separators for comparison
	name = strings.ReplaceAll(name, "\\", "/")
	for _, f := range r.File {
		if strings.ReplaceAll(f.Name, "\\", "/") == name {
			return f
		}
	}
	return nil
}

// xmlTagRE matches XML/HTML tags — reuse stripHTML from text.go
var xmlTagRE = regexp.MustCompile(`<[^>]+>`)
