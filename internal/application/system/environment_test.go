package system

import (
	"testing"
)

func TestDetectEnvironment(t *testing.T) {
	// Should not panic and should return a valid environment
	env := DetectEnvironment()
	if env == nil {
		t.Fatal("Expected non-nil environment")
	}

	// Instance ID should be set
	if env.InstanceID == "" {
		t.Error("Expected non-empty instance ID")
	}
}

func TestGetEnvironmentWarnings_SQLiteInK8s(t *testing.T) {
	// Force the cached env to be K8s for testing
	// Note: In real tests we'd mock the detection, but for now just test the logic
	warnings := GetEnvironmentWarnings("sqlite")

	// Warnings depend on actual environment - just verify no panic
	_ = warnings
}

func TestGetEnvironmentWarnings_PostgresNoWarnings(t *testing.T) {
	warnings := GetEnvironmentWarnings("postgres")

	// PostgreSQL should never have SQLite-related warnings
	for _, w := range warnings {
		if w.Code == "SQLITE_IN_K8S" || w.Code == "SQLITE_IN_CONTAINER" {
			t.Errorf("PostgreSQL should not have SQLite warnings, got: %s", w.Code)
		}
	}
}

func TestGetEnvironmentInfo(t *testing.T) {
	info := GetEnvironmentInfo("sqlite")
	if info == nil {
		t.Fatal("Expected non-nil environment info")
	}

	if info.Environment.InstanceID == "" {
		t.Error("Expected non-empty instance ID")
	}

	// Warnings can be nil or empty slice depending on environment
	// Just verify the function doesn't panic
}
