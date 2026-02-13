package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

type evidenceResponse struct {
	AssetID        string  `json:"asset_id"`
	Path           string  `json:"path"`
	Filename       string  `json:"filename"`
	MimeType       *string `json:"mime_type"`
	SizeBytes      int64   `json:"size_bytes"`
	Exists         bool    `json:"exists"`
	EvidenceAnchor any     `json:"evidence_anchor,omitempty"`
	ChunkText      *string `json:"chunk_text,omitempty"`
}

func EvidenceRouter(db *storage.Database) chi.Router {
	r := chi.NewRouter()

	r.Get("/{asset_id}", func(w http.ResponseWriter, r *http.Request) {
		assetID := chi.URLParam(r, "asset_id")
		asset, err := db.GetFileAsset(assetID)
		if err != nil || asset == nil {
			http.Error(w, "Asset not found: "+assetID, http.StatusNotFound)
			return
		}

		_, fileExists := os.Stat(asset.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(evidenceResponse{
			AssetID:   asset.ID,
			Path:      asset.Path,
			Filename:  asset.Filename,
			MimeType:  asset.MimeType,
			SizeBytes: asset.SizeBytes,
			Exists:    fileExists == nil,
		})
	})

	r.Get("/chunk/{chunk_id}", func(w http.ResponseWriter, r *http.Request) {
		chunkID := chi.URLParam(r, "chunk_id")
		chunk, err := db.GetChunk(chunkID)
		if err != nil || chunk == nil {
			http.Error(w, "Chunk not found: "+chunkID, http.StatusNotFound)
			return
		}
		asset, _ := db.GetFileAsset(chunk.AssetID)
		if asset == nil {
			http.Error(w, "Asset not found for chunk: "+chunkID, http.StatusNotFound)
			return
		}

		var anchor any
		if chunk.EvidenceAnchor != "" {
			json.Unmarshal([]byte(chunk.EvidenceAnchor), &anchor)
		}

		_, fileExists := os.Stat(asset.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(evidenceResponse{
			AssetID:        asset.ID,
			Path:           asset.Path,
			Filename:       asset.Filename,
			MimeType:       asset.MimeType,
			SizeBytes:      asset.SizeBytes,
			Exists:         fileExists == nil,
			EvidenceAnchor: anchor,
			ChunkText:      &chunk.ChunkText,
		})
	})

	r.Get("/chunk/{chunk_id}/annotation", func(w http.ResponseWriter, r *http.Request) {
		chunkID := chi.URLParam(r, "chunk_id")
		ann, err := db.GetCurrentAnnotation(chunkID)
		if err != nil || ann == nil {
			http.Error(w, "No annotation for chunk: "+chunkID, http.StatusNotFound)
			return
		}

		var topics, entities, claims, qualityFlags any
		if ann.TopicsJSON != nil {
			json.Unmarshal([]byte(*ann.TopicsJSON), &topics)
		}
		if ann.EntitiesJSON != nil {
			json.Unmarshal([]byte(*ann.EntitiesJSON), &entities)
		}
		if ann.ClaimsJSON != nil {
			json.Unmarshal([]byte(*ann.ClaimsJSON), &claims)
		}
		if ann.QualityFlagsJSON != nil {
			json.Unmarshal([]byte(*ann.QualityFlagsJSON), &qualityFlags)
		}

		if topics == nil {
			topics = []any{}
		}
		if entities == nil {
			entities = []any{}
		}
		if claims == nil {
			claims = []any{}
		}
		if qualityFlags == nil {
			qualityFlags = []any{}
		}

		resp := map[string]any{
			"chunk_id":       ann.ChunkID,
			"model_id":       ann.ModelID,
			"prompt_id":      ann.PromptID,
			"prompt_version": ann.PromptVersion,
			"topics":         topics,
			"sentiment": map[string]any{
				"label":      ann.SentimentLabel,
				"confidence": ann.SentimentConfidence,
			},
			"entities":      entities,
			"claims":        claims,
			"summary":       ann.Summary,
			"quality_flags": qualityFlags,
			"created_at":    ann.CreatedAt,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	r.Get("/assets/all", func(w http.ResponseWriter, r *http.Request) {
		assets, err := db.GetAllAssets()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := make([]map[string]any, len(assets))
		for i, a := range assets {
			resp[i] = map[string]any{
				"id":         a.ID,
				"path":       a.Path,
				"filename":   a.Filename,
				"mime_type":  a.MimeType,
				"size_bytes": a.SizeBytes,
				"status":     string(a.Status),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	return r
}
