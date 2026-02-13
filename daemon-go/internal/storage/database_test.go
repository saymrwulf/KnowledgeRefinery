package storage

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if err := db.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestFileAssetCRUD(t *testing.T) {
	db := newTestDB(t)

	a := NewFileAsset("abc123", "/tmp/test.txt", "test.txt")
	mime := "text/plain"
	a.MimeType = &mime
	a.SizeBytes = 1024
	a.MtimeNs = 123456789

	if err := db.UpsertFileAsset(a); err != nil {
		t.Fatalf("UpsertFileAsset: %v", err)
	}

	got, err := db.GetFileAsset("abc123")
	if err != nil {
		t.Fatalf("GetFileAsset: %v", err)
	}
	if got == nil {
		t.Fatal("expected asset, got nil")
	}
	if got.Filename != "test.txt" {
		t.Errorf("expected test.txt, got %s", got.Filename)
	}
	if *got.MimeType != "text/plain" {
		t.Errorf("expected text/plain, got %s", *got.MimeType)
	}

	// Get by path
	got2, err := db.GetFileAssetByPath("/tmp/test.txt")
	if err != nil {
		t.Fatalf("GetFileAssetByPath: %v", err)
	}
	if got2 == nil || got2.ID != "abc123" {
		t.Error("GetFileAssetByPath failed")
	}

	// Update status
	errMsg := "test error"
	if err := db.UpdateAssetStatus("abc123", StatusError, &errMsg); err != nil {
		t.Fatalf("UpdateAssetStatus: %v", err)
	}
	got3, _ := db.GetFileAsset("abc123")
	if got3.Status != StatusError {
		t.Errorf("expected error status, got %s", got3.Status)
	}

	// Count by status
	counts, err := db.CountAssetsByStatus()
	if err != nil {
		t.Fatalf("CountAssetsByStatus: %v", err)
	}
	if counts["error"] != 1 {
		t.Errorf("expected 1 error, got %d", counts["error"])
	}

	// Get by status
	assets, err := db.GetAssetsByStatus(StatusError, 100)
	if err != nil {
		t.Fatalf("GetAssetsByStatus: %v", err)
	}
	if len(assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(assets))
	}

	// GetAllAssets
	all, err := db.GetAllAssets()
	if err != nil {
		t.Fatalf("GetAllAssets: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 asset, got %d", len(all))
	}
}

func TestContentAtomCRUD(t *testing.T) {
	db := newTestDB(t)

	// Insert asset first (foreign key)
	a := NewFileAsset("asset1", "/tmp/test.txt", "test.txt")
	db.UpsertFileAsset(a)

	atom := NewContentAtom("atom1", "asset1", AtomText, 0, `{"asset_id":"asset1"}`)
	text := "Hello world"
	atom.PayloadText = &text

	if err := db.InsertContentAtom(atom); err != nil {
		t.Fatalf("InsertContentAtom: %v", err)
	}

	atoms, err := db.GetAtomsForAsset("asset1")
	if err != nil {
		t.Fatalf("GetAtomsForAsset: %v", err)
	}
	if len(atoms) != 1 {
		t.Errorf("expected 1 atom, got %d", len(atoms))
	}
	if *atoms[0].PayloadText != "Hello world" {
		t.Errorf("expected Hello world, got %s", *atoms[0].PayloadText)
	}

	// Batch insert
	atom2 := NewContentAtom("atom2", "asset1", AtomText, 1, `{"asset_id":"asset1"}`)
	text2 := "Goodbye"
	atom2.PayloadText = &text2
	if err := db.InsertContentAtoms([]ContentAtom{atom2}); err != nil {
		t.Fatalf("InsertContentAtoms: %v", err)
	}

	atoms2, _ := db.GetAtomsForAsset("asset1")
	if len(atoms2) != 2 {
		t.Errorf("expected 2 atoms, got %d", len(atoms2))
	}

	// Delete
	if err := db.DeleteAtomsForAsset("asset1"); err != nil {
		t.Fatalf("DeleteAtomsForAsset: %v", err)
	}
	atoms3, _ := db.GetAtomsForAsset("asset1")
	if len(atoms3) != 0 {
		t.Errorf("expected 0 atoms after delete, got %d", len(atoms3))
	}
}

