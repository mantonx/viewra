package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// parseID parses a string ID to int64
func parseID(idStr string) (int64, error) {
	return strconv.ParseInt(idStr, 10, 64)
}

// parseIDList parses a comma-separated string of IDs to a slice of int64
func parseIDList(idsStr string) ([]int64, error) {
	if idsStr == "" {
		return []int64{}, nil
	}

	parts := splitCSV(idsStr)
	ids := make([]int64, 0, len(parts))

	for _, part := range parts {
		id, err := parseID(part)
		if err != nil {
			return nil, fmt.Errorf("invalid ID: %s", part)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// splitCSV splits a comma-separated string into trimmed parts
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseCommaSeparatedHeader parses a comma-separated HTTP header value into a slice of strings.
// Returns nil if the header is empty.
func parseCommaSeparatedHeader(header string) []string {
	if header == "" {
		return nil
	}
	return splitCSV(header)
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

// parseFloat parses a string to float64
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
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

// formatTime formats a time.Time to RFC3339 string.
// Returns empty string if time is zero.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// getRequiredQueryInt64 extracts a required int64 query parameter.
// Returns the parsed value and true if successful.
// Returns 0 and false if the parameter is missing or invalid, and sends an error response.
func getRequiredQueryInt64(c *gin.Context, paramName string) (int64, bool) {
	valueStr := c.Query(paramName)
	if valueStr == "" {
		respondError(c, http.StatusBadRequest, "MISSING_PARAMETER", fmt.Sprintf("%s query parameter is required", paramName))
		return 0, false
	}

	value, err := parseID(valueStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PARAMETER", fmt.Sprintf("Invalid %s: %s", paramName, err.Error()))
		return 0, false
	}

	return value, true
}

// getOptionalQueryInt64 extracts an optional int64 query parameter.
// Returns the parsed value and true if present and valid.
// Returns 0 and false if the parameter is missing or invalid.
// Does NOT send error responses - caller should handle validation.
func getOptionalQueryInt64(c *gin.Context, paramName string) (int64, bool) {
	valueStr := c.Query(paramName)
	if valueStr == "" {
		return 0, false
	}

	value, err := parseID(valueStr)
	if err != nil {
		return 0, false
	}

	return value, true
}

// getPathInt64 extracts an int64 path parameter.
// Returns the parsed value and true if successful.
// Returns 0 and false if invalid, and sends an error response.
func getPathInt64(c *gin.Context, paramName string) (int64, bool) {
	valueStr := c.Param(paramName)
	value, err := parseID(valueStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PARAMETER", fmt.Sprintf("Invalid %s: %s", paramName, err.Error()))
		return 0, false
	}
	return value, true
}

// getQueryInt extracts an int query parameter with a default value.
func getQueryInt(c *gin.Context, paramName string, defaultValue int) int {
	valueStr := c.Query(paramName)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
