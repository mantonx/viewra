// Package handlers contains HTTP request handlers organized by functionality.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
)

// HandleHealthCheck returns the basic health status of the service
func HandleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "viewra",
	})
}

// HandleHello returns a simple greeting message for connectivity testing
func HandleHello(c *gin.Context) {
	c.String(http.StatusOK, "Hello from Viewra backend!")
}

// HandleDBStatus checks and returns the database connection status
func HandleDBStatus(c *gin.Context) {
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to get database instance: " + err.Error(),
		})
		return
	}
	
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Database ping failed: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":   "connected",
		"database": "ready",
	})
}

// HandleConnectionPoolStats returns detailed connection pool statistics
func HandleConnectionPoolStats(c *gin.Context) {
	stats, err := database.GetConnectionStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get connection pool stats: " + err.Error(),
		})
		return
	}
	
	// Calculate utilization percentages
	var openUtilization, idleUtilization float64
	if stats.MaxOpenConnections > 0 {
		openUtilization = float64(stats.OpenConnections) / float64(stats.MaxOpenConnections) * 100
	}
	if stats.OpenConnections > 0 {
		idleUtilization = float64(stats.Idle) / float64(stats.OpenConnections) * 100
	}
	
	c.JSON(http.StatusOK, gin.H{
		"connection_pool": gin.H{
			"open_connections":       stats.OpenConnections,
			"max_open_connections":   stats.MaxOpenConnections,
			"in_use":                stats.InUse,
			"idle":                  stats.Idle,
			"wait_count":            stats.WaitCount,
			"wait_duration":         stats.WaitDuration.String(),
			"max_idle_closed":       stats.MaxIdleClosed,
			"max_idle_time_closed":  stats.MaxIdleTimeClosed,
			"max_lifetime_closed":   stats.MaxLifetimeClosed,
		},
		"utilization": gin.H{
			"open_connection_percent": openUtilization,
			"idle_connection_percent": idleUtilization,
			"busy_connection_percent": 100 - idleUtilization,
		},
		"performance_indicators": gin.H{
			"connection_waits":      stats.WaitCount > 0,
			"high_utilization":      openUtilization > 80,
			"connection_churning":   stats.MaxLifetimeClosed > int64(stats.OpenConnections*10),
			"idle_timeout_issues":   stats.MaxIdleTimeClosed > int64(stats.OpenConnections*5),
		},
		"health_status": func() string {
			if stats.WaitCount > 100 {
				return "warning"
			}
			if openUtilization > 90 {
				return "warning"
			}
			return "healthy"
		}(),
	})
}

// HandleDatabaseHealth performs a comprehensive database health check
func HandleDatabaseHealth(c *gin.Context) {
	if err := database.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}
	
	// Get additional health metrics
	stats, err := database.GetConnectionStats()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "Failed to get connection stats: " + err.Error(),
		})
		return
	}
	
	// Determine health status based on multiple factors
	healthStatus := "healthy"
	healthIssues := []string{}
	
	if stats.WaitCount > 0 {
		healthIssues = append(healthIssues, "connection_waits_detected")
	}
	
	if stats.OpenConnections == 0 {
		healthStatus = "critical"
		healthIssues = append(healthIssues, "no_open_connections")
	}
	
	utilization := float64(stats.OpenConnections) / float64(stats.MaxOpenConnections) * 100
	if utilization > 90 {
		healthStatus = "warning"
		healthIssues = append(healthIssues, "high_connection_utilization")
	}
	
	response := gin.H{
		"status":              healthStatus,
		"open_connections":    stats.OpenConnections,
		"max_connections":     stats.MaxOpenConnections,
		"utilization_percent": utilization,
		"wait_count":          stats.WaitCount,
	}
	
	if len(healthIssues) > 0 {
		response["issues"] = healthIssues
	}
	
	// Set appropriate HTTP status code
	statusCode := http.StatusOK
	if healthStatus == "warning" {
		statusCode = http.StatusOK // Still 200 but with warnings
	} else if healthStatus == "critical" {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, response)
}
