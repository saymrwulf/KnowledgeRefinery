package pipeline

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/oho/knowledge-refinery-daemon/internal/config"
	"github.com/oho/knowledge-refinery-daemon/internal/lmstudio"
	"github.com/oho/knowledge-refinery-daemon/internal/pipeline/extractors"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// Orchestrator coordinates the full ingestion pipeline.
type Orchestrator struct {
	db              *storage.Database
	vs              *storage.VectorStore
	lm              *lmstudio.Client
	cfg             config.Config
	scanner         *Scanner
	registry        *extractors.Registry
	chunker         *Chunker
	embedder        *Embedder
	annotator       *Annotator
	conceptualizer  *Conceptualizer
	running         bool
	currentJobID    *string
	mu              sync.Mutex
	liveProgress    map[string]any
	activityLog     []map[string]any
	logMu           sync.Mutex
}

func NewOrchestrator(db *storage.Database, vs *storage.VectorStore, lm *lmstudio.Client, cfg config.Config) *Orchestrator {
	return &Orchestrator{
		db:             db,
		vs:             vs,
		lm:             lm,
		cfg:            cfg,
		scanner:        NewScanner(db, cfg),
		registry:       extractors.CreateDefaultRegistry(),
		chunker:        NewChunker(cfg.Pipeline),
		embedder:       NewEmbedder(lm, vs, db, cfg.LMStudio.EmbeddingBatchSize),
		annotator:      NewAnnotator(lm, db, cfg.Pipeline.Version),
		conceptualizer: NewConceptualizer(db, vs, lm, cfg.Pipeline.Version),
		liveProgress:   make(map[string]any),
	}
}

func (o *Orchestrator) IsRunning() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.running
}

func (o *Orchestrator) Conceptualizer() *Conceptualizer {
	return o.conceptualizer
}

func (o *Orchestrator) emit(stage, action, detail string, counts map[string]int) {
	entry := map[string]any{
		"ts":     time.Now().UTC().Format("15:04:05"),
		"stage":  stage,
		"action": action,
		"detail": detail,
	}
	if counts != nil {
		entry["counts"] = counts
	}
	o.logMu.Lock()
	o.activityLog = append(o.activityLog, entry)
	if len(o.activityLog) > 200 {
		o.activityLog = o.activityLog[len(o.activityLog)-200:]
	}
	o.logMu.Unlock()
}

