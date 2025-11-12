package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func TestHealthCheck_DatabaseOK(t *testing.T) {
	// Create in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create health handler
	handler := NewHealthHandler(db)

	// Setup gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", handler.Check)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response.Status)
	}

	if response.Database.Status != "ok" {
		t.Errorf("Expected database status 'ok', got '%s'", response.Database.Status)
	}

	if response.Database.Ping == "" {
		t.Error("Expected database ping time, got empty string")
	}
}

func TestHealthCheck_DatabaseDown(t *testing.T) {
	// Create database and close it immediately to simulate failure
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	db.Close() // Close to simulate failure

	// Create health handler
	handler := NewHealthHandler(db)

	// Setup gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", handler.Check)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response - should be 503 when database is down
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Errorf("Expected status 'degraded', got '%s'", response.Status)
	}

	if response.Database.Status != "error" {
		t.Errorf("Expected database status 'error', got '%s'", response.Database.Status)
	}
}
