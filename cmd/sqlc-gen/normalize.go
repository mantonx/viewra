package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// normalizeFieldOrder ensures PostgreSQL Params structs have the same field
// order as SQLite. This is necessary because sqlc orders fields by parameter
// position, and PostgreSQL/SQLite parse LIMIT/OFFSET differently.
func normalizeFieldOrder(cfg Config) error {
	// Parse SQLite package to get canonical field order
	sqliteStructs, err := parsePackageStructs(cfg.SQLiteDir)
	if err != nil {
		return fmt.Errorf("parsing sqlite: %w", err)
	}

	// Parse PostgreSQL package BEFORE fixing to get original order
	postgresStructs, err := parsePackageStructs(cfg.PostgresDir)
	if err != nil {
		return fmt.Errorf("parsing postgres: %w", err)
	}

	// Fix PostgreSQL SQL placeholders to match SQLite arg order.
	// This must be done BEFORE fixing struct order since we need the original
	// PostgreSQL field order to know how SQLC assigned placeholder numbers.
	placeholderFixed, err := fixPostgresPlaceholders(cfg.PostgresDir, sqliteStructs, postgresStructs)
	if err != nil {
		return fmt.Errorf("fixing postgres placeholders: %w", err)
	}
	if placeholderFixed > 0 {
		fmt.Printf("  Fixed SQL placeholders in %d files\n", placeholderFixed)
	}

	// Parse and fix PostgreSQL struct field order
	fixed, err := fixPackageStructOrder(cfg.PostgresDir, sqliteStructs)
	if err != nil {
		return fmt.Errorf("fixing postgres: %w", err)
	}

	if fixed > 0 {
		fmt.Printf("  Fixed field order in %d structs\n", fixed)
	} else {
		fmt.Println("  No field order fixes needed")
	}

	// Fix SQLite slice types: []sql.NullInt64 -> []int64
	sliceFixed, err := fixSQLiteSliceTypes(cfg.SQLiteDir)
	if err != nil {
		return fmt.Errorf("fixing sqlite slice types: %w", err)
	}
	if sliceFixed > 0 {
		fmt.Printf("  Fixed slice types in %d files\n", sliceFixed)
	}

	return nil
}

// parsePackageStructs extracts all Params struct field orders from a package.
func parsePackageStructs(dir string) (map[string][]string, error) {
	result := make(map[string][]string)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		return nil, err
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}

				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					// Only process Params structs
					if !strings.HasSuffix(typeSpec.Name.Name, "Params") {
						continue
					}

					structType, ok := typeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					var fields []string
					for _, field := range structType.Fields.List {
						for _, name := range field.Names {
							fields = append(fields, name.Name)
						}
					}

					result[typeSpec.Name.Name] = fields
				}
			}
		}
	}

	return result, nil
}

// fixPackageStructOrder reorders fields in Params structs to match canonical order.
func fixPackageStructOrder(dir string, canonicalOrder map[string][]string) (int, error) {
	fixedCount := 0

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		fixed, err := fixFileStructOrder(file, canonicalOrder)
		if err != nil {
			return 0, fmt.Errorf("fixing %s: %w", file, err)
		}
		fixedCount += fixed
	}

	return fixedCount, nil
}

