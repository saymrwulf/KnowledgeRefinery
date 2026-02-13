package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oho/knowledge-refinery-daemon/internal/lmstudio"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

type searchRequest struct {
	Query           string  `json:"query"`
	Limit           int     `json:"limit"`
	FilterAssetType *string `json:"filter_asset_type"`
}

type searchResultItem struct {
	ChunkID        string   `json:"chunk_id"`
	Score          float64  `json:"score"`
	Text           string   `json:"text"`
	AssetID        string   `json:"asset_id"`
	AssetPath      string   `json:"asset_path"`
	EvidenceAnchor string   `json:"evidence_anchor"`
	Topics         *string  `json:"topics"`
	Summary        *string  `json:"summary"`
	Sentiment      *string  `json:"sentiment"`
	Entities       []string `json:"entities"`
}

func SearchRouter(lm *lmstudio.Client, vs *storage.VectorStore, db *storage.Database) chi.Router {
	r := chi.NewRouter()

	doSearch := func(query string, limit int) ([]searchResultItem, error) {
		rawVec, err := lm.EmbedSingle(query, nil)
		if err != nil {
			return nil, err
		}

		// Convert float64 to float32
		queryVec := make([]float32, len(rawVec))
		for i, v := range rawVec {
			queryVec[i] = float32(v)
		}

		results := vs.Search(queryVec, limit)

		items := make([]searchResultItem, len(results))
		for i, res := range results {
			item := searchResultItem{
				ChunkID:        res.ID,
				Score:          res.Distance,
				Text:           res.Text,
				AssetID:        res.AssetID,
				AssetPath:      res.AssetPath,
				EvidenceAnchor: res.EvidenceAnchor,
			}

			if res.Topics != "" {
				item.Topics = &res.Topics
			}

			// Enrich with annotation
			ann, _ := db.GetCurrentAnnotation(res.ID)
			if ann != nil {
				if ann.TopicsJSON != nil {
					var topics []string
					json.Unmarshal([]byte(*ann.TopicsJSON), &topics)
					if len(topics) > 0 {
						joined := ""
						for i, t := range topics {
							if i > 0 {
								joined += ", "
							}
							joined += t
						}
						item.Topics = &joined
					}
				}
				item.Summary = ann.Summary
				item.Sentiment = ann.SentimentLabel
				if ann.EntitiesJSON != nil {
					var entities []map[string]string
					json.Unmarshal([]byte(*ann.EntitiesJSON), &entities)
					for _, e := range entities {
						if name, ok := e["name"]; ok {
							item.Entities = append(item.Entities, name)
						}
					}
				}
			}

			items[i] = item
		}
		return items, nil
	}

	r.Post("/", func(w http.ResponseWriter, r *http.Request) {
		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Limit <= 0 {
			req.Limit = 20
		}

		items, err := doSearch(req.Query, req.Limit)
		if err != nil {
			http.Error(w, "Failed to embed query: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	})

	r.Get("/quick", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "q parameter required", http.StatusBadRequest)
			return
		}
		limit := 10
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}

		items, err := doSearch(q, limit)
		if err != nil {
			http.Error(w, "Failed to embed query: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	})

	return r
}
