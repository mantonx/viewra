package migration

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/system"
)

// ConfigSaver is a function that saves database configuration after migration.
// This is used to break the import cycle with the config package.
type ConfigSaver func(driver string, pgHost string, pgPort int, pgUser, pgDatabase, pgSSLMode string) error

// Service orchestrates database migrations.
type Service struct {
	mu             sync.RWMutex
	sourceDB       *sql.DB
	sourceDriver   string
	state          State
	maintenanceMgr *system.MaintenanceManager
	configSaver    ConfigSaver
	onChange       func(state State)
}

// NewService creates a new migration service.
func NewService(sourceDB *sql.DB, sourceDriver string, maintenanceMgr *system.MaintenanceManager, configSaver ConfigSaver) *Service {
	return &Service{
		sourceDB:       sourceDB,
		sourceDriver:   sourceDriver,
		maintenanceMgr: maintenanceMgr,
		configSaver:    configSaver,
		state:          State{Status: StatusIdle},
	}
}

// SetOnChange sets a callback for state changes.
func (s *Service) SetOnChange(fn func(state State)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// GetState returns the current migration state.
func (s *Service) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// IsInProgress returns whether a migration is currently in progress.
func (s *Service) IsInProgress() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Status == StatusInProgress
}

// TestConnection tests a connection to the target database.
func (s *Service) TestConnection(ctx context.Context, config TargetConfig) ConnectionTestResult {
	return TestConnection(ctx, config)
}

// Estimate estimates the migration time and data size.
func (s *Service) Estimate(ctx context.Context, targetDriver string) (*EstimateResponse, error) {
	return EstimateMigration(ctx, s.sourceDB, s.sourceDriver, targetDriver)
}

// Start begins a database migration.
func (s *Service) Start(ctx context.Context, config MigrationRequest) (*MigrationStartResponse, error) {
	s.mu.Lock()
	if s.state.Status == StatusInProgress {
		s.mu.Unlock()
		return &MigrationStartResponse{
			Started:     false,
			MigrationID: s.state.MigrationID,
			Error:       "Migration already in progress",
		}, nil
	}

	// Generate migration ID
	migrationID := generateMigrationID()
	now := time.Now().UTC()

	// Initialize state
	s.state = State{
		Status:      StatusInProgress,
		MigrationID: migrationID,
		Phase:       PhaseMaintenanceMode,
		StartedAt:   &now,
		Phases:      s.initPhases(),
		Progress: &Progress{
			TablesTotal: 0,
			RowsTotal:   0,
		},
	}
	s.mu.Unlock()

	// Notify listeners
	s.notifyChange()

	// Start migration in background with a fresh context
	// (the HTTP request context will be cancelled when the response is sent)
	go s.runMigration(context.Background(), config)

	return &MigrationStartResponse{
		Started:     true,
		MigrationID: migrationID,
		Message:     "Migration started. Server will enter maintenance mode.",
	}, nil
}

