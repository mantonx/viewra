package internal

import (
	"context"
	"log/slog"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const queryRewriteSystemPrompt = `You are a search query optimizer for a media library. Your job is to rewrite user queries to better match content in a movie/TV database.

IMPORTANT RULES:
1. Detect user INTENT, not just literal words
2. If user says they're feeling sad/down/depressed and want cheering up, they want HAPPY/UPLIFTING content
3. If user is stressed/anxious, they likely want RELAXING/CALMING content  
4. If user is bored, they want EXCITING/ENGAGING content
5. Expand abbreviations and colloquialisms
6. Add relevant genre/mood keywords that match the intent
7. Keep the output concise (under 20 words)
8. Output ONLY the rewritten query, nothing else

Examples:
- "feeling sad need cheering up" → "uplifting heartwarming feel-good comedy happy ending"
- "stressed out need to relax" → "relaxing calm peaceful slow-paced soothing"
- "something scary" → "horror scary terrifying thriller suspense"
- "80s action" → "1980s action movie eighties explosive"
- "like Die Hard" → "action thriller hostage hero one-man-army skyscraper"
- "Korean thriller" → "Korean South Korea thriller suspense mystery Language: Korean"`

// QueryRewriter uses an LLM to rewrite search queries for better semantic matching.
type QueryRewriter struct {
	llmClient pluginv1.HostLLMClient
	logger    *slog.Logger
	enabled   bool
}

// NewQueryRewriter creates a new query rewriter.
func NewQueryRewriter(llmClient pluginv1.HostLLMClient, logger *slog.Logger) *QueryRewriter {
	return &QueryRewriter{
		llmClient: llmClient,
		logger:    logger,
		enabled:   llmClient != nil,
	}
}

// Rewrite rewrites a query using the LLM to better match user intent.
// Returns the original query if rewriting fails or is disabled.
func (r *QueryRewriter) Rewrite(ctx context.Context, query string) string {
	if !r.enabled || r.llmClient == nil {
		return query
	}

	// Skip very short queries or queries that look like direct searches
	if len(query) < 10 || !needsRewriting(query) {
		return query
	}

	resp, err := r.llmClient.Chat(ctx, &pluginv1.ChatRequest{
		Messages: []*pluginv1.ChatMessage{
			{
				Role:    "system",
				Content: queryRewriteSystemPrompt,
			},
			{
				Role:    "user",
				Content: query,
			},
		},
		Temperature: 0.3,
		MaxTokens:   50,
	})
	if err != nil {
		r.logger.Debug("query rewrite failed, using original", "error", err)
		return query
	}

	rewritten := strings.TrimSpace(resp.Content)
	if rewritten == "" || len(rewritten) > 200 {
		return query
	}

	r.logger.Debug("rewrote query",
		"original", query,
		"rewritten", rewritten,
	)

	return rewritten
}

// needsRewriting returns true if the query might benefit from rewriting.
func needsRewriting(query string) bool {
	q := strings.ToLower(query)

	// Queries expressing emotional state or needs
	emotionalIndicators := []string{
		"feeling", "i'm", "im ", "i am", "need", "want",
		"something", "mood", "like", "similar",
		"cheer", "relax", "stressed", "bored", "sad",
		"happy", "scared", "excited",
	}

	for _, indicator := range emotionalIndicators {
		if strings.Contains(q, indicator) {
			return true
		}
	}

	return false
}
