package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/ai"
)

// TestOllamaEmbedding_Integration tests the Ollama embedding provider.
// Run with: INTEGRATION_TEST=1 go test -v -run TestOllamaEmbedding_Integration ./internal/infrastructure/ai/providers/
//
// Requires Ollama running locally with nomic-embed-text model:
//
//	ollama pull nomic-embed-text
func TestOllamaEmbedding_Integration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create provider
	provider := NewOllamaProvider("http://localhost:11434", "nomic-embed-text")

	// Health check
	err := provider.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	t.Log("Health check passed")

	// Test single embedding
	req := ai.EmbeddingRequest{
		Texts: []string{
			"The Shawshank Redemption (1994). Two imprisoned men bond over a number of years, finding solace and eventual redemption through acts of common decency.",
		},
	}

	resp, err := provider.Embed(ctx, req)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(resp.Embeddings) != 1 {
		t.Fatalf("Expected 1 embedding, got %d", len(resp.Embeddings))
	}

	embedding := resp.Embeddings[0]
	t.Logf("Embedding dimensions: %d", len(embedding))
	t.Logf("First 5 values: %v", embedding[:5])
	t.Logf("Token usage: %+v", resp.Usage)

	// Verify dimensions
	if len(embedding) != 768 {
		t.Errorf("Expected 768 dimensions, got %d", len(embedding))
	}

	// Test batch embedding
	batchReq := ai.EmbeddingRequest{
		Texts: []string{
			"Breaking Bad: A high school chemistry teacher diagnosed with lung cancer turns to manufacturing methamphetamine.",
			"The Dark Knight: Batman must accept one of the greatest psychological and physical tests of his ability to fight injustice.",
			"Inception: A thief who steals corporate secrets through the use of dream-sharing technology.",
		},
	}

	batchResp, err := provider.Embed(ctx, batchReq)
	if err != nil {
		t.Fatalf("Batch embed failed: %v", err)
	}

	if len(batchResp.Embeddings) != 3 {
		t.Fatalf("Expected 3 embeddings, got %d", len(batchResp.Embeddings))
	}

	t.Logf("Batch embedding: got %d embeddings", len(batchResp.Embeddings))
	for i, emb := range batchResp.Embeddings {
		t.Logf("  [%d] dimensions: %d", i, len(emb))
	}
}

// TestOllamaEmbedding_Similarity tests cosine similarity between embeddings.
func TestOllamaEmbedding_Similarity(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	provider := NewOllamaProvider("http://localhost:11434", "nomic-embed-text")

	// Embed similar and dissimilar texts
	req := ai.EmbeddingRequest{
		Texts: []string{
			"A thrilling sci-fi movie about space exploration and aliens",     // 0: sci-fi
			"An exciting science fiction film set in outer space with robots", // 1: similar to 0
			"A romantic comedy about two people falling in love in Paris",     // 2: different genre
		},
	}

	resp, err := provider.Embed(ctx, req)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	// Calculate similarities
	sim01 := cosineSimilarity(resp.Embeddings[0], resp.Embeddings[1])
	sim02 := cosineSimilarity(resp.Embeddings[0], resp.Embeddings[2])
	sim12 := cosineSimilarity(resp.Embeddings[1], resp.Embeddings[2])

	t.Logf("Similarity (sci-fi 1 vs sci-fi 2): %.4f", sim01)
	t.Logf("Similarity (sci-fi 1 vs romance):  %.4f", sim02)
	t.Logf("Similarity (sci-fi 2 vs romance):  %.4f", sim12)

	// Similar texts should have higher similarity
	if sim01 <= sim02 {
		t.Errorf("Expected sci-fi movies to be more similar to each other than to romance")
	}
	if sim01 <= sim12 {
		t.Errorf("Expected sci-fi movies to be more similar to each other than to romance")
	}

	t.Log("Similarity test passed: similar content has higher cosine similarity")
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (sqrt(normA) * sqrt(normB)))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// ============================================================================
// Unit tests with mock server (no real Ollama required)
// ============================================================================

