package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	appSearch "github.com/mantonx/viewra/internal/application/search"
	"github.com/mantonx/viewra/internal/domain/search"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// SearchHandler handles /api/search requests.
// It first checks if a semantic search plugin is available, and if not,
// falls back to basic text search.
type SearchHandler struct {
	capabilityRegistry *plugins.CapabilityRegistry
	httpProxy          *plugins.HTTPProxy
	searchService      *appSearch.Service
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(
	capabilityRegistry *plugins.CapabilityRegistry,
	httpProxy *plugins.HTTPProxy,
	searchService *appSearch.Service,
) *SearchHandler {
	return &SearchHandler{
		capabilityRegistry: capabilityRegistry,
		httpProxy:          httpProxy,
		searchService:      searchService,
	}
}

// Search handles GET /api/search
// @Summary Search media
// @Description Searches for movies, TV shows, and other media. Uses semantic search if available, otherwise falls back to text search.
// @Tags search
// @Produce json
// @Param q query string true "Search query"
// @Param types query []string false "Filter by media types (movie, tv_show, tv_episode)"
// @Param limit query int false "Maximum results to return (default 20, max 100)"
// @Success 200 {object} search.Response
// @Router /api/search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	// Check if semantic search plugin is available
	if mapping := h.capabilityRegistry.Resolve("semantic_search"); mapping != nil {
		// Proxy to the plugin
		h.httpProxy.HandleCapabilityRoute("semantic_search")(c)
		return
	}

	// Fallback to basic text search
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Parse media types filter
	var mediaTypes []string
	if types := c.QueryArray("types"); len(types) > 0 {
		mediaTypes = types
	}

	req := &search.Request{
		Query:      query,
		MediaTypes: mediaTypes,
		Limit:      limit,
	}

	resp, err := h.searchService.Search(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
