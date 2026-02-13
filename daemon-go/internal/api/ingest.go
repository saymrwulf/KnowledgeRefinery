package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oho/knowledge-refinery-daemon/internal/pipeline"
)

type startIngestRequest struct {
	Paths []string `json:"paths"`
}

func IngestRouter(orch *pipeline.Orchestrator) chi.Router {
	r := chi.NewRouter()

	r.Post("/start", func(w http.ResponseWriter, r *http.Request) {
		if orch.IsRunning() {
			http.Error(w, "Pipeline is already running", http.StatusConflict)
			return
		}

		var req startIngestRequest
		json.NewDecoder(r.Body).Decode(&req) // may be empty body

		jobID, err := orch.RunPipeline(req.Paths)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"job_id": jobID,
			"status": "started",
		})
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orch.GetStatus())
	})

	return r
}
