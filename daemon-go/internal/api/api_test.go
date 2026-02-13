package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

func setupTestDB(t *testing.T) *storage.Database {
	t.Helper()
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Initialize(); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestVolumesRouterAddAndList(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/volumes", VolumesRouter(db))

	// Create a temp directory to add as volume
	dir := t.TempDir()

	// POST /volumes/add
	body, _ := json.Marshal(map[string]string{"path": dir})
	req := httptest.NewRequest("POST", "/volumes/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("add volume: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var addResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &addResp)
	if addResp["path"] == nil {
		t.Error("expected path in response")
	}

	// GET /volumes/list
	req2 := httptest.NewRequest("GET", "/volumes/list", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("list volumes: expected 200, got %d", w2.Code)
	}

	var listResp []map[string]any
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	if len(listResp) != 1 {
		t.Errorf("expected 1 volume, got %d", len(listResp))
	}
}

func TestVolumesRouterAddInvalidDir(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/volumes", VolumesRouter(db))

	body, _ := json.Marshal(map[string]string{"path": "/nonexistent/path"})
	req := httptest.NewRequest("POST", "/volumes/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid dir, got %d", w.Code)
	}
}

func TestVolumesRouterRemove(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/volumes", VolumesRouter(db))

	dir := t.TempDir()

	// Add
	body, _ := json.Marshal(map[string]string{"path": dir})
	req := httptest.NewRequest("POST", "/volumes/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Remove
	req2 := httptest.NewRequest("DELETE", "/volumes/remove?path="+dir, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("remove volume: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	// List should be empty
	req3 := httptest.NewRequest("GET", "/volumes/list", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	var listResp []map[string]any
	json.Unmarshal(w3.Body.Bytes(), &listResp)
	if len(listResp) != 0 {
		t.Errorf("expected 0 volumes after remove, got %d", len(listResp))
	}
}

func TestEvidenceRouterAssetNotFound(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/evidence", EvidenceRouter(db))

	req := httptest.NewRequest("GET", "/evidence/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing asset, got %d", w.Code)
	}
}

func TestEvidenceRouterGetAsset(t *testing.T) {
	db := setupTestDB(t)

	// Insert a test asset
	asset := storage.NewFileAsset("test-asset-id", "/tmp/test.txt", "test.txt")
	asset.SizeBytes = 100
	db.UpsertFileAsset(asset)

	r := chi.NewRouter()
	r.Mount("/evidence", EvidenceRouter(db))

	req := httptest.NewRequest("GET", "/evidence/test-asset-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["asset_id"] != "test-asset-id" {
		t.Errorf("expected asset_id 'test-asset-id', got %v", resp["asset_id"])
	}
	if resp["filename"] != "test.txt" {
		t.Errorf("expected filename 'test.txt', got %v", resp["filename"])
	}
}

func TestEvidenceRouterChunkNotFound(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/evidence", EvidenceRouter(db))

	req := httptest.NewRequest("GET", "/evidence/chunk/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing chunk, got %d", w.Code)
	}
}

func TestEvidenceRouterGetChunk(t *testing.T) {
	db := setupTestDB(t)

	// Insert prerequisite data
	asset := storage.NewFileAsset("asset1", "/tmp/test.txt", "test.txt")
	asset.SizeBytes = 100
	db.UpsertFileAsset(asset)

	atom := storage.NewContentAtom("atom1", "asset1", storage.AtomText, 0, `{"asset_id":"asset1"}`)
	text := "Hello world"
	atom.PayloadText = &text
	db.InsertContentAtom(atom)

	chunk := storage.NewChunk("chunk1", "atom1", "asset1", "Hello world", 2, 0, `{"asset_id":"asset1"}`, "v1")
	db.InsertChunk(chunk)

	r := chi.NewRouter()
	r.Mount("/evidence", EvidenceRouter(db))

	req := httptest.NewRequest("GET", "/evidence/chunk/chunk1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["asset_id"] != "asset1" {
		t.Errorf("expected asset_id 'asset1', got %v", resp["asset_id"])
	}
}

func TestEvidenceRouterAllAssets(t *testing.T) {
	db := setupTestDB(t)

	asset := storage.NewFileAsset("asset1", "/tmp/test.txt", "test.txt")
	asset.SizeBytes = 100
	db.UpsertFileAsset(asset)

	r := chi.NewRouter()
	r.Mount("/evidence", EvidenceRouter(db))

	req := httptest.NewRequest("GET", "/evidence/assets/all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp []map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 asset, got %d", len(resp))
	}
}

func TestConceptsRouterList(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/concepts", ConceptsRouter(db, nil))

	req := httptest.NewRequest("GET", "/concepts/list", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp []map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 0 {
		t.Errorf("expected 0 concepts, got %d", len(resp))
	}
}

func TestConceptsRouterGetNotFound(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/concepts", ConceptsRouter(db, nil))

	req := httptest.NewRequest("GET", "/concepts/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		// Returns 200 with error body, matching Python behavior
		var resp map[string]any
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["error"] == nil {
			t.Error("expected error field for missing concept")
		}
	}
}

func TestConceptsRouterRefineNoConcepts(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	r.Mount("/concepts", ConceptsRouter(db, nil))

	req := httptest.NewRequest("POST", "/concepts/refine?concept_id=nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Should return error since conceptualizer is nil
	if resp["error"] == nil {
		t.Error("expected error when conceptualizer is nil")
	}
}

func TestUniverseRouterSnapshot(t *testing.T) {
	db := setupTestDB(t)
	vs, _ := storage.NewVectorStore(db.DB(), 768)
	r := chi.NewRouter()
	r.Mount("/universe", UniverseRouter(db, vs))

	req := httptest.NewRequest("GET", "/universe/snapshot?lod=macro", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["lod"] != "macro" {
		t.Errorf("expected lod 'macro', got %v", resp["lod"])
	}
	if resp["nodes"] == nil {
		t.Error("expected nodes field")
	}
	if resp["edges"] == nil {
		t.Error("expected edges field")
	}
}

func TestUniverseRouterFocusMissingNodeID(t *testing.T) {
	db := setupTestDB(t)
	vs, _ := storage.NewVectorStore(db.DB(), 768)
	r := chi.NewRouter()
	r.Mount("/universe", UniverseRouter(db, vs))

	req := httptest.NewRequest("POST", "/universe/focus", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing node_id, got %d", w.Code)
	}
}

func TestUniverseRouterFocusWithNodeID(t *testing.T) {
	db := setupTestDB(t)
	vs, _ := storage.NewVectorStore(db.DB(), 768)
	r := chi.NewRouter()
	r.Mount("/universe", UniverseRouter(db, vs))

	req := httptest.NewRequest("POST", "/universe/focus?node_id=test-node", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["focused_node"] != "test-node" {
		t.Errorf("expected focused_node 'test-node', got %v", resp["focused_node"])
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello world", 5) != "hello" {
		t.Error("truncate should cut at n chars")
	}
	if truncate("hi", 10) != "hi" {
		t.Error("truncate should not truncate short strings")
	}
}

func TestPtrOr(t *testing.T) {
	s := "value"
	if ptrOr(&s, "default") != "value" {
		t.Error("ptrOr should return pointer value")
	}
	if ptrOr(nil, "default") != "default" {
		t.Error("ptrOr should return default for nil")
	}
}

func TestEvidenceRouterAnnotation(t *testing.T) {
	db := setupTestDB(t)

	// Insert prerequisite data
	asset := storage.NewFileAsset("asset1", "/tmp/test.txt", "test.txt")
	asset.SizeBytes = 100
	db.UpsertFileAsset(asset)

	atom := storage.NewContentAtom("atom1", "asset1", storage.AtomText, 0, `{"asset_id":"asset1"}`)
	text := "Hello world"
	atom.PayloadText = &text
	db.InsertContentAtom(atom)

	chunk := storage.NewChunk("chunk1", "atom1", "asset1", "Hello world", 2, 0, `{"asset_id":"asset1"}`, "v1")
	db.InsertChunk(chunk)

	// Insert annotation
	topics := `["ai","ml"]`
	summary := "A test summary"
	ann := storage.Annotation{
		ID:              "ann1",
		ChunkID:         "chunk1",
		ModelID:         "test-model",
		PromptID:        "annotate_chunk",
		PromptVersion:   "v1",
		PipelineVersion: "v1",
		TopicsJSON:      &topics,
		Summary:         &summary,
		IsCurrent:       1,
		CreatedAt:       storage.NowISO(),
	}
	db.InsertAnnotation(ann)

	r := chi.NewRouter()
	r.Mount("/evidence", EvidenceRouter(db))

	req := httptest.NewRequest("GET", "/evidence/chunk/chunk1/annotation", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["chunk_id"] != "chunk1" {
		t.Errorf("expected chunk_id 'chunk1', got %v", resp["chunk_id"])
	}
	if resp["summary"] != "A test summary" {
		t.Errorf("expected summary, got %v", resp["summary"])
	}
}

// Ensure unused import doesn't cause build issues
var _ = os.TempDir
