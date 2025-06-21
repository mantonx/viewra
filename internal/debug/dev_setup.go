package debug

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/logger"
)

// DevEnvironmentManager manages the development environment setup
type DevEnvironmentManager struct {
	projectRoot string
	config     *DevConfig
}

// DevConfig configuration for development environment
type DevConfig struct {
	EnableHotReload    bool              `json:"enable_hot_reload"`
	EnableDebugTools   bool              `json:"enable_debug_tools"`
	EnableProfiling    bool              `json:"enable_profiling"`
	WatchForChanges    bool              `json:"watch_for_changes"`
	BuildParallel      bool              `json:"build_parallel"`
	AutoStartServices  bool              `json:"auto_start_services"`
	HealthCheckTimeout time.Duration     `json:"health_check_timeout"`
	Services          map[string]string `json:"services"`
	Environment       map[string]string `json:"environment"`
}

// DevEnvironmentStatus represents the status of the development environment
type DevEnvironmentStatus struct {
	ProjectRoot        string            `json:"project_root"`
	GitStatus          *GitStatus        `json:"git_status"`
	Dependencies       *DependencyStatus `json:"dependencies"`
	Services           *ServiceStatus    `json:"services"`
	Database           *DatabaseStatus   `json:"database"`
	Plugins            *PluginStatus     `json:"plugins"`
	Environment        map[string]string `json:"environment"`
	HealthChecks       []HealthCheck     `json:"health_checks"`
	Recommendations    []string          `json:"recommendations"`
	LastChecked        time.Time         `json:"last_checked"`
}

// GitStatus represents git repository status
type GitStatus struct {
	Branch         string   `json:"branch"`
	HasChanges     bool     `json:"has_changes"`
	UntrackedFiles []string `json:"untracked_files"`
	ModifiedFiles  []string `json:"modified_files"`
	CommitHash     string   `json:"commit_hash"`
	RemoteStatus   string   `json:"remote_status"`
}

// DependencyStatus represents dependency status
type DependencyStatus struct {
	Go       *ToolStatus `json:"go"`
	Node     *ToolStatus `json:"node"`
	Docker   *ToolStatus `json:"docker"`
	Make     *ToolStatus `json:"make"`
	Git      *ToolStatus `json:"git"`
	Modules  bool        `json:"modules_tidy"`
}

// ToolStatus represents individual tool status
type ToolStatus struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Path      string `json:"path"`
}

// ServiceStatus represents running services status
type ServiceStatus struct {
	Backend  *ServiceInfo `json:"backend"`
	Frontend *ServiceInfo `json:"frontend"`
	Database *ServiceInfo `json:"database"`
}

// ServiceInfo represents information about a service
type ServiceInfo struct {
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	URL     string `json:"url"`
	Health  string `json:"health"`
	Uptime  string `json:"uptime"`
}

// DatabaseStatus represents database status
type DatabaseStatus struct {
	Available bool   `json:"available"`
	Type      string `json:"type"`
	Location  string `json:"location"`
	Size      string `json:"size"`
	Tables    int    `json:"tables"`
}

// PluginStatus represents plugin system status
type PluginStatus struct {
	Available      int      `json:"available"`
	Built          int      `json:"built"`
	Running        int      `json:"running"`
	HotReload      bool     `json:"hot_reload_enabled"`
	BuildTime      string   `json:"last_build_time"`
	FailedBuilds   []string `json:"failed_builds"`
}

// HealthCheck represents a health check result
type HealthCheck struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Message  string        `json:"message"`
	Duration time.Duration `json:"duration"`
	Critical bool          `json:"critical"`
}

// DefaultDevConfig returns default development configuration
func DefaultDevConfig() *DevConfig {
	return &DevConfig{
		EnableHotReload:    true,
		EnableDebugTools:   true,
		EnableProfiling:    false,
		WatchForChanges:    true,
		BuildParallel:      true,
		AutoStartServices:  false,
		HealthCheckTimeout: 30 * time.Second,
		Services: map[string]string{
			"backend":  "http://localhost:8080",
			"frontend": "http://localhost:5175",
			"debug":    "http://localhost:6061",
			"pprof":    "http://localhost:6060",
		},
		Environment: map[string]string{
			"LOG_LEVEL":  "debug",
			"LOG_FORMAT": "human", // human or json
			"DEV_MODE":   "true",
		},
	}
}

