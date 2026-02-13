package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/oho/knowledge-refinery-daemon/internal/lmstudio"
	"github.com/oho/knowledge-refinery-daemon/internal/mathutil"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

// Conceptualizer builds concept clusters from embeddings.
type Conceptualizer struct {
	db              *storage.Database
	vs              *storage.VectorStore
	lm              *lmstudio.Client
	pipelineVersion string
}

func NewConceptualizer(db *storage.Database, vs *storage.VectorStore, lm *lmstudio.Client, pipelineVersion string) *Conceptualizer {
	return &Conceptualizer{db: db, vs: vs, lm: lm, pipelineVersion: pipelineVersion}
}

// BuildConcepts clusters all chunks into concept nodes.
func (c *Conceptualizer) BuildConcepts(level int, nClusters *int) []storage.ConceptNode {
	ids, vectors, texts := c.vs.GetAllVectors()
	if len(ids) == 0 {
		slog.Info("No vectors to cluster")
		return nil
	}

	k := len(ids) / 3
	if k < 2 {
		k = 2
	}
	if k > 20 {
		k = 20
	}
	if nClusters != nil {
		k = *nClusters
	}
	if len(ids) < k {
		k = len(ids)
		if k < 1 {
			k = 1
		}
	}

	slog.Info("Clustering", "chunks", len(ids), "clusters", k, "level", level)

	labels, centroids := mathutil.KMeans(vectors, k, 50)

	var concepts []storage.ConceptNode
	for clusterIdx := 0; clusterIdx < k; clusterIdx++ {
		// Gather members
		var memberIDs []string
		var memberTexts []string
		var memberVecs [][]float32
		for i, lbl := range labels {
			if lbl == clusterIdx {
				memberIDs = append(memberIDs, ids[i])
				memberTexts = append(memberTexts, texts[i])
				memberVecs = append(memberVecs, vectors[i])
			}
		}
		if len(memberIDs) == 0 {
			continue
		}

		conceptID := fmt.Sprintf("%x", sha256.Sum256([]byte(
			fmt.Sprintf("concept:%d:%d:%s", level, clusterIdx, c.pipelineVersion),
		)))[:32]

		// Find 3 exemplars closest to centroid
		exemplarIndices := closestToCentroid(memberVecs, centroids[clusterIdx], 3)
		exemplarIDs := make([]string, len(exemplarIndices))
		for i, idx := range exemplarIndices {
			exemplarIDs[i] = memberIDs[idx]
		}

		// Label the concept via LLM
		label, description := c.labelConcept(memberTexts, exemplarIndices)

		exemplarJSON, _ := json.Marshal(exemplarIDs)
		exemplarStr := string(exemplarJSON)
		chatModel := c.lm.GetChatModel()

		node := storage.ConceptNode{
			ID:               conceptID,
			Level:            level,
			Label:            &label,
			Description:      &description,
			ExemplarChunkIDs: &exemplarStr,
			PipelineVersion:  &c.pipelineVersion,
			ModelID:          chatModel,
			CreatedAt:        storage.NowISO(),
		}
		c.db.InsertConceptNode(node)
		concepts = append(concepts, node)

		// Create concept membership edges
		for _, chunkID := range memberIDs {
			edgeID := fmt.Sprintf("%x", sha256.Sum256([]byte(
				fmt.Sprintf("edge:%s:%s", conceptID, chunkID),
			)))[:32]
			edge := storage.GraphEdge{
				ID:              edgeID,
				SourceID:        conceptID,
				TargetID:        chunkID,
				EdgeType:        "concept_member",
				Weight:          1.0,
				PipelineVersion: &c.pipelineVersion,
				CreatedAt:       storage.NowISO(),
			}
			c.db.InsertGraphEdge(edge)
		}
	}

	slog.Info("Created concept nodes", "count", len(concepts), "level", level)
	return concepts
}

