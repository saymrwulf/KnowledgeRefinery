package storage

import (
	"math"
	"path/filepath"
	"testing"
)

func newTestVectorStore(t *testing.T) *VectorStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	vs, err := NewVectorStore(db.DB(), 3)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	return vs
}

func TestAddAndSearch(t *testing.T) {
	vs := newTestVectorStore(t)

	records := []VectorRecord{
		{ID: "v1", Vector: []float32{1, 0, 0}, Text: "alpha", AssetID: "a1", AssetPath: "/a", AtomType: "text"},
		{ID: "v2", Vector: []float32{0, 1, 0}, Text: "beta", AssetID: "a1", AssetPath: "/a", AtomType: "text"},
		{ID: "v3", Vector: []float32{0, 0, 1}, Text: "gamma", AssetID: "a2", AssetPath: "/b", AtomType: "text"},
	}

	if err := vs.AddVectors(records); err != nil {
		t.Fatalf("AddVectors: %v", err)
	}

	if vs.Count() != 3 {
		t.Errorf("expected 3 vectors, got %d", vs.Count())
	}

	// Search for something close to [1, 0, 0]
	results := vs.Search([]float32{0.9, 0.1, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "v1" {
		t.Errorf("expected v1 as top result, got %s", results[0].ID)
	}
}

func TestDeleteByAsset(t *testing.T) {
	vs := newTestVectorStore(t)

	records := []VectorRecord{
		{ID: "v1", Vector: []float32{1, 0, 0}, Text: "alpha", AssetID: "a1", AssetPath: "/a", AtomType: "text"},
		{ID: "v2", Vector: []float32{0, 1, 0}, Text: "beta", AssetID: "a2", AssetPath: "/b", AtomType: "text"},
	}
	vs.AddVectors(records)

	if err := vs.DeleteByAsset("a1"); err != nil {
		t.Fatalf("DeleteByAsset: %v", err)
	}
	if vs.Count() != 1 {
		t.Errorf("expected 1 vector after delete, got %d", vs.Count())
	}
}

func TestLoadAll(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	vs1, _ := NewVectorStore(db.DB(), 3)
	vs1.AddVectors([]VectorRecord{
		{ID: "v1", Vector: []float32{1, 0, 0}, Text: "alpha", AssetID: "a1", AssetPath: "/a", AtomType: "text"},
	})

	// Create new vector store from same DB and load
	vs2, _ := NewVectorStore(db.DB(), 3)
	if err := vs2.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if vs2.Count() != 1 {
		t.Errorf("expected 1 vector after LoadAll, got %d", vs2.Count())
	}
}

func TestGetAllVectors(t *testing.T) {
	vs := newTestVectorStore(t)

	records := []VectorRecord{
		{ID: "v1", Vector: []float32{1, 0, 0}, Text: "alpha", AssetID: "a1", AssetPath: "/a", AtomType: "text"},
		{ID: "v2", Vector: []float32{0, 1, 0}, Text: "beta", AssetID: "a2", AssetPath: "/b", AtomType: "text"},
	}
	vs.AddVectors(records)

	ids, vectors, texts := vs.GetAllVectors()
	if len(ids) != 2 {
		t.Errorf("expected 2 ids, got %d", len(ids))
	}
	if len(vectors) != 2 {
		t.Errorf("expected 2 vectors, got %d", len(vectors))
	}
	if len(texts) != 2 {
		t.Errorf("expected 2 texts, got %d", len(texts))
	}
}

func TestNormalize(t *testing.T) {
	v := []float32{3, 4, 0}
	n := normalize(v)
	expectedNorm := float64(1.0)
	var gotNorm float64
	for _, x := range n {
		gotNorm += float64(x) * float64(x)
	}
	gotNorm = math.Sqrt(gotNorm)
	if math.Abs(gotNorm-expectedNorm) > 0.001 {
		t.Errorf("expected norm 1.0, got %f", gotNorm)
	}
}

func TestBlobRoundTrip(t *testing.T) {
	original := []float32{1.5, -2.3, 0.0, 99.99}
	blob := float32ToBlob(original)
	restored := blobToFloat32(blob)

	if len(restored) != len(original) {
		t.Fatalf("expected %d floats, got %d", len(original), len(restored))
	}
	for i := range original {
		if original[i] != restored[i] {
			t.Errorf("index %d: expected %f, got %f", i, original[i], restored[i])
		}
	}
}
