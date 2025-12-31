package migration

import (
	"time"
)

// Status represents the current migration status.
type Status string

const (
	StatusIdle       Status = "idle"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// Phase represents a migration phase.
type Phase string

const (
	PhaseMaintenanceMode Phase = "maintenance_mode"
	PhaseBackup          Phase = "backup"
	PhaseConnectTarget   Phase = "connect_target"
	PhaseCreateSchema    Phase = "create_schema"
	PhaseCopying         Phase = "copying"
	PhaseVerification    Phase = "verification"
	PhaseUpdateConfig    Phase = "update_config"
)

// PhaseStatus represents the status of a migration phase.
type PhaseStatus string

const (
	PhaseStatusPending    PhaseStatus = "pending"
	PhaseStatusInProgress PhaseStatus = "in_progress"
	PhaseStatusCompleted  PhaseStatus = "completed"
	PhaseStatusFailed     PhaseStatus = "failed"
	PhaseStatusSkipped    PhaseStatus = "skipped"
)

// PhaseInfo contains information about a migration phase.
type PhaseInfo struct {
	Name   Phase       `json:"name"`
	Status PhaseStatus `json:"status"`
}

// Progress contains migration progress information.
type Progress struct {
	CurrentTable              string `json:"currentTable,omitempty"`
	TablesCompleted           int    `json:"tablesCompleted"`
	TablesTotal               int    `json:"tablesTotal"`
	RowsCopied                int64  `json:"rowsCopied"`
	RowsTotal                 int64  `json:"rowsTotal"`
	BytesCopied               int64  `json:"bytesCopied"`
	BytesTotal                int64  `json:"bytesTotal"`
	PercentComplete           int    `json:"percentComplete"`
	ElapsedSeconds            int    `json:"elapsedSeconds"`
	EstimatedRemainingSeconds int    `json:"estimatedRemainingSeconds"`
}

// State represents the current migration state.
type State struct {
	Status      Status      `json:"status"`
	MigrationID string      `json:"migrationId,omitempty"`
	Phase       Phase       `json:"phase,omitempty"`
	StartedAt   *time.Time  `json:"startedAt,omitempty"`
	CompletedAt *time.Time  `json:"completedAt,omitempty"`
	FailedAt    *time.Time  `json:"failedAt,omitempty"`
	Progress    *Progress   `json:"progress,omitempty"`
	Phases      []PhaseInfo `json:"phases,omitempty"`
	Result      *Result     `json:"result,omitempty"`
	Error       *Error      `json:"error,omitempty"`
}

// Result contains migration result information.
type Result struct {
	TablesMigrated     int    `json:"tablesMigrated"`
	RowsMigrated       int64  `json:"rowsMigrated"`
	VerificationPassed bool   `json:"verificationPassed"`
	OldDatabasePath    string `json:"oldDatabasePath,omitempty"`
	RequiresRestart    bool   `json:"requiresRestart"`
	DurationSeconds    int    `json:"durationSeconds"`
}

// Error contains migration error information.
type Error struct {
	Phase   Phase  `json:"phase"`
	Table   string `json:"table,omitempty"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// TargetConfig contains configuration for the target database.
type TargetConfig struct {
	Driver   string          `json:"driver"`
	Postgres *PostgresConfig `json:"postgres,omitempty"`
	SQLite   *SQLiteConfig   `json:"sqlite,omitempty"`
}

// PostgresConfig contains PostgreSQL connection settings.
type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"sslMode"`
}

// SQLiteConfig contains SQLite connection settings.
type SQLiteConfig struct {
	Path string `json:"path"`
}

// ConnectionTestResult contains the result of a connection test.
type ConnectionTestResult struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Error   string                 `json:"error,omitempty"`
	Details *ConnectionTestDetails `json:"details,omitempty"`
}

// ConnectionTestDetails contains detailed connection information.
type ConnectionTestDetails struct {
	Version        string    `json:"version"`
	ServerTime     time.Time `json:"serverTime"`
	IsEmpty        bool      `json:"isEmpty"`
	ExistingTables int       `json:"existingTables"`
}

// EstimateRequest contains the request for migration estimation.
type EstimateRequest struct {
	TargetDriver string `json:"targetDriver"`
}

// EstimateResponse contains the migration estimation result.
type EstimateResponse struct {
	Source   SourceInfo   `json:"source"`
	Estimate EstimateInfo `json:"estimate"`
	Warnings []string     `json:"warnings"`
}

// SourceInfo contains information about the source database.
type SourceInfo struct {
	Driver     string `json:"driver"`
	SizeBytes  int64  `json:"sizeBytes"`
	TableCount int    `json:"tableCount"`
	TotalRows  int64  `json:"totalRows"`
}

// EstimateInfo contains estimated migration metrics.
type EstimateInfo struct {
	DurationSeconds int         `json:"durationSeconds"`
	DurationHuman   string      `json:"durationHuman"`
	DataSizeBytes   int64       `json:"dataSizeBytes"`
	Tables          []TableInfo `json:"tables"`
}

// TableInfo contains information about a table.
type TableInfo struct {
	Name      string `json:"name"`
	Rows      int64  `json:"rows"`
	SizeBytes int64  `json:"sizeBytes"`
}

// MigrationRequest contains the request to start a migration.
type MigrationRequest struct {
	TargetDriver string          `json:"targetDriver"`
	Postgres     *PostgresConfig `json:"postgres,omitempty"`
	SQLite       *SQLiteConfig   `json:"sqlite,omitempty"`
}

// MigrationStartResponse contains the response when starting a migration.
type MigrationStartResponse struct {
	Started     bool   `json:"started"`
	MigrationID string `json:"migrationId,omitempty"`
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
}
