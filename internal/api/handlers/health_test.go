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

	// Create health handler (scheduler and transcode queue are optional, pass nil)
	handler := NewHealthHandler(db, nil, nil)

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

	// Check overall status (new structure)
	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response.Status)
	}

	// Check database component status (new structure)
	dbCheck, ok := response.Components["database"]
	if !ok {
		t.Fatal("Expected database component in response")
	}

	if dbCheck.Status != "pass" {
		t.Errorf("Expected database status 'pass', got '%s'", dbCheck.Status)
	}

	if dbCheck.Message == "" {
		t.Error("Expected database ping message, got empty string")
	}

	// Verify system info is present
	if response.System == nil {
		t.Error("Expected system info in response")
	}
}

func TestHealthCheck_DatabaseDown(t *testing.T) {
	// Create database and close it immediately to simulate failure
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	db.Close() // Close to simulate failure

	// Create health handler (scheduler and transcode queue are optional, pass nil)
	handler := NewHealthHandler(db, nil, nil)

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

	// Check overall status (new structure)
	if response.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got '%s'", response.Status)
	}

	// Check database component status (new structure)
	dbCheck, ok := response.Components["database"]
	if !ok {
		t.Fatal("Expected database component in response")
	}

	if dbCheck.Status != "fail" {
		t.Errorf("Expected database status 'fail', got '%s'", dbCheck.Status)
	}
}
