package lmstudio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Client communicates with LM Studio's OpenAI-compatible API.
type Client struct {
	baseURL       string // e.g. http://127.0.0.1:1234/v1
	rootURL       string // e.g. http://127.0.0.1:1234 (for native API)
	httpClient    *http.Client
	contextLength *int // cached after first query
}

func NewClient(baseURL string, timeout float64) *Client {
	root := strings.TrimRight(baseURL, "/")
	root = strings.TrimSuffix(root, "/v1")
	return &Client{
		baseURL: baseURL,
		rootURL: root,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout * float64(time.Second)),
		},
	}
}

// -- Model types --

type modelEntry struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

type modelsResponse struct {
	Data []modelEntry `json:"data"`
}

type embeddingItem struct {
	Embedding []float64 `json:"embedding"`
}

type embeddingsResponse struct {
	Data []embeddingItem `json:"data"`
}

// ChatMessage represents a chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatMsg creates a ChatMessage.
func ChatMsg(role, content string) ChatMessage {
	return ChatMessage{Role: role, Content: content}
}

type chatChoice struct {
	Message ChatMessage `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

// HealthCheck returns true if LM Studio has at least one model loaded.
func (c *Client) HealthCheck() bool {
	models := c.ListModels()
	return len(models) > 0
}

// ListModels returns all loaded models.
func (c *Client) ListModels() []modelEntry {
	resp, err := c.httpClient.Get(c.baseURL + "/models")
	if err != nil {
		slog.Warn("LM Studio health check failed", "error", err)
		return nil
	}
	defer resp.Body.Close()

	var result modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Warn("Failed to decode models response", "error", err)
		return nil
	}
	return result.Data
}

// GetContextLength queries LM Studio's native API for the loaded context window size.
func (c *Client) GetContextLength(modelID *string) int {
	if c.contextLength != nil {
		return *c.contextLength
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.rootURL + "/api/v0/models")
	if err != nil {
		slog.Warn("Failed to query context length", "error", err)
		fallback := 4096
		c.contextLength = &fallback
		return fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fallback := 4096
		c.contextLength = &fallback
		return fallback
	}

	var result struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fallback := 4096
		c.contextLength = &fallback
		return fallback
	}

	target := ""
	if modelID != nil {
		target = *modelID
	} else if m := c.GetChatModel(); m != nil {
		target = *m
	}

	for _, m := range result.Data {
		if id, ok := m["id"].(string); ok && id == target {
			ctx := getContextFromModel(m)
			c.contextLength = &ctx
			slog.Info("LM Studio context window", "model", target, "tokens", ctx)
			return ctx
		}
	}
	// Fallback: first LLM model
	for _, m := range result.Data {
		if t, ok := m["type"].(string); ok && t == "llm" {
			ctx := getContextFromModel(m)
			c.contextLength = &ctx
			return ctx
		}
	}

	fallback := 4096
	c.contextLength = &fallback
	return fallback
}

func getContextFromModel(m map[string]any) int {
	if v, ok := m["loaded_context_length"].(float64); ok && v > 0 {
		return int(v)
	}
	if v, ok := m["max_context_length"].(float64); ok && v > 0 {
		return int(v)
	}
	return 4096
}

// GetEmbeddingModel returns the first embedding-like model, or first model as fallback.
func (c *Client) GetEmbeddingModel() *string {
	models := c.ListModels()
	embedKeywords := []string{"embed", "e5", "bge", "gte", "nomic"}
	for _, m := range models {
		lower := strings.ToLower(m.ID)
		for _, kw := range embedKeywords {
			if strings.Contains(lower, kw) {
				return &m.ID
			}
		}
	}
	if len(models) > 0 {
		return &models[0].ID
	}
	return nil
}

// GetChatModel returns the first non-embedding model.
func (c *Client) GetChatModel() *string {
	models := c.ListModels()
	excludeKeywords := []string{"embed", "e5", "bge", "gte", "nomic", "whisper"}
	for _, m := range models {
		lower := strings.ToLower(m.ID)
		isExcluded := false
		for _, kw := range excludeKeywords {
			if strings.Contains(lower, kw) {
				isExcluded = true
				break
			}
		}
		if !isExcluded {
			return &m.ID
		}
	}
	if len(models) > 0 {
		return &models[0].ID
	}
	return nil
}

// Embed sends texts to the embedding endpoint and returns vectors.
func (c *Client) Embed(texts []string, model *string) ([][]float64, error) {
	if model == nil {
		model = c.GetEmbeddingModel()
	}
	if model == nil {
		return nil, fmt.Errorf("no embedding model available in LM Studio")
	}

	body := map[string]any{
		"model": *model,
		"input": texts,
	}
	payload, _ := json.Marshal(body)

	resp, err := c.httpClient.Post(c.baseURL+"/embeddings", "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed failed (status %d): %s", resp.StatusCode, string(b))
	}

	var result embeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embeddings: %w", err)
	}

	vectors := make([][]float64, len(result.Data))
	for i, item := range result.Data {
		vectors[i] = item.Embedding
	}
	return vectors, nil
}

// EmbedSingle embeds a single text.
func (c *Client) EmbedSingle(text string, model *string) ([]float64, error) {
	vecs, err := c.Embed([]string{text}, model)
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return vecs[0], nil
}

// Chat sends messages to the chat completions endpoint.
func (c *Client) Chat(messages []ChatMessage, model *string, temperature float64, maxTokens int) (string, error) {
	if model == nil {
		model = c.GetChatModel()
	}
	if model == nil {
		return "", fmt.Errorf("no chat model available in LM Studio")
	}

	body := map[string]any{
		"model":       *model,
		"messages":    messages,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}
	payload, _ := json.Marshal(body)

	resp, err := c.httpClient.Post(c.baseURL+"/chat/completions", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat failed (status %d): %s", resp.StatusCode, string(b))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", nil
	}

	text := result.Choices[0].Message.Content
	// Strip <think>...</think> blocks from thinking models
	if strings.Contains(text, "</think>") {
		parts := strings.SplitN(text, "</think>", 2)
		text = strings.TrimSpace(parts[1])
	} else if strings.HasPrefix(text, "<think>") {
		text = ""
	}

	return text, nil
}

// AnnotateChunk sends a chunk to the LLM for annotation, with context-aware truncation.
func (c *Client) AnnotateChunk(chunkText, promptTemplate string, model *string) (string, error) {
	ctx := c.GetContextLength(model)
	maxChunkChars := max(400, (ctx-2000)*3)

	truncated := chunkText
	if len(truncated) > maxChunkChars {
		truncated = truncated[:maxChunkChars]
	}

	messages := []ChatMessage{
		{Role: "system", Content: promptTemplate},
		{Role: "user", Content: truncated},
	}

	raw, err := c.Chat(messages, model, 0.1, 2048)
	if err != nil {
		return "", err
	}

	// Strip markdown code fences
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

	return text, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