// NewDevEnvironmentManager creates a new development environment manager
func NewDevEnvironmentManager(projectRoot string, config *DevConfig) *DevEnvironmentManager {
	if config == nil {
		config = DefaultDevConfig()
	}

	return &DevEnvironmentManager{
		projectRoot: projectRoot,
		config:      config,
	}
}

// Setup sets up the development environment
func (dem *DevEnvironmentManager) Setup(ctx context.Context) error {
	logger.Info("Setting up development environment", "project_root", dem.projectRoot)

	// Check if project root exists
	if _, err := os.Stat(dem.projectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project root does not exist: %s", dem.projectRoot)
	}

	// Change to project directory
	if err := os.Chdir(dem.projectRoot); err != nil {
		return fmt.Errorf("failed to change to project directory: %w", err)
	}

	// Run setup steps
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"Environment Variables", dem.setupEnvironment},
		{"Dependencies", dem.checkDependencies},
		{"Go Modules", dem.setupGoModules},
		{"Database", dem.setupDatabase},
		{"Plugins", dem.buildPlugins},
	}

	for _, step := range steps {
		logger.Info("Setting up: " + step.name)
		if err := step.fn(ctx); err != nil {
			logger.Error("Setup step failed", "step", step.name, "error", err)
			return fmt.Errorf("failed to setup %s: %w", step.name, err)
		}
		logger.Info("✅ " + step.name + " setup complete")
	}

	logger.Info("🚀 Development environment setup complete!")
	return nil
}

// CheckStatus checks the status of the development environment
func (dem *DevEnvironmentManager) CheckStatus(ctx context.Context) (*DevEnvironmentStatus, error) {
	status := &DevEnvironmentStatus{
		ProjectRoot:     dem.projectRoot,
		Environment:     dem.config.Environment,
		HealthChecks:    []HealthCheck{},
		Recommendations: []string{},
		LastChecked:     time.Now(),
	}

	// Change to project directory
	if err := os.Chdir(dem.projectRoot); err != nil {
		return nil, fmt.Errorf("failed to change to project directory: %w", err)
	}

	// Check git status
	status.GitStatus = dem.checkGitStatus()

	// Check dependencies
	status.Dependencies = dem.checkDependencyStatus()

	// Check services
	status.Services = dem.checkServiceStatus(ctx)

	// Check database
	status.Database = dem.checkDatabaseStatus()

	// Check plugins
	status.Plugins = dem.checkPluginStatus()

	// Run health checks
	status.HealthChecks = dem.runHealthChecks(ctx)

	// Generate recommendations
	status.Recommendations = dem.generateRecommendations(status)

	return status, nil
}

// setupEnvironment sets up environment variables
func (dem *DevEnvironmentManager) setupEnvironment(ctx context.Context) error {
	for key, value := range dem.config.Environment {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
		logger.Debug("Set environment variable", "key", key, "value", value)
	}
	return nil
}

// checkDependencies checks for required development dependencies
func (dem *DevEnvironmentManager) checkDependencies(ctx context.Context) error {
	dependencies := []string{"go", "node", "docker", "make", "git"}
	
	for _, dep := range dependencies {
		if _, err := exec.LookPath(dep); err != nil {
			return fmt.Errorf("required dependency not found: %s", dep)
		}
		logger.Debug("Dependency available", "tool", dep)
	}
	return nil
}

// setupGoModules sets up Go modules
func (dem *DevEnvironmentManager) setupGoModules(ctx context.Context) error {
	// Run go mod tidy
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w\nOutput: %s", err, output)
	}

	// Download dependencies
	cmd = exec.CommandContext(ctx, "go", "mod", "download")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod download failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// setupDatabase sets up the development database
func (dem *DevEnvironmentManager) setupDatabase(ctx context.Context) error {
	// Run database migration/setup if needed
	dbPath := filepath.Join(dem.projectRoot, "viewra-data", "viewra.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Create viewra-data directory
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return fmt.Errorf("failed to create viewra-data directory: %w", err)
		}
		logger.Info("Created viewra-data directory")
	}

	return nil
}

