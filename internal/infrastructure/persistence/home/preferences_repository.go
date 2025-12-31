package home

import (
	"context"
	"database/sql"
	"time"

	"github.com/mantonx/viewra/internal/domain/home"
)

// PreferencesRepository implements home.PreferencesRepository using SQL storage.
type PreferencesRepository struct {
	db       *sql.DB
	dbDriver string
}

// NewPreferencesRepository creates a new preferences repository.
func NewPreferencesRepository(db *sql.DB, dbDriver string) *PreferencesRepository {
	return &PreferencesRepository{db: db, dbDriver: dbDriver}
}

// Get returns all preferences for a user.
func (r *PreferencesRepository) Get(ctx context.Context, userID string) ([]*home.WidgetPreference, error) {
	query := `
		SELECT widget_id, location, position, hidden, created_at, updated_at
		FROM widget_preferences
		WHERE user_id = ?
		ORDER BY location, position
	`

	// Postgres uses $1 instead of ?
	if r.dbDriver == "postgres" {
		query = `
			SELECT widget_id, location, position, hidden, created_at, updated_at
			FROM widget_preferences
			WHERE user_id = $1
			ORDER BY location, position
		`
	}

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []*home.WidgetPreference
	for rows.Next() {
		var p home.WidgetPreference
		var hidden int
		var createdAt, updatedAt string
		if err := rows.Scan(&p.WidgetID, &p.Location, &p.Position, &hidden, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		p.UserID = userID
		p.Hidden = hidden != 0
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		prefs = append(prefs, &p)
	}

	return prefs, rows.Err()
}

// GetForWidget returns a specific widget preference.
func (r *PreferencesRepository) GetForWidget(ctx context.Context, userID, widgetID string) (*home.WidgetPreference, error) {
	query := `
		SELECT location, position, hidden, created_at, updated_at
		FROM widget_preferences
		WHERE user_id = ? AND widget_id = ?
	`

	if r.dbDriver == "postgres" {
		query = `
			SELECT location, position, hidden, created_at, updated_at
			FROM widget_preferences
			WHERE user_id = $1 AND widget_id = $2
		`
	}

	var p home.WidgetPreference
	var hidden int
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, query, userID, widgetID).Scan(
		&p.Location, &p.Position, &hidden, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.UserID = userID
	p.WidgetID = widgetID
	p.Hidden = hidden != 0
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &p, nil
}

// Save creates or updates a widget preference.
func (r *PreferencesRepository) Save(ctx context.Context, pref *home.WidgetPreference) error {
	query := `
		INSERT INTO widget_preferences (user_id, widget_id, location, position, hidden, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, widget_id) DO UPDATE SET
			location = excluded.location,
			position = excluded.position,
			hidden = excluded.hidden,
			updated_at = excluded.updated_at
	`

	if r.dbDriver == "postgres" {
		query = `
			INSERT INTO widget_preferences (user_id, widget_id, location, position, hidden, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT(user_id, widget_id) DO UPDATE SET
				location = EXCLUDED.location,
				position = EXCLUDED.position,
				hidden = EXCLUDED.hidden,
				updated_at = EXCLUDED.updated_at
		`
	}

	hidden := 0
	if pref.Hidden {
		hidden = 1
	}
	now := time.Now().Format(time.RFC3339)

	_, err := r.db.ExecContext(ctx, query,
		pref.UserID, pref.WidgetID, pref.Location, pref.Position, hidden, now, now,
	)
	return err
}

// SaveAll saves multiple preferences in a transaction.
func (r *PreferencesRepository) SaveAll(ctx context.Context, prefs []*home.WidgetPreference) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	query := `
		INSERT INTO widget_preferences (user_id, widget_id, location, position, hidden, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, widget_id) DO UPDATE SET
			location = excluded.location,
			position = excluded.position,
			hidden = excluded.hidden,
			updated_at = excluded.updated_at
	`

	if r.dbDriver == "postgres" {
		query = `
			INSERT INTO widget_preferences (user_id, widget_id, location, position, hidden, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT(user_id, widget_id) DO UPDATE SET
				location = EXCLUDED.location,
				position = EXCLUDED.position,
				hidden = EXCLUDED.hidden,
				updated_at = EXCLUDED.updated_at
		`
	}

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Format(time.RFC3339)
	for _, pref := range prefs {
		hidden := 0
		if pref.Hidden {
			hidden = 1
		}
		_, err = stmt.ExecContext(ctx,
			pref.UserID, pref.WidgetID, pref.Location, pref.Position, hidden, now, now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Delete removes a specific preference.
func (r *PreferencesRepository) Delete(ctx context.Context, userID, widgetID string) error {
	query := `DELETE FROM widget_preferences WHERE user_id = ? AND widget_id = ?`
	if r.dbDriver == "postgres" {
		query = `DELETE FROM widget_preferences WHERE user_id = $1 AND widget_id = $2`
	}
	_, err := r.db.ExecContext(ctx, query, userID, widgetID)
	return err
}

// DeleteAll removes all preferences for a user (reset to defaults).
func (r *PreferencesRepository) DeleteAll(ctx context.Context, userID string) error {
	query := `DELETE FROM widget_preferences WHERE user_id = ?`
	if r.dbDriver == "postgres" {
		query = `DELETE FROM widget_preferences WHERE user_id = $1`
	}
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}
