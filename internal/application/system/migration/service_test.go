package migration

import (
	"context"
	"database/sql"
	"testing"

	"github.com/mantonx/viewra/internal/application/system"
	_ "github.com/mattn/go-sqlite3"
)

func TestNewService(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	maintenanceMgr := system.NewMaintenanceManager()
	svc := NewService(db, "sqlite", maintenanceMgr, nil)

	if svc == nil {
		t.Fatal("Expected non-nil service")
	}

	state := svc.GetState()
	if state.Status != StatusIdle {
		t.Errorf("Expected status %s, got %s", StatusIdle, state.Status)
	}
}

func TestService_IsInProgress(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	maintenanceMgr := system.NewMaintenanceManager()
	svc := NewService(db, "sqlite", maintenanceMgr, nil)

	if svc.IsInProgress() {
		t.Error("Expected IsInProgress to be false initially")
	}
}

func TestService_TestConnection_InvalidDriver(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	maintenanceMgr := system.NewMaintenanceManager()
	svc := NewService(db, "sqlite", maintenanceMgr, nil)

	result := svc.TestConnection(context.Background(), TargetConfig{
		Driver: "invalid",
	})

	if result.Success {
		t.Error("Expected connection test to fail for invalid driver")
	}
}

func TestService_TestConnection_SQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	maintenanceMgr := system.NewMaintenanceManager()
	svc := NewService(db, "sqlite", maintenanceMgr, nil)

	result := svc.TestConnection(context.Background(), TargetConfig{
		Driver: "sqlite",
		SQLite: &SQLiteConfig{Path: ":memory:"},
	})

	if !result.Success {
		t.Errorf("Expected connection test to succeed, got error: %s", result.Error)
	}
}

func TestService_Estimate(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	_, err = db.Exec("INSERT INTO test (name) VALUES ('a'), ('b'), ('c')")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	maintenanceMgr := system.NewMaintenanceManager()
	svc := NewService(db, "sqlite", maintenanceMgr, nil)

	estimate, err := svc.Estimate(context.Background(), "postgres")
	if err != nil {
		t.Fatalf("Estimate failed: %v", err)
	}

	if estimate.Source.Driver != "sqlite" {
		t.Errorf("Expected driver 'sqlite', got '%s'", estimate.Source.Driver)
	}

	if estimate.Source.TableCount == 0 {
		t.Error("Expected at least one table")
	}
}

func TestGenerateMigrationID(t *testing.T) {
	id1 := generateMigrationID()
	id2 := generateMigrationID()

	if id1 == id2 {
		t.Error("Expected unique migration IDs")
	}

	if len(id1) < 10 {
		t.Errorf("Expected longer migration ID, got %s", id1)
	}

	if id1[:4] != "mig_" {
		t.Errorf("Expected migration ID to start with 'mig_', got %s", id1)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  int
		expected string
	}{
		{30, "~30 seconds"},
		{60, "~1 minutes"},
		{120, "~2 minutes"},
		{3600, "~1 hours"},
		{3660, "~1 hours 1 minutes"},
		{7200, "~2 hours"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.seconds)
		if result != tt.expected {
			t.Errorf("formatDuration(%d) = %s, expected %s", tt.seconds, result, tt.expected)
		}
	}
}
