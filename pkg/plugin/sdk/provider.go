// Provider plugin support for ViewRA AI provider plugins.
//
// This file provides the ProviderPlugin interface and ServeProvider() helper
// for building AI provider plugins (like Ollama, OpenAI, Anthropic).
//
// # Quick Start
//
// Create a provider plugin that implements the ProviderPlugin interface:
//
//	type MyProvider struct {
//	    sdk.Base
//	    apiKey string
//	}
//
//	func (p *MyProvider) GetProviderCapabilities() sdk.ProviderCapabilities {
//	    return sdk.ProviderCapabilities{
//	        ProviderID:        "my-provider",
//	        DisplayName:       "My Provider",
//	        SupportsChat:      true,
//	        SupportsEmbedding: true,
//	    }
//	}
//
//	// ... implement other methods
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-provider")
//	    plugin := &MyProvider{}
//	    plugin.SetLogger(logger)
//	    sdk.ServeProvider(plugin, hclogger)
//	}
package sdk

import (
	"context"
	"io"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// ProviderPlugin is the interface that AI provider plugins must implement.
// Providers offer LLM capabilities (embeddings, chat) to other plugins via the host.
//
// Plugin identity comes from the plugin.yml manifest file.
type ProviderPlugin interface {
	// mustEmbedBase ensures plugins embed sdk.Base
	mustEmbedBase()

	// GetProviderCapabilities returns what this provider supports.
	GetProviderCapabilities() ProviderCapabilities

	// Initialize is called when the plugin is loaded.
	// Config contains the contents of config.yml passed by the host.
	Initialize(ctx context.Context, dataDir string, config []byte, systemInfo *SystemInfo) error

	// Shutdown is called before the plugin is unloaded.
	Shutdown(ctx context.Context) error

	// HealthCheck verifies the provider is working (API key valid, service reachable).
	HealthCheck(ctx context.Context) (*ProviderHealth, error)

	// ListModels returns available models from this provider.
	ListModels(ctx context.Context) ([]ProviderModel, error)

	// GenerateEmbedding generates an embedding for a single text.
	GenerateEmbedding(ctx context.Context, text, model string) ([]float32, error)

	// GenerateEmbeddingBatch generates embeddings for multiple texts.
	GenerateEmbeddingBatch(ctx context.Context, texts []string, model string) ([][]float32, error)

	// Chat sends a chat completion request.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// ChatStream sends a streaming chat completion request.
	// Implementations should send chunks to the provided channel and close it when done.
	ChatStream(ctx context.Context, req *ChatRequest, chunks chan<- ChatChunk) error
}

// ConfigurableProvider is an optional interface for providers with settings UI.
type ConfigurableProvider interface {
	// GetSettingsSchema returns the JSON Schema for provider settings.
	// Use Schema.Build() to generate this from your schema definition.
	GetSettingsSchema() ([]byte, error)

	// Configure applies new settings to the provider.
	Configure(settings []byte) error
}

// PluginsClientAware is an optional interface for providers that need
// access to the capability broker for discovering and routing to other plugins.
// Implement this interface to receive a PluginsClient during initialization.
//
// Example use case: The ai-local plugin uses this to set capability preferences
// when configured, routing embedding/chat requests to the selected provider.
type PluginsClientAware interface {
	// SetPluginsClient receives the PluginsClient for capability broker access.
	// Called after Initialize() if the host provides the HostPlugins service.
	SetPluginsClient(client *PluginsClient)
}

// HTTPProvider is an optional interface for providers that expose HTTP routes.
// Implement this for model management endpoints, health checks, etc.
type HTTPProvider interface {
	// GetRoutes returns HTTP routes this provider exposes.
	GetRoutes() []Route

	// HandleHTTP handles a non-streaming HTTP request.
	HandleHTTP(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)

	// HandleHTTPStream handles a streaming HTTP request (e.g., model download).
	// Return nil if not implemented.
	HandleHTTPStream(ctx context.Context, req *HTTPRequest, stream HTTPStreamWriter) error
}

// ProviderCapabilities describes what an AI provider supports.
type ProviderCapabilities struct {
	// ProviderID is a unique identifier, e.g., "ollama", "openai", "anthropic"
	ProviderID string

	// DisplayName is the human-readable name, e.g., "Ollama (Local)"
	DisplayName string

	// Description describes the provider
	Description string

	// SupportsChat indicates if this provider supports chat completions
	SupportsChat bool

	// SupportsEmbedding indicates if this provider supports embedding generation
	SupportsEmbedding bool

	// SupportsStreaming indicates if this provider supports streaming responses
	SupportsStreaming bool

	// RequiresAPIKey indicates if this provider requires an API key
	RequiresAPIKey bool

	// RequiresURL indicates if this provider requires a custom URL (e.g., Ollama)
	RequiresURL bool

	// IsLocal indicates if this provider runs locally (affects UI display)
	IsLocal bool

	// DefaultChatModel is the default model ID for chat
	DefaultChatModel string

	// DefaultEmbeddingModel is the default model ID for embeddings
	DefaultEmbeddingModel string
}

// ProviderModel describes a model available from a provider.
type ProviderModel struct {
	// ID is the model identifier, e.g., "llama3.2", "gpt-4o-mini"
	ID string

	// Name is the human-readable name
	Name string

	// Description describes the model
	Description string

	// IsChat indicates if this is a chat/completion model
	IsChat bool

	// IsEmbedding indicates if this is an embedding model
	IsEmbedding bool

	// ContextLength is the context window size (for chat models)
	ContextLength int

	// EmbeddingDimensions is the embedding vector size (for embedding models)
	EmbeddingDimensions int

	// Size is the model size for display, e.g., "7B", "3.8GB"
	Size string

	// Recommended indicates if this model is recommended
	Recommended bool
}

// ProviderHealth indicates the health of a provider.
type ProviderHealth struct {
	// Healthy indicates if the provider is ready to serve requests
	Healthy bool

	// Message is a human-readable status message
	Message string

	// Latency is the health check latency
	Latency time.Duration

	// Error is the error message if unhealthy
	Error string
}

// ChatRequest contains the parameters for a chat completion.
type ChatRequest struct {
	// Messages is the conversation history
	Messages []ChatMessage

	// Model is the model to use (uses provider default if empty)
	Model string

	// Temperature controls randomness (0.0-2.0)
	Temperature float32

	// MaxTokens limits the response length
	MaxTokens int
}

// ChatMessage represents a message in a chat conversation.
type ChatMessage struct {
	// Role is "system", "user", or "assistant"
	Role string

	// Content is the message text
	Content string
}

// ChatResponse is the result of a chat completion.
type ChatResponse struct {
	// Content is the generated text
	Content string

	// FinishReason indicates why generation stopped: "stop", "length", "content_filter"
	FinishReason string

	// PromptTokens is the number of tokens in the prompt
	PromptTokens int

	// CompletionTokens is the number of tokens in the response
	CompletionTokens int
}

// ChatChunk is a chunk of a streaming chat response.
type ChatChunk struct {
	// Content is the text in this chunk
	Content string

	// Done indicates if this is the final chunk
	Done bool

	// FinishReason is set on the final chunk
	FinishReason string
}

// SystemInfo contains host system resource information.
// Useful for providers that need to recommend models based on available resources.
type SystemInfo struct {
	// RAM in bytes
	RAMBytes uint64

	// Available RAM in bytes
	RAMAvailableBytes uint64

	// Whether the system has a GPU
	HasGPU bool

	// VRAM in bytes
	VRAMBytes uint64

	// GPU name
	GPUName string

	// CPU core count
	CPUCores int

	// CPU model name
	CPUModel string

	// Operating system: "linux", "darwin", "windows"
	OS string

	// Architecture: "amd64", "arm64"
	Arch string
}

// --- gRPC Server Implementations ---
// We use two separate server types to match the proto service definitions:
// - providerCoreServer implements PluginCoreServer (lifecycle, settings, HTTP)
// - providerServiceServer implements PluginProviderServer (AI capabilities)

// providerCoreServer implements PluginCoreServer for provider plugins.
type providerCoreServer struct {
	pluginv1.UnimplementedPluginCoreServer
	impl       ProviderPlugin
	base       *Base
	broker     *plugin.GRPCBroker
	systemInfo *SystemInfo
}

func (s *providerCoreServer) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	s.base.Init(req.DataDir)
	logger := s.base.Log()

	// Convert system info
	if sysInfo := req.GetSystemInfo(); sysInfo != nil {
		s.systemInfo = &SystemInfo{
			RAMBytes:          sysInfo.GetRamBytes(),
			RAMAvailableBytes: sysInfo.GetRamAvailableBytes(),
			HasGPU:            sysInfo.GetHasGpu(),
			VRAMBytes:         sysInfo.GetVramBytes(),
			GPUName:           sysInfo.GetGpuName(),
			CPUCores:          int(sysInfo.GetCpuCores()),
			CPUModel:          sysInfo.GetCpuModel(),
			OS:                sysInfo.GetOs(),
			Arch:              sysInfo.GetArch(),
		}
	}

	if err := s.impl.Initialize(ctx, req.DataDir, req.Config, s.systemInfo); err != nil {
		return &pluginv1.InitResponse{Success: false, Error: err.Error()}, nil
	}

	// Connect to HostPlugins service if available and plugin wants it
	if aware, ok := s.impl.(PluginsClientAware); ok {
		if req.GetHostPluginsBrokerId() > 0 && s.broker != nil {
			conn, err := s.broker.Dial(req.GetHostPluginsBrokerId())
			if err != nil {
				logger.Error("failed to dial host plugins service", "error", err)
			} else {
				pluginsClient := NewPluginsClient(conn, s.broker)
				aware.SetPluginsClient(pluginsClient)
				logger.Debug("connected to host plugins service")
			}
		} else {
			logger.Debug("host plugins service not available")
		}
	}

	return &pluginv1.InitResponse{Success: true}, nil
}

