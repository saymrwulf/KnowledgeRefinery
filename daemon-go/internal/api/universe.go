package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

func UniverseRouter(db *storage.Database, vs *storage.VectorStore) chi.Router {
	r := chi.NewRouter()

	r.Get("/snapshot", func(w http.ResponseWriter, r *http.Request) {
		lod := r.URL.Query().Get("lod")
		if lod == "" {
			lod = "macro"
		}

		var nodes []map[string]any
		var edges []map[string]any

		// Get level-0 concepts
		level0 := 0
		concepts, _ := db.GetConceptNodes(&level0)
		for i, c := range concepts {
			hue := float64(i) / float64(max(len(concepts), 1)) * 360
			var exemplars any
			if c.ExemplarChunkIDs != nil {
				json.Unmarshal([]byte(*c.ExemplarChunkIDs), &exemplars)
			}
			if exemplars == nil {
				exemplars = []any{}
			}
			node := map[string]any{
				"id":                 c.ID,
				"label":             ptrOr(c.Label, "Unlabeled"),
				"level":             c.Level,
				"type":              "concept",
				"size":              20,
				"color":             fmt.Sprintf("hsl(%.0f, 70%%, 60%%)", hue),
				"cluster":           i,
				"description":       c.Description,
				"exemplar_chunk_ids": exemplars,
			}
			nodes = append(nodes, node)
		}

		if lod == "mid" || lod == "near" {
			// Add sub-concepts
			allConcepts, _ := db.GetConceptNodes(nil)
			for _, c := range allConcepts {
				if c.Level == 0 {
					continue
				}
				parentCluster := 0
				for j, pc := range concepts {
					if c.ParentID != nil && pc.ID == *c.ParentID {
						parentCluster = j
						break
					}
				}
				hue := float64(parentCluster) / float64(max(len(concepts), 1)) * 360
				nodes = append(nodes, map[string]any{
					"id":        c.ID,
					"label":     ptrOr(c.Label, "Sub-concept"),
					"level":     c.Level,
					"type":      "sub_concept",
					"size":      12,
					"color":     fmt.Sprintf("hsl(%.0f, 50%%, 50%%)", hue),
					"cluster":   parentCluster,
					"parent_id": c.ParentID,
				})
			}
		}

		if lod == "near" {
			// Add chunk nodes
			allAssets, _ := db.GetAllAssets()
			for _, asset := range allAssets {
				chunks, _ := db.GetChunksForAsset(asset.ID)
				for _, chunk := range chunks {
					ann, _ := db.GetCurrentAnnotation(chunk.ID)
					topicsStr := ""
					if ann != nil && ann.TopicsJSON != nil {
						var topics []string
						json.Unmarshal([]byte(*ann.TopicsJSON), &topics)
						if len(topics) > 3 {
							topics = topics[:3]
						}
						for i, t := range topics {
							if i > 0 {
								topicsStr += ", "
							}
							topicsStr += t
						}
					}
					label := topicsStr
					if label == "" {
						label = truncate(chunk.ChunkText, 40) + "..."
					}
					node := map[string]any{
						"id":         chunk.ID,
						"label":      label,
						"level":      99,
						"type":       "chunk",
						"size":       5,
						"color":      "hsl(210, 30%, 50%)",
						"cluster":    -1,
						"asset_path": asset.Path,
					}
					if ann != nil {
						node["summary"] = ann.Summary
					}
					nodes = append(nodes, node)
				}
			}
		}

		// Get edges
		graphEdges, _ := db.GetGraphEdges("weight DESC", 500)
		nodeIDs := make(map[string]bool)
		for _, n := range nodes {
			nodeIDs[n["id"].(string)] = true
		}
		for _, e := range graphEdges {
			if nodeIDs[e.SourceID] && nodeIDs[e.TargetID] {
				edges = append(edges, map[string]any{
					"source": e.SourceID,
					"target": e.TargetID,
					"weight": e.Weight,
					"type":   e.EdgeType,
				})
			}
		}

		if nodes == nil {
			nodes = []map[string]any{}
		}
		if edges == nil {
			edges = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"lod":        lod,
			"nodes":      nodes,
			"edges":      edges,
			"node_count": len(nodes),
			"edge_count": len(edges),
		})
	})

	r.Post("/focus", func(w http.ResponseWriter, r *http.Request) {
		nodeID := r.URL.Query().Get("node_id")
		if nodeID == "" {
			http.Error(w, "node_id required", http.StatusBadRequest)
			return
		}

		edgeRows, _ := db.GetEdgesForNode(nodeID, 50)

		neighborIDs := make(map[string]bool)
		var edges []map[string]any
		for _, e := range edgeRows {
			neighborIDs[e.SourceID] = true
			neighborIDs[e.TargetID] = true
			edges = append(edges, map[string]any{
				"source": e.SourceID,
				"target": e.TargetID,
				"weight": e.Weight,
				"type":   e.EdgeType,
			})
		}

		var nodes []map[string]any
		for nid := range neighborIDs {
			concept, _ := db.GetConceptNodeByID(nid)
			if concept != nil {
				nodes = append(nodes, map[string]any{
					"id":      concept.ID,
					"label":   ptrOr(concept.Label, "Concept"),
					"level":   concept.Level,
					"type":    "concept",
					"size":    15,
					"focused": concept.ID == nodeID,
				})
			} else {
				chunk, _ := db.GetChunk(nid)
				if chunk != nil {
					ann, _ := db.GetCurrentAnnotation(nid)
					node := map[string]any{
						"id":      nid,
						"label":   truncate(chunk.ChunkText, 50) + "...",
						"level":   99,
						"type":    "chunk",
						"size":    5,
						"focused": nid == nodeID,
					}
					if ann != nil {
						node["summary"] = ann.Summary
					}
					nodes = append(nodes, node)
				}
			}
		}

		if nodes == nil {
			nodes = []map[string]any{}
		}
		if edges == nil {
			edges = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"focused_node": nodeID,
			"nodes":        nodes,
			"edges":        edges,
		})
	})

	return r
}

func ptrOr(p *string, def string) string {
	if p != nil {
		return *p
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
