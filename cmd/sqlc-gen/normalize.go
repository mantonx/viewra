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
	"sort"
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

	// Parse and fix PostgreSQL package
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
