// Command sqlc-gen performs post-processing on sqlc-generated code to enable
// a unified database layer that works with both SQLite and PostgreSQL.
//
// This tool is run automatically as part of `make sqlc-gen` after `sqlc generate`.
// It performs three steps:
//
//  1. Normalize struct field order (PostgreSQL → SQLite order)
//  2. Generate unified type aliases (types.go)
//  3. Generate unified Querier wrapper (querier_gen.go)
//
// Usage:
//
//	go run cmd/sqlc-gen/main.go
//
// Or via make:
//
//	make sqlc-gen
package main

import (
	"fmt"
	"os"
	"strings"
)

// Config holds paths and settings for code generation.
// Paths are relative to the project root.
type Config struct {
	// ModulePath is the Go module path (read from go.mod)
	ModulePath string

	// SQLiteDir is the path to sqlc-generated SQLite code
	SQLiteDir string

	// PostgresDir is the path to sqlc-generated PostgreSQL code
	PostgresDir string

	// UnifiedDir is the path to the unified package
	UnifiedDir string

	// TypesFile is the output path for type aliases
	TypesFile string

	// QuerierFile is the output path for the unified Querier
	QuerierFile string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		SQLiteDir:   "internal/infrastructure/database/sqlc_sqlite",
		PostgresDir: "internal/infrastructure/database/sqlc_postgres",
		UnifiedDir:  "internal/infrastructure/database/unified",
		TypesFile:   "internal/infrastructure/database/unified/types.go",
		QuerierFile: "internal/infrastructure/database/unified/querier_gen.go",
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := DefaultConfig()

	// Read module path from go.mod
	modulePath, err := readModulePath()
	if err != nil {
		return fmt.Errorf("reading go.mod: %w", err)
	}
	cfg.ModulePath = modulePath

	// Step 1: Normalize struct field order
	fmt.Println("Step 1: Normalizing struct field order...")
	if err := normalizeFieldOrder(cfg); err != nil {
		return fmt.Errorf("normalizing field order: %w", err)
	}

	// Step 2: Generate unified type aliases
	fmt.Println("Step 2: Generating unified type aliases...")
	if err := generateTypeAliases(cfg); err != nil {
		return fmt.Errorf("generating type aliases: %w", err)
	}

	// Step 3: Generate unified Querier wrapper
	fmt.Println("Step 3: Generating unified Querier wrapper...")
	if err := generateQuerier(cfg); err != nil {
		return fmt.Errorf("generating querier: %w", err)
	}

	fmt.Println("Done!")
	return nil
}

// readModulePath extracts the module path from go.mod.
func readModulePath() (string, error) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}