// runMigration executes the migration process.
func (s *Service) runMigration(ctx context.Context, config MigrationRequest) {
	var targetDB *sql.DB
	var err error

	// Recover from panics to ensure we don't leave the service in a broken state
	defer func() {
		if r := recover(); r != nil {
			s.failMigration(s.state.Phase, "", fmt.Errorf("migration panicked: %v", r))
		}
	}()

	defer func() {
		if targetDB != nil {
			targetDB.Close()
		}
	}()

	// Phase 1: Enable maintenance mode
	if err := s.executePhase(PhaseMaintenanceMode, func() error {
		s.maintenanceMgr.Enable("Database migration in progress", 0)
		return nil
	}); err != nil {
		s.failMigration(PhaseMaintenanceMode, "", err)
		return
	}

	// Phase 2: Backup (placeholder - actual backup logic would go here)
	if err := s.executePhase(PhaseBackup, func() error {
		// For now, we just mark this as complete
		// In a real implementation, we'd create a backup of the source database
		return nil
	}); err != nil {
		s.failMigration(PhaseBackup, "", err)
		return
	}

	// Phase 3: Connect to target
	if err := s.executePhase(PhaseConnectTarget, func() error {
		targetConfig := TargetConfig{
			Driver:   config.TargetDriver,
			Postgres: config.Postgres,
			SQLite:   config.SQLite,
		}
		targetDB, err = OpenTargetDB(targetConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to target: %w", err)
		}
		return targetDB.PingContext(ctx)
	}); err != nil {
		s.failMigration(PhaseConnectTarget, "", err)
		return
	}

	// Phase 4: Create schema
	if err := s.executePhase(PhaseCreateSchema, func() error {
		return s.createSchema(ctx, targetDB, config.TargetDriver)
	}); err != nil {
		s.failMigration(PhaseCreateSchema, "", err)
		return
	}

	// Phase 5: Copy data
	if err := s.executePhase(PhaseCopying, func() error {
		return s.copyData(ctx, targetDB, config.TargetDriver)
	}); err != nil {
		s.failMigration(PhaseCopying, "", err)
		return
	}

	// Phase 6: Verification
	if err := s.executePhase(PhaseVerification, func() error {
		return s.verifyMigration(ctx, targetDB)
	}); err != nil {
		s.failMigration(PhaseVerification, "", err)
		return
	}

	// Phase 7: Update config
	if err := s.executePhase(PhaseUpdateConfig, func() error {
		return s.updateConfig(config)
	}); err != nil {
		s.failMigration(PhaseUpdateConfig, "", err)
		return
	}

	// Complete migration
	s.completeMigration()
}

// executePhase executes a migration phase.
func (s *Service) executePhase(phase Phase, fn func() error) error {
	s.mu.Lock()
	s.state.Phase = phase
	s.updatePhaseStatus(phase, PhaseStatusInProgress)
	s.mu.Unlock()
	s.notifyChange()

	if err := fn(); err != nil {
		s.mu.Lock()
		s.updatePhaseStatus(phase, PhaseStatusFailed)
		s.mu.Unlock()
		return err
	}

	s.mu.Lock()
	s.updatePhaseStatus(phase, PhaseStatusCompleted)
	s.mu.Unlock()
	s.notifyChange()

	return nil
}

// failMigration marks the migration as failed.
func (s *Service) failMigration(phase Phase, table string, err error) {
	s.mu.Lock()
	now := time.Now().UTC()
	s.state.Status = StatusFailed
	s.state.FailedAt = &now
	s.state.Error = &Error{
		Phase:   phase,
		Table:   table,
		Message: err.Error(),
	}
	s.mu.Unlock()

	// Disable maintenance mode on failure (outside lock)
	s.maintenanceMgr.Disable()

	// Notify after releasing lock to avoid deadlock
	s.notifyChange()
}

// completeMigration marks the migration as complete.
func (s *Service) completeMigration() {
	s.mu.Lock()
	now := time.Now().UTC()
	var durationSeconds int
	if s.state.StartedAt != nil {
		durationSeconds = int(now.Sub(*s.state.StartedAt).Seconds())
	}

	s.state.Status = StatusCompleted
	s.state.CompletedAt = &now
	s.state.Result = &Result{
		TablesMigrated:     s.state.Progress.TablesTotal,
		RowsMigrated:       s.state.Progress.RowsTotal,
		VerificationPassed: true,
		RequiresRestart:    true,
		DurationSeconds:    durationSeconds,
	}
	s.mu.Unlock()

	// Don't disable maintenance mode - server needs to restart
	s.maintenanceMgr.UpdateReason("Migration complete. Restart required.")

	// Notify after releasing lock to avoid deadlock
	s.notifyChange()
}

// notifyChange notifies listeners of state changes.
func (s *Service) notifyChange() {
	s.mu.RLock()
	onChange := s.onChange
	state := s.state
	s.mu.RUnlock()

	if onChange != nil {
		go onChange(state)
	}
}

// updatePhaseStatus updates the status of a phase.
func (s *Service) updatePhaseStatus(phase Phase, status PhaseStatus) {
	for i := range s.state.Phases {
		if s.state.Phases[i].Name == phase {
			s.state.Phases[i].Status = status
			break
		}
	}
}

