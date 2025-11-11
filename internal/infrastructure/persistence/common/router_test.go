package common

import (
	"errors"
	"testing"
)

func TestQueryRouter_Route(t *testing.T) {
	tests := []struct {
		name           string
		dbType         string
		postgresResult interface{}
		sqliteResult   interface{}
		expectedResult interface{}
	}{
		{
			name:           "postgres routes to postgres function",
			dbType:         "postgres",
			postgresResult: "postgres-result",
			sqliteResult:   "sqlite-result",
			expectedResult: "postgres-result",
		},
		{
			name:           "postgresql routes to postgres function",
			dbType:         "postgresql",
			postgresResult: "postgres-result",
			sqliteResult:   "sqlite-result",
			expectedResult: "postgres-result",
		},
		{
			name:           "sqlite routes to sqlite function",
			dbType:         "sqlite",
			postgresResult: "postgres-result",
			sqliteResult:   "sqlite-result",
			expectedResult: "sqlite-result",
		},
		{
			name:           "sqlite3 routes to sqlite function",
			dbType:         "sqlite3",
			postgresResult: "postgres-result",
			sqliteResult:   "sqlite-result",
			expectedResult: "sqlite-result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewQueryRouter(tt.dbType)

			result, err := router.Route(
				func() (interface{}, error) {
					return tt.postgresResult, nil
				},
				func() (interface{}, error) {
					return tt.sqliteResult, nil
				},
			)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result != tt.expectedResult {
				t.Errorf("expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestQueryRouter_Route_WithError(t *testing.T) {
	expectedErr := errors.New("test error")

	t.Run("postgres error is returned", func(t *testing.T) {
		router := NewQueryRouter("postgres")

		_, err := router.Route(
			func() (interface{}, error) {
				return nil, expectedErr
			},
			func() (interface{}, error) {
				return "sqlite-result", nil
			},
		)

		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("sqlite error is returned", func(t *testing.T) {
		router := NewQueryRouter("sqlite")

		_, err := router.Route(
			func() (interface{}, error) {
				return "postgres-result", nil
			},
			func() (interface{}, error) {
				return nil, expectedErr
			},
		)

		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestQueryRouter_RouteVoid(t *testing.T) {
	tests := []struct {
		name   string
		dbType string
		pgErr  error
		sqErr  error
		expErr error
	}{
		{
			name:   "postgres routes to postgres function",
			dbType: "postgres",
			pgErr:  errors.New("pg error"),
			sqErr:  errors.New("sq error"),
			expErr: errors.New("pg error"),
		},
		{
			name:   "sqlite routes to sqlite function",
			dbType: "sqlite",
			pgErr:  errors.New("pg error"),
			sqErr:  errors.New("sq error"),
			expErr: errors.New("sq error"),
		},
		{
			name:   "postgres no error",
			dbType: "postgres",
			pgErr:  nil,
			sqErr:  errors.New("sq error"),
			expErr: nil,
		},
		{
			name:   "sqlite no error",
			dbType: "sqlite",
			pgErr:  errors.New("pg error"),
			sqErr:  nil,
			expErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewQueryRouter(tt.dbType)

			err := router.RouteVoid(
				func() error {
					return tt.pgErr
				},
				func() error {
					return tt.sqErr
				},
			)

			if (err == nil && tt.expErr != nil) || (err != nil && tt.expErr == nil) {
				t.Errorf("expected error %v, got %v", tt.expErr, err)
			}

			if err != nil && tt.expErr != nil && err.Error() != tt.expErr.Error() {
				t.Errorf("expected error %v, got %v", tt.expErr, err)
			}
		})
	}
}

func TestQueryRouter_IsPostgresDB(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		expected bool
	}{
		{"postgres", "postgres", true},
		{"postgresql", "postgresql", true},
		{"sqlite", "sqlite", false},
		{"sqlite3", "sqlite3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewQueryRouter(tt.dbType)
			result := router.IsPostgresDB()

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
