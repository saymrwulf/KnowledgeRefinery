package pipeline

import (
	"log/slog"

	"github.com/oho/knowledge-refinery-daemon/internal/lmstudio"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// Embedder embeds chunks via LM Studio and stores vectors.
type Embedder struct {
	lm        *lmstudio.Client
	vs        *storage.VectorStore
	db        *storage.Database
	batchSize int
	model     *string
	dimDetected bool
}

func NewEmbedder(lm *lmstudio.Client, vs *storage.VectorStore, db *storage.Database, batchSize int) *Embedder {
	return &Embedder{
		lm:        lm,
		vs:        vs,
		db:        db,
		batchSize: batchSize,
	}
}

// EmbedChunks embeds a list of chunks and stores them in the vector store.
func (e *Embedder) EmbedChunks(chunks []storage.Chunk) int {
	if len(chunks) == 0 {
		return 0
	}

	// Auto-detect model
	if e.model == nil {
		e.model = e.lm.GetEmbeddingModel()
		if e.model == nil {
			slog.Error("No embedding model available in LM Studio")
			return 0
		}
	}

	// Detect dimension from a test call
	if !e.dimDetected {
		vec, err := e.lm.EmbedSingle("hello world", e.model)
		if err != nil {
			slog.Error("Failed to detect embedding dimension", "error", err)
			return 0
		}
		e.vs.SetDimension(len(vec))
		e.dimDetected = true
		slog.Info("Detected embedding dimension", "dim", len(vec), "model", *e.model)
	}

	embeddedCount := 0

	for i := 0; i < len(chunks); i += e.batchSize {
		end := i + e.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		texts := make([]string, len(batch))
		for j, c := range batch {
			texts[j] = c.ChunkText
		}

		rawVecs, err := e.lm.Embed(texts, e.model)
		if err != nil {
			slog.Error("Embedding batch failed", "error", err)
			continue
		}

		// Build vector records
		records := make([]storage.VectorRecord, len(batch))
		for j, c := range batch {
			// Convert float64 to float32
			vec := make([]float32, len(rawVecs[j]))
			for k, v := range rawVecs[j] {
				vec[k] = float32(v)
			}

			asset, _ := e.db.GetFileAsset(c.AssetID)
			assetPath := ""
			if asset != nil {
				assetPath = asset.Path
			}

			records[j] = storage.VectorRecord{
				ID:              c.ID,
				Vector:          vec,
				Text:            c.ChunkText,
				AssetID:         c.AssetID,
				AssetPath:       assetPath,
				EvidenceAnchor:  c.EvidenceAnchor,
				PipelineVersion: c.PipelineVersion,
				AtomType:        "text",
			}
		}

		if err := e.vs.AddVectors(records); err != nil {
			slog.Error("Failed to add vectors", "error", err)
			continue
		}

		// Mark chunks as having embeddings
		for _, c := range batch {
			e.db.UpdateChunkEmbedding(c.ID, c.ID)
		}

		embeddedCount += len(batch)
		slog.Info("Embedded batch", "batch", i/e.batchSize+1, "count", len(batch))
	}

	return embeddedCount
}
