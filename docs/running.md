# Running Knowledge Refinery

## Prerequisites

- macOS 15+ (Sequoia or later)
- Go 1.22+
- Xcode or Xcode Command Line Tools (Swift 6.2+)
- LM Studio running locally with models loaded

## 1. Start LM Studio

1. Open LM Studio
2. Load an embedding model: `nomic-embed-text-v1.5` (768-dim)
3. Load a chat model: `qwen3-4b-2507` (or similar small model)
4. Start the local server (default port 1234)
5. Verify: `curl http://127.0.0.1:1234/v1/models`

## 2. Start the Daemon

### Via the App (Recommended)

The SwiftUI app auto-starts daemons for all workspaces on launch. Each workspace runs an independent Go daemon with its own port and data directory.

### Manually

```bash
cd daemon-go
go build -o knowledge-refinery-daemon .
./knowledge-refinery-daemon
```

The daemon will:
- Create data directory at `~/.knowledge-refinery/workspaces/<id>/`
- Initialize SQLite database (metadata + vectors)
- Connect to LM Studio
- Write a PID file to `{data_dir}/daemon.pid` for process detection
- Listen on its assigned port (default `http://127.0.0.1:8742`)

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

### From .app Bundle
```bash
make build
open "dist/Knowledge Refinery.app"
```

### Development Mode
```bash
make app-run
```

The app will:
- Auto-start daemons for all workspaces on launch
- Detect already-running daemons via PID files
- Auto-restart crashed daemons (up to 3 times)
- Show connection status in the toolbar

## 4. Process Documents

1. In the app, go to **Source Folders** tab
2. Click **Add Folder** and select a directory
3. Click **Process Documents** to run the pipeline
4. Watch live pipeline progress in the **Processing Steps** panel:
   - **Stage tracker**: Each of the 6 stages shows a checkmark when complete or a progress bar when running
   - **Animated counters**: Live tallies for passages, indexed, insights, themes, and links
   - **Interaction indicators**: Visual status of App-to-Daemon and Daemon-to-LM Studio connections
   - **Activity log**: Auto-scrolling log of the last 50 pipeline events
5. The universe auto-refreshes every 5 seconds during processing

### Via API

```bash
# Add a source folder
curl -X POST http://127.0.0.1:8742/volumes/add \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/documents"}'

# Start processing
curl -X POST http://127.0.0.1:8742/ingest/start \
  -H "Content-Type: application/json" \
  -d '{}'

# Check status
curl http://127.0.0.1:8742/ingest/status
```

## 5. Search

Use the **Search** tab in the app, or:

```bash
curl -X POST http://127.0.0.1:8742/search \
  -H "Content-Type: application/json" \
  -d '{"query": "machine learning", "limit": 10}'
```

## Troubleshooting

- **Daemon won't start**: Check that port 8742 is free (`lsof -i :8742`)
- **LM Studio unavailable**: Ensure LM Studio server is running on port 1234
- **No embeddings**: Verify an embedding model is loaded in LM Studio
- **App can't connect**: Check daemon is running on the expected port
- **Build fails**: Ensure Go 1.22+ and Swift 6.2+ are installed
