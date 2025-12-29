package common

// Transaction is an abstract interface for database transactions.
// This allows domain layer code to use transactions without depending
// on specific database implementations like database/sql.
//
// Infrastructure layer implementations should wrap *sql.Tx to satisfy
// this interface, allowing the domain to remain persistence-agnostic.
//
// Example usage in domain repository interface:
//
//	type Repository interface {
//	    CreateWithTx(ctx context.Context, tx common.Transaction, entity *Entity) error
//	}
//
// Example implementation in infrastructure:
//
//	type SQLTransaction struct {
//	    tx *sql.Tx
//	}
//
//	func (t *SQLTransaction) Unwrap() any { return t.tx }
type Transaction interface {
	// Unwrap returns the underlying transaction implementation.
	// Infrastructure code can type-assert this to access the concrete type.
	// For SQL databases, this would return *sql.Tx.
	Unwrap() any
}

// SQLTransaction wraps a *sql.Tx to implement the Transaction interface.
// This is provided as a convenience for SQL-based repositories.
type SQLTransaction struct {
	tx any // *sql.Tx - using any to avoid importing database/sql in domain
}

// NewSQLTransaction creates a new SQLTransaction wrapping the given *sql.Tx.
// The tx parameter should be a *sql.Tx but is typed as any to avoid
// importing database/sql in the domain layer.
func NewSQLTransaction(tx any) *SQLTransaction {
	return &SQLTransaction{tx: tx}
}

// Unwrap returns the underlying *sql.Tx.
func (t *SQLTransaction) Unwrap() any {
	return t.tx
}