func (s *providerCoreServer) Shutdown(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	if err := s.impl.Shutdown(ctx); err != nil {
		s.base.Log().Error("shutdown error", "error", err)
	}
	return &pluginv1.Empty{}, nil
}

func (s *providerCoreServer) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
	health, err := s.impl.HealthCheck(ctx)
	if err != nil {
		return &pluginv1.HealthStatus{
			Status:  pluginv1.HealthStatus_UNHEALTHY,
			Message: err.Error(),
		}, nil
	}

	status := pluginv1.HealthStatus_HEALTHY
	if !health.Healthy {
		status = pluginv1.HealthStatus_UNHEALTHY
	}

	metrics := s.base.Metrics()
	return &pluginv1.HealthStatus{
		Status:        status,
		Message:       health.Message,
		RequestsTotal: metrics.RequestsTotal,
		ErrorsTotal:   metrics.ErrorsTotal,
		AvgLatencyMs:  float64(metrics.AvgLatency.Milliseconds()),
	}, nil
}

func (s *providerCoreServer) GetSettingsSchema(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	if configurable, ok := s.impl.(ConfigurableProvider); ok {
		schema, err := configurable.GetSettingsSchema()
		if err != nil {
			return nil, err
		}
		return &pluginv1.SettingsSchema{JsonSchema: schema}, nil
	}
	return &pluginv1.SettingsSchema{}, nil
}

