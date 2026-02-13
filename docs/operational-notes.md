# Operational Notes

## Data Locations

Each workspace has its own data directory under `~/.knowledge-refinery/workspaces/<id>/`.

| Item | Path |
|------|------|
| Workspace root | `~/.knowledge-refinery/workspaces/<id>/` |
| SQLite DB | `~/.knowledge-refinery/workspaces/<id>/refinery.db` |
| Vector DB | `~/.knowledge-refinery/workspaces/<id>/vectors/` |
| Thumbnails | `~/.knowledge-refinery/workspaces/<id>/thumbnails/` |
| Temp files | `~/.knowledge-refinery/workspaces/<id>/tmp/` |
| PID file | `~/.knowledge-refinery/workspaces/<id>/daemon.pid` |

## Resetting

To start fresh, remove the data directory:
```bash
rm -rf ~/.knowledge-refinery
```

## Monitoring

The daemon logs to stdout. Key log patterns:
- `Stage N: ...` - Pipeline stage progress
- `Embedded batch N: X chunks` - Embedding progress
- `ERROR` - Errors during processing

### Live Pipeline Monitoring (M8)

During pipeline execution, real-time progress is available via the enriched `/ingest/status` endpoint. The daemon maintains:

- **Live progress dict**: Per-stage status (pending/running/done) with progress percentages
- **Counters**: chunk_count, annotation_count, concept_count, edge_count
- **Activity log**: 200-entry ring buffer; the last 50 events are returned via the API

The SwiftUI app polls at 1.5-second intervals and renders a full Pipeline Progress Panel with stage checkmarks, animated counters, and an auto-scrolling activity log. Polling auto-stops when the pipeline reaches idle/done state. The 3D universe auto-refreshes every 5 seconds during ingestion using `mergeUniverse()` for incremental node injection.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /health | Health check |
| POST | /volumes/add | Add watched directory |
| GET | /volumes/list | List watched directories |
| DELETE | /volumes/remove | Remove watched directory |
| POST | /ingest/start | Start pipeline |
| GET | /ingest/status | Pipeline status |
| POST | /search | Vector search |
| GET | /search/quick?q=... | Quick search |
| GET | /evidence/{asset_id} | Get asset info |
| GET | /evidence/chunk/{chunk_id} | Get chunk details |
| GET | /evidence/assets/all | List all assets |
| GET | /universe/snapshot | Universe snapshot |
| POST | /universe/focus | Focus on node |
| POST | /concepts/refine | Refine concept |
| GET | /concepts/list | List concepts |

## Performance Considerations

- Large files (>500MB) are skipped by default
- Embedding batch size defaults to 32 (adjustable)
- SQLite uses WAL mode for concurrent reads
- Pipeline runs in a background thread
- Incremental processing skips unchanged files (content hash comparison)
