package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/apiroutes" // Updated import
)

// ApiRootHandler serves the main /api endpoint, listing available routes.
func ApiRootHandler(c *gin.Context) {
	registeredRoutes := apiroutes.Get() // Use new package

	endpointsMap := make(map[string]string)
	// Populate endpointsMap from registeredRoutes for major categories
	// This is a simple heuristic; a more robust way might involve explicit naming or tagging of routes.
	for _, route := range registeredRoutes {
		pathSegments := strings.Split(strings.TrimPrefix(route.Path, "/api/"), "/")
		if len(pathSegments) > 0 {
			key := pathSegments[0]
			// If we have versioning like /v1/, take the next segment
			if (key == "v1" || key == "v2") && len(pathSegments) > 1 {
				key = pathSegments[1]
			}
			// Add to map if it's a base path for a category and not already set
			// to avoid overriding with sub-paths.
			if _, exists := endpointsMap[key]; !exists && (strings.HasSuffix(route.Path, key) || strings.HasSuffix(route.Path, key+"/")) {
				// This logic is a bit naive for deriving the summary. 
				// For now, we'll use the paths we registered explicitly in server.go for the main endpointsMap.
			}
		}
	}

	// For a cleaner endpointsMap, let's use the explicitly registered top-level ones for now.
	// The full list will be in "registered_routes".
	// The routes registered in server.go already provide a good overview.
	for _, route := range registeredRoutes {
		if route.Path == "/api/v1/users" { endpointsMap["users"] = route.Path }
		if route.Path == "/api/v1/media" { endpointsMap["media"] = route.Path }
		if route.Path == "/api/v1/plugins" { endpointsMap["plugins"] = route.Path }
		if route.Path == "/swagger/index.html" { endpointsMap["docs"] = route.Path }
		if route.Path == "/api" { endpointsMap["self"] = route.Path }
	}

	c.JSON(http.StatusOK, gin.H{
		"endpoints":         endpointsMap,
		"version":           "v1", // This could be made dynamic later
		"status":            "OK",
		"registered_routes": registeredRoutes, // Include the detailed list
	})
} 