package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/domain/ai"
)

const (
	anthropicBaseURL = "https://api.anthropic.com/v1"
	anthropicVersion = "2023-06-01"
	anthropicTimeout = 120 * time.Second
)

// AnthropicProvider implements LLMProvider for Anthropic Claude.
type AnthropicProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: anthropicTimeout,
		},
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "Anthropic"
}

// Model returns the current model name.
func (p *AnthropicProvider) Model() string {
	return p.model
}

// Chat sends a chat completion request.
func (p *AnthropicProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	anthropicReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: req.MaxTokens,
		Messages:  make([]anthropicMessage, 0, len(req.Messages)),
	}

	if req.MaxTokens == 0 {
		anthropicReq.MaxTokens = 4096 // Anthropic requires max_tokens
	}

	// Handle system message separately (Anthropic API requirement)
	for _, msg := range req.Messages {
		if msg.Role == ai.RoleSystem {
			anthropicReq.System = msg.Content
		} else {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicBaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract content from the response
	var content string
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &ai.ChatResponse{
		Content:      content,
		FinishReason: anthropicResp.StopReason,
		Usage: ai.TokenUsage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *AnthropicProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatStreamEvent, error) {
	anthropicReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: req.MaxTokens,
		Messages:  make([]anthropicMessage, 0, len(req.Messages)),
		Stream:    true,
	}

	if req.MaxTokens == 0 {
		anthropicReq.MaxTokens = 4096
	}

	for _, msg := range req.Messages {
		if msg.Role == ai.RoleSystem {
			anthropicReq.System = msg.Content
		} else {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			})
		}
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicBaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}

	if err := p.handleErrorResponse(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	events := make(chan ai.ChatStreamEvent)
	go func() {
		defer resp.Body.Close()
		defer close(events)

		reader := resp.Body
		buf := make([]byte, 4096)
		var partial string

		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					select {
					case events <- ai.ChatStreamEvent{Error: err}:
					case <-ctx.Done():
					}
				}
				return
			}

			partial += string(buf[:n])
			lines := strings.Split(partial, "\n")

			partial = lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				data := strings.TrimPrefix(line, "data: ")
				var streamEvent anthropicStreamEvent
				if err := json.Unmarshal([]byte(data), &streamEvent); err != nil {
					continue
				}

				switch streamEvent.Type {
				case "content_block_delta":
					event := ai.ChatStreamEvent{
						Content: streamEvent.Delta.Text,
					}
					select {
					case events <- event:
					case <-ctx.Done():
						return
					}
				case "message_stop":
					event := ai.ChatStreamEvent{
						Done: true,
					}
					select {
					case events <- event:
					case <-ctx.Done():
						return
					}
					return
				}
			}
		}
	}()

	return events, nil
}

// HealthCheck verifies the Anthropic API is accessible.
func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// Anthropic doesn't have a simple health check endpoint, so we send a minimal request
	req := ai.ChatRequest{
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "hi"},
		},
		MaxTokens: 1,
	}

	_, err := p.Chat(ctx, req)
	if err != nil {
		if err == ai.ErrInvalidAPIKey {
			return err
		}
		// Ignore other errors for health check, just verify connectivity
		return nil
	}
	return nil
}

func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
}

func (p *AnthropicProvider) handleErrorResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ai.ErrInvalidAPIKey
	case http.StatusTooManyRequests:
		return ai.ErrRateLimitExceeded
	default:
		return fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}
}

// Anthropic API types

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}