// initPhases initializes the phases list.
func (s *Service) initPhases() []PhaseInfo {
	return []PhaseInfo{
		{Name: PhaseMaintenanceMode, Status: PhaseStatusPending},
		{Name: PhaseBackup, Status: PhaseStatusPending},
		{Name: PhaseConnectTarget, Status: PhaseStatusPending},
		{Name: PhaseCreateSchema, Status: PhaseStatusPending},
		{Name: PhaseCopying, Status: PhaseStatusPending},
		{Name: PhaseVerification, Status: PhaseStatusPending},
		{Name: PhaseUpdateConfig, Status: PhaseStatusPending},
	}
}

// getTableInfo gets information about tables in the source database.
func (s *Service) getTableInfo(ctx context.Context) ([]TableInfo, error) {
	return GetTableInfo(ctx, s.sourceDB, s.sourceDriver)
}

// createSchema creates the schema in the target database.
func (s *Service) createSchema(_ context.Context, targetDB *sql.DB, targetDriver string) error {
	return CreateSchema(targetDB, targetDriver, GetMigrationsPath())
}

// copyData copies data from source to target.
func (s *Service) copyData(ctx context.Context, targetDB *sql.DB, targetDriver string) error {
	return TransferData(ctx, s.sourceDB, s.sourceDriver, targetDB, targetDriver, func(progress TransferProgress) {
		s.mu.Lock()
		s.state.Progress.CurrentTable = progress.CurrentTable
		s.state.Progress.TablesCompleted = progress.TablesCompleted
		s.state.Progress.TablesTotal = progress.TablesTotal
		s.state.Progress.RowsCopied = progress.RowsCopied
		s.state.Progress.RowsTotal = progress.RowsTotal
		if progress.RowsTotal > 0 {
			s.state.Progress.PercentComplete = int(float64(progress.RowsCopied) / float64(progress.RowsTotal) * 100)
		}
		s.mu.Unlock()
		s.notifyChange()
	})
}

// verifyMigration verifies the migration was successful.
func (s *Service) verifyMigration(ctx context.Context, targetDB *sql.DB) error {
	// Get target driver from the database
	// We need to determine the driver based on the DB connection
	// For now, we'll use the quick verify which just checks row counts
	result, err := VerifyMigration(ctx, s.sourceDB, s.sourceDriver, targetDB, s.getTargetDriver(targetDB))
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("verification failed: %v", result.Errors)
	}

	return nil
}

// getTargetDriver attempts to determine the database driver from a connection.
func (s *Service) getTargetDriver(db *sql.DB) string {
	// Try to detect PostgreSQL
	var version string
	if err := db.QueryRow("SELECT version()").Scan(&version); err == nil {
		if len(version) > 10 && version[:10] == "PostgreSQL" {
			return "postgres"
		}
	}

	// Try to detect SQLite
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err == nil {
		return "sqlite"
	}

	// Default to postgres (safer assumption for production)
	return "postgres"
}

// updateConfig saves the new database configuration after successful migration.
func (s *Service) updateConfig(config MigrationRequest) error {
	if s.configSaver == nil {
		// No config saver configured - this is fine for tests
		return nil
	}

	switch config.TargetDriver {
	case "postgres", "postgresql":
		if config.Postgres == nil {
			return fmt.Errorf("postgres config is required for postgres driver")
		}
		return s.configSaver(
			"postgres",
			config.Postgres.Host,
			config.Postgres.Port,
			config.Postgres.User,
			config.Postgres.Database,
			config.Postgres.SSLMode,
		)
	case "sqlite", "sqlite3":
		if config.SQLite == nil {
			return fmt.Errorf("sqlite config is required for sqlite driver")
		}
		// For SQLite, we pass empty postgres values
		return s.configSaver("sqlite", "", 0, "", config.SQLite.Path, "")
	default:
		return fmt.Errorf("unsupported target driver: %s", config.TargetDriver)
	}
}
