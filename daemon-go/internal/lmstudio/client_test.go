package lmstudio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheckConnected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(modelsResponse{
				Data: []modelEntry{{ID: "test-model", Object: "model"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", 10)
	if !client.HealthCheck() {
		t.Error("expected HealthCheck to return true")
	}
}

func TestHealthCheckUnavailable(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", 1) // nothing on port 1
	if client.HealthCheck() {
		t.Error("expected HealthCheck to return false")
	}
}

func TestGetEmbeddingModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(modelsResponse{
			Data: []modelEntry{
				{ID: "qwen3-4b", Object: "model"},
				{ID: "nomic-embed-text-v1.5", Object: "model"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", 10)
	model := client.GetEmbeddingModel()
	if model == nil || *model != "nomic-embed-text-v1.5" {
		t.Errorf("expected nomic-embed-text-v1.5, got %v", model)
	}
}

func TestGetChatModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(modelsResponse{
			Data: []modelEntry{
				{ID: "nomic-embed-text-v1.5", Object: "model"},
				{ID: "qwen3-4b-2507", Object: "model"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", 10)
	model := client.GetChatModel()
	if model == nil || *model != "qwen3-4b-2507" {
		t.Errorf("expected qwen3-4b-2507, got %v", model)
	}
}

func TestEmbed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(modelsResponse{
				Data: []modelEntry{{ID: "test-embed", Object: "model"}},
			})
			return
		}
		if r.URL.Path == "/v1/embeddings" {
			json.NewEncoder(w).Encode(embeddingsResponse{
				Data: []embeddingItem{
					{Embedding: []float64{0.1, 0.2, 0.3}},
					{Embedding: []float64{0.4, 0.5, 0.6}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", 10)
	vecs, err := client.Embed([]string{"hello", "world"}, nil)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Errorf("expected 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) != 3 {
		t.Errorf("expected 3 dims, got %d", len(vecs[0]))
	}
}

func TestChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(modelsResponse{
				Data: []modelEntry{{ID: "test-chat", Object: "model"}},
			})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []chatChoice{
					{Message: ChatMessage{Role: "assistant", Content: `{"topics": ["test"]}`}},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", 10)
	msgs := []ChatMessage{{Role: "user", Content: "hello"}}
	result, err := client.Chat(msgs, nil, 0.1, 2048)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result != `{"topics": ["test"]}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestChatThinkStripping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(modelsResponse{
				Data: []modelEntry{{ID: "test-chat", Object: "model"}},
			})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []chatChoice{
					{Message: ChatMessage{
						Role:    "assistant",
						Content: "<think>reasoning here</think>The actual response",
					}},
				},
			})
			return
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", 10)
	msgs := []ChatMessage{{Role: "user", Content: "hello"}}
	result, err := client.Chat(msgs, nil, 0.1, 2048)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result != "The actual response" {
		t.Errorf("expected 'The actual response', got '%s'", result)
	}
}

func TestAnnotateChunkCodeFenceStripping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			json.NewEncoder(w).Encode(modelsResponse{
				Data: []modelEntry{{ID: "test-chat", Object: "model"}},
			})
			return
		}
		if r.URL.Path == "/v1/chat/completions" {
			json.NewEncoder(w).Encode(chatResponse{
				Choices: []chatChoice{
					{Message: ChatMessage{
						Role:    "assistant",
						Content: "```json\n{\"topics\": [\"ai\"]}\n```",
					}},
				},
			})
			return
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", 10)
	ctx := 4096
	client.contextLength = &ctx
	result, err := client.AnnotateChunk("some text", "analyze this", nil)
	if err != nil {
		t.Fatalf("AnnotateChunk: %v", err)
	}
	if result != `{"topics": ["ai"]}` {
		t.Errorf("expected stripped JSON, got '%s'", result)
	}
}
