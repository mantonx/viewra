// Package location provides persistence for user location preferences.
package location

import (
	"context"
	"database/sql"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// UserLocationPreference represents a user's location settings.
type UserLocationPreference struct {
	UserID       int64
	Latitude     float64
	Longitude    float64
	Timezone     string
	Enabled      bool
	LocationName string
}

// Repository provides access to user location preferences.
type Repository struct {
	querier *unified.Querier
}

// NewRepository creates a new location repository.
func NewRepository(db *sql.DB, dbType string) *Repository {
	return &Repository{
		querier: unified.NewQuerier(db, dbType),
	}
}

// Get retrieves a user's location preferences from the users table.
func (r *Repository) Get(ctx context.Context, userID int64) (*UserLocationPreference, error) {
	row, err := r.querier.GetUserLocation(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &UserLocationPreference{
		UserID:       row.ID,
		Latitude:     nullFloat64Value(row.LocationLatitude),
		Longitude:    nullFloat64Value(row.LocationLongitude),
		Timezone:     nullStringValue(row.LocationTimezone, "auto"),
		Enabled:      common.NullInt64ToBool(row.LocationEnabled),
		LocationName: nullStringValue(row.LocationName, ""),
	}, nil
}

// Upsert updates a user's location preferences in the users table.
func (r *Repository) Upsert(ctx context.Context, prefs *UserLocationPreference) error {
	return r.querier.UpdateUserLocation(ctx, unified.UpdateUserLocationParams{
		LocationLatitude:  sql.NullFloat64{Float64: prefs.Latitude, Valid: prefs.Latitude != 0},
		LocationLongitude: sql.NullFloat64{Float64: prefs.Longitude, Valid: prefs.Longitude != 0},
		LocationTimezone:  sql.NullString{String: prefs.Timezone, Valid: prefs.Timezone != ""},
		LocationEnabled:   common.NullInt64FromBool(prefs.Enabled),
		LocationName:      sql.NullString{String: prefs.LocationName, Valid: prefs.LocationName != ""},
		UpdatedAt:         time.Now(),
		ID:                prefs.UserID,
	})
}

func nullFloat64Value(n sql.NullFloat64) float64 {
	if n.Valid {
		return n.Float64
	}
	return 0
}

func nullStringValue(n sql.NullString, defaultVal string) string {
	if n.Valid && n.String != "" {
		return n.String
	}
	return defaultVal
}
