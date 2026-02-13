package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oho/knowledge-refinery-daemon/internal/config"
	"github.com/oho/knowledge-refinery-daemon/internal/storage"
)

func TestCountTokens(t *testing.T) {
	n := CountTokens("Hello world, this is a test.")
	if n <= 0 {
		t.Errorf("expected positive token count, got %d", n)
	}
}

func TestNormalizeText(t *testing.T) {
	got := NormalizeText("  Hello   World  ")
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestComputeChunkIDDeterministic(t *testing.T) {
	id1 := ComputeChunkID("asset1", `{"asset_id":"asset1"}`, "Hello world")
	id2 := ComputeChunkID("asset1", `{"asset_id":"asset1"}`, "Hello world")
	if id1 != id2 {
		t.Errorf("chunk IDs should be deterministic: %s != %s", id1, id2)
	}
	if len(id1) != 32 {
		t.Errorf("chunk ID should be 32 chars, got %d", len(id1))
	}
}

func TestComputeChunkIDCaseInsensitive(t *testing.T) {
	id1 := ComputeChunkID("a", "", "Hello World")
	id2 := ComputeChunkID("a", "", "hello world")
	if id1 != id2 {
		t.Errorf("chunk IDs should be case-insensitive: %s != %s", id1, id2)
	}
}

func TestChunkerSmallText(t *testing.T) {
	cfg := config.PipelineConfig{
		ChunkTargetTokens:  512,
		ChunkMinTokens:     50,
		ChunkMaxTokens:     1024,
		ChunkOverlapTokens: 50,
		Version:            "test",
	}
	chunker := NewChunker(cfg)

	text := "This is a short text that should not be split."
	atom := storage.NewContentAtom("atom1", "asset1", storage.AtomText, 0, `{"asset_id":"asset1"}`)
	atom.PayloadText = &text

	chunks := chunker.ChunkAtoms([]storage.ContentAtom{atom}, "asset1")
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for short text, got %d", len(chunks))
	}
	if chunks[0].ChunkText != text {
		t.Errorf("chunk text mismatch")
	}
}

func TestChunkerLongText(t *testing.T) {
	cfg := config.PipelineConfig{
		ChunkTargetTokens:  50,
		ChunkMinTokens:     10,
		ChunkMaxTokens:     100,
		ChunkOverlapTokens: 10,
		Version:            "test",
	}
	chunker := NewChunker(cfg)

	// Generate a long text with many distinct sentences
	var sentences []string
	for i := 0; i < 50; i++ {
		sentences = append(sentences, fmt.Sprintf("This is sentence number %d and it has quite a few words in it to reach a reasonable token count.", i))
	}
	text := strings.Join(sentences, " ")

	atom := storage.NewContentAtom("atom1", "asset1", storage.AtomText, 0, `{"asset_id":"asset1"}`)
	atom.PayloadText = &text

	chunks := chunker.ChunkAtoms([]storage.ContentAtom{atom}, "asset1")
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for long text, got %d", len(chunks))
	}

	// Each chunk should have a unique ID
	ids := make(map[string]bool)
	for _, c := range chunks {
		if ids[c.ID] {
			t.Errorf("duplicate chunk ID: %s", c.ID)
		}
		ids[c.ID] = true
		if c.TokenCount <= 0 {
			t.Errorf("chunk should have positive token count")
		}
		if c.AssetID != "asset1" {
			t.Errorf("unexpected asset ID: %s", c.AssetID)
		}
	}
}

func TestChunkerSkipsNonTextAtoms(t *testing.T) {
	cfg := config.PipelineConfig{
		ChunkTargetTokens:  512,
		ChunkMinTokens:     50,
		ChunkMaxTokens:     1024,
		ChunkOverlapTokens: 50,
		Version:            "test",
	}
	chunker := NewChunker(cfg)

	atom := storage.NewContentAtom("atom1", "asset1", storage.AtomImage, 0, `{}`)
	chunks := chunker.ChunkAtoms([]storage.ContentAtom{atom}, "asset1")
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for image atom, got %d", len(chunks))
	}
}

func TestChunkerAdaptToContext(t *testing.T) {
	cfg := config.PipelineConfig{
		ChunkTargetTokens:  512,
		ChunkMinTokens:     50,
		ChunkMaxTokens:     1024,
		ChunkOverlapTokens: 50,
		Version:            "test",
	}
	chunker := NewChunker(cfg)

	// Small context window should reduce chunk sizes
	chunker.AdaptToContext(3000)
	if chunker.max >= 1024 {
		t.Errorf("max should decrease with small context, got %d", chunker.max)
	}
}
