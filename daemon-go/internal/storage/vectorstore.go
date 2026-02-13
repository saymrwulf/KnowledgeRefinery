package storage

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
)

// VectorRecord holds a chunk's embedding and metadata.
type VectorRecord struct {
	ID             string    `json:"id"`
	Vector         []float32 `json:"-"`
	Text           string    `json:"text"`
	AssetID        string    `json:"asset_id"`
	AssetPath      string    `json:"asset_path"`
	EvidenceAnchor string    `json:"evidence_anchor"`
	Topics         string    `json:"topics"`
	AtomType       string    `json:"atom_type"`
	PipelineVersion string  `json:"pipeline_version"`
}

// SearchResult is a VectorRecord with a distance score.
type SearchResult struct {
	VectorRecord
	Distance float64 `json:"_distance"`
}

// VectorStore stores and searches embeddings using SQLite + in-memory index.
type VectorStore struct {
	db        *sql.DB
	dimension int
	mu        sync.RWMutex
	cache     []cachedVec // in-memory normalized vectors for fast search
	loaded    bool
}

type cachedVec struct {
	id     string
	vector []float32 // pre-normalized
	rec    VectorRecord
}

const vectorTableDDL = `
CREATE TABLE IF NOT EXISTS chunk_vectors (
    id TEXT PRIMARY KEY,
    vector BLOB NOT NULL,
    text TEXT NOT NULL,
    asset_id TEXT NOT NULL,
    asset_path TEXT NOT NULL,
    evidence_anchor TEXT,
    topics TEXT,
    atom_type TEXT,
    pipeline_version TEXT
);
CREATE INDEX IF NOT EXISTS idx_chunk_vectors_asset ON chunk_vectors(asset_id);
`

func NewVectorStore(db *sql.DB, dimension int) (*VectorStore, error) {
	if _, err := db.Exec(vectorTableDDL); err != nil {
		return nil, fmt.Errorf("create vector table: %w", err)
	}
	vs := &VectorStore{db: db, dimension: dimension}
	return vs, nil
}

func (vs *VectorStore) SetDimension(dim int) {
	vs.mu.Lock()
	vs.dimension = dim
	vs.mu.Unlock()
}

func (vs *VectorStore) Dimension() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.dimension
}

// LoadAll loads all vectors from SQLite into memory for fast search.
func (vs *VectorStore) LoadAll() error {
	rows, err := vs.db.Query("SELECT id, vector, text, asset_id, asset_path, evidence_anchor, topics, atom_type, pipeline_version FROM chunk_vectors")
	if err != nil {
		return err
	}
	defer rows.Close()

	var cache []cachedVec
	for rows.Next() {
		var rec VectorRecord
		var vecBlob []byte
		err := rows.Scan(&rec.ID, &vecBlob, &rec.Text, &rec.AssetID, &rec.AssetPath,
			&rec.EvidenceAnchor, &rec.Topics, &rec.AtomType, &rec.PipelineVersion)
		if err != nil {
			return err
		}
		vec := blobToFloat32(vecBlob)
		rec.Vector = vec
		normalized := normalize(vec)
		cache = append(cache, cachedVec{id: rec.ID, vector: normalized, rec: rec})
	}

	vs.mu.Lock()
	vs.cache = cache
	vs.loaded = true
	if len(cache) > 0 {
		vs.dimension = len(cache[0].vector)
	}
	vs.mu.Unlock()
	return rows.Err()
}

// AddVectors inserts vectors into SQLite and adds them to the in-memory cache.
func (vs *VectorStore) AddVectors(records []VectorRecord) error {
	tx, err := vs.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO chunk_vectors
		(id, vector, text, asset_id, asset_path, evidence_anchor, topics, atom_type, pipeline_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	vs.mu.Lock()
	defer vs.mu.Unlock()

	for _, rec := range records {
		blob := float32ToBlob(rec.Vector)
		_, err := stmt.Exec(rec.ID, blob, rec.Text, rec.AssetID, rec.AssetPath,
			rec.EvidenceAnchor, rec.Topics, rec.AtomType, rec.PipelineVersion)
		if err != nil {
			tx.Rollback()
			return err
		}
		// Add to cache
		normalized := normalize(rec.Vector)
		vs.cache = append(vs.cache, cachedVec{id: rec.ID, vector: normalized, rec: rec})
	}
	return tx.Commit()
}

// Search finds the k nearest neighbors using brute-force cosine similarity.
func (vs *VectorStore) Search(queryVec []float32, limit int) []SearchResult {
	normalized := normalize(queryVec)

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	type scored struct {
		idx  int
		dist float64
	}
	scores := make([]scored, 0, len(vs.cache))
	for i, cv := range vs.cache {
		sim := dotProduct(normalized, cv.vector)
		// Convert cosine similarity to distance (lower = more similar, matching LanceDB behavior)
		dist := 1.0 - float64(sim)
		scores = append(scores, scored{idx: i, dist: dist})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].dist < scores[j].dist
	})

	n := limit
	if n > len(scores) {
		n = len(scores)
	}

	results := make([]SearchResult, n)
	for i := 0; i < n; i++ {
		cv := vs.cache[scores[i].idx]
		results[i] = SearchResult{
			VectorRecord: cv.rec,
			Distance:     scores[i].dist,
		}
	}
	return results
}

// DeleteByAsset removes all vectors for an asset.
func (vs *VectorStore) DeleteByAsset(assetID string) error {
	_, err := vs.db.Exec("DELETE FROM chunk_vectors WHERE asset_id=?", assetID)
	if err != nil {
		return err
	}
	// Remove from cache
	vs.mu.Lock()
	filtered := vs.cache[:0]
	for _, cv := range vs.cache {
		if cv.rec.AssetID != assetID {
			filtered = append(filtered, cv)
		}
	}
	vs.cache = filtered
	vs.mu.Unlock()
	return nil
}

// Count returns the number of vectors.
func (vs *VectorStore) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.cache)
}

// GetAllVectors returns all cached vectors (ids, vectors, texts).
func (vs *VectorStore) GetAllVectors() (ids []string, vectors [][]float32, texts []string) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	ids = make([]string, len(vs.cache))
	vectors = make([][]float32, len(vs.cache))
	texts = make([]string, len(vs.cache))
	for i, cv := range vs.cache {
		ids[i] = cv.id
		vectors[i] = cv.rec.Vector // original (unnormalized)
		texts[i] = cv.rec.Text
	}
	return
}

// -- Vector math helpers --

func float32ToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func blobToFloat32(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func normalize(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		out := make([]float32, len(v))
		return out
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

func dotProduct(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
