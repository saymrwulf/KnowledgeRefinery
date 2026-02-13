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
    ├──▶ Vector (LanceDB)   │
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

## Tables

### file_assets
Tracks every file in watched volumes. Status progresses through:
`pending` → `extracted` → `chunked` → `embedded` → `annotated` → `conceptualized`

### content_atoms
Raw content extracted from files. Types: text, image, table, metadata, binary.
Each atom has an evidence_anchor linking to exact source location.

### chunks
Deterministic text segments (500-800 tokens). IDs are stable across re-processing.
Linked to LanceDB vectors via embedding_id.

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

## Live Progress State (M8, In-Memory)

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

Each stage transitions through `pending` -> `running` -> `done`.

### Activity Log Ring Buffer
A fixed-size circular buffer (200 entries) that records pipeline events. The API returns the most recent 50 entries. Each entry contains a timestamp and message string:

```json
{"timestamp": "2026-02-12T10:30:03Z", "message": "Found 47 files, 12 new"}
```

The ring buffer prevents unbounded memory growth during long pipeline runs. It is reset at the start of each new pipeline execution.

### Enriched Status Counters
The `/ingest/status` response includes running totals updated as each stage completes:

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

## Vector Schema (LanceDB)

| Field | Type | Description |
|-------|------|-------------|
| id | string | Matches chunks.id |
| vector | float32[] | Embedding vector |
| text | string | Chunk text |
| asset_id | string | Source file |
| asset_path | string | File path |
| evidence_anchor | string | JSON anchor |
| topics | string | Comma-separated topics |
| atom_type | string | text/image/etc |
| pipeline_version | string | Version tag |
