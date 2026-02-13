package pipeline

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/oho/knowledge-refinery-daemon/internal/lmstudio"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

const (
	defaultPromptID      = "annotate_chunk_v1"
	defaultPromptVersion = "1.0"
)

var annotationPrompt = `You are a knowledge extraction assistant. Analyze the following text chunk and produce a JSON object with these fields:
- "topics": array of topic labels (2-5 labels)
- "sentiment": {"label": "positive"|"negative"|"neutral"|"mixed", "confidence": 0.0-1.0}
- "entities": array of {"name": string, "type": "person"|"org"|"location"|"concept"|"date"|"other"}
- "claims": array of {"claim": string, "confidence": 0.0-1.0}
- "summary": a 1-2 sentence summary
- "quality_flags": array of any quality issues (e.g., "truncated", "low_quality", "technical", "multilingual")

Respond with ONLY the JSON object, no other text.`

// Annotator annotates chunks using the LLM.
type Annotator struct {
	lm              *lmstudio.Client
	db              *storage.Database
	pipelineVersion string
	prompt          string
	model           *string
}

func NewAnnotator(lm *lmstudio.Client, db *storage.Database, pipelineVersion string) *Annotator {
	return &Annotator{
		lm:              lm,
		db:              db,
		pipelineVersion: pipelineVersion,
		prompt:          annotationPrompt,
	}
}

type annotationJSON struct {
	Topics       []string          `json:"topics"`
	Sentiment    sentimentJSON     `json:"sentiment"`
	Entities     []json.RawMessage `json:"entities"`
	Claims       []json.RawMessage `json:"claims"`
	Summary      string            `json:"summary"`
	QualityFlags []string          `json:"quality_flags"`
}

type sentimentJSON struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

// AnnotateChunk annotates a single chunk with the LLM. Retries on failure.
func (a *Annotator) AnnotateChunk(chunk storage.Chunk, maxRetries int) *storage.Annotation {
	if a.model == nil {
		a.model = a.lm.GetChatModel()
		if a.model == nil {
			slog.Error("No chat model available for annotation")
			return nil
		}
	}

	var parsed annotationJSON
	for attempt := 0; attempt < maxRetries; attempt++ {
		response, err := a.lm.AnnotateChunk(chunk.ChunkText, a.prompt, a.model)
		if err != nil {
			wait := time.Duration(5*(attempt+1)) * time.Second
			slog.Warn("Annotation attempt failed",
				"attempt", attempt+1, "max", maxRetries,
				"chunk", chunk.ID, "error", err, "wait", wait)
			time.Sleep(wait)
			continue
		}

		if err := json.Unmarshal([]byte(response), &parsed); err != nil {
			slog.Warn("Failed to parse annotation JSON", "chunk", chunk.ID, "error", err)
			return nil
		}
		break
	}

	if parsed.Summary == "" && len(parsed.Topics) == 0 {
		slog.Error("Annotation failed after retries", "chunk", chunk.ID)
		return nil
	}

	annID := fmt.Sprintf("%x", sha256.Sum256([]byte(
		fmt.Sprintf("%s:%s:%s:%s", chunk.ID, *a.model, defaultPromptID, defaultPromptVersion),
	)))[:32]

	topicsJSON, _ := json.Marshal(parsed.Topics)
	entitiesJSON, _ := json.Marshal(parsed.Entities)
	claimsJSON, _ := json.Marshal(parsed.Claims)
	qualityJSON, _ := json.Marshal(parsed.QualityFlags)

	topicsStr := string(topicsJSON)
	entitiesStr := string(entitiesJSON)
	claimsStr := string(claimsJSON)
	qualityStr := string(qualityJSON)

	var sentLabel *string
	var sentConf *float64
	if parsed.Sentiment.Label != "" {
		sentLabel = &parsed.Sentiment.Label
		sentConf = &parsed.Sentiment.Confidence
	}

	var summary *string
	if parsed.Summary != "" {
		summary = &parsed.Summary
	}

	return &storage.Annotation{
		ID:                  annID,
		ChunkID:             chunk.ID,
		ModelID:             *a.model,
		PromptID:            defaultPromptID,
		PromptVersion:       defaultPromptVersion,
		PipelineVersion:     a.pipelineVersion,
		TopicsJSON:          &topicsStr,
		SentimentLabel:      sentLabel,
		SentimentConfidence: sentConf,
		EntitiesJSON:        &entitiesStr,
		ClaimsJSON:          &claimsStr,
		Summary:             summary,
		QualityFlagsJSON:    &qualityStr,
		IsCurrent:           1,
		CreatedAt:           storage.NowISO(),
	}
}

// AnnotateChunks annotates multiple chunks. Returns count of successful annotations.
func (a *Annotator) AnnotateChunks(chunks []storage.Chunk) int {
	count := 0
	for _, chunk := range chunks {
		// Skip if already annotated with current model
		existing, _ := a.db.GetCurrentAnnotation(chunk.ID)
		if existing != nil && a.model != nil && existing.ModelID == *a.model {
			continue
		}

		ann := a.AnnotateChunk(chunk, 3)
		if ann != nil {
			if err := a.db.InsertAnnotation(*ann); err != nil {
				slog.Error("Failed to insert annotation", "error", err)
				continue
			}
			count++
			slog.Debug("Annotated chunk", "chunk", chunk.ID)
		}
		// Brief pause between requests for local model recovery
		time.Sleep(1 * time.Second)
	}
	return count
}