// fixPostgresPlaceholders swaps PostgreSQL $N placeholders in SQL strings when
// the corresponding function arguments have been reordered. PostgreSQL SQLC
// assigns placeholder numbers based on alphabetical order of named args, but
// we reorder args to match SQLite's positional order. This function swaps the
// placeholder numbers in the SQL string to match the reordered arg positions.
//
// For example, if SQLite has LIMIT ? OFFSET ? (positions 2, 3 for limit, offset)
// but PostgreSQL has LIMIT $3 OFFSET $2 (because 'l'imit < 'o'ffset alphabetically),
// we need to swap $2 and $3 in the SQL string so LIMIT $2 OFFSET $3.
func fixPostgresPlaceholders(dir string, sqliteStructs map[string][]string, postgresStructs map[string][]string) (int, error) {
	fixedCount := 0

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return 0, fmt.Errorf("reading %s: %w", file, err)
		}

		original := string(content)
		modified := original

		// For each Params struct, check if field order differs from SQLite
		for structName, sqliteFields := range sqliteStructs {
			pgFields, exists := postgresStructs[structName]
			if !exists || len(pgFields) < 2 {
				continue
			}

			// Build mapping from PostgreSQL position to SQLite position
			// PostgreSQL position is the order SQLC used (alphabetical by named arg)
			// SQLite position is the canonical order we want
			pgToSqlite := buildPlaceholderMapping(pgFields, sqliteFields)
			if pgToSqlite == nil {
				continue // No reordering needed
			}

			// Find the SQL const name (structName without "Params" suffix, lowercase first char)
			queryName := strings.TrimSuffix(structName, "Params")
			queryName = strings.ToLower(queryName[:1]) + queryName[1:]

			// Find and fix the SQL const in the file
			modified = fixSQLConst(modified, queryName, pgToSqlite)
		}

		if modified != original {
			if err := os.WriteFile(file, []byte(modified), 0644); err != nil {
				return 0, fmt.Errorf("writing %s: %w", file, err)
			}
			fixedCount++
		}
	}

	return fixedCount, nil
}

// buildPlaceholderMapping returns a map from PostgreSQL placeholder position to
// SQLite placeholder position, or nil if no reordering is needed.
// Positions are 1-based (matching PostgreSQL $1, $2, $3 style).
func buildPlaceholderMapping(pgFields, sqliteFields []string) map[int]int {
	if len(pgFields) != len(sqliteFields) {
		return nil
	}

	// Build position maps (0-based index in slice = position - 1 in SQL)
	pgPos := make(map[string]int)
	sqlitePos := make(map[string]int)
	for i, name := range pgFields {
		pgPos[name] = i + 1 // 1-based
	}
	for i, name := range sqliteFields {
		sqlitePos[name] = i + 1 // 1-based
	}

	// Check if any reordering is needed
	needsReorder := false
	for name, pg := range pgPos {
		sqlite, exists := sqlitePos[name]
		if !exists {
			return nil // Field mismatch
		}
		if pg != sqlite {
			needsReorder = true
			break
		}
	}

	if !needsReorder {
		return nil
	}

	// Build the mapping: pg position -> sqlite position
	mapping := make(map[int]int)
	for name, pg := range pgPos {
		mapping[pg] = sqlitePos[name]
	}

	return mapping
}

// fixSQLConst finds a SQL const by name and swaps placeholder numbers according to mapping.
func fixSQLConst(content, queryName string, mapping map[int]int) string {
	// Pattern to find the const definition
	constPattern := regexp.MustCompile(`(?s)(const\s+` + regexp.QuoteMeta(queryName) + `\s*=\s*` + "`" + `)([^` + "`" + `]+)(` + "`" + `)`)

	return constPattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := constPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		prefix := parts[1]
		sql := parts[2]
		suffix := parts[3]

		// Replace $N with temporary placeholders first to avoid conflicts
		placeholderPattern := regexp.MustCompile(`\$(\d+)`)

		// First pass: replace with temporary markers
		tempSQL := placeholderPattern.ReplaceAllStringFunc(sql, func(ph string) string {
			numStr := ph[1:] // Remove $
			num, err := strconv.Atoi(numStr)
			if err != nil {
				return ph
			}
			if newNum, exists := mapping[num]; exists {
				return fmt.Sprintf("__PLACEHOLDER_%d__", newNum)
			}
			return ph
		})

		// Second pass: replace temporary markers with final $N
		tempPattern := regexp.MustCompile(`__PLACEHOLDER_(\d+)__`)
		finalSQL := tempPattern.ReplaceAllStringFunc(tempSQL, func(temp string) string {
			numStr := strings.TrimPrefix(strings.TrimSuffix(temp, "__"), "__PLACEHOLDER_")
			return "$" + numStr
		})

		return prefix + finalSQL + suffix
	})
}