// buildPlugins builds all plugins
func (dem *DevEnvironmentManager) buildPlugins(ctx context.Context) error {
	if !dem.config.BuildParallel {
		// Build sequentially
		cmd := exec.CommandContext(ctx, "make", "build-plugins")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("plugin build failed: %w\nOutput: %s", err, output)
		}
	} else {
		// TODO: Implement parallel building
		logger.Info("Parallel plugin building not implemented yet, using sequential")
		return dem.buildPlugins(ctx)
	}

	return nil
}

// checkGitStatus checks git repository status
func (dem *DevEnvironmentManager) checkGitStatus() *GitStatus {
	status := &GitStatus{}

	// Get current branch
	if output, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		status.Branch = strings.TrimSpace(string(output))
	}

	// Get commit hash
	if output, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		status.CommitHash = strings.TrimSpace(string(output))[:8]
	}

	// Check for changes
	if output, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			status.HasChanges = true
			if strings.HasPrefix(line, "??") {
				status.UntrackedFiles = append(status.UntrackedFiles, strings.TrimSpace(line[2:]))
			} else {
				status.ModifiedFiles = append(status.ModifiedFiles, strings.TrimSpace(line[2:]))
			}
		}
	}

	// Check remote status
	if output, err := exec.Command("git", "status", "-b", "--porcelain").Output(); err == nil {
		firstLine := strings.Split(string(output), "\n")[0]
		if strings.Contains(firstLine, "ahead") {
			status.RemoteStatus = "ahead"
		} else if strings.Contains(firstLine, "behind") {
			status.RemoteStatus = "behind"
		} else {
			status.RemoteStatus = "up-to-date"
		}
	}

	return status
}

// checkDependencyStatus checks status of all dependencies
func (dem *DevEnvironmentManager) checkDependencyStatus() *DependencyStatus {
	checkTool := func(name string) *ToolStatus {
		path, err := exec.LookPath(name)
		if err != nil {
			return &ToolStatus{Available: false}
		}

		var version string
		if output, err := exec.Command(name, "--version").Output(); err == nil {
			version = strings.Split(string(output), "\n")[0]
		}

		return &ToolStatus{
			Available: true,
			Version:   version,
			Path:      path,
		}
	}

	// Check if go modules are tidy
	modulesTidy := true
	if output, err := exec.Command("go", "mod", "tidy", "-diff").Output(); err != nil || len(output) > 0 {
		modulesTidy = false
	}

	return &DependencyStatus{
		Go:      checkTool("go"),
		Node:    checkTool("node"),
		Docker:  checkTool("docker"),
		Make:    checkTool("make"),
		Git:     checkTool("git"),
		Modules: modulesTidy,
	}
}

// checkServiceStatus checks status of running services
func (dem *DevEnvironmentManager) checkServiceStatus(ctx context.Context) *ServiceStatus {
	checkService := func(name, url string) *ServiceInfo {
		info := &ServiceInfo{URL: url}
		
		// Try to connect to the service
		client := &http.Client{Timeout: 2 * time.Second}
		if resp, err := client.Get(url + "/api/health"); err == nil {
			resp.Body.Close()
			info.Running = true
			info.Health = "healthy"
		} else {
			info.Running = false
			info.Health = "unreachable"
		}

		return info
	}

	return &ServiceStatus{
		Backend:  checkService("backend", dem.config.Services["backend"]),
		Frontend: checkService("frontend", dem.config.Services["frontend"]),
		Database: &ServiceInfo{Running: true, Health: "file-based"}, // SQLite
	}
}

// checkDatabaseStatus checks database status
func (dem *DevEnvironmentManager) checkDatabaseStatus() *DatabaseStatus {
	dbPath := filepath.Join(dem.projectRoot, "viewra-data", "viewra.db")
	status := &DatabaseStatus{
		Type:     "sqlite",
		Location: dbPath,
	}

	if stat, err := os.Stat(dbPath); err == nil {
		status.Available = true
		status.Size = fmt.Sprintf("%.2f MB", float64(stat.Size())/(1024*1024))
	}

	return status
}

// checkPluginStatus checks plugin system status
func (dem *DevEnvironmentManager) checkPluginStatus() *PluginStatus {
	status := &PluginStatus{}

	// Count available plugins
	pluginsDir := filepath.Join(dem.projectRoot, "plugins")
	if entries, err := os.ReadDir(pluginsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.Contains(entry.Name(), "_") {
				status.Available++
				
				// Check if binary exists
				binaryPath := filepath.Join(pluginsDir, entry.Name(), entry.Name())
				if _, err := os.Stat(binaryPath); err == nil {
					status.Built++
				}
			}
		}
	}

	return status
}

