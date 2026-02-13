# Knowledge Refinery Architecture

## Overview

Knowledge Refinery is a local-first macOS application that ingests heterogeneous document corpora, extracts structured knowledge using local LLMs, and provides semantic search and visualization.

## System Components

```
┌──────────────────────────────────┐
│   SwiftUI macOS App              │
│  ┌────────┬────────────┐         │
│  │ Search │ Evidence    │         │
│  │ View   │ Panel (QL)  │         │
│  ├────────┼────────────┤         │
│  │Pipeline│ Source      │         │
│  │Progress│ Folders     │         │
│  │ Panel  │             │         │
│  └────────┴────────────┘         │
│       │ HTTP (localhost)          │
│       │ ┌──────────────────┐     │
│       │ │ 1.5s poll loop   │     │
│       │ │ /ingest/status   │◀─┐  │
│       │ └──────────────────┘  │  │
│       │    auto-stop on done  │  │
│       │                       │  │
│       │ ┌──────────────────┐  │  │
│       │ │ 5s universe      │  │  │
│       │ │ auto-refresh     │──┘  │
│       │ └──────────────────┘     │
└───────┼──────────────────────────┘
        ▼
┌──────────────────────────────────┐
│   Go Daemon (11MB binary)        │
│   Per-workspace on independent   │
│   port + data dir                │
│  ┌──────────────────────┐        │
│  │  chi Router + CORS   │        │
│  └──────────┬───────────┘        │
│             ▼                    │
│  ┌──────────────────────┐        │
│  │  Pipeline            │        │
│  │  Orchestrator        │        │
│  └──┬──┬──┬──┬──┬──┬────┘        │
│     │  │  │  │  │  │             │
│     ▼  ▼  ▼  ▼  ▼  ▼             │
│  Scan Extract Chunk Embed        │
│           Annotate Conceptualize │
│             │                    │
│             ▼                    │
│  ┌──────────────────────┐        │
│  │  Live Progress Dict  │        │
│  │  + Activity Log Ring │        │
│  │    (200-entry buf)   │        │
│  └──────────────────────┘        │
│                                  │
│  ┌──────────────────────┐        │
│  │ SQLite (WAL mode)    │        │
│  │  metadata + vectors  │        │
│  │  + graph + state     │        │
│  └──────────────────────┘        │
└───────┼──────────────────────────┘
        ▼
┌──────────────────────────────────┐
│   LM Studio                      │
│   (127.0.0.1:1234)               │
│   Embeddings + Chat              │
└──────────────────────────────────┘
```

## Pipeline Stages

1. **Scan** - Walk directories, compute content hashes, detect changes
2. **Extract** - Produce ContentAtoms with evidence anchors (PDF pages, text lines, etc.)
3. **Chunk** - Deterministic text splitting (500-800 tokens, 50 token overlap)
4. **Embed** - Generate vector embeddings via LM Studio
5. **Annotate** - Structured LLM annotation (topics, entities, claims, sentiment)
6. **Conceptualize** - Build similarity graph and concept clusters

## Data Flow

Files → FileAsset → ContentAtom → Chunk → Vector (SQLite BLOB) + Annotation
                                                      ↓
                                            ConceptNode + GraphEdge

## Live Progress Data Flow

During pipeline execution, the daemon maintains in-memory state that the app polls:

```
Pipeline Orchestrator (goroutine)
    │
    ├──▶ live progress dict (per-stage status: pending/running/done)
    │       stage_name, progress_pct, item_count
    │
    ├──▶ counters: chunk_count, annotation_count, concept_count, edge_count
    │
    └──▶ activity_log ring buffer (200 entries, last 50 returned via API)
            timestamp + message per event

SwiftUI App polling loop (1.5s interval):
    GET /ingest/status ──▶ stages, counters, activity_log
    │
    ├── Pipeline Progress Panel: checkmarks + progress bars per stage
    ├── Animated counters: passages, indexed, insights, themes, links
    ├── Interaction indicators: App↔Daemon, Daemon↔LM Studio
    ├── Auto-scrolling activity log
    └── Auto-stop polling when pipeline status = idle/done

Universe auto-refresh (5s timer during ingestion):
    GET /universe/snapshot ──▶ mergeUniverse() for incremental node injection
```

## Key Design Decisions

- **Go single binary** over Python: Zero dependencies, instant startup, 11MB, no venv/pip issues
- **SQLite for everything**: Metadata, vectors (as BLOBs with brute-force cosine search), graph — one file, WAL mode
- **chi router**: Lightweight HTTP routing with path params, CORS middleware
- **modernc.org/sqlite**: Pure Go SQLite driver, no CGo, true single binary
- **tiktoken-go**: Accurate token counting matching OpenAI tokenizer
- **Deterministic chunk IDs**: SHA-256(asset_id + anchor + normalized_text_hash)
- **Versioned annotations**: Never overwrite, mark active by pipeline version
- **Evidence-native**: Every derived insight links back to source file + location
- **Fast polling over WebSocket**: 1.5s HTTP polls are simpler and sufficient for pipeline status
- **Ring buffer for activity log**: Fixed 200-entry buffer prevents memory growth during long runs
