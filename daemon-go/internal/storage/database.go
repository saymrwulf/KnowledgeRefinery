package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS file_assets (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL,
    filename TEXT NOT NULL,
    uti TEXT,
    mime_type TEXT,
    size_bytes INTEGER,
    mtime_ns INTEGER,
    content_hash TEXT,
    scan_version INTEGER DEFAULT 1,
    status TEXT DEFAULT 'pending',
    error_message TEXT,
    created_at TEXT,
    updated_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_file_assets_path ON file_assets(path);
CREATE INDEX IF NOT EXISTS idx_file_assets_status ON file_assets(status);
CREATE INDEX IF NOT EXISTS idx_file_assets_content_hash ON file_assets(content_hash);

CREATE TABLE IF NOT EXISTS content_atoms (
    id TEXT PRIMARY KEY,
    asset_id TEXT REFERENCES file_assets(id),
    atom_type TEXT NOT NULL,
    sequence_index INTEGER,
    payload_text TEXT,
    payload_ref TEXT,
    metadata_json TEXT,
    evidence_anchor TEXT NOT NULL,
    created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_content_atoms_asset ON content_atoms(asset_id);

CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    atom_id TEXT REFERENCES content_atoms(id),
    asset_id TEXT REFERENCES file_assets(id),
    chunk_text TEXT NOT NULL,
    token_count INTEGER,
    chunk_index INTEGER,
    evidence_anchor TEXT NOT NULL,
    embedding_id TEXT,
    pipeline_version TEXT,
    created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_chunks_asset ON chunks(asset_id);
CREATE INDEX IF NOT EXISTS idx_chunks_atom ON chunks(atom_id);

CREATE TABLE IF NOT EXISTS annotations (
    id TEXT PRIMARY KEY,
    chunk_id TEXT REFERENCES chunks(id),
    model_id TEXT NOT NULL,
    prompt_id TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    pipeline_version TEXT NOT NULL,
    topics_json TEXT,
    sentiment_label TEXT,
    sentiment_confidence REAL,
    entities_json TEXT,
    claims_json TEXT,
    summary TEXT,
    quality_flags_json TEXT,
    is_current INTEGER DEFAULT 1,
    created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_annotations_chunk ON annotations(chunk_id);
CREATE INDEX IF NOT EXISTS idx_annotations_current ON annotations(is_current);

CREATE TABLE IF NOT EXISTS concept_nodes (
    id TEXT PRIMARY KEY,
    level INTEGER NOT NULL,
    label TEXT,
    description TEXT,
    parent_id TEXT REFERENCES concept_nodes(id),
    exemplar_chunk_ids TEXT,
    pipeline_version TEXT,
    model_id TEXT,
    created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_concept_nodes_level ON concept_nodes(level);

CREATE TABLE IF NOT EXISTS graph_edges (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    edge_type TEXT NOT NULL,
    weight REAL,
    evidence_json TEXT,
    pipeline_version TEXT,
    created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_graph_edges_source ON graph_edges(source_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_target ON graph_edges(target_id);

CREATE TABLE IF NOT EXISTS pipeline_jobs (
    id TEXT PRIMARY KEY,
    job_type TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    progress_json TEXT,
    created_at TEXT,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS watched_volumes (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    label TEXT,
    added_at TEXT,
    last_scan_at TEXT
);
`

// Database provides thread-safe SQLite operations.
type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite pragmas
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=10000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec %s: %w", pragma, err)
		}
	}
	return &Database{db: db}, nil
}

func (d *Database) Initialize() error {
	_, err := d.db.Exec(schemaDDL)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) DB() *sql.DB {
	return d.db
}

// -- FileAsset operations --

func (d *Database) UpsertFileAsset(a FileAsset) error {
	now := nowISO()
	_, err := d.db.Exec(`
		INSERT INTO file_assets (id, path, filename, uti, mime_type, size_bytes, mtime_ns,
			content_hash, scan_version, status, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path, filename=excluded.filename, uti=excluded.uti,
			mime_type=excluded.mime_type, size_bytes=excluded.size_bytes,
			mtime_ns=excluded.mtime_ns, content_hash=excluded.content_hash,
			scan_version=excluded.scan_version, status=excluded.status,
			error_message=excluded.error_message, updated_at=?`,
		a.ID, a.Path, a.Filename, a.UTI, a.MimeType, a.SizeBytes, a.MtimeNs,
		a.ContentHash, a.ScanVersion, string(a.Status), a.ErrorMessage, a.CreatedAt, now, now,
	)
	return err
}

func (d *Database) GetFileAsset(assetID string) (*FileAsset, error) {
	return d.scanFileAsset(d.db.QueryRow("SELECT * FROM file_assets WHERE id=?", assetID))
}

func (d *Database) GetFileAssetByPath(path string) (*FileAsset, error) {
	return d.scanFileAsset(d.db.QueryRow("SELECT * FROM file_assets WHERE path=?", path))
}

func (d *Database) scanFileAsset(row *sql.Row) (*FileAsset, error) {
	var a FileAsset
	var status string
	err := row.Scan(
		&a.ID, &a.Path, &a.Filename, &a.UTI, &a.MimeType, &a.SizeBytes, &a.MtimeNs,
		&a.ContentHash, &a.ScanVersion, &status, &a.ErrorMessage, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Status = AssetStatus(status)
	return &a, nil
}

func (d *Database) GetAssetsByStatus(status AssetStatus, limit int) ([]FileAsset, error) {
	rows, err := d.db.Query("SELECT * FROM file_assets WHERE status=? LIMIT ?", string(status), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanFileAssets(rows)
}

func (d *Database) GetAllAssets() ([]FileAsset, error) {
	rows, err := d.db.Query("SELECT * FROM file_assets")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanFileAssets(rows)
}

func (d *Database) scanFileAssets(rows *sql.Rows) ([]FileAsset, error) {
	var assets []FileAsset
	for rows.Next() {
		var a FileAsset
		var status string
		err := rows.Scan(
			&a.ID, &a.Path, &a.Filename, &a.UTI, &a.MimeType, &a.SizeBytes, &a.MtimeNs,
			&a.ContentHash, &a.ScanVersion, &status, &a.ErrorMessage, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		a.Status = AssetStatus(status)
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

func (d *Database) UpdateAssetStatus(assetID string, status AssetStatus, errMsg *string) error {
	now := nowISO()
	_, err := d.db.Exec(
		"UPDATE file_assets SET status=?, error_message=?, updated_at=? WHERE id=?",
		string(status), errMsg, now, assetID,
	)
	return err
}

func (d *Database) CountAssetsByStatus() (map[string]int, error) {
	rows, err := d.db.Query("SELECT status, COUNT(*) as cnt FROM file_assets GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int)
	for rows.Next() {
		var status string
		var cnt int
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, err
		}
		result[status] = cnt
	}
	return result, rows.Err()
}

// -- ContentAtom operations --

func (d *Database) InsertContentAtom(a ContentAtom) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO content_atoms
		(id, asset_id, atom_type, sequence_index, payload_text, payload_ref,
		 metadata_json, evidence_anchor, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.AssetID, string(a.AtomType), a.SequenceIndex,
		a.PayloadText, a.PayloadRef, a.MetadataJSON,
		a.EvidenceAnchor, a.CreatedAt,
	)
	return err
}

func (d *Database) InsertContentAtoms(atoms []ContentAtom) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO content_atoms
		(id, asset_id, atom_type, sequence_index, payload_text, payload_ref,
		 metadata_json, evidence_anchor, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, a := range atoms {
		_, err := stmt.Exec(
			a.ID, a.AssetID, string(a.AtomType), a.SequenceIndex,
			a.PayloadText, a.PayloadRef, a.MetadataJSON,
			a.EvidenceAnchor, a.CreatedAt,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *Database) GetAtomsForAsset(assetID string) ([]ContentAtom, error) {
	rows, err := d.db.Query(
		"SELECT * FROM content_atoms WHERE asset_id=? ORDER BY sequence_index",
		assetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var atoms []ContentAtom
	for rows.Next() {
		var a ContentAtom
		var atomType string
		err := rows.Scan(
			&a.ID, &a.AssetID, &atomType, &a.SequenceIndex,
			&a.PayloadText, &a.PayloadRef, &a.MetadataJSON,
			&a.EvidenceAnchor, &a.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		a.AtomType = AtomType(atomType)
		atoms = append(atoms, a)
	}
	return atoms, rows.Err()
}

func (d *Database) DeleteAtomsForAsset(assetID string) error {
	_, err := d.db.Exec("DELETE FROM content_atoms WHERE asset_id=?", assetID)
	return err
}

// -- Chunk operations --

func (d *Database) InsertChunk(c Chunk) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO chunks
		(id, atom_id, asset_id, chunk_text, token_count, chunk_index,
		 evidence_anchor, embedding_id, pipeline_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.AtomID, c.AssetID, c.ChunkText, c.TokenCount, c.ChunkIndex,
		c.EvidenceAnchor, c.EmbeddingID, c.PipelineVersion, c.CreatedAt,
	)
	return err
}

func (d *Database) InsertChunks(chunks []Chunk) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO chunks
		(id, atom_id, asset_id, chunk_text, token_count, chunk_index,
		 evidence_anchor, embedding_id, pipeline_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, c := range chunks {
		_, err := stmt.Exec(
			c.ID, c.AtomID, c.AssetID, c.ChunkText, c.TokenCount, c.ChunkIndex,
			c.EvidenceAnchor, c.EmbeddingID, c.PipelineVersion, c.CreatedAt,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *Database) GetChunksForAsset(assetID string) ([]Chunk, error) {
	rows, err := d.db.Query("SELECT * FROM chunks WHERE asset_id=? ORDER BY chunk_index", assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanChunks(rows)
}

func (d *Database) GetChunksWithoutEmbeddings(limit int) ([]Chunk, error) {
	rows, err := d.db.Query("SELECT * FROM chunks WHERE embedding_id IS NULL LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanChunks(rows)
}

func (d *Database) GetChunk(chunkID string) (*Chunk, error) {
	row := d.db.QueryRow("SELECT * FROM chunks WHERE id=?", chunkID)
	var c Chunk
	err := row.Scan(
		&c.ID, &c.AtomID, &c.AssetID, &c.ChunkText, &c.TokenCount, &c.ChunkIndex,
		&c.EvidenceAnchor, &c.EmbeddingID, &c.PipelineVersion, &c.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *Database) UpdateChunkEmbedding(chunkID, embeddingID string) error {
	_, err := d.db.Exec("UPDATE chunks SET embedding_id=? WHERE id=?", embeddingID, chunkID)
	return err
}

func (d *Database) DeleteChunksForAsset(assetID string) error {
	_, err := d.db.Exec("DELETE FROM chunks WHERE asset_id=?", assetID)
	return err
}

func (d *Database) CountChunks() (int, error) {
	var cnt int
	err := d.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&cnt)
	return cnt, err
}

func (d *Database) scanChunks(rows *sql.Rows) ([]Chunk, error) {
	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		err := rows.Scan(
			&c.ID, &c.AtomID, &c.AssetID, &c.ChunkText, &c.TokenCount, &c.ChunkIndex,
			&c.EvidenceAnchor, &c.EmbeddingID, &c.PipelineVersion, &c.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// -- Annotation operations --

func (d *Database) InsertAnnotation(ann Annotation) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	// Mark previous annotations as non-current
	_, err = tx.Exec("UPDATE annotations SET is_current=0 WHERE chunk_id=? AND is_current=1", ann.ChunkID)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO annotations
		(id, chunk_id, model_id, prompt_id, prompt_version, pipeline_version,
		 topics_json, sentiment_label, sentiment_confidence, entities_json,
		 claims_json, summary, quality_flags_json, is_current, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ann.ID, ann.ChunkID, ann.ModelID, ann.PromptID, ann.PromptVersion,
		ann.PipelineVersion, ann.TopicsJSON, ann.SentimentLabel,
		ann.SentimentConfidence, ann.EntitiesJSON, ann.ClaimsJSON,
		ann.Summary, ann.QualityFlagsJSON, ann.IsCurrent, ann.CreatedAt,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (d *Database) GetCurrentAnnotation(chunkID string) (*Annotation, error) {
	row := d.db.QueryRow(
		"SELECT * FROM annotations WHERE chunk_id=? AND is_current=1",
		chunkID,
	)
	var a Annotation
	err := row.Scan(
		&a.ID, &a.ChunkID, &a.ModelID, &a.PromptID, &a.PromptVersion,
		&a.PipelineVersion, &a.TopicsJSON, &a.SentimentLabel,
		&a.SentimentConfidence, &a.EntitiesJSON, &a.ClaimsJSON,
		&a.Summary, &a.QualityFlagsJSON, &a.IsCurrent, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *Database) CountAnnotations() (int, error) {
	var cnt int
	err := d.db.QueryRow("SELECT COUNT(*) FROM annotations WHERE is_current=1").Scan(&cnt)
	return cnt, err
}

// -- ConceptNode operations --

func (d *Database) InsertConceptNode(node ConceptNode) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO concept_nodes
		(id, level, label, description, parent_id, exemplar_chunk_ids,
		 pipeline_version, model_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.Level, node.Label, node.Description, node.ParentID,
		node.ExemplarChunkIDs, node.PipelineVersion, node.ModelID, node.CreatedAt,
	)
	return err
}

func (d *Database) GetConceptNodes(level *int) ([]ConceptNode, error) {
	var rows *sql.Rows
	var err error
	if level != nil {
		rows, err = d.db.Query("SELECT * FROM concept_nodes WHERE level=?", *level)
	} else {
		rows, err = d.db.Query("SELECT * FROM concept_nodes")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []ConceptNode
	for rows.Next() {
		var n ConceptNode
		err := rows.Scan(
			&n.ID, &n.Level, &n.Label, &n.Description, &n.ParentID,
			&n.ExemplarChunkIDs, &n.PipelineVersion, &n.ModelID, &n.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (d *Database) CountConcepts() (int, error) {
	var cnt int
	err := d.db.QueryRow("SELECT COUNT(*) FROM concept_nodes").Scan(&cnt)
	return cnt, err
}

// -- GraphEdge operations --

func (d *Database) InsertGraphEdge(edge GraphEdge) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO graph_edges
		(id, source_id, target_id, edge_type, weight, evidence_json,
		 pipeline_version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		edge.ID, edge.SourceID, edge.TargetID, edge.EdgeType,
		edge.Weight, edge.EvidenceJSON, edge.PipelineVersion, edge.CreatedAt,
	)
	return err
}

func (d *Database) CountEdges() (int, error) {
	var cnt int
	err := d.db.QueryRow("SELECT COUNT(*) FROM graph_edges").Scan(&cnt)
	return cnt, err
}

// GetGraphEdges retrieves edges with optional ordering and limit.
func (d *Database) GetGraphEdges(orderBy string, limit int) ([]GraphEdge, error) {
	q := "SELECT * FROM graph_edges"
	if orderBy != "" {
		q += " ORDER BY " + orderBy
	}
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := d.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanEdges(rows)
}

// GetEdgesForNode retrieves edges involving a specific node.
func (d *Database) GetEdgesForNode(nodeID string, limit int) ([]GraphEdge, error) {
	rows, err := d.db.Query(
		"SELECT * FROM graph_edges WHERE source_id=? OR target_id=? ORDER BY weight DESC LIMIT ?",
		nodeID, nodeID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanEdges(rows)
}

// GetMemberChunkIDs returns target_ids for concept_member edges from a concept.
func (d *Database) GetMemberChunkIDs(conceptID string) ([]string, error) {
	rows, err := d.db.Query(
		"SELECT target_id FROM graph_edges WHERE source_id=? AND edge_type='concept_member'",
		conceptID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *Database) scanEdges(rows *sql.Rows) ([]GraphEdge, error) {
	var edges []GraphEdge
	for rows.Next() {
		var e GraphEdge
		err := rows.Scan(
			&e.ID, &e.SourceID, &e.TargetID, &e.EdgeType,
			&e.Weight, &e.EvidenceJSON, &e.PipelineVersion, &e.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// -- PipelineJob operations --

func (d *Database) UpsertPipelineJob(job PipelineJob) error {
	now := nowISO()
	_, err := d.db.Exec(`
		INSERT INTO pipeline_jobs (id, job_type, status, progress_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, progress_json=excluded.progress_json, updated_at=?`,
		job.ID, job.JobType, string(job.Status), job.ProgressJSON, job.CreatedAt, now, now,
	)
	return err
}

func (d *Database) GetLatestJob(jobType *string) (*PipelineJob, error) {
	var row *sql.Row
	if jobType != nil {
		row = d.db.QueryRow(
			"SELECT * FROM pipeline_jobs WHERE job_type=? ORDER BY created_at DESC LIMIT 1",
			*jobType,
		)
	} else {
		row = d.db.QueryRow("SELECT * FROM pipeline_jobs ORDER BY created_at DESC LIMIT 1")
	}
	var j PipelineJob
	var status string
	err := row.Scan(&j.ID, &j.JobType, &status, &j.ProgressJSON, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j.Status = JobStatus(status)
	return &j, nil
}

func (d *Database) UpdateJobStatus(jobID string, status JobStatus, progress *string) error {
	now := nowISO()
	_, err := d.db.Exec(
		"UPDATE pipeline_jobs SET status=?, progress_json=?, updated_at=? WHERE id=?",
		string(status), progress, now, jobID,
	)
	return err
}

// -- WatchedVolume operations --

func (d *Database) AddWatchedVolume(vol WatchedVolume) error {
	_, err := d.db.Exec(`
		INSERT INTO watched_volumes (id, path, label, added_at, last_scan_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET label=excluded.label`,
		vol.ID, vol.Path, vol.Label, vol.AddedAt, vol.LastScanAt,
	)
	return err
}

func (d *Database) GetWatchedVolumes() ([]WatchedVolume, error) {
	rows, err := d.db.Query("SELECT * FROM watched_volumes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vols []WatchedVolume
	for rows.Next() {
		var v WatchedVolume
		if err := rows.Scan(&v.ID, &v.Path, &v.Label, &v.AddedAt, &v.LastScanAt); err != nil {
			return nil, err
		}
		vols = append(vols, v)
	}
	return vols, rows.Err()
}

func (d *Database) RemoveWatchedVolume(path string) error {
	_, err := d.db.Exec("DELETE FROM watched_volumes WHERE path=?", path)
	return err
}

func (d *Database) UpdateVolumeScanTime(volID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec("UPDATE watched_volumes SET last_scan_at=? WHERE id=?", now, volID)
	return err
}

// GetConceptNodeByID retrieves a single concept node.
func (d *Database) GetConceptNodeByID(id string) (*ConceptNode, error) {
	row := d.db.QueryRow("SELECT * FROM concept_nodes WHERE id=?", id)
	var n ConceptNode
	err := row.Scan(
		&n.ID, &n.Level, &n.Label, &n.Description, &n.ParentID,
		&n.ExemplarChunkIDs, &n.PipelineVersion, &n.ModelID, &n.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}
