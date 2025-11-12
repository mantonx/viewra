package handlers

import (
	"fmt"
	"strconv"
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