func TestChunkCRUD(t *testing.T) {
	db := newTestDB(t)

	a := NewFileAsset("asset1", "/tmp/test.txt", "test.txt")
	db.UpsertFileAsset(a)

	// Insert atom (FK parent for chunks)
	atom := NewContentAtom("atom1", "asset1", AtomText, 0, `{"asset_id":"asset1"}`)
	text := "Hello world"
	atom.PayloadText = &text
	db.InsertContentAtom(atom)

	c := NewChunk("chunk1", "atom1", "asset1", "Hello world", 2, 0, `{"asset_id":"asset1"}`, "v1.0")
	if err := db.InsertChunk(c); err != nil {
		t.Fatalf("InsertChunk: %v", err)
	}

	got, err := db.GetChunk("chunk1")
	if err != nil {
		t.Fatalf("GetChunk: %v", err)
	}
	if got == nil || got.ChunkText != "Hello world" {
		t.Error("GetChunk returned wrong data")
	}

	// Chunks without embeddings
	unembedded, err := db.GetChunksWithoutEmbeddings(100)
	if err != nil {
		t.Fatalf("GetChunksWithoutEmbeddings: %v", err)
	}
	if len(unembedded) != 1 {
		t.Errorf("expected 1 unembedded, got %d", len(unembedded))
	}

	// Update embedding
	if err := db.UpdateChunkEmbedding("chunk1", "chunk1"); err != nil {
		t.Fatalf("UpdateChunkEmbedding: %v", err)
	}
	unembedded2, _ := db.GetChunksWithoutEmbeddings(100)
	if len(unembedded2) != 0 {
		t.Errorf("expected 0 unembedded after update, got %d", len(unembedded2))
	}

	// Count
	cnt, err := db.CountChunks()
	if err != nil {
		t.Fatalf("CountChunks: %v", err)
	}
	if cnt != 1 {
		t.Errorf("expected 1 chunk, got %d", cnt)
	}

	// Batch insert
	c2 := NewChunk("chunk2", "atom1", "asset1", "Goodbye", 1, 1, `{"asset_id":"asset1"}`, "v1.0")
	if err := db.InsertChunks([]Chunk{c2}); err != nil {
		t.Fatalf("InsertChunks: %v", err)
	}

	chunks, _ := db.GetChunksForAsset("asset1")
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestAnnotationCRUD(t *testing.T) {
	db := newTestDB(t)

	// Create parent records for FK constraints
	a := NewFileAsset("asset1", "/tmp/test.txt", "test.txt")
	db.UpsertFileAsset(a)
	atom := NewContentAtom("atom1", "asset1", AtomText, 0, `{"asset_id":"asset1"}`)
	db.InsertContentAtom(atom)
	c := NewChunk("chunk1", "atom1", "asset1", "text", 1, 0, `{"asset_id":"asset1"}`, "v1.0")
	db.InsertChunk(c)

	ann := Annotation{
		ID:              "ann1",
		ChunkID:         "chunk1",
		ModelID:         "model1",
		PromptID:        "prompt1",
		PromptVersion:   "1.0",
		PipelineVersion: "v1.0",
		IsCurrent:       1,
		CreatedAt:       nowISO(),
	}

	if err := db.InsertAnnotation(ann); err != nil {
		t.Fatalf("InsertAnnotation: %v", err)
	}

	got, err := db.GetCurrentAnnotation("chunk1")
	if err != nil {
		t.Fatalf("GetCurrentAnnotation: %v", err)
	}
	if got == nil || got.ID != "ann1" {
		t.Error("GetCurrentAnnotation failed")
	}

	// Insert newer annotation — should supersede
	ann2 := ann
	ann2.ID = "ann2"
	if err := db.InsertAnnotation(ann2); err != nil {
		t.Fatalf("InsertAnnotation 2: %v", err)
	}
	got2, _ := db.GetCurrentAnnotation("chunk1")
	if got2.ID != "ann2" {
		t.Errorf("expected ann2 as current, got %s", got2.ID)
	}

	cnt, _ := db.CountAnnotations()
	if cnt != 1 {
		t.Errorf("expected 1 current annotation, got %d", cnt)
	}
}

func TestConceptNodeCRUD(t *testing.T) {
	db := newTestDB(t)

	node := ConceptNode{
		ID:        "concept1",
		Level:     0,
		CreatedAt: nowISO(),
	}
	label := "Test Concept"
	node.Label = &label

	if err := db.InsertConceptNode(node); err != nil {
		t.Fatalf("InsertConceptNode: %v", err)
	}

	level0 := 0
	nodes, err := db.GetConceptNodes(&level0)
	if err != nil {
		t.Fatalf("GetConceptNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}

	cnt, _ := db.CountConcepts()
	if cnt != 1 {
		t.Errorf("expected 1 concept, got %d", cnt)
	}
}

func TestGraphEdgeCRUD(t *testing.T) {
	db := newTestDB(t)

	edge := GraphEdge{
		ID:        "edge1",
		SourceID:  "a",
		TargetID:  "b",
		EdgeType:  "similarity",
		Weight:    0.95,
		CreatedAt: nowISO(),
	}
	if err := db.InsertGraphEdge(edge); err != nil {
		t.Fatalf("InsertGraphEdge: %v", err)
	}

	cnt, _ := db.CountEdges()
	if cnt != 1 {
		t.Errorf("expected 1 edge, got %d", cnt)
	}

	edges, _ := db.GetGraphEdges("weight DESC", 10)
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}

	nodeEdges, _ := db.GetEdgesForNode("a", 10)
	if len(nodeEdges) != 1 {
		t.Errorf("expected 1 edge for node a, got %d", len(nodeEdges))
	}
}

func TestPipelineJobCRUD(t *testing.T) {
	db := newTestDB(t)

	job := PipelineJob{
		ID:        "job1",
		JobType:   "full_ingest",
		Status:    JobRunning,
		CreatedAt: nowISO(),
		UpdatedAt: nowISO(),
	}
	if err := db.UpsertPipelineJob(job); err != nil {
		t.Fatalf("UpsertPipelineJob: %v", err)
	}

	jt := "full_ingest"
	got, err := db.GetLatestJob(&jt)
	if err != nil {
		t.Fatalf("GetLatestJob: %v", err)
	}
	if got == nil || got.ID != "job1" {
		t.Error("GetLatestJob failed")
	}

	progress := `{"stage":"scanning"}`
	if err := db.UpdateJobStatus("job1", JobCompleted, &progress); err != nil {
		t.Fatalf("UpdateJobStatus: %v", err)
	}

	got2, _ := db.GetLatestJob(&jt)
	if got2.Status != JobCompleted {
		t.Errorf("expected completed, got %s", got2.Status)
	}
}

func TestWatchedVolumeCRUD(t *testing.T) {
	db := newTestDB(t)

	label := "My Docs"
	vol := NewWatchedVolume("vol1", "/tmp/docs", &label)
	if err := db.AddWatchedVolume(vol); err != nil {
		t.Fatalf("AddWatchedVolume: %v", err)
	}

	vols, err := db.GetWatchedVolumes()
	if err != nil {
		t.Fatalf("GetWatchedVolumes: %v", err)
	}
	if len(vols) != 1 {
		t.Errorf("expected 1 volume, got %d", len(vols))
	}
	if vols[0].Path != "/tmp/docs" {
		t.Errorf("expected /tmp/docs, got %s", vols[0].Path)
	}

	// Update scan time
	if err := db.UpdateVolumeScanTime("vol1"); err != nil {
		t.Fatalf("UpdateVolumeScanTime: %v", err)
	}

	// Remove
	if err := db.RemoveWatchedVolume("/tmp/docs"); err != nil {
		t.Fatalf("RemoveWatchedVolume: %v", err)
	}
	vols2, _ := db.GetWatchedVolumes()
	if len(vols2) != 0 {
		t.Errorf("expected 0 volumes after remove, got %d", len(vols2))
	}
}
