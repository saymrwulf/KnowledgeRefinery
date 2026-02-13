package pipeline

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/oho/knowledge-refinery-daemon/internal/config"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
	"github.com/pkoukk/tiktoken-go"
)

var encoder *tiktoken.Tiktoken

func init() {
	var err error
	encoder, err = tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		slog.Warn("tiktoken cl100k_base unavailable, using word-based estimate")
	}
}

// CountTokens counts tokens using tiktoken, fallback to word-based estimate.
func CountTokens(text string) int {
	if encoder != nil {
		return len(encoder.Encode(text, nil, nil))
	}
	return int(float64(len(strings.Fields(text))) * 1.33)
}

// NormalizeText collapses whitespace and lowercases for stable hashing.
func NormalizeText(text string) string {
	return wsRE.ReplaceAllString(strings.TrimSpace(strings.ToLower(text)), " ")
}

var wsRE = regexp.MustCompile(`\s+`)

// ComputeChunkID generates a deterministic chunk ID.
func ComputeChunkID(assetID, anchorJSON, text string) string {
	norm := NormalizeText(text)
	normHash := sha256.Sum256([]byte(norm))
	payload := fmt.Sprintf("%s:%s:%x", assetID, anchorJSON, normHash)
	h := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("%x", h)[:32]
}

// Chunker splits text atoms into chunks with overlap.
type Chunker struct {
	target          int
	min             int
	max             int
	overlap         int
	pipelineVersion string
}

func NewChunker(cfg config.PipelineConfig) *Chunker {
	return &Chunker{
		target:          cfg.ChunkTargetTokens,
		min:             cfg.ChunkMinTokens,
		max:             cfg.ChunkMaxTokens,
		overlap:         cfg.ChunkOverlapTokens,
		pipelineVersion: cfg.Version,
	}
}

// AdaptToContext adjusts chunk sizes based on the LLM's context window.
func (c *Chunker) AdaptToContext(contextLength int) {
	available := contextLength - 2000
	if available < 400 {
		available = 400
	}
	newTarget := available * 6 / 10
	if newTarget > c.target {
		newTarget = c.target
	}
	newMax := available * 8 / 10
	if newMax > c.max {
		newMax = c.max
	}
	newMin := newTarget * 2 / 3
	if newMin > c.min {
		newMin = c.min
	}
	if newMax != c.max {
		slog.Info("Adapted chunk sizes to context",
			"context", contextLength, "target", newTarget, "min", newMin, "max", newMax)
		c.target = newTarget
		c.min = newMin
		c.max = newMax
	}
}

// ChunkAtoms splits all text atoms into Chunk records.
func (c *Chunker) ChunkAtoms(atoms []storage.ContentAtom, assetID string) []storage.Chunk {
	var allChunks []storage.Chunk
	chunkIndex := 0

	for _, atom := range atoms {
		if atom.AtomType != storage.AtomText || atom.PayloadText == nil {
			continue
		}
		text := *atom.PayloadText
		textChunks := c.splitText(text)

		for _, chunkText := range textChunks {
			tokenCount := CountTokens(chunkText)
			chunkID := ComputeChunkID(assetID, atom.EvidenceAnchor, chunkText)

			allChunks = append(allChunks, storage.NewChunk(
				chunkID, atom.ID, assetID, chunkText,
				tokenCount, chunkIndex, atom.EvidenceAnchor, c.pipelineVersion,
			))
			chunkIndex++
		}
	}
	return allChunks
}

// sentenceBoundaryRE matches sentence-ending punctuation followed by whitespace.
// Go's regexp doesn't support lookbehinds, so we match the full pattern and
// reconstruct sentences by keeping the punctuation with the preceding text.
var sentenceBoundaryRE = regexp.MustCompile(`([.!?])\s+`)

func (c *Chunker) splitText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	totalTokens := CountTokens(text)
	if totalTokens <= c.max {
		return []string{text}
	}

	sentences := c.splitSentences(text)
	var chunks []string
	var current []string
	currentTokens := 0

	for _, sentence := range sentences {
		sentTokens := CountTokens(sentence)

		if currentTokens+sentTokens > c.max && len(current) > 0 {
			// Emit current chunk
			chunkText := strings.TrimSpace(strings.Join(current, " "))
			if CountTokens(chunkText) >= c.min {
				chunks = append(chunks, chunkText)
			}

			// Keep overlap
			overlapTokens := 0
			overlapStart := len(current)
			for i := len(current) - 1; i >= 0; i-- {
				st := CountTokens(current[i])
				if overlapTokens+st > c.overlap {
					break
				}
				overlapTokens += st
				overlapStart = i
			}
			current = current[overlapStart:]
			currentTokens = 0
			for _, s := range current {
				currentTokens += CountTokens(s)
			}
		}

		current = append(current, sentence)
		currentTokens += sentTokens
	}

	// Emit final chunk
	if len(current) > 0 {
		chunkText := strings.TrimSpace(strings.Join(current, " "))
		if chunkText != "" {
			chunks = append(chunks, chunkText)
		}
	}

	return chunks
}

func (c *Chunker) splitSentences(text string) []string {
	// Split on sentence boundaries while keeping the punctuation with the sentence.
	// We find all matches of `[.!?]\s+` and use their positions to split.
	indices := sentenceBoundaryRE.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		// No sentence boundaries found — split on newlines or return as-is
		return c.splitLongBlock(text)
	}

	var parts []string
	start := 0
	for _, idx := range indices {
		// idx[0] is the position of the punctuation, idx[1] is after the whitespace
		// Keep the punctuation char with the sentence (split after it)
		end := idx[0] + 1 // include the punctuation
		part := strings.TrimSpace(text[start:end])
		if part != "" {
			parts = append(parts, part)
		}
		start = idx[1] // skip the whitespace
	}
	// Remainder after last match
	if start < len(text) {
		part := strings.TrimSpace(text[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}

	// Further split any sentences that are too long
	var result []string
	for _, part := range parts {
		if CountTokens(part) > c.max {
			result = append(result, c.splitLongBlock(part)...)
		} else {
			result = append(result, part)
		}
	}
	return result
}

func (c *Chunker) splitLongBlock(text string) []string {
	var result []string
	for _, sub := range strings.Split(text, "\n") {
		sub = strings.TrimSpace(sub)
		if sub != "" {
			result = append(result, sub)
		}
	}
	if len(result) == 0 && strings.TrimSpace(text) != "" {
		result = append(result, strings.TrimSpace(text))
	}
	return result
}
