package storage

import (
	"encoding/json"
	"time"
)

// AssetStatus represents the processing state of a file asset.
type AssetStatus string

const (
	StatusPending   AssetStatus = "pending"
	StatusExtracted AssetStatus = "extracted"
	StatusChunked   AssetStatus = "chunked"
	StatusEmbedded  AssetStatus = "embedded"
	StatusAnnotated AssetStatus = "annotated"
	StatusError     AssetStatus = "error"
)

// AtomType represents the type of a content atom.
type AtomType string

const (
	AtomText     AtomType = "text"
	AtomImage    AtomType = "image"
	AtomTable    AtomType = "table"
	AtomMetadata AtomType = "metadata"
	AtomBinary   AtomType = "binary"
)

// JobStatus represents the state of a pipeline job.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
)

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// NowISO is the exported version of nowISO for use by other packages.
func NowISO() string {
	return nowISO()
}

// EvidenceAnchor identifies where content came from within a source file.
type EvidenceAnchor struct {
	AssetID      string    `json:"asset_id"`
	Page         *int      `json:"page,omitempty"`
	Bbox         []float64 `json:"bbox,omitempty"`
	Chapter      *string   `json:"chapter,omitempty"`
	Offset       *int      `json:"offset,omitempty"`
	ArchiveChain *string   `json:"archive_chain,omitempty"`
	LineStart    *int      `json:"line_start,omitempty"`
	LineEnd      *int      `json:"line_end,omitempty"`
}

func (ea EvidenceAnchor) ToJSON() string {
	b, _ := json.Marshal(ea)
	return string(b)
}

func ParseEvidenceAnchor(s string) (EvidenceAnchor, error) {
	var ea EvidenceAnchor
	err := json.Unmarshal([]byte(s), &ea)
	return ea, err
}

// FileAsset represents a tracked file in the system.
type FileAsset struct {
	ID           string      `json:"id"`
	Path         string      `json:"path"`
	Filename     string      `json:"filename"`
	UTI          *string     `json:"uti,omitempty"`
	MimeType     *string     `json:"mime_type,omitempty"`
	SizeBytes    int64       `json:"size_bytes"`
	MtimeNs      int64       `json:"mtime_ns"`
	ContentHash  *string     `json:"content_hash,omitempty"`
	ScanVersion  int         `json:"scan_version"`
	Status       AssetStatus `json:"status"`
	ErrorMessage *string     `json:"error_message,omitempty"`
	CreatedAt    string      `json:"created_at"`
	UpdatedAt    string      `json:"updated_at"`
}

func NewFileAsset(id, path, filename string) FileAsset {
	now := nowISO()
	return FileAsset{
		ID:          id,
		Path:        path,
		Filename:    filename,
		ScanVersion: 1,
		Status:      StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ContentAtom represents an extracted content unit from a file.
type ContentAtom struct {
	ID             string   `json:"id"`
	AssetID        string   `json:"asset_id"`
	AtomType       AtomType `json:"atom_type"`
	SequenceIndex  int      `json:"sequence_index"`
	PayloadText    *string  `json:"payload_text,omitempty"`
	PayloadRef     *string  `json:"payload_ref,omitempty"`
	MetadataJSON   *string  `json:"metadata_json,omitempty"`
	EvidenceAnchor string   `json:"evidence_anchor"`
	CreatedAt      string   `json:"created_at"`
}

func NewContentAtom(id, assetID string, atomType AtomType, seqIdx int, anchor string) ContentAtom {
	return ContentAtom{
		ID:             id,
		AssetID:        assetID,
		AtomType:       atomType,
		SequenceIndex:  seqIdx,
		EvidenceAnchor: anchor,
		CreatedAt:      nowISO(),
	}
}

// Chunk represents a text chunk derived from a content atom.
type Chunk struct {
	ID              string  `json:"id"`
	AtomID          string  `json:"atom_id"`
	AssetID         string  `json:"asset_id"`
	ChunkText       string  `json:"chunk_text"`
	TokenCount      int     `json:"token_count"`
	ChunkIndex      int     `json:"chunk_index"`
	EvidenceAnchor  string  `json:"evidence_anchor"`
	EmbeddingID     *string `json:"embedding_id,omitempty"`
	PipelineVersion string  `json:"pipeline_version"`
	CreatedAt       string  `json:"created_at"`
}

func NewChunk(id, atomID, assetID, text string, tokenCount, chunkIndex int, anchor, pipelineVersion string) Chunk {
	return Chunk{
		ID:              id,
		AtomID:          atomID,
		AssetID:         assetID,
		ChunkText:       text,
		TokenCount:      tokenCount,
		ChunkIndex:      chunkIndex,
		EvidenceAnchor:  anchor,
		PipelineVersion: pipelineVersion,
		CreatedAt:       nowISO(),
	}
}

// Annotation represents an LLM-generated analysis of a chunk.
type Annotation struct {
	ID                  string   `json:"id"`
	ChunkID             string   `json:"chunk_id"`
	ModelID             string   `json:"model_id"`
	PromptID            string   `json:"prompt_id"`
	PromptVersion       string   `json:"prompt_version"`
	PipelineVersion     string   `json:"pipeline_version"`
	TopicsJSON          *string  `json:"topics_json,omitempty"`
	SentimentLabel      *string  `json:"sentiment_label,omitempty"`
	SentimentConfidence *float64 `json:"sentiment_confidence,omitempty"`
	EntitiesJSON        *string  `json:"entities_json,omitempty"`
	ClaimsJSON          *string  `json:"claims_json,omitempty"`
	Summary             *string  `json:"summary,omitempty"`
	QualityFlagsJSON    *string  `json:"quality_flags_json,omitempty"`
	IsCurrent           int      `json:"is_current"`
	CreatedAt           string   `json:"created_at"`
}

// ConceptNode represents a cluster of related chunks.
type ConceptNode struct {
	ID               string  `json:"id"`
	Level            int     `json:"level"`
	Label            *string `json:"label,omitempty"`
	Description      *string `json:"description,omitempty"`
	ParentID         *string `json:"parent_id,omitempty"`
	ExemplarChunkIDs *string `json:"exemplar_chunk_ids,omitempty"`
	PipelineVersion  *string `json:"pipeline_version,omitempty"`
	ModelID          *string `json:"model_id,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

// GraphEdge represents a relationship between two nodes.
type GraphEdge struct {
	ID              string  `json:"id"`
	SourceID        string  `json:"source_id"`
	TargetID        string  `json:"target_id"`
	EdgeType        string  `json:"edge_type"`
	Weight          float64 `json:"weight"`
	EvidenceJSON    *string `json:"evidence_json,omitempty"`
	PipelineVersion *string `json:"pipeline_version,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

// PipelineJob tracks a pipeline run.
type PipelineJob struct {
	ID           string    `json:"id"`
	JobType      string    `json:"job_type"`
	Status       JobStatus `json:"status"`
	ProgressJSON *string   `json:"progress_json,omitempty"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

// WatchedVolume represents a directory being monitored for ingestion.
type WatchedVolume struct {
	ID         string  `json:"id"`
	Path       string  `json:"path"`
	Label      *string `json:"label,omitempty"`
	AddedAt    string  `json:"added_at"`
	LastScanAt *string `json:"last_scan_at,omitempty"`
}

func NewWatchedVolume(id, path string, label *string) WatchedVolume {
	return WatchedVolume{
		ID:      id,
		Path:    path,
		Label:   label,
		AddedAt: nowISO(),
	}
}