func generateJobID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RunPipeline starts the pipeline in a background goroutine. Returns job ID.
func (o *Orchestrator) RunPipeline(volumePaths []string) (string, error) {
	o.mu.Lock()
	if o.running {
		o.mu.Unlock()
		return "", fmt.Errorf("pipeline already running")
	}
	o.running = true
	o.mu.Unlock()

	jobID := generateJobID()
	o.currentJobID = &jobID

	now := storage.NowISO()
	progressJSON := fmt.Sprintf(`{"stage":"starting","started_at":"%s"}`, now)
	job := storage.PipelineJob{
		ID:           jobID,
		JobType:      "full_ingest",
		Status:       storage.JobRunning,
		ProgressJSON: &progressJSON,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	o.db.UpsertPipelineJob(job)

	go o.runPipelineWorker(jobID, volumePaths)
	return jobID, nil
}

func (o *Orchestrator) runPipelineWorker(jobID string, volumePaths []string) {
	o.liveProgress = make(map[string]any)
	o.logMu.Lock()
	o.activityLog = nil
	o.logMu.Unlock()

	progress := map[string]any{"stage": "scanning", "stages": map[string]any{}}

	defer func() {
		o.mu.Lock()
		o.running = false
		o.currentJobID = nil
		o.mu.Unlock()
	}()

	// Adapt chunk sizes
	ctx := o.lm.GetContextLength(nil)
	o.chunker.AdaptToContext(ctx)
	slog.Info("LLM context window", "tokens", ctx)

	// Stage 1: Scan
	slog.Info("=== Stage 1: Scanning ===")
	o.updateProgress(jobID, progress)

	if len(volumePaths) == 0 {
		vols, _ := o.db.GetWatchedVolumes()
		for _, v := range vols {
			volumePaths = append(volumePaths, v.Path)
		}
	}

	scanStats := ScanStats{}
	for i, path := range volumePaths {
		o.liveProgress = map[string]any{"scan": map[string]any{
			"current_path": path, "done": i, "total": len(volumePaths),
		}}
		stats, err := o.scanner.ScanDirectory(path)
		if err != nil {
			slog.Error("Scan error", "path", path, "error", err)
			scanStats.Errors++
			continue
		}
		scanStats.Add(stats)
		o.emit("scanning", "scanned", filepath.Base(path), scanStats.ToMap())
	}
	progress["stages"].(map[string]any)["scan"] = scanStats.ToMap()
	slog.Info("Scan complete", "stats", scanStats.ToMap())

	// Stage 2: Extract
	slog.Info("=== Stage 2: Extracting ===")
	progress["stage"] = "extracting"
	o.updateProgress(jobID, progress)

	pending, _ := o.db.GetAssetsByStatus(storage.StatusPending, 10000)
	extractCount := 0
	extractErrors := 0

	for i, asset := range pending {
		o.liveProgress = map[string]any{"extract": map[string]any{
			"current_file": asset.Filename, "done": i, "total": len(pending),
		}}

		o.db.DeleteAtomsForAsset(asset.ID)
		o.db.DeleteChunksForAsset(asset.ID)
		o.vs.DeleteByAsset(asset.ID)

		atoms, err := o.registry.Extract(asset)
		if err != nil {
			slog.Error("Extract error", "file", asset.Filename, "error", err)
			errMsg := err.Error()
			o.db.UpdateAssetStatus(asset.ID, storage.StatusError, &errMsg)
			extractErrors++
			continue
		}
		if len(atoms) > 0 {
			o.db.InsertContentAtoms(atoms)
		}
		o.db.UpdateAssetStatus(asset.ID, storage.StatusExtracted, nil)
		extractCount++
		o.emit("extracting", "extracted", asset.Filename, map[string]int{"done": i + 1, "total": len(pending)})
	}
	progress["stages"].(map[string]any)["extract"] = map[string]any{
		"processed": extractCount, "errors": extractErrors,
	}
	slog.Info("Extract complete", "count", extractCount, "errors", extractErrors)

	// Stage 3: Chunk
	slog.Info("=== Stage 3: Chunking ===")
	progress["stage"] = "chunking"
	o.updateProgress(jobID, progress)

	extracted, _ := o.db.GetAssetsByStatus(storage.StatusExtracted, 10000)
	chunkCount := 0

	for i, asset := range extracted {
		o.liveProgress = map[string]any{"chunk": map[string]any{
			"current_file": asset.Filename, "done": i, "total": len(extracted), "chunks_created": chunkCount,
		}}

		atoms, _ := o.db.GetAtomsForAsset(asset.ID)
		chunks := o.chunker.ChunkAtoms(atoms, asset.ID)
		if len(chunks) > 0 {
			o.db.InsertChunks(chunks)
			chunkCount += len(chunks)
		}
		o.db.UpdateAssetStatus(asset.ID, storage.StatusChunked, nil)
		o.emit("chunking", "chunked", asset.Filename, map[string]int{
			"done": i + 1, "total": len(extracted), "chunks_created": chunkCount,
		})
	}
	progress["stages"].(map[string]any)["chunk"] = map[string]any{"chunks_created": chunkCount}
	slog.Info("Chunk complete", "chunks", chunkCount)

	// Stage 4: Embed
	slog.Info("=== Stage 4: Embedding ===")
	progress["stage"] = "embedding"
	o.updateProgress(jobID, progress)

	unembedded, _ := o.db.GetChunksWithoutEmbeddings(10000)
	if len(unembedded) > 0 {
		o.liveProgress = map[string]any{"embed": map[string]any{
			"embedded": 0, "total": len(unembedded),
		}}
		o.emit("embedding", "started", fmt.Sprintf("%d chunks to embed", len(unembedded)), nil)
		embeddedCount := o.embedder.EmbedChunks(unembedded)
		o.liveProgress = map[string]any{"embed": map[string]any{
			"embedded": embeddedCount, "total": len(unembedded),
		}}
		o.emit("embedding", "embedded", fmt.Sprintf("%d chunks", embeddedCount),
			map[string]int{"embedded": embeddedCount, "total": len(unembedded)})
		progress["stages"].(map[string]any)["embed"] = map[string]any{"embedded": embeddedCount}
		slog.Info("Embed complete", "count", embeddedCount)

		// Mark assets as embedded
		chunked, _ := o.db.GetAssetsByStatus(storage.StatusChunked, 10000)
		for _, asset := range chunked {
			assetChunks, _ := o.db.GetChunksForAsset(asset.ID)
			allEmbedded := true
			for _, c := range assetChunks {
				if c.EmbeddingID == nil {
					allEmbedded = false
					break
				}
			}
			if allEmbedded {
				o.db.UpdateAssetStatus(asset.ID, storage.StatusEmbedded, nil)
			}
		}
	} else {
		progress["stages"].(map[string]any)["embed"] = map[string]any{
			"embedded": 0, "note": "all chunks already embedded",
		}
	}

	// Stage 5: Annotate
	slog.Info("=== Stage 5: Annotating ===")
	progress["stage"] = "annotating"
	o.updateProgress(jobID, progress)

	embeddedAssets, _ := o.db.GetAssetsByStatus(storage.StatusEmbedded, 10000)
	annotateCount := 0
	for i, asset := range embeddedAssets {
		o.liveProgress = map[string]any{"annotate": map[string]any{
			"current_file": asset.Filename, "done": i, "total": len(embeddedAssets),
			"annotated_chunks": annotateCount,
		}}
		chunks, _ := o.db.GetChunksForAsset(asset.ID)
		count := o.annotator.AnnotateChunks(chunks)
		annotateCount += count
		if count > 0 {
			o.db.UpdateAssetStatus(asset.ID, storage.StatusAnnotated, nil)
		}
		o.emit("annotating", "annotated", asset.Filename, map[string]int{
			"done": i + 1, "total": len(embeddedAssets), "annotated_chunks": annotateCount,
		})
	}
	progress["stages"].(map[string]any)["annotate"] = map[string]any{"annotated": annotateCount}
	slog.Info("Annotate complete", "count", annotateCount)

	// Stage 6: Conceptualize
	slog.Info("=== Stage 6: Conceptualizing ===")
	progress["stage"] = "conceptualizing"
	o.updateProgress(jobID, progress)
	o.liveProgress = map[string]any{"conceptualize": map[string]any{"status": "building concepts"}}
	o.emit("conceptualizing", "started", "building concept clusters", nil)

	concepts := o.conceptualizer.BuildConcepts(0, nil)
	o.emit("conceptualizing", "concepts_built", fmt.Sprintf("%d concepts", len(concepts)), nil)
	o.liveProgress = map[string]any{"conceptualize": map[string]any{
		"status": "building graph", "concepts": len(concepts),
	}}
	edgeCount := o.conceptualizer.BuildSimilarityGraph(5)
	o.emit("conceptualizing", "graph_built", fmt.Sprintf("%d edges", edgeCount), nil)
	progress["stages"].(map[string]any)["conceptualize"] = map[string]any{
		"concepts": len(concepts), "edges": edgeCount,
	}
	slog.Info("Conceptualize complete", "concepts", len(concepts), "edges", edgeCount)

	// Done
	progress["stage"] = "completed"
	progress["completed_at"] = storage.NowISO()
	progressJSON, _ := json.Marshal(progress)
	progressStr := string(progressJSON)
	o.db.UpdateJobStatus(jobID, storage.JobCompleted, &progressStr)
	o.liveProgress = make(map[string]any)
	o.emit("completed", "done", "Pipeline finished", nil)
	slog.Info("=== Pipeline completed ===")
}

func (o *Orchestrator) updateProgress(jobID string, progress map[string]any) {
	data, _ := json.Marshal(progress)
	s := string(data)
	o.db.UpdateJobStatus(jobID, storage.JobRunning, &s)
}

// GetStatus returns the current pipeline status.
func (o *Orchestrator) GetStatus() map[string]any {
	counts, _ := o.db.CountAssetsByStatus()
	total := 0
	for _, v := range counts {
		total += v
	}

	jobType := "full_ingest"
	job, _ := o.db.GetLatestJob(&jobType)
	jobInfo := map[string]any{}
	if job != nil {
		var prog any
		if job.ProgressJSON != nil {
			json.Unmarshal([]byte(*job.ProgressJSON), &prog)
		}
		jobInfo = map[string]any{
			"job_id":   job.ID,
			"status":   string(job.Status),
			"progress": prog,
		}
	}

	o.logMu.Lock()
	var recentLog []map[string]any
	if len(o.activityLog) > 50 {
		recentLog = make([]map[string]any, 50)
		copy(recentLog, o.activityLog[len(o.activityLog)-50:])
	} else {
		recentLog = make([]map[string]any, len(o.activityLog))
		copy(recentLog, o.activityLog)
	}
	o.logMu.Unlock()

	chunkCount, _ := o.db.CountChunks()
	annotationCount, _ := o.db.CountAnnotations()
	conceptCount, _ := o.db.CountConcepts()
	edgeCount, _ := o.db.CountEdges()

	o.mu.Lock()
	running := o.running
	var currentJobID *string
	if o.currentJobID != nil {
		s := *o.currentJobID
		currentJobID = &s
	}
	o.mu.Unlock()

	live := map[string]any{}
	if running {
		for k, v := range o.liveProgress {
			live[k] = v
		}
	}

	return map[string]any{
		"running":          running,
		"current_job_id":   currentJobID,
		"total_assets":     total,
		"status_counts":    counts,
		"latest_job":       jobInfo,
		"vector_count":     o.vs.Count(),
		"chunk_count":      chunkCount,
		"annotation_count": annotationCount,
		"concept_count":    conceptCount,
		"edge_count":       edgeCount,
		"live":             live,
		"activity_log":     recentLog,
	}
}