func closestToCentroid(vecs [][]float32, centroid []float32, k int) []int {
	type scored struct {
		idx  int
		dist float64
	}
	var scores []scored
	for i, v := range vecs {
		d := 0.0
		for j := range v {
			diff := float64(v[j]) - float64(centroid[j])
			d += diff * diff
		}
		scores = append(scores, scored{i, d})
	}
	// Sort by distance (ascending)
	for i := range scores {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].dist < scores[i].dist {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
	n := k
	if n > len(scores) {
		n = len(scores)
	}
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = scores[i].idx
	}
	return indices
}

func (c *Conceptualizer) labelConcept(texts []string, exemplarIndices []int) (string, string) {
	var exemplarTexts []string
	for _, i := range exemplarIndices {
		if i < len(texts) {
			t := texts[i]
			if len(t) > 500 {
				t = t[:500]
			}
			exemplarTexts = append(exemplarTexts, t)
		}
	}
	if len(exemplarTexts) == 0 {
		return "Unknown", "No exemplar texts available"
	}

	var sb strings.Builder
	sb.WriteString("Given the following representative text excerpts from a cluster of related documents, ")
	sb.WriteString("provide a concise concept label and description.\n\n")
	sb.WriteString("Respond with a JSON object:\n")
	sb.WriteString("- \"label\": a short (2-5 word) concept label\n")
	sb.WriteString("- \"description\": a 1-2 sentence description of what this concept cluster represents\n")
	sb.WriteString("- \"keywords\": array of 3-7 keywords that characterize this concept\n\n")
	sb.WriteString("Respond with ONLY the JSON object, no other text.\n\nExcerpts:\n")
	for i, t := range exemplarTexts {
		sb.WriteString(fmt.Sprintf("\n--- Excerpt %d ---\n%s\n", i+1, t))
	}

	for attempt := 0; attempt < 3; attempt++ {
		msgs := []lmstudio.ChatMessage{lmstudio.ChatMsg("user", sb.String())}
		raw, err := c.lm.Chat(msgs, nil, 0.1, 2048)
		if err != nil {
			wait := time.Duration(5*(attempt+1)) * time.Second
			slog.Warn("Concept labeling failed", "attempt", attempt+1, "error", err, "wait", wait)
			time.Sleep(wait)
			continue
		}

		text := strings.TrimSpace(raw)
		if strings.HasPrefix(text, "```") {
			lines := strings.Split(text, "\n")
			var filtered []string
			for _, l := range lines {
				if !strings.HasPrefix(strings.TrimSpace(l), "```") {
					filtered = append(filtered, l)
				}
			}
			text = strings.TrimSpace(strings.Join(filtered, "\n"))
		}

		var data struct {
			Label       string `json:"label"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal([]byte(text), &data); err != nil {
			slog.Warn("Failed to parse concept label JSON", "error", err)
			break
		}
		if data.Label == "" {
			data.Label = "Unlabeled"
		}
		return data.Label, data.Description
	}

	// Fallback
	fallback := exemplarTexts[0]
	if len(fallback) > 50 {
		fallback = fallback[:50]
	}
	return "Cluster: " + fallback + "...", "Auto-generated from exemplar text"
}

// BuildSimilarityGraph creates a kNN graph from embeddings.
func (c *Conceptualizer) BuildSimilarityGraph(k int) int {
	ids, vectors, _ := c.vs.GetAllVectors()
	n := len(ids)
	if n < 2 {
		return 0
	}
	if k > n-1 {
		k = n - 1
	}

	slog.Info("Building kNN similarity graph", "chunks", n, "k", k)

	// Normalize all vectors
	normalized := make([][]float32, n)
	for i, v := range vectors {
		normalized[i] = mathutil.Normalize(v)
	}

	edgeCount := 0
	for i := 0; i < n; i++ {
		// Compute similarities
		type scored struct {
			idx int
			sim float64
		}
		var scores []scored
		for j := 0; j < n; j++ {
			if j == i {
				continue
			}
			sim := mathutil.DotProduct(normalized[i], normalized[j])
			if sim > 0 {
				scores = append(scores, scored{j, sim})
			}
		}

		// Sort by similarity descending
		for a := range scores {
			for b := a + 1; b < len(scores); b++ {
				if scores[b].sim > scores[a].sim {
					scores[a], scores[b] = scores[b], scores[a]
				}
			}
		}

		topK := k
		if topK > len(scores) {
			topK = len(scores)
		}

		for _, s := range scores[:topK] {
			edgeID := fmt.Sprintf("%x", sha256.Sum256([]byte(
				fmt.Sprintf("sim:%s:%s", ids[i], ids[s.idx]),
			)))[:32]

			evidenceJSON := fmt.Sprintf(`{"method":"cosine_knn","k":%d}`, k)
			edge := storage.GraphEdge{
				ID:              edgeID,
				SourceID:        ids[i],
				TargetID:        ids[s.idx],
				EdgeType:        "similarity",
				Weight:          s.sim,
				EvidenceJSON:    &evidenceJSON,
				PipelineVersion: &c.pipelineVersion,
				CreatedAt:       storage.NowISO(),
			}
			c.db.InsertGraphEdge(edge)
			edgeCount++
		}
	}

	slog.Info("Created similarity edges", "count", edgeCount)
	return edgeCount
}

// RefineConcept sub-clusters a concept's members.
func (c *Conceptualizer) RefineConcept(conceptID string, nSub int) []storage.ConceptNode {
	node, _ := c.db.GetConceptNodeByID(conceptID)
	if node == nil {
		return nil
	}

	memberIDs, _ := c.db.GetMemberChunkIDs(conceptID)
	if len(memberIDs) < nSub {
		return nil
	}

	// Get vectors for members
	allIDs, allVecs, allTexts := c.vs.GetAllVectors()
	idSet := make(map[string]bool)
	for _, mid := range memberIDs {
		idSet[mid] = true
	}

	var subIDs []string
	var subVecs [][]float32
	var subTexts []string
	for i, id := range allIDs {
		if idSet[id] {
			subIDs = append(subIDs, id)
			subVecs = append(subVecs, allVecs[i])
			subTexts = append(subTexts, allTexts[i])
		}
	}

	if len(subIDs) < nSub {
		return nil
	}

	newLevel := node.Level + 1
	labels, centroids := mathutil.KMeans(subVecs, nSub, 50)

	var subConcepts []storage.ConceptNode
	for clusterIdx := 0; clusterIdx < nSub; clusterIdx++ {
		var memberTexts []string
		var memberVecsForCluster [][]float32
		for i, lbl := range labels {
			if lbl == clusterIdx {
				memberTexts = append(memberTexts, subTexts[i])
				memberVecsForCluster = append(memberVecsForCluster, subVecs[i])
			}
		}
		if len(memberTexts) == 0 {
			continue
		}

		subConceptID := fmt.Sprintf("%x", sha256.Sum256([]byte(
			fmt.Sprintf("concept:%d:%s:%d", newLevel, conceptID, clusterIdx),
		)))[:32]

		exemplarIndices := closestToCentroid(memberVecsForCluster, centroids[clusterIdx], 3)
		label, description := c.labelConcept(memberTexts, exemplarIndices)

		chatModel := c.lm.GetChatModel()
		subNode := storage.ConceptNode{
			ID:              subConceptID,
			Level:           newLevel,
			Label:           &label,
			Description:     &description,
			ParentID:        &conceptID,
			PipelineVersion: &c.pipelineVersion,
			ModelID:         chatModel,
			CreatedAt:       storage.NowISO(),
		}
		c.db.InsertConceptNode(subNode)
		subConcepts = append(subConcepts, subNode)
	}

	slog.Info("Refined concept", "parent", conceptID, "sub_concepts", len(subConcepts))
	return subConcepts
}
