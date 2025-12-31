package system

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"sync"
)

// Environment contains detected environment information.
type Environment struct {
	IsContainer  bool   `json:"isContainer"`
	IsKubernetes bool   `json:"isKubernetes"`
	InstanceID   string `json:"instanceId"`
}

// Warning represents a system warning.
type Warning struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "warning", "error", "info"
}

// EnvironmentInfo holds environment detection results and warnings.
type EnvironmentInfo struct {
	Environment Environment `json:"environment"`
	Warnings    []Warning   `json:"warnings"`
}

var (
	cachedEnv     *Environment
	cachedEnvOnce sync.Once
)

// DetectEnvironment detects the current runtime environment.
// Results are cached for the lifetime of the process.
func DetectEnvironment() *Environment {
	cachedEnvOnce.Do(func() {
		cachedEnv = &Environment{
			IsContainer:  detectContainer(),
			IsKubernetes: detectKubernetes(),
			InstanceID:   generateInstanceID(),
		}
	})
	return cachedEnv
}

// detectContainer checks if we're running inside a container.
func detectContainer() bool {
	// Check for Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroup for container indicators
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "crio") {
			return true
		}
	}

	// Check for container runtime environment variables
	containerEnvVars := []string{
		"DOCKER_CONTAINER",
		"container",
		"PODMAN_CONTAINER",
	}
	for _, env := range containerEnvVars {
		if os.Getenv(env) != "" {
			return true
		}
	}

	return false
}

// detectKubernetes checks if we're running inside Kubernetes.
func detectKubernetes() bool {
	// Kubernetes sets these environment variables in pods
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Check for Kubernetes service account
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount"); err == nil {
		return true
	}

	return false
}

// generateInstanceID creates a unique identifier for this instance.
// Uses hostname if available, otherwise generates a random ID.
func generateInstanceID() string {
	// Try to get hostname (usually set to pod name in K8s)
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}

	// Generate random ID as fallback
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return "viewra-" + hex.EncodeToString(b)
}

// GetEnvironmentWarnings returns warnings based on current environment and database driver.
func GetEnvironmentWarnings(dbDriver string) []Warning {
	env := DetectEnvironment()
	var warnings []Warning

	// Warn about SQLite in container/Kubernetes environments
	if dbDriver == "sqlite" || dbDriver == "sqlite3" {
		if env.IsKubernetes {
			warnings = append(warnings, Warning{
				Code:     "SQLITE_IN_K8S",
				Message:  "SQLite is not recommended in Kubernetes environments. Data may be lost when pods restart. Consider migrating to PostgreSQL.",
				Severity: "warning",
			})
		} else if env.IsContainer {
			warnings = append(warnings, Warning{
				Code:     "SQLITE_IN_CONTAINER",
				Message:  "SQLite in a container requires persistent storage. Ensure your volume mount is configured correctly to prevent data loss.",
				Severity: "info",
			})
		}
	}

	return warnings
}

// GetEnvironmentInfo returns complete environment information including warnings.
func GetEnvironmentInfo(dbDriver string) *EnvironmentInfo {
	return &EnvironmentInfo{
		Environment: *DetectEnvironment(),
		Warnings:    GetEnvironmentWarnings(dbDriver),
	}
}
