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
│  │Pipeline│ Volume      │         │
│  │Progress│ Manager     │         │
│  │ Panel  │             │         │
│  └────────┴────────────┘         │
│       │ HTTP (localhost)            │
│       │ ┌──────────────────┐      │
│       │ │ 1.5s poll loop   │      │
│       │ │ /ingest/status   │◀─┐   │
│       │ └──────────────────┘  │   │
│       │    auto-stop on done  │   │
│       │                       │   │
│       │ ┌──────────────────┐  │   │
│       │ │ 5s universe      │  │   │
│       │ │ auto-refresh     │──┘   │
│       │ └──────────────────┘      │
└───────┼──────────────────────────┘
        ▼
┌──────────────────────────────────┐
│   Refinery Daemon (Python)       │
│   Per-workspace on independent   │
│   port + data dir                │
│  ┌──────────────────────┐        │
│  │  FastAPI Server      │        │
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
│  ┌─────────┬───────────┐         │
│  │ SQLite  │ LanceDB   │         │
│  │ (meta)  │ (vectors) │         │
│  └─────────┴───────────┘         │
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

Files → FileAsset → ContentAtom → Chunk → Embedding (LanceDB) + Annotation (SQLite)
                                                        ↓
                                              ConceptNode + GraphEdge

## Live Progress Data Flow (M8)

During pipeline execution, the daemon maintains in-memory state that the app polls:

```
Pipeline Orchestrator
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
    ├── Animated counters: chunks, vectors, annotations, concepts, edges
    ├── Interaction indicators: App↔Daemon, Daemon↔LM Studio
    ├── Auto-scrolling activity log
    └── Auto-stop polling when pipeline status = idle/done

Universe auto-refresh (5s timer during ingestion):
    GET /universe/snapshot ──▶ mergeUniverse() for incremental node injection
```

## Key Design Decisions

- **LanceDB** over Qdrant: Embedded, no separate server, local-first
- **SQLite** for metadata/graph: Simple, reliable, WAL mode for concurrency
- **Deterministic chunk IDs**: SHA-256(asset_id + anchor + normalized_text_hash)
- **Versioned annotations**: Never overwrite, mark active by pipeline version
- **Evidence-native**: Every derived insight links back to source file + location
- **Fast polling over WebSocket**: 1.5s HTTP polls are simpler and sufficient for pipeline status; avoids connection lifecycle complexity
- **Ring buffer for activity log**: Fixed 200-entry buffer prevents memory growth during long pipeline runs