func (s *providerCoreServer) Configure(ctx context.Context, settings *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	if configurable, ok := s.impl.(ConfigurableProvider); ok {
		if err := configurable.Configure(settings.Json); err != nil {
			return &pluginv1.ConfigureResponse{Success: false, Error: err.Error()}, nil
		}
		return &pluginv1.ConfigureResponse{Success: true}, nil
	}
	return &pluginv1.ConfigureResponse{Success: true}, nil
}

func (s *providerCoreServer) GetSubscriptions(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.EventSubscriptions, error) {
	return &pluginv1.EventSubscriptions{}, nil
}

func (s *providerCoreServer) OnEvent(ctx context.Context, event *pluginv1.Event) (*pluginv1.EventResponse, error) {
	return &pluginv1.EventResponse{Handled: false}, nil
}

func (s *providerCoreServer) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	if httpProvider, ok := s.impl.(HTTPProvider); ok {
		routes := httpProvider.GetRoutes()
		protoRoutes := make([]*pluginv1.PluginRoute, len(routes))
		for i, r := range routes {
			protoRoutes[i] = &pluginv1.PluginRoute{
				Path:        r.Path,
				Methods:     r.Methods,
				AdminOnly:   r.AdminOnly,
				Description: r.Description,
				Streaming:   r.Streaming,
			}
		}
		return &pluginv1.PluginRoutes{Routes: protoRoutes}, nil
	}
	return &pluginv1.PluginRoutes{}, nil
}

