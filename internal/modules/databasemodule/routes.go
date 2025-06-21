package databasemodule

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// RegisterRoutes registers the database module routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	logger.Info("Registering database module routes")

	api := router.Group("/api/database")
	{
		// Health and status endpoints
		api.GET("/health", m.getHealth)
		api.GET("/status", m.getStatus)
		api.GET("/stats", m.getStats)

		// Connection pool endpoints
		api.GET("/connections", m.getConnectionStats)
		api.GET("/connections/health", m.getConnectionHealth)

		// Migration endpoints
		api.GET("/migrations", m.getMigrations)
		api.POST("/migrations/execute", m.executePendingMigrations)
		api.POST("/migrations/:id/rollback", m.rollbackMigration)

		// Model registry endpoints
		api.GET("/models", m.getRegisteredModels)
		api.GET("/models/stats", m.getModelStats)
		api.POST("/models/migrate", m.autoMigrateModels)

		// Transaction endpoints (mainly for monitoring)
		api.GET("/transactions/stats", m.getTransactionStats)
	}
}

// getHealth returns the health status of the database module
func (m *Module) getHealth(c *gin.Context) {
	if err := m.Health(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"module": m.Name(),
	})
}

// getStatus returns the overall status of the database module
func (m *Module) getStatus(c *gin.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := gin.H{
		"module_id":   m.ID(),
		"module_name": m.Name(),
		"initialized": m.initialized,
		"core_module": m.Core(),
	}

	// Add component status
	if m.connectionPool != nil {
		status["connection_pool"] = "initialized"
	}
	if m.migrationManager != nil {
		status["migration_manager"] = "initialized"
	}
	if m.transactionMgr != nil {
		status["transaction_manager"] = "initialized"
	}
	if m.modelRegistry != nil {
		status["model_registry"] = "initialized"
		status["registered_models"] = m.modelRegistry.GetModelCount()
	}

	c.JSON(http.StatusOK, status)
}

// getStats returns comprehensive statistics for the database module
func (m *Module) getStats(c *gin.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := gin.H{
		"module": gin.H{
			"id":          m.ID(),
			"name":        m.Name(),
			"initialized": m.initialized,
			"core":        m.Core(),
		},
	}

	// Add connection pool stats
	if m.connectionPool != nil {
		stats["connection_pool"] = m.connectionPool.GetStats()
	}

	// Add migration stats
	if m.migrationManager != nil {
		migrationStats, err := m.migrationManager.GetMigrationStatus()
		if err != nil {
			logger.Error("Failed to get migration stats: %v", err)
			stats["migration_error"] = err.Error()
		} else {
			stats["migrations"] = migrationStats
		}
	}

	// Add transaction stats
	if m.transactionMgr != nil {
		stats["transactions"] = m.transactionMgr.GetStats()
	}

	// Add model registry stats
	if m.modelRegistry != nil {
		stats["models"] = m.modelRegistry.GetStats()
	}

	c.JSON(http.StatusOK, stats)
}

// getConnectionStats returns connection pool statistics
func (m *Module) getConnectionStats(c *gin.Context) {
	if m.connectionPool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Connection pool not initialized",
		})
		return
	}

	stats := m.connectionPool.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"connection_stats": stats,
	})
}

// getConnectionHealth checks the health of all database connections
func (m *Module) getConnectionHealth(c *gin.Context) {
	if m.connectionPool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Connection pool not initialized",
		})
		return
	}

	if err := m.connectionPool.Health(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"connections": "all connections are healthy",
	})
}

// getMigrations returns the status of all migrations
func (m *Module) getMigrations(c *gin.Context) {
	if m.migrationManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Migration manager not initialized",
		})
		return
	}

	status, err := m.migrationManager.GetMigrationStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// executePendingMigrations executes all pending database migrations
func (m *Module) executePendingMigrations(c *gin.Context) {
	if m.migrationManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Migration manager not initialized",
		})
		return
	}

	if err := m.migrationManager.ExecutePendingMigrations(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pending migrations executed successfully",
	})
}

// rollbackMigration rolls back a specific migration
func (m *Module) rollbackMigration(c *gin.Context) {
	migrationID := c.Param("id")
	if migrationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Migration ID is required",
		})
		return
	}

	if m.migrationManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Migration manager not initialized",
		})
		return
	}

	if err := m.migrationManager.RollbackMigration(migrationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Migration rolled back successfully",
		"migration_id": migrationID,
	})
}

// getRegisteredModels returns information about registered models
func (m *Module) getRegisteredModels(c *gin.Context) {
	if m.modelRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Model registry not initialized",
		})
		return
	}

	models := m.modelRegistry.GetModelInfo()
	c.JSON(http.StatusOK, gin.H{
		"models": models,
		"count":  len(models),
	})
}

// getModelStats returns model registry statistics
func (m *Module) getModelStats(c *gin.Context) {
	if m.modelRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Model registry not initialized",
		})
		return
	}

	stats := m.modelRegistry.GetStats()
	c.JSON(http.StatusOK, stats)
}

// autoMigrateModels performs auto-migration for all registered models
func (m *Module) autoMigrateModels(c *gin.Context) {
	if m.modelRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Model registry not initialized",
		})
		return
	}

	if err := m.modelRegistry.AutoMigrateAll(m.db); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Auto-migration completed successfully",
		"models_migrated": m.modelRegistry.GetModelCount(),
	})
}

// getTransactionStats returns transaction manager statistics
func (m *Module) getTransactionStats(c *gin.Context) {
	if m.transactionMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Transaction manager not initialized",
		})
		return
	}

	stats := m.transactionMgr.GetStats()
	c.JSON(http.StatusOK, stats)
}