func fixFileStructOrder(filename string, canonicalOrder map[string][]string) (int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return 0, err
	}

	fixedCount := 0
	modified := false

	// Build a map of struct name -> field reorder mapping
	reorderMaps := make(map[string]map[string]int)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			if !strings.HasSuffix(typeSpec.Name.Name, "Params") {
				continue
			}

			canonical, exists := canonicalOrder[typeSpec.Name.Name]
			if !exists {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Check if reordering is needed
			currentOrder := getFieldOrder(structType)
			if equalOrder(currentOrder, canonical) {
				continue
			}

			// Build reorder map for this struct
			reorderMap := make(map[string]int)
			for i, name := range canonical {
				reorderMap[name] = i
			}
			reorderMaps[typeSpec.Name.Name] = reorderMap

			// Reorder fields
			if reorderFields(structType, canonical) {
				fixedCount++
				modified = true
			}
		}
	}

	// Fix function calls that use the reordered structs
	if len(reorderMaps) > 0 {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			methodName := sel.Sel.Name
			if methodName != "QueryContext" && methodName != "ExecContext" {
				return true
			}

			fixCallArgs(call, reorderMaps)
			return true
		})
	}

	if modified {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, file); err != nil {
			return 0, fmt.Errorf("formatting: %w", err)
		}

		if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
			return 0, fmt.Errorf("writing: %w", err)
		}
	}

	return fixedCount, nil
}

func fixCallArgs(call *ast.CallExpr, reorderMaps map[string]map[string]int) {
	if len(call.Args) < 3 {
		return
	}

	type argInfo struct {
		index     int
		fieldName string
		expr      ast.Expr
	}

	var argFields []argInfo
	for i := 2; i < len(call.Args); i++ {
		sel, ok := call.Args[i].(*ast.SelectorExpr)
		if !ok {
			continue
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "arg" {
			continue
		}
		argFields = append(argFields, argInfo{
			index:     i,
			fieldName: sel.Sel.Name,
			expr:      call.Args[i],
		})
	}

	if len(argFields) < 2 {
		return
	}

	for _, reorderMap := range reorderMaps {
		allMatch := true
		for _, af := range argFields {
			if _, exists := reorderMap[af.fieldName]; !exists {
				allMatch = false
				break
			}
		}

		if !allMatch {
			continue
		}

		needsReorder := false
		for i := 0; i < len(argFields)-1; i++ {
			pos1 := reorderMap[argFields[i].fieldName]
			pos2 := reorderMap[argFields[i+1].fieldName]
			if pos1 > pos2 {
				needsReorder = true
				break
			}
		}

		if !needsReorder {
			continue
		}

		sort.Slice(argFields, func(i, j int) bool {
			return reorderMap[argFields[i].fieldName] < reorderMap[argFields[j].fieldName]
		})

		for i, af := range argFields {
			call.Args[2+i] = af.expr
		}
		break
	}
}

func getFieldOrder(s *ast.StructType) []string {
	var order []string
	for _, field := range s.Fields.List {
		for _, name := range field.Names {
			order = append(order, name.Name)
		}
	}
	return order
}

func equalOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func reorderFields(s *ast.StructType, order []string) bool {
	fieldMap := make(map[string]*ast.Field)
	for _, field := range s.Fields.List {
		for _, name := range field.Names {
			fieldMap[name.Name] = field
		}
	}

	if len(fieldMap) != len(order) {
		return false
	}

	for _, name := range order {
		if _, exists := fieldMap[name]; !exists {
			return false
		}
	}

	newFields := make([]*ast.Field, 0, len(order))
	for _, name := range order {
		newFields = append(newFields, fieldMap[name])
	}

	s.Fields.List = newFields
	return true
}

// fixSQLiteSliceTypes fixes sqlc.slice() generated types from []sql.NullInt64 to []int64.
func fixSQLiteSliceTypes(dir string) (int, error) {
	fixedCount := 0

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return 0, err
	}

	replacements := map[string]string{
		"[]sql.NullInt64": "[]int64",
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return 0, fmt.Errorf("reading %s: %w", file, err)
		}

		original := string(content)
		modified := original

		for old, new := range replacements {
			if strings.Contains(modified, old) {
				modified = strings.ReplaceAll(modified, old, new)
			}
		}

		if modified != original {
			if err := os.WriteFile(file, []byte(modified), 0644); err != nil {
				return 0, fmt.Errorf("writing %s: %w", file, err)
			}
			fixedCount++
		}
	}

	return fixedCount, nil
}
