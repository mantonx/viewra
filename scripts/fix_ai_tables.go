//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	sqlite_vec.Auto()
}

func main() {
	db, err := sql.Open("sqlite3", "data/viewra.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Try to drop vec_embeddings using writable_schema
	fmt.Println("Attempting to remove orphaned vec_embeddings...")

	_, err = db.Exec("PRAGMA writable_schema = ON")
	if err != nil {
		fmt.Printf("Warning: PRAGMA writable_schema failed: %v\n", err)
	}

	_, err = db.Exec("DELETE FROM sqlite_master WHERE type = 'table' AND name = 'vec_embeddings'")
	if err != nil {
		fmt.Printf("Warning: DELETE from sqlite_master failed: %v\n", err)
	} else {
		fmt.Println("Removed vec_embeddings from schema")
	}

	_, err = db.Exec("PRAGMA writable_schema = OFF")
	if err != nil {
		fmt.Printf("Warning: PRAGMA writable_schema OFF failed: %v\n", err)
	}

	// Integrity check
	_, err = db.Exec("PRAGMA integrity_check")
	if err != nil {
		fmt.Printf("Warning: integrity_check failed: %v\n", err)
	}

	// Vacuum to clean up
	_, err = db.Exec("VACUUM")
	if err != nil {
		fmt.Printf("Warning: VACUUM failed: %v\n", err)
	}

	// Verify
	var count int
	db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name = 'vec_embeddings'").Scan(&count)
	fmt.Printf("vec_embeddings remaining: %d\n", count)

	// Check migration state
	var version int
	var dirty int
	db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
	fmt.Printf("Migration state: version=%d, dirty=%d\n", version, dirty)
}
