package common

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite" // SQLite driver
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create a simple test table
	_, err = db.Exec(`
		CREATE TABLE test_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	return db
}

func TestTxManager_WithTransaction_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tm := NewTxManager(db)

	err := tm.WithTransaction(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test_data (value) VALUES (?)", "test1")
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO test_data (value) VALUES (?)", "test2")
		return err
	})

	if err != nil {
		t.Errorf("WithTransaction() unexpected error = %v", err)
	}

	// Verify data was committed
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_data").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows, got %d", count)
	}
}

func TestTxManager_WithTransaction_Rollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tm := NewTxManager(db)

	testErr := errors.New("test error")

	err := tm.WithTransaction(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test_data (value) VALUES (?)", "test1")
		if err != nil {
			return err
		}
		// Return error to trigger rollback
		return testErr
	})

	if err != testErr {
		t.Errorf("WithTransaction() error = %v, want %v", err, testErr)
	}

	// Verify data was rolled back
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_data").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 rows after rollback, got %d", count)
	}
}

func TestTxManager_WithTransaction_Panic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tm := NewTxManager(db)

	defer func() {
		if r := recover(); r == nil {
			t.Error("WithTransaction() should have panicked")
		}

		// Verify data was rolled back after panic
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM test_data").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query count: %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 rows after panic rollback, got %d", count)
		}
	}()

	_ = tm.WithTransaction(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test_data (value) VALUES (?)", "test1")
		if err != nil {
			return err
		}
		panic("test panic")
	})
}

func TestTxManager_WithReadOnlyTransaction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert some test data
	_, err := db.Exec("INSERT INTO test_data (value) VALUES (?)", "test1")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	tm := NewTxManager(db)

	var value string
	err = tm.WithReadOnlyTransaction(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow("SELECT value FROM test_data WHERE id = 1").Scan(&value)
	})

	if err != nil {
		t.Errorf("WithReadOnlyTransaction() unexpected error = %v", err)
	}

	if value != "test1" {
		t.Errorf("WithReadOnlyTransaction() value = %v, want test1", value)
	}
}

func TestTxManager_NewTxManager(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tm := NewTxManager(db)

	if tm == nil {
		t.Error("NewTxManager() returned nil")
	}

	if tm.db != db {
		t.Error("NewTxManager() db field not set correctly")
	}
}
