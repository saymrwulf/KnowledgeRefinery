# Knowledge Refinery

A local-first macOS application that ingests heterogeneous document corpora, extracts structured knowledge via local LLMs (LM Studio), and provides semantic search with 3D concept visualization.

## Installation

### Prerequisites
- **macOS 15+** (Sequoia or later) on Apple Silicon
- **Xcode** or Xcode Command Line Tools (for Swift 6.2+)
- **Go 1.22+** from [go.dev](https://go.dev/dl/) or `brew install go`
- **LM Studio** from [lmstudio.ai](https://lmstudio.ai)

### One-Line Install

```bash
git clone https://github.com/saymrwulf/KnowledgeRefinery.git && cd KnowledgeRefinery && bash scripts/install.sh
```

This will:
1. Check all prerequisites (Go, Swift, Xcode)
2. Run all Go daemon tests (89 tests)
3. Build the Go daemon + SwiftUI app into a `.app` bundle
4. Install to `/Applications`
5. Create the data directory at `~/.knowledge-refinery/`

### Manual Build

```bash
# Build Go daemon + .app bundle
make build

# Or run in development mode
make daemon       # Build Go daemon binary
make app-run      # Run SwiftUI app (builds daemon first)
```

### LM Studio Setup

Before launching Knowledge Refinery:
1. Open LM Studio
2. Load models:
   - **Chat**: `qwen3-4b-2507` (or any small chat model)
   - **Embeddings**: `nomic-embed-text-v1.5` (768-dim)
3. Start the local server on port **1234**

## Quick Start

1. Launch **Knowledge Refinery** from Applications or Spotlight
2. The dashboard shows LM Studio status (green = connected)
3. Click **New Workspace** — name it, add source folders
4. Open the workspace and click **Process Documents** to run the pipeline
5. Watch live progress: stage tracker, animated counters, activity log
6. Search, explore the concept universe, browse themes

## Architecture

- **SwiftUI App** — Multi-workspace dashboard, LM Studio monitoring, daemon lifecycle, live pipeline visibility
- **Go Daemon** (chi router) — 11MB single binary, zero dependencies, per-workspace instances on independent ports
- **SQLite** — All storage: metadata, vectors (as BLOBs with brute-force cosine search), graph, pipeline state
- **6-Stage Pipeline** — scan, extract, chunk, embed, annotate, conceptualize
- **LM Studio** — Local LLM inference at `127.0.0.1:1234` (embeddings + chat)
- **WebGPU / Canvas2D** — 3D concept universe visualization with interactive fallback

## Project Structure

```
apps/macos/KnowledgeRefinery/   SwiftUI macOS application
daemon-go/                      Go daemon (chi, SQLite, tiktoken)
shared/                         Prompt templates, schemas
docs/                           Architecture and operational docs
scripts/                        Build and install scripts
test_corpus/                    Sample documents for testing
dist/                           Built .app bundle (after make build)
```

## Development

```bash
make help          # Show all commands
make test          # Run 89 Go tests + Swift build check
make app-run       # Run app via swift run (dev mode)
make daemon-run    # Run daemon standalone
make clean         # Remove build artifacts
```

## Milestones

- **M1-M6**: Core pipeline, search, annotation, clustering, WebGPU, extended formats
- **M7**: Master Control App (multi-workspace, LM Studio monitoring, daemon lifecycle)
- **M8**: Live Pipeline Visibility (real-time progress, activity log, universe auto-refresh)
- **M9-M10**: UX language overhaul, macOS sizing improvements
- **M11**: Go daemon rewrite (Python replaced with single Go binary, 89 tests)