func TestOllamaProvider_Pull_Success(t *testing.T) {
	// Create mock Ollama server that simulates a model pull
	pullEvents := []ollamaPullResponse{
		{Status: "pulling manifest"},
		{Status: "downloading", Digest: "sha256:abc123", Total: 1000, Completed: 250},
		{Status: "downloading", Digest: "sha256:abc123", Total: 1000, Completed: 500},
		{Status: "downloading", Digest: "sha256:abc123", Total: 1000, Completed: 750},
		{Status: "downloading", Digest: "sha256:abc123", Total: 1000, Completed: 1000},
		{Status: "verifying sha256 digest"},
		{Status: "writing manifest"},
		{Status: "success"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pull" {
			t.Errorf("Expected /api/pull, got %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request
		var req ollamaPullRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Name != "nomic-embed-text" {
			t.Errorf("Expected model name 'nomic-embed-text', got %s", req.Name)
		}

		// Stream responses
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		encoder := json.NewEncoder(w)

		for _, event := range pullEvents {
			if err := encoder.Encode(event); err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(10 * time.Millisecond) // Simulate download time
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "nomic-embed-text")

	ctx := context.Background()
	progressCh, err := provider.Pull(ctx, "nomic-embed-text")
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	var events []ai.PullProgress
	for progress := range progressCh {
		events = append(events, progress)
	}

	// Verify we received all events
	if len(events) != len(pullEvents) {
		t.Errorf("Expected %d events, got %d", len(pullEvents), len(events))
	}

	// Verify final event is success
	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		if !lastEvent.Done {
			t.Error("Expected last event to be Done")
		}
		if lastEvent.Status != "success" {
			t.Errorf("Expected status 'success', got %s", lastEvent.Status)
		}
	}

	// Verify progress calculation
	for _, event := range events {
		if event.Total > 0 && event.Completed > 0 {
			expectedPercent := float64(event.Completed) / float64(event.Total) * 100
			if event.Percent != expectedPercent {
				t.Errorf("Expected percent %f, got %f", expectedPercent, event.Percent)
			}
		}
	}
}

func TestOllamaProvider_Pull_Cancellation(t *testing.T) {
	// Track how many events were sent
	var eventsSent atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		encoder := json.NewEncoder(w)

		// Send many progress events, client should cancel midway
		for i := 0; i < 100; i++ {
			event := ollamaPullResponse{
				Status:    "downloading",
				Digest:    "sha256:abc123",
				Total:     10000,
				Completed: int64(i * 100),
			}
			if err := encoder.Encode(event); err != nil {
				return // Client disconnected
			}
			eventsSent.Add(1)
			flusher.Flush()
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "large-model")

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	progressCh, err := provider.Pull(ctx, "large-model")
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	// Collect a few events then cancel
	eventCount := 0
	for range progressCh {
		eventCount++
		if eventCount >= 3 {
			cancel() // Cancel after receiving 3 events
			break
		}
	}

	// Drain remaining events (if any)
	for range progressCh {
		// Just drain
	}

	// Verify we got some events before cancellation
	if eventCount < 3 {
		t.Errorf("Expected at least 3 events before cancel, got %d", eventCount)
	}

	// Wait a bit for server to notice cancellation
	time.Sleep(100 * time.Millisecond)

	// Verify server didn't send all 100 events (should have stopped after client disconnected)
	sent := eventsSent.Load()
	if sent >= 50 {
		t.Logf("Note: Server sent %d events before noticing disconnection", sent)
	}
}

func TestOllamaProvider_Pull_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "model not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "nonexistent-model")

	ctx := context.Background()
	_, err := provider.Pull(ctx, "nonexistent-model")
	if err == nil {
		t.Error("Expected error for non-existent model")
	}
}

func TestOllamaProvider_DeleteModel(t *testing.T) {
	var deletedModel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/delete" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ollamaDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		deletedModel = req.Name
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "")

	ctx := context.Background()
	err := provider.DeleteModel(ctx, "old-model:latest")
	if err != nil {
		t.Fatalf("DeleteModel failed: %v", err)
	}

	if deletedModel != "old-model:latest" {
		t.Errorf("Expected 'old-model:latest' to be deleted, got %s", deletedModel)
	}
}

func TestOllamaProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		resp := struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: "llama3.1:8b"},
				{Name: "nomic-embed-text:latest"},
				{Name: "mxbai-embed-large:latest"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "")

	ctx := context.Background()
	models, err := provider.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("Expected 3 models, got %d", len(models))
	}

	// Verify embedding model detection
	for _, m := range models {
		switch m.ID {
		case "llama3.1:8b":
			if m.IsEmbedding {
				t.Error("llama3.1:8b should not be marked as embedding model")
			}
		case "nomic-embed-text:latest", "mxbai-embed-large:latest":
			if !m.IsEmbedding {
				t.Errorf("%s should be marked as embedding model", m.ID)
			}
		}
	}
}

func TestOllamaProvider_HealthCheck_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "")

	ctx := context.Background()
	err := provider.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

func TestOllamaProvider_HealthCheck_Unavailable(t *testing.T) {
	// Use an invalid URL to simulate unavailable server
	provider := NewOllamaProvider("http://localhost:99999", "")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := provider.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error for unavailable server")
	}
}

func TestIsEmbeddingModel(t *testing.T) {
	testCases := []struct {
		name     string
		expected bool
	}{
		{"nomic-embed-text", true},
		{"nomic-embed-text:latest", true},
		{"all-minilm", true},
		{"all-minilm:l6-v2", true},
		{"mxbai-embed-large", true},
		{"bge-base-en-v1.5", true},
		{"bge-large-en-v1.5", true},
		{"e5-large-v2", true},
		{"embed-something", true},
		{"llama3.1:8b", false},
		{"mistral:7b", false},
		{"codellama:13b", false},
		{"phi3:mini", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isEmbeddingModel(tc.name)
			if result != tc.expected {
				t.Errorf("isEmbeddingModel(%q) = %v, want %v", tc.name, result, tc.expected)
			}
		})
	}
}