func (s *providerCoreServer) HandleHTTP(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if httpProvider, ok := s.impl.(HTTPProvider); ok {
		sdkReq := protoToHTTPRequest(req)
		resp, err := httpProvider.HandleHTTP(ctx, sdkReq)
		if err != nil {
			return &pluginv1.PluginHTTPResponse{
				StatusCode:  500,
				ContentType: "application/json",
				Body:        []byte(`{"error":"` + err.Error() + `"}`),
			}, nil
		}
		return httpResponseToProto(resp), nil
	}
	return &pluginv1.PluginHTTPResponse{
		StatusCode:  404,
		ContentType: "application/json",
		Body:        []byte(`{"error":"not found"}`),
	}, nil
}

func (s *providerCoreServer) HandleHTTPStream(stream pluginv1.PluginCore_HandleHTTPStreamServer) error {
	httpProvider, ok := s.impl.(HTTPProvider)
	if !ok {
		return sendStreamError(stream, 404, "not found")
	}

	// Receive the request
	chunk, err := stream.Recv()
	if err != nil {
		return err
	}
	if chunk.Type != pluginv1.PluginHTTPChunk_REQUEST_START {
		return sendStreamError(stream, 400, "expected request start")
	}

	req := protoToHTTPRequest(chunk.Request)

	// Collect request body
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if chunk.Type == pluginv1.PluginHTTPChunk_REQUEST_BODY {
			req.Body = append(req.Body, chunk.Data...)
		} else if chunk.Type == pluginv1.PluginHTTPChunk_REQUEST_END {
			break
		}
	}

	// Create stream writer
	writer := &grpcStreamWriter{stream: stream, started: false}

	// Handle the request
	if err := httpProvider.HandleHTTPStream(stream.Context(), req, writer); err != nil {
		if !writer.started {
			return sendStreamError(stream, 500, err.Error())
		}
		// If we already started, send error as body
		_ = writer.WriteChunk([]byte(`{"error":"` + err.Error() + `"}`))
	}

	// Send end
	return stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_END,
	})
}

// ExposeService exposes the provider's gRPC service for other plugins to connect.
// Called by the host when another plugin requests this provider's capability.
func (s *providerCoreServer) ExposeService(ctx context.Context, req *pluginv1.ExposeServiceRequest) (*pluginv1.ExposeServiceResponse, error) {
	if s.broker == nil {
		return &pluginv1.ExposeServiceResponse{
			Success: false,
			Error:   "broker not available",
		}, nil
	}

	// Check if we provide the requested capability
	caps := s.impl.GetProviderCapabilities()
	canProvide := false

	switch req.Capability {
	case "embedding":
		canProvide = caps.SupportsEmbedding
	case "chat":
		canProvide = caps.SupportsChat
	case "provider":
		// Generic provider capability - always supported
		canProvide = true
	default:
		// Check for provider-specific capability like "provider:ollama"
		if req.Capability == "provider:"+caps.ProviderID {
			canProvide = true
		}
	}

	if !canProvide {
		return &pluginv1.ExposeServiceResponse{
			Success: false,
			Error:   "capability not provided: " + req.Capability,
		}, nil
	}

	// Get a unique broker ID and start the PluginProvider service
	brokerID := s.broker.NextId()

	go s.broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		srv := grpc.NewServer(opts...)
		// Register the PluginProvider service
		pluginv1.RegisterPluginProviderServer(srv, &providerServiceServer{
			impl: s.impl,
			base: s.base,
		})
		return srv
	})

	s.base.Log().Debug("exposed provider service",
		"capability", req.Capability,
		"broker_id", brokerID,
		"requesting_plugin", req.RequestingPluginId)

	return &pluginv1.ExposeServiceResponse{
		Success:     true,
		BrokerId:    brokerID,
		ServiceName: "viewra.plugin.v1.PluginProvider",
	}, nil
}

// providerServiceServer implements PluginProviderServer for AI capabilities.
type providerServiceServer struct {
	pluginv1.UnimplementedPluginProviderServer
	impl ProviderPlugin
	base *Base
}

