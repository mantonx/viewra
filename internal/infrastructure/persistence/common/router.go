package common

// QueryRouter provides a generic mechanism to route database queries
// to the appropriate database implementation (PostgreSQL or SQLite).
// This eliminates repetitive if/else branching in repository methods.
type QueryRouter struct {
	dbType string
}

// NewQueryRouter creates a new query router with the specified database type.
func NewQueryRouter(dbType string) *QueryRouter {
	return &QueryRouter{dbType: dbType}
}

// Route executes the appropriate database-specific function based on the database type.
// It takes two functions as parameters: one for PostgreSQL and one for SQLite.
// The function returns the result from the executed function.
//
// Example usage:
//
//	result, err := router.Route(
//		func() (any, error) {
//			return r.postgres.GetLibraryByID(ctx, int32(id))
//		},
//		func() (any, error) {
//			return r.sqlite.GetLibraryByID(ctx, id)
//		},
//	)
func (qr *QueryRouter) Route(
	postgresFunc func() (any, error),
	sqliteFunc func() (any, error),
) (any, error) {
	if IsPostgres(qr.dbType) {
		return postgresFunc()
	}
	return sqliteFunc()
}

// RouteVoid executes the appropriate database-specific function that returns only an error.
// This is useful for operations like Create, Update, Delete that don't return data.
//
// Example usage:
//
//	err := router.RouteVoid(
//		func() error {
//			return r.postgres.DeleteLibrary(ctx, int32(id))
//		},
//		func() error {
//			return r.sqlite.DeleteLibrary(ctx, id)
//		},
//	)
func (qr *QueryRouter) RouteVoid(
	postgresFunc func() error,
	sqliteFunc func() error,
) error {
	if IsPostgres(qr.dbType) {
		return postgresFunc()
	}
	return sqliteFunc()
}

// IsPostgresDB returns true if the router is configured for PostgreSQL.
func (qr *QueryRouter) IsPostgresDB() bool {
	return IsPostgres(qr.dbType)
}
