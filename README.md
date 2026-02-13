# Knowledge Refinery

A local-first macOS Tahoe application that ingests heterogeneous document corpora, extracts structured knowledge via local LLMs (LM Studio), and provides semantic search with 3D concept visualization.

## Installation

### Prerequisites
- **macOS Tahoe** (26.x) on Apple Silicon
- **Xcode** or Xcode Command Line Tools (for Swift 6.2+)
- **Python 3.12+** (system Python or from python.org)
- **LM Studio** from [lmstudio.ai](https://lmstudio.ai)

### One-Line Install

```bash
git clone <repo-url> && cd LongLocalTimeHorizonInfoRetrieval && bash scripts/install.sh
```

This will:
1. Check all prerequisites
2. Create a Python virtual environment and install dependencies
3. Build the SwiftUI application
4. Create a proper `.app` bundle
5. Install to `/Applications`

### Manual Build

```bash
# Set up daemon
cd daemon
python3 -m venv .venv
.venv/bin/pip install -e ".[dev]"

# Build app bundle
cd ..
make build

# Or just run in development mode
make app-run
```

### LM Studio Setup

Before launching Knowledge Refinery:
1. Open LM Studio
2. Load models:
   - **Chat**: `gemma-3-4b` (or any chat model)
   - **Embeddings**: `nomic-embed-text-v1.5` (768-dim)
3. Start the local server on port **1234**

## Quick Start

1. Launch **Knowledge Refinery** from Applications or Spotlight
2. The dashboard shows LM Studio status (green = connected)
3. Click **New Workspace** — name it, add data lake folders
4. Click **Start All** to launch all workspace daemons and auto-start ingestion
5. Watch live pipeline progress: stage tracker, animated counters, activity log
6. Search, explore the concept universe, browse clusters

## Architecture

- **SwiftUI Master Control App** — Multi-workspace dashboard, LM Studio monitoring, daemon lifecycle, live pipeline visibility
- **Python Daemon** (FastAPI) — Per-workspace instances with independent ports and data directories (`~/.knowledge-refinery/workspaces/<id>/`)
- **Live Pipeline Progress** — 1.5s fast polling during ingestion, enriched `/ingest/status` with per-stage progress, counters, and activity log
- **LanceDB** — Embedded vector store for semantic search
- **SQLite** — Metadata, graph store, pipeline state
- **LM Studio** — Local LLM inference (embeddings + chat)
- **WebGPU** — 3D concept universe visualization with auto-refresh during ingestion

## Project Structure

```
apps/macos/KnowledgeRefinery/   SwiftUI macOS application
daemon/                         Python backend daemon
shared/                         Prompt templates, schemas
docs/                           Architecture and operational docs
scripts/                        Build and install scripts
test_corpus/                    Sample documents for testing
dist/                           Built .app bundle (after make build)
```

## Development

```bash
make help          # Show all commands
make test          # Run daemon tests + Swift build check
make app-run       # Run app via swift run (dev mode)
make daemon-run    # Run daemon directly
make clean         # Remove build artifacts
```

## Milestones

- **M1**: Core ingestion + search + evidence
- **M2**: LLM structured annotation
- **M3**: Concept clustering + labeling
- **M4**: WebGPU 3D Universe visualization
- **M5**: Semantic zoom + lenses
- **M6**: Extended format support (EPUB, archives, DICOM)
- **M7**: Master Control App (multi-workspace, LM Studio monitoring, daemon lifecycle)
- **M8**: Live Pipeline Visibility (real-time progress panel, activity log, universe auto-refresh)