func (s *providerServiceServer) GetCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderCapabilities, error) {
	caps := s.impl.GetProviderCapabilities()
	return &pluginv1.ProviderCapabilities{
		ProviderId:            caps.ProviderID,
		DisplayName:           caps.DisplayName,
		Description:           caps.Description,
		SupportsChat:          caps.SupportsChat,
		SupportsEmbeddings:    caps.SupportsEmbedding,
		SupportsStreaming:     caps.SupportsStreaming,
		RequiresApiKey:        caps.RequiresAPIKey,
		RequiresUrl:           caps.RequiresURL,
		IsLocal:               caps.IsLocal,
		DefaultChatModel:      caps.DefaultChatModel,
		DefaultEmbeddingModel: caps.DefaultEmbeddingModel,
	}, nil
}

func (s *providerServiceServer) ListModels(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderModelList, error) {
	models, err := s.impl.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	protoModels := make([]*pluginv1.ProviderModel, len(models))
	for i, m := range models {
		protoModels[i] = &pluginv1.ProviderModel{
			Id:                  m.ID,
			Name:                m.Name,
			Description:         m.Description,
			IsChat:              m.IsChat,
			IsEmbedding:         m.IsEmbedding,
			ContextLength:       int32(m.ContextLength),
			EmbeddingDimensions: int32(m.EmbeddingDimensions),
			Size:                m.Size,
			Recommended:         m.Recommended,
		}
	}

	return &pluginv1.ProviderModelList{Models: protoModels}, nil
}

func (s *providerServiceServer) GenerateEmbedding(ctx context.Context, req *pluginv1.ProviderEmbeddingRequest) (*pluginv1.ProviderEmbeddingResponse, error) {
	start := time.Now()

	embedding, err := s.impl.GenerateEmbedding(ctx, req.Text, req.Model)
	if err != nil {
		s.base.RecordError()
		return nil, err
	}

	s.base.RecordRequest(time.Since(start))
	return &pluginv1.ProviderEmbeddingResponse{
		Embedding:  embedding,
		Dimensions: int32(len(embedding)),
	}, nil
}

func (s *providerServiceServer) GenerateEmbeddingBatch(ctx context.Context, req *pluginv1.ProviderEmbeddingBatchRequest) (*pluginv1.ProviderEmbeddingBatchResponse, error) {
	start := time.Now()

	embeddings, err := s.impl.GenerateEmbeddingBatch(ctx, req.Texts, req.Model)
	if err != nil {
		s.base.RecordError()
		return nil, err
	}

	s.base.RecordRequest(time.Since(start))

	results := make([]*pluginv1.ProviderEmbeddingResult, len(embeddings))
	for i, emb := range embeddings {
		results[i] = &pluginv1.ProviderEmbeddingResult{
			Embedding:  emb,
			Dimensions: int32(len(emb)),
		}
	}

	return &pluginv1.ProviderEmbeddingBatchResponse{Embeddings: results}, nil
}

func (s *providerServiceServer) Chat(ctx context.Context, req *pluginv1.ProviderChatRequest) (*pluginv1.ProviderChatResponse, error) {
	start := time.Now()

	// Convert proto request to SDK request
	sdkReq := &ChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
	}
	for _, m := range req.Messages {
		sdkReq.Messages = append(sdkReq.Messages, ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	resp, err := s.impl.Chat(ctx, sdkReq)
	if err != nil {
		s.base.RecordError()
		return nil, err
	}

	s.base.RecordRequest(time.Since(start))
	return &pluginv1.ProviderChatResponse{
		Content:          resp.Content,
		FinishReason:     resp.FinishReason,
		PromptTokens:     int32(resp.PromptTokens),
		CompletionTokens: int32(resp.CompletionTokens),
	}, nil
}

func (s *providerServiceServer) ChatStream(req *pluginv1.ProviderChatRequest, stream pluginv1.PluginProvider_ChatStreamServer) error {
	start := time.Now()

	// Convert proto request to SDK request
	sdkReq := &ChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
	}
	for _, m := range req.Messages {
		sdkReq.Messages = append(sdkReq.Messages, ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Create channel for chunks
	chunks := make(chan ChatChunk, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunks)
		errCh <- s.impl.ChatStream(stream.Context(), sdkReq, chunks)
	}()

	for chunk := range chunks {
		if err := stream.Send(&pluginv1.ProviderChatStreamChunk{
			Content:      chunk.Content,
			Done:         chunk.Done,
			FinishReason: chunk.FinishReason,
		}); err != nil {
			return err
		}
	}

	if err := <-errCh; err != nil {
		s.base.RecordError()
		return err
	}

	s.base.RecordRequest(time.Since(start))
	return nil
}

func (s *providerServiceServer) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderHealthStatus, error) {
	health, err := s.impl.HealthCheck(ctx)
	if err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy: false,
			Error:   err.Error(),
		}, nil
	}

	return &pluginv1.ProviderHealthStatus{
		Healthy:   health.Healthy,
		Message:   health.Message,
		LatencyMs: health.Latency.Milliseconds(),
		Error:     health.Error,
	}, nil
}

