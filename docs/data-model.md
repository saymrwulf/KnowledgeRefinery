# Knowledge Refinery Data Model

## Entity Relationship

```
WatchedVolume
    │
    ▼ (contains files)
FileAsset ──────────────────┐
    │                       │
    ▼ (extracted into)      │
ContentAtom                 │
    │                       │
    ▼ (split into)          │
Chunk ──────────────────────┤
    │                       │
    ├──▶ Vector (SQLite)    │
    │    768-dim BLOB       │
    │                       │
    ├──▶ Annotation         │
    │    (versioned)        │
    │                       │
    └──▶ GraphEdge ◀────────┘
         │
         ▼
    ConceptNode
    (hierarchical)
```

## Tables (SQLite)

### file_assets
Tracks every file in watched volumes. Status progresses through:
`pending` → `extracted` → `chunked` → `embedded` → `annotated` → `conceptualized`

### content_atoms
Raw content extracted from files. Types: text, image, table, metadata, binary.
Each atom has an evidence_anchor linking to exact source location.

### chunks
Deterministic text segments (500-800 tokens). IDs are stable across re-processing.
Linked to vectors in `chunk_vectors` table via chunk ID.

### chunk_vectors
Embedding vectors stored as binary BLOBs (768 x float32 = 3072 bytes per vector).
Loaded into memory at startup for brute-force cosine similarity search.

| Field | Type | Description |
|-------|------|-------------|
| id | TEXT PRIMARY KEY | Matches chunks.id |
| vector | BLOB | 768-dim float32 embedding |
| text | TEXT | Chunk text |
| asset_id | TEXT | Source file |
| asset_path | TEXT | File path |
| evidence_anchor | TEXT | JSON anchor |
| pipeline_version | TEXT | Version tag |

### annotations
LLM-generated structured metadata per chunk. **Never overwritten** - new annotations
are added with `is_current=1` and previous ones marked `is_current=0`.
Versioned by model_id + prompt_id + prompt_version.

### concept_nodes
Hierarchical concept clusters derived from embedding similarity.
Level 0 = macro concepts, higher levels = finer granularity.

### graph_edges
Typed, weighted edges: similarity, concept membership, co-occurrence.
Each edge stores evidence references back to source chunks.

### pipeline_jobs
Crash recovery: tracks job state so processing resumes after restart.

## Live Progress State (In-Memory)

During pipeline execution, the daemon maintains ephemeral in-memory structures that are not persisted to SQLite:

### Live Progress Dict
Per-stage status object returned in the `live` field of `/ingest/status`:

```json
{
  "scan":          {"status": "done",    "progress_pct": 100},
  "extract":       {"status": "running", "progress_pct": 72},
  "chunk":         {"status": "pending", "progress_pct": 0},
  "embed":         {"status": "pending", "progress_pct": 0},
  "annotate":      {"status": "pending", "progress_pct": 0},
  "conceptualize": {"status": "pending", "progress_pct": 0}
}
```

### Activity Log Ring Buffer
A fixed-size circular buffer (200 entries) that records pipeline events. The API returns the most recent 50 entries.

### Enriched Status Counters

| Counter | Description |
|---------|-------------|
| `chunk_count` | Total chunks produced so far |
| `annotation_count` | Total annotations generated |
| `concept_count` | Total concept nodes created |
| `edge_count` | Total graph edges created |

## Evidence Anchors

Every derived artifact links back to source via JSON evidence anchors:

```json
{
    "asset_id": "abc123...",
    "page": 5,
    "bbox": [100, 200, 400, 250],
    "chapter": "Introduction",
    "offset": 1024,
    "archive_chain": "docs.zip::papers/paper.pdf::page=5",
    "line_start": 42,
    "line_end": 58
}
```
