package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oho/knowledge-refinery-daemon/internal/pipeline"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

func ConceptsRouter(db *storage.Database, conceptualizer *pipeline.Conceptualizer) chi.Router {
	r := chi.NewRouter()

	r.Get("/list", func(w http.ResponseWriter, r *http.Request) {
		var levelPtr *int
		if l := r.URL.Query().Get("level"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				levelPtr = &n
			}
		}

		concepts, err := db.GetConceptNodes(levelPtr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := make([]map[string]any, len(concepts))
		for i, c := range concepts {
			var exemplars any
			if c.ExemplarChunkIDs != nil {
				json.Unmarshal([]byte(*c.ExemplarChunkIDs), &exemplars)
			}
			if exemplars == nil {
				exemplars = []any{}
			}
			resp[i] = map[string]any{
				"id":                 c.ID,
				"level":             c.Level,
				"label":             c.Label,
				"description":       c.Description,
				"parent_id":         c.ParentID,
				"exemplar_chunk_ids": exemplars,
				"model_id":          c.ModelID,
				"created_at":        c.CreatedAt,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	r.Get("/{concept_id}", func(w http.ResponseWriter, r *http.Request) {
		conceptID := chi.URLParam(r, "concept_id")
		node, err := db.GetConceptNodeByID(conceptID)
		if err != nil || node == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Concept not found"})
			return
		}

		memberIDs, _ := db.GetMemberChunkIDs(conceptID)

		var members []map[string]any
		limit := 20
		if limit > len(memberIDs) {
			limit = len(memberIDs)
		}
		for _, mid := range memberIDs[:limit] {
			chunk, _ := db.GetChunk(mid)
			if chunk != nil {
				ann, _ := db.GetCurrentAnnotation(mid)
				member := map[string]any{
					"chunk_id": mid,
					"text":     truncate(chunk.ChunkText, 200),
					"asset_id": chunk.AssetID,
				}
				if ann != nil {
					member["summary"] = ann.Summary
				}
				members = append(members, member)
			}
		}
		if members == nil {
			members = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":           node.ID,
			"level":        node.Level,
			"label":        node.Label,
			"description":  node.Description,
			"parent_id":    node.ParentID,
			"member_count": len(memberIDs),
			"members":      members,
		})
	})

	r.Post("/refine", func(w http.ResponseWriter, r *http.Request) {
		conceptID := r.URL.Query().Get("concept_id")
		nSub := 5
		if n := r.URL.Query().Get("n_sub"); n != "" {
			if v, err := strconv.Atoi(n); err == nil && v >= 2 && v <= 20 {
				nSub = v
			}
		}

		if conceptualizer == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Conceptualizer not available"})
			return
		}

		subConcepts := conceptualizer.RefineConcept(conceptID, nSub)
		var subs []map[string]any
		for _, sc := range subConcepts {
			subs = append(subs, map[string]any{
				"id":          sc.ID,
				"label":       sc.Label,
				"description": sc.Description,
				"level":       sc.Level,
			})
		}
		if subs == nil {
			subs = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"parent_concept_id": conceptID,
			"sub_concepts":      subs,
		})
	})

	r.Get("/{concept_id}/why", func(w http.ResponseWriter, r *http.Request) {
		conceptID := chi.URLParam(r, "concept_id")
		node, _ := db.GetConceptNodeByID(conceptID)
		if node == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "Concept not found"})
			return
		}

		var exemplarIDs []string
		if node.ExemplarChunkIDs != nil {
			json.Unmarshal([]byte(*node.ExemplarChunkIDs), &exemplarIDs)
		}

		var evidence []map[string]any
		for _, eid := range exemplarIDs {
			chunk, _ := db.GetChunk(eid)
			if chunk != nil {
				asset, _ := db.GetFileAsset(chunk.AssetID)
				ann, _ := db.GetCurrentAnnotation(eid)

				var anchor any
				if chunk.EvidenceAnchor != "" {
					json.Unmarshal([]byte(chunk.EvidenceAnchor), &anchor)
				}
				if anchor == nil {
					anchor = map[string]any{}
				}

				var topics any
				if ann != nil && ann.TopicsJSON != nil {
					json.Unmarshal([]byte(*ann.TopicsJSON), &topics)
				}
				if topics == nil {
					topics = []any{}
				}

				ev := map[string]any{
					"chunk_id":        eid,
					"chunk_text":      truncate(chunk.ChunkText, 300),
					"evidence_anchor": anchor,
					"topics":          topics,
				}
				if asset != nil {
					ev["asset_path"] = asset.Path
					ev["asset_filename"] = asset.Filename
				}
				if ann != nil {
					ev["annotation_summary"] = ann.Summary
				}
				evidence = append(evidence, ev)
			}
		}
		if evidence == nil {
			evidence = []map[string]any{}
		}

		labelStr := ptrOr(node.Label, "unknown")
		modelStr := ptrOr(node.ModelID, "unknown model")
		explanation := fmt.Sprintf(
			"This concept '%s' was formed by clustering %d text chunks based on embedding similarity using %s. The label was generated by analyzing representative excerpts.",
			labelStr, len(exemplarIDs), modelStr,
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"concept_id":       conceptID,
			"label":            node.Label,
			"description":      node.Description,
			"pipeline_version": node.PipelineVersion,
			"model_id":         node.ModelID,
			"evidence":         evidence,
			"explanation":      explanation,
		})
	})

	return r
}
