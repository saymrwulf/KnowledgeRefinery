package extractors

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// Extractor extracts content atoms from a file asset.
type Extractor interface {
	CanHandle(asset storage.FileAsset) bool
	Extract(asset storage.FileAsset) ([]storage.ContentAtom, error)
	Name() string
	Priority() int
}

// Registry holds extractors sorted by priority (highest first).
type Registry struct {
	extractors []Extractor
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(e Extractor) {
	r.extractors = append(r.extractors, e)
	sort.Slice(r.extractors, func(i, j int) bool {
		return r.extractors[i].Priority() > r.extractors[j].Priority()
	})
}

// Extract tries each extractor in priority order.
func (r *Registry) Extract(asset storage.FileAsset) ([]storage.ContentAtom, error) {
	for _, e := range r.extractors {
		if e.CanHandle(asset) {
			return e.Extract(asset)
		}
	}
	return nil, fmt.Errorf("no extractor can handle: %s", asset.Filename)
}

// ComputeAtomID generates a deterministic atom ID.
func ComputeAtomID(assetID string, atomType storage.AtomType, seqIdx int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", assetID, string(atomType), seqIdx)))
	return fmt.Sprintf("%x", h)[:32]
}

// CreateDefaultRegistry builds a registry with all extractors.
func CreateDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&PDFExtractor{})
	r.Register(&EPUBExtractor{})
	r.Register(&ImageExtractor{})
	r.Register(&DICOMExtractor{})
	r.Register(&TextExtractor{})
	r.Register(&ArchiveExtractor{})
	r.Register(&TikaFallbackExtractor{})
	return r
}
