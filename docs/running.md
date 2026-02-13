# Running Knowledge Refinery

## Prerequisites

- macOS Tahoe (26.x)
- Python 3.12+
- Xcode 26+
- LM Studio running locally with at least one model loaded

## 1. Start LM Studio

1. Open LM Studio
2. Load an embedding model (e.g., `nomic-embed-text-v1.5` or `text-embedding-3-small`)
3. Load a chat model (e.g., `llama-3.2-3b-instruct` or similar)
4. Start the local server (default port 1234)
5. Verify: `curl http://127.0.0.1:1234/v1/models`

## 2. Start the Daemon

```bash
cd daemon
source .venv/bin/activate
python -m knowledge_refinery.main
```

The daemon will:
- Create data directory at `~/.knowledge-refinery/workspaces/<id>/`
- Initialize SQLite database
- Connect to LM Studio
- Write a PID file to `{data_dir}/daemon.pid` for process detection
- Listen on its assigned port (default `http://127.0.0.1:8742`)

> **Tip**: Use **Start All** in the app toolbar to launch all workspace daemons at once. Each workspace runs an independent daemon with its own port and data directory. After connection, ingestion auto-starts.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KR_DATA_DIR` | `~/.knowledge-refinery` | Data directory |
| `KR_LM_STUDIO_URL` | `http://127.0.0.1:1234/v1` | LM Studio API URL |
| `KR_PORT` | `8742` | Daemon port |

### Verify Daemon

```bash
curl http://127.0.0.1:8742/health
```

## 3. Run the macOS App

```bash
cd apps/macos/KnowledgeRefinery
swift run
```

Or open in Xcode:
```bash
open Package.swift
```

The app will:
- Auto-start daemons for all workspaces on launch
- Detect already-running daemons via PID files
- Auto-restart crashed daemons (up to 3 times)
- Show connection status in the toolbar

## 4. Ingest Documents

1. In the app, go to **Volumes** tab
2. Click **Add Folder** and select a directory
3. Go to **Ingest** tab and click **Start Ingestion**, or use **Start All** from the dashboard
4. Watch live pipeline progress in the **Pipeline Progress Panel**:
   - **Stage tracker**: Each of the 6 stages (Scan, Extract, Chunk, Embed, Annotate, Conceptualize) shows a checkmark when complete or an animated progress bar when running
   - **Animated counters**: Live tallies for chunks, vectors, annotations, concepts, and edges
   - **Interaction indicators**: Visual status of App-to-Daemon and Daemon-to-LM Studio connections
   - **Activity log**: Auto-scrolling log of the last 50 pipeline events
5. The dashboard card shows a compact spinner with the current stage name and chunk count
6. The 3D universe auto-refreshes every 5 seconds during ingestion, using incremental node injection

The app polls `/ingest/status` every 1.5 seconds during pipeline execution and automatically stops polling when the pipeline completes.

### Via API

```bash
# Add a volume
curl -X POST http://127.0.0.1:8742/volumes/add \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/documents"}'

# Start ingestion
curl -X POST http://127.0.0.1:8742/ingest/start \
  -H "Content-Type: application/json" \
  -d '{}'

# Check status (enriched response with live progress)
curl http://127.0.0.1:8742/ingest/status
```

The enriched `/ingest/status` response includes:

```json
{
  "status": "running",
  "stage": "embed",
  "chunk_count": 142,
  "annotation_count": 87,
  "concept_count": 12,
  "edge_count": 45,
  "live": {
    "scan": {"status": "done", "progress_pct": 100},
    "extract": {"status": "done", "progress_pct": 100},
    "chunk": {"status": "done", "progress_pct": 100},
    "embed": {"status": "running", "progress_pct": 64},
    "annotate": {"status": "pending", "progress_pct": 0},
    "conceptualize": {"status": "pending", "progress_pct": 0}
  },
  "activity_log": [
    {"timestamp": "2026-02-12T10:30:01Z", "message": "Scanning 3 volumes..."},
    {"timestamp": "2026-02-12T10:30:03Z", "message": "Found 47 files, 12 new"},
    "..."
  ]
}
```

## 5. Search

Use the **Search** tab in the app, or:

```bash
curl -X POST http://127.0.0.1:8742/search \
  -H "Content-Type: application/json" \
  -d '{"query": "machine learning", "limit": 10}'
```

## Troubleshooting

- **Daemon won't start**: Check that port 8742 is free
- **LM Studio unavailable**: Ensure LM Studio server is running on port 1234
- **No embeddings**: Verify an embedding model is loaded in LM Studio
- **App can't connect**: Check daemon is running on the expected port