// --- Helper functions ---

// protoToHTTPRequest converts a proto request to an SDK request.
func protoToHTTPRequest(req *pluginv1.PluginHTTPRequest) *HTTPRequest {
	return &HTTPRequest{
		Path:       req.Path,
		Method:     req.Method,
		Headers:    req.Headers,
		Query:      req.Query,
		PathParams: req.PathParams,
		Body:       req.Body,
		UserID:     req.UserId,
		IsAdmin:    req.IsAdmin,
	}
}

// httpResponseToProto converts an SDK response to a proto response.
func httpResponseToProto(resp *HTTPResponse) *pluginv1.PluginHTTPResponse {
	return &pluginv1.PluginHTTPResponse{
		StatusCode:  int32(resp.StatusCode),
		Headers:     resp.Headers,
		Body:        resp.Body,
		ContentType: resp.ContentType,
	}
}

func sendStreamError(stream pluginv1.PluginCore_HandleHTTPStreamServer, status int32, msg string) error {
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type:        pluginv1.PluginHTTPChunk_RESPONSE_START,
		StatusCode:  status,
		ContentType: "application/json",
	}); err != nil {
		return err
	}
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
		Data: []byte(`{"error":"` + msg + `"}`),
	}); err != nil {
		return err
	}
	return stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_END,
	})
}

// grpcStreamWriter implements HTTPStreamWriter for gRPC streams.
type grpcStreamWriter struct {
	stream  pluginv1.PluginCore_HandleHTTPStreamServer
	started bool
}

func (w *grpcStreamWriter) WriteHeader(statusCode int, contentType string, headers map[string]string) error {
	if headers == nil {
		headers = make(map[string]string)
	}
	w.started = true
	return w.stream.Send(&pluginv1.PluginHTTPChunk{
		Type:        pluginv1.PluginHTTPChunk_RESPONSE_START,
		StatusCode:  int32(statusCode),
		ContentType: contentType,
		Headers:     headers,
	})
}

func (w *grpcStreamWriter) WriteChunk(data []byte) error {
	return w.stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
		Data: data,
	})
}

// --- go-plugin integration ---

// ProviderCoreGRPCPlugin is the go-plugin for PluginCore service.
type ProviderCoreGRPCPlugin struct {
	plugin.Plugin
	Impl   ProviderPlugin
	base   *Base
	broker *plugin.GRPCBroker
}

func (p *ProviderCoreGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	if p.base == nil {
		p.base = &Base{}
	}
	p.broker = broker
	server := &providerCoreServer{
		impl:   p.Impl,
		base:   p.base,
		broker: broker,
	}
	pluginv1.RegisterPluginCoreServer(s, server)
	return nil
}

func (p *ProviderCoreGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewPluginCoreClient(c), nil
}

// ProviderServiceGRPCPlugin is the go-plugin for PluginProvider service.
type ProviderServiceGRPCPlugin struct {
	plugin.Plugin
	Impl ProviderPlugin
	base *Base
}

func (p *ProviderServiceGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	if p.base == nil {
		p.base = &Base{}
	}
	server := &providerServiceServer{
		impl: p.Impl,
		base: p.base,
	}
	pluginv1.RegisterPluginProviderServer(s, server)
	return nil
}

func (p *ProviderServiceGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewPluginProviderClient(c), nil
}

// ServeProvider starts a provider plugin server.
// Call this from your plugin's main() function.
//
// Example:
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-provider")
//	    p := NewMyProvider()
//	    p.SetLogger(logger)
//	    sdk.ServeProvider(p, hclogger)
//	}
func ServeProvider(impl ProviderPlugin, logger hclog.Logger) {
	base := &Base{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"core":     &ProviderCoreGRPCPlugin{Impl: impl, base: base},
			"provider": &ProviderServiceGRPCPlugin{Impl: impl, base: base},
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
