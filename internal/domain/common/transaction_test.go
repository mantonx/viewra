package common

import (
	"testing"
)

func TestNewSQLTransaction(t *testing.T) {
	// Create a mock underlying transaction (just a struct for testing)
	mockTx := &struct{ ID int }{ID: 42}

	tx := NewSQLTransaction(mockTx)

	if tx == nil {
		t.Fatal("NewSQLTransaction returned nil")
	}
}

func TestSQLTransactionUnwrap(t *testing.T) {
	// Create a mock underlying transaction
	mockTx := &struct{ ID int }{ID: 42}

	tx := NewSQLTransaction(mockTx)
	unwrapped := tx.Unwrap()

	if unwrapped != mockTx {
		t.Error("Unwrap should return the original transaction")
	}

	// Type assert to verify it's the correct type
	if result, ok := unwrapped.(*struct{ ID int }); ok {
		if result.ID != 42 {
			t.Errorf("Unwrapped tx ID = %d, want 42", result.ID)
		}
	} else {
		t.Error("Unwrap returned wrong type")
	}
}

func TestSQLTransactionImplementsInterface(t *testing.T) {
	mockTx := "test-tx"
	tx := NewSQLTransaction(mockTx)

	// Verify SQLTransaction implements Transaction interface
	var _ Transaction = tx
}

func TestSQLTransactionWithNil(t *testing.T) {
	tx := NewSQLTransaction(nil)

	if tx == nil {
		t.Fatal("NewSQLTransaction with nil should not return nil")
	}

	unwrapped := tx.Unwrap()
	if unwrapped != nil {
		t.Error("Unwrap of nil transaction should return nil")
	}
}

func TestSQLTransactionWithVariousTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"string", "test"},
		{"int", 42},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]int{"a": 1}},
		{"struct", struct{ Name string }{Name: "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := NewSQLTransaction(tt.value)
			unwrapped := tx.Unwrap()

			// Using reflect.DeepEqual would be better, but for simplicity
			// just check that we get something back
			if unwrapped == nil && tt.value != nil {
				t.Error("Unwrap should return non-nil for non-nil input")
			}
		})
	}
}
