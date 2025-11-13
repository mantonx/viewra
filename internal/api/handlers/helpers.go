package handlers

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseID parses a string ID to int64
func parseID(idStr string) (int64, error) {
	return strconv.ParseInt(idStr, 10, 64)
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// paginationParams holds pagination parameters
type paginationParams struct {
	limit  int
	offset int
}

// parsePaginationParams extracts and validates pagination parameters from query string.
func parsePaginationParams(c *gin.Context) paginationParams {
	limit := 50
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	return paginationParams{limit: limit, offset: offset}
}

// getCurrentUserID returns the current user ID (hardcoded to 1 for single-user mode).
func getCurrentUserID() int64 {
	return 1
}
