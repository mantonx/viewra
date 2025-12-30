// AIClient provides typed access to AI capabilities (embedding, chat).
//
// This client wraps the low-level Invoke calls and handles proto marshaling
// internally, so plugin code doesn't need to import proto or pluginv1.
//
// # Usage
//
//	ai := sdk.NewAIClient(plugins)
//
//	// Generate embeddings
//	embedding, err := ai.GenerateEmbedding(ctx, "hello world")
//
//	// Chat completion
//	resp, err := ai.Chat(ctx, sdk.ChatRequest{
//	    Messages: []sdk.ChatMessage{
//	        {Role: "user", Content: "Hello!"},
//	    },
//	})
package sdk

import (
	"context"
	"fmt"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/protobuf/proto"
)

// AIClient provides typed access to AI capabilities.
// It wraps PluginsClient and handles proto marshaling internally.
type AIClient struct {
	plugins *PluginsClient
}

// NewAIClient creates a new AI client from a PluginsClient.
// Returns nil if plugins is nil.
func NewAIClient(plugins *PluginsClient) *AIClient {
	if plugins == nil {
		return nil
	}
	return &AIClient{plugins: plugins}
}

// --- Embedding Methods ---

// GenerateEmbedding generates an embedding vector for a single text.
// Uses the "embedding" capability.
//
// Example:
//
//	embedding, err := ai.GenerateEmbedding(ctx, "hello world")
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Embedding dimensions: %d\n", len(embedding))
func (c *AIClient) GenerateEmbedding(ctx context.Context, text string, opts ...InvokeOption) ([]float32, error) {
	return c.GenerateEmbeddingWithModel(ctx, text, "", opts...)
}

// GenerateEmbeddingWithModel generates an embedding using a specific model.
func (c *AIClient) GenerateEmbeddingWithModel(ctx context.Context, text, model string, opts ...InvokeOption) ([]float32, error) {
	if c.plugins == nil {
		return nil, fmt.Errorf("plugins client not available")
	}

	req := &pluginv1.ProviderEmbeddingRequest{
		Text:  text,
		Model: model,
	}

	respBytes, _, err := c.plugins.Invoke(ctx, "embedding", "GenerateEmbedding", req, opts...)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	var resp pluginv1.ProviderEmbeddingResponse
	if err := proto.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}

	return resp.Embedding, nil
}

// GenerateEmbeddingBatch generates embeddings for multiple texts.
// Uses the "embedding" capability.
//
// Example:
//
//	embeddings, err := ai.GenerateEmbeddingBatch(ctx, []string{"hello", "world"})
func (c *AIClient) GenerateEmbeddingBatch(ctx context.Context, texts []string, opts ...InvokeOption) ([][]float32, error) {
	return c.GenerateEmbeddingBatchWithModel(ctx, texts, "", opts...)
}

// GenerateEmbeddingBatchWithModel generates embeddings using a specific model.
func (c *AIClient) GenerateEmbeddingBatchWithModel(ctx context.Context, texts []string, model string, opts ...InvokeOption) ([][]float32, error) {
	if c.plugins == nil {
		return nil, fmt.Errorf("plugins client not available")
	}

	req := &pluginv1.ProviderEmbeddingBatchRequest{
		Texts: texts,
		Model: model,
	}

	respBytes, _, err := c.plugins.Invoke(ctx, "embedding", "GenerateEmbeddingBatch", req, opts...)
	if err != nil {
		return nil, fmt.Errorf("generate embeddings: %w", err)
	}

	var resp pluginv1.ProviderEmbeddingBatchResponse
	if err := proto.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal embedding batch response: %w", err)
	}

	embeddings := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		embeddings[i] = emb.Embedding
	}

	return embeddings, nil
}

// --- Chat Methods ---

// Chat sends a chat completion request.
// Uses the "chat" capability.
//
// Example:
//
//	resp, err := ai.Chat(ctx, sdk.ChatRequest{
//	    Messages: []sdk.ChatMessage{
//	        {Role: "system", Content: "You are a helpful assistant."},
//	        {Role: "user", Content: "Hello!"},
//	    },
//	    Temperature: 0.7,
//	})
func (c *AIClient) Chat(ctx context.Context, req ChatRequest, opts ...InvokeOption) (*ChatResponse, error) {
	if c.plugins == nil {
		return nil, fmt.Errorf("plugins client not available")
	}

	// Convert SDK types to proto
	messages := make([]*pluginv1.ProviderChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = &pluginv1.ProviderChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	protoReq := &pluginv1.ProviderChatRequest{
		Messages:    messages,
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int32(req.MaxTokens),
	}

	respBytes, _, err := c.plugins.Invoke(ctx, "chat", "Chat", protoReq, opts...)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}

	var protoResp pluginv1.ProviderChatResponse
	if err := proto.Unmarshal(respBytes, &protoResp); err != nil {
		return nil, fmt.Errorf("unmarshal chat response: %w", err)
	}

	return &ChatResponse{
		Content:          protoResp.Content,
		FinishReason:     protoResp.FinishReason,
		PromptTokens:     int(protoResp.PromptTokens),
		CompletionTokens: int(protoResp.CompletionTokens),
	}, nil
}

// ChatStream sends a streaming chat completion request.
// Returns a channel that receives response chunks until closed.
//
// Example:
//
//	chunks, err := ai.ChatStream(ctx, sdk.ChatRequest{...})
//	for chunk := range chunks {
//	    if chunk.Done {
//	        break
//	    }
//	    fmt.Print(chunk.Content)
//	}
func (c *AIClient) ChatStream(ctx context.Context, req ChatRequest, opts ...InvokeOption) (<-chan ChatChunk, error) {
	if c.plugins == nil {
		return nil, fmt.Errorf("plugins client not available")
	}

	// Convert SDK types to proto
	messages := make([]*pluginv1.ProviderChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = &pluginv1.ProviderChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	protoReq := &pluginv1.ProviderChatRequest{
		Messages:    messages,
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int32(req.MaxTokens),
	}

	streamCh, err := c.plugins.InvokeStream(ctx, "chat", "ChatStream", protoReq, opts...)
	if err != nil {
		return nil, fmt.Errorf("chat stream: %w", err)
	}

	ch := make(chan ChatChunk)
	go func() {
		defer close(ch)
		for chunk := range streamCh {
			if chunk.Error != nil {
				// Signal error through channel - caller should check Done and handle
				ch <- ChatChunk{Done: true, FinishReason: "error"}
				return
			}

			var protoChunk pluginv1.ProviderChatStreamChunk
			if err := proto.Unmarshal(chunk.Payload, &protoChunk); err != nil {
				ch <- ChatChunk{Done: true, FinishReason: "error"}
				return
			}

			ch <- ChatChunk{
				Content:      protoChunk.Content,
				Done:         protoChunk.Done,
				FinishReason: protoChunk.FinishReason,
			}
		}
	}()

	return ch, nil
}

// --- Availability Checks ---

// IsEmbeddingAvailable checks if any embedding provider is available.
func (c *AIClient) IsEmbeddingAvailable(ctx context.Context) bool {
	if c.plugins == nil {
		return false
	}
	return c.plugins.IsAvailable(ctx, "embedding")
}

// IsChatAvailable checks if any chat provider is available.
func (c *AIClient) IsChatAvailable(ctx context.Context) bool {
	if c.plugins == nil {
		return false
	}
	return c.plugins.IsAvailable(ctx, "chat")
}
