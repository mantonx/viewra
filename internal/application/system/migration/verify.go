package migration

import (
	"context"
	"database/sql"
	"fmt"
)

// VerificationResult contains the result of migration verification.
type VerificationResult struct {
	Success      bool                      `json:"success"`
	TableResults []TableVerificationResult `json:"tableResults"`
	Errors       []string                  `json:"errors,omitempty"`
}

// TableVerificationResult contains verification result for a single table.
type TableVerificationResult struct {
	TableName  string `json:"tableName"`
	SourceRows int64  `json:"sourceRows"`
	TargetRows int64  `json:"targetRows"`
	Match      bool   `json:"match"`
	Error      string `json:"error,omitempty"`
}

// VerifyMigration verifies that data was correctly migrated by comparing row counts.
// It only verifies tables that exist in BOTH source AND target (some source tables
// like plugin tables may not exist in target).
func VerifyMigration(
	ctx context.Context,
	sourceDB *sql.DB,
	sourceDriver string,
	targetDB *sql.DB,
	targetDriver string,
) (*VerificationResult, error) {
	result := &VerificationResult{
		Success: true,
	}

	// Get tables from source
	sourceTables, err := GetTableInfo(ctx, sourceDB, sourceDriver)
	if err != nil {
		return nil, fmt.Errorf("failed to get source table info: %w", err)
	}

	// Get tables from target to filter verification
	targetTables, err := GetTableInfo(ctx, targetDB, targetDriver)
	if err != nil {
		return nil, fmt.Errorf("failed to get target table info: %w", err)
	}
	targetTableSet := make(map[string]bool)
	for _, t := range targetTables {
		targetTableSet[t.Name] = true
	}

	// Verify each source table that exists in target
	for _, sourceTable := range sourceTables {
		tableName := sourceTable.Name

		// Skip tables that don't exist in target (e.g., plugin tables)
		if !targetTableSet[tableName] {
			continue
		}

		sourceRows := sourceTable.Rows

		tableResult := TableVerificationResult{
			TableName:  tableName,
			SourceRows: sourceRows,
		}

		// Get target row count
		targetRows, err := getRowCount(ctx, targetDB, targetDriver, tableName)
		if err != nil {
			tableResult.Error = fmt.Sprintf("failed to get target row count: %v", err)
			tableResult.Match = false
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", tableName, tableResult.Error))
		} else {
			tableResult.TargetRows = targetRows
			tableResult.Match = sourceRows == targetRows
			if !tableResult.Match {
				result.Success = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: row count mismatch (source=%d, target=%d)",
						tableName, sourceRows, targetRows))
			}
		}

		result.TableResults = append(result.TableResults, tableResult)
	}

	return result, nil
}

// getRowCount returns the number of rows in a table.
func getRowCount(ctx context.Context, db *sql.DB, driver, tableName string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %q", tableName)

	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// QuickVerify performs a quick verification by comparing total row counts.
// This is faster than full verification but less thorough.
func QuickVerify(
	ctx context.Context,
	sourceDB *sql.DB,
	sourceDriver string,
	targetDB *sql.DB,
	targetDriver string,
) (bool, error) {
	// Get source table info
	sourceTables, err := GetTableInfo(ctx, sourceDB, sourceDriver)
	if err != nil {
		return false, fmt.Errorf("failed to get source table info: %w", err)
	}

	// Get target table info
	targetTables, err := GetTableInfo(ctx, targetDB, targetDriver)
	if err != nil {
		return false, fmt.Errorf("failed to get target table info: %w", err)
	}

	// Build target table map
	targetTableMap := make(map[string]int64)
	for _, t := range targetTables {
		targetTableMap[t.Name] = t.Rows
	}

	// Compare row counts
	for _, sourceTable := range sourceTables {
		targetRows, exists := targetTableMap[sourceTable.Name]
		if !exists {
			return false, nil
		}
		if sourceTable.Rows != targetRows {
			return false, nil
		}
	}

	return true, nil
}

// VerifySchema verifies that the target database has the expected schema.
func VerifySchema(ctx context.Context, db *sql.DB, driver string) error {
	// Check that all expected tables exist
	tables, err := GetTableInfo(ctx, db, driver)
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}

	tableSet := make(map[string]bool)
	for _, t := range tables {
		tableSet[t.Name] = true
	}

	// Check for core tables that should always exist
	requiredTables := []string{
		"libraries",
		"media",
		"users",
		"sessions",
		"system_settings",
		"plugins",
	}

	var missing []string
	for _, table := range requiredTables {
		if !tableSet[table] {
			missing = append(missing, table)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tables: %v", missing)
	}

	return nil
}
