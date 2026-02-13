package server

import (
	"encoding/json"
	"net/http"

	"github.com/oho/knowledge-refinery-daemon/internal/config"
	"github.com/oho/knowledge-refinery-daemon/internal/lmstudio"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

type HealthResponse struct {
	Status         string   `json:"status"`
	LMStudio       string   `json:"lm_studio"`
	VectorCount    int      `json:"vector_count"`
	DB             string   `json:"db"`
	ChatModel      *string  `json:"chat_model"`
	EmbeddingModel *string  `json:"embedding_model"`
	DataDir        string   `json:"data_dir"`
	Port           int      `json:"port"`
	WatchedVolumes []string `json:"watched_volumes"`
	ContextLength  *int     `json:"context_length"`
}

// HealthHandler returns a handler for GET /health.
func HealthHandler(cfg config.Config, db *storage.Database, vs *storage.VectorStore, lm *lmstudio.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lmOK := lm.HealthCheck()

		lmStatus := "unavailable"
		if lmOK {
			lmStatus = "connected"
		}

		dbStatus := "connected"
		if db == nil {
			dbStatus = "unavailable"
		}

		var watchedPaths []string
		if db != nil {
			vols, _ := db.GetWatchedVolumes()
			for _, v := range vols {
				watchedPaths = append(watchedPaths, v.Path)
			}
		}
		if watchedPaths == nil {
			watchedPaths = []string{}
		}

		var contextLength *int
		if lmOK {
			ctx := lm.GetContextLength(nil)
			contextLength = &ctx
		}

		resp := HealthResponse{
			Status:         "ok",
			LMStudio:       lmStatus,
			VectorCount:    vs.Count(),
			DB:             dbStatus,
			ChatModel:      lm.GetChatModel(),
			EmbeddingModel: lm.GetEmbeddingModel(),
			DataDir:        cfg.DataDir,
			Port:           cfg.Port,
			WatchedVolumes: watchedPaths,
			ContextLength:  contextLength,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