// runHealthChecks runs comprehensive health checks
func (dem *DevEnvironmentManager) runHealthChecks(ctx context.Context) []HealthCheck {
	checks := []HealthCheck{}

	// Add health checks
	healthChecks := []struct {
		name     string
		critical bool
		fn       func() (string, error)
	}{
		{"Go Installation", true, func() (string, error) {
			if _, err := exec.LookPath("go"); err != nil {
				return "failed", fmt.Errorf("Go not found in PATH")
			}
			return "passed", nil
		}},
		{"Docker Availability", false, func() (string, error) {
			if _, err := exec.LookPath("docker"); err != nil {
				return "warning", fmt.Errorf("Docker not found (optional for local development)")
			}
			return "passed", nil
		}},
		{"Project Structure", true, func() (string, error) {
			requiredDirs := []string{"internal", "plugins", "cmd", "sdk"}
			for _, dir := range requiredDirs {
				if _, err := os.Stat(filepath.Join(dem.projectRoot, dir)); os.IsNotExist(err) {
					return "failed", fmt.Errorf("required directory missing: %s", dir)
				}
			}
			return "passed", nil
		}},
	}

	for _, check := range healthChecks {
		start := time.Now()
		status, err := check.fn()
		duration := time.Since(start)

		hc := HealthCheck{
			Name:     check.name,
			Status:   status,
			Duration: duration,
			Critical: check.critical,
		}

		if err != nil {
			hc.Message = err.Error()
		} else {
			hc.Message = "OK"
		}

		checks = append(checks, hc)
	}

	return checks
}

// generateRecommendations generates recommendations based on status
func (dem *DevEnvironmentManager) generateRecommendations(status *DevEnvironmentStatus) []string {
	recommendations := []string{}

	// Git recommendations
	if status.GitStatus.HasChanges {
		recommendations = append(recommendations, "You have uncommitted changes. Consider committing them.")
	}

	// Dependency recommendations
	if !status.Dependencies.Modules {
		recommendations = append(recommendations, "Run 'go mod tidy' to clean up module dependencies.")
	}

	// Service recommendations
	if !status.Services.Backend.Running {
		recommendations = append(recommendations, "Backend service is not running. Start it with 'docker-compose up -d backend'.")
	}

	// Plugin recommendations
	if status.Plugins.Available > status.Plugins.Built {
		recommendations = append(recommendations, fmt.Sprintf("Some plugins are not built (%d/%d). Run 'make build-plugins'.", 
			status.Plugins.Built, status.Plugins.Available))
	}

	// Health check recommendations
	for _, check := range status.HealthChecks {
		if check.Status == "failed" && check.Critical {
			recommendations = append(recommendations, fmt.Sprintf("Critical issue: %s - %s", check.Name, check.Message))
		}
	}

	return recommendations
}

// Quick commands for common development tasks

// QuickStart starts the development environment
func (dem *DevEnvironmentManager) QuickStart(ctx context.Context) error {
	logger.Info("🚀 Quick starting development environment...")
	
	steps := []struct {
		name string
		cmd  []string
	}{
		{"Building plugins", []string{"make", "build-plugins"}},
		{"Starting backend", []string{"docker-compose", "up", "-d", "backend"}},
	}

	for _, step := range steps {
		logger.Info("Running: " + step.name)
		cmd := exec.CommandContext(ctx, step.cmd[0], step.cmd[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("Step failed", "step", step.name, "error", err, "output", string(output))
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}

	logger.Info("✅ Development environment started!")
	return nil
}

// CleanBuild performs a clean build of everything
func (dem *DevEnvironmentManager) CleanBuild(ctx context.Context) error {
	logger.Info("🧹 Performing clean build...")
	
	steps := [][]string{
		{"make", "clean"},
		{"go", "mod", "tidy"},
		{"make", "build-plugins"},
	}

	for _, step := range steps {
		logger.Info("Running: " + strings.Join(step, " "))
		cmd := exec.CommandContext(ctx, step[0], step[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("Clean build step failed", "cmd", strings.Join(step, " "), "error", err, "output", string(output))
			return fmt.Errorf("clean build failed at step '%s': %w", strings.Join(step, " "), err)
		}
	}

	logger.Info("✅ Clean build completed!")
	return nil
}