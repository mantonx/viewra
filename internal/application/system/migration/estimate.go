package migration

import (
	"context"
	"database/sql"
	"fmt"
)

// GetTableInfo gets information about tables in a database.
func GetTableInfo(ctx context.Context, db *sql.DB, driver string) ([]TableInfo, error) {
	var tables []TableInfo

	switch driver {
	case "postgres":
		rows, err := db.QueryContext(ctx, `
			SELECT 
				table_name,
				(SELECT COUNT(*) FROM information_schema.columns c WHERE c.table_name = t.table_name AND c.table_schema = 'public') as col_count
			FROM information_schema.tables t
			WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
			ORDER BY table_name
		`)
		if err != nil {
			return nil, fmt.Errorf("failed to query tables: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			var colCount int
			if err := rows.Scan(&name, &colCount); err != nil {
				continue
			}
			// Get row count
			var rowCount int64
			if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", name)).Scan(&rowCount); err != nil {
				rowCount = 0
			}

			tables = append(tables, TableInfo{
				Name:      name,
				Rows:      rowCount,
				SizeBytes: rowCount * int64(colCount) * 100, // Rough estimate: 100 bytes per column per row
			})
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating tables: %w", err)
		}

	case "sqlite":
		rows, err := db.QueryContext(ctx, `
			SELECT name FROM sqlite_master 
			WHERE type='table' AND name NOT LIKE 'sqlite_%'
			ORDER BY name
		`)
		if err != nil {
			return nil, fmt.Errorf("failed to query tables: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				continue
			}
			// Get row count
			var rowCount int64
			if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", name)).Scan(&rowCount); err != nil {
				rowCount = 0
			}

			// Get column count for size estimation
			var colCount int
			colRows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", name))
			if err == nil {
				for colRows.Next() {
					colCount++
				}
				colRows.Close()
			}
			if colCount == 0 {
				colCount = 10 // Default estimate
			}

			tables = append(tables, TableInfo{
				Name:      name,
				Rows:      rowCount,
				SizeBytes: rowCount * int64(colCount) * 100, // Rough estimate
			})
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating tables: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	return tables, nil
}

// EstimateMigration estimates the migration time and data size.
func EstimateMigration(ctx context.Context, db *sql.DB, sourceDriver, targetDriver string) (*EstimateResponse, error) {
	tables, err := GetTableInfo(ctx, db, sourceDriver)
	if err != nil {
		return nil, fmt.Errorf("failed to get table info: %w", err)
	}

	var totalRows int64
	var totalSize int64
	for _, t := range tables {
		totalRows += t.Rows
		totalSize += t.SizeBytes
	}

	// Estimate duration: ~1000 rows/second for SQLite->PG, ~5000 for PG->SQLite
	// These are conservative estimates accounting for network latency and type conversions
	rowsPerSecond := int64(1000)
	if targetDriver == "sqlite" {
		rowsPerSecond = 5000 // SQLite writes are faster (local file)
	}

	durationSeconds := int(totalRows / rowsPerSecond)
	if durationSeconds < 10 {
		durationSeconds = 10 // Minimum 10 seconds for setup overhead
	}

	// Add overhead for schema creation and verification
	durationSeconds += 10

	var warnings []string
	if sourceDriver == targetDriver {
		warnings = append(warnings, "Source and target use the same database driver")
	}
	if totalRows > 1000000 {
		warnings = append(warnings, "Large database detected - migration may take significant time")
	}

	return &EstimateResponse{
		Source: SourceInfo{
			Driver:     sourceDriver,
			SizeBytes:  totalSize,
			TableCount: len(tables),
			TotalRows:  totalRows,
		},
		Estimate: EstimateInfo{
			DurationSeconds: durationSeconds,
			DurationHuman:   formatDuration(durationSeconds),
			DataSizeBytes:   totalSize,
			Tables:          tables,
		},
		Warnings: warnings,
	}, nil
}
