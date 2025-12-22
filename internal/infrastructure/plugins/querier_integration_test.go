//go:build integration

package plugins

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	sqlc_sqlite "github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/database/router"
)

func TestMediaQuerierLanguageField(t *testing.T) {
	db, err := sql.Open("sqlite3", "../../../data/viewra.db")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	dbRouter := router.New(nil, sqlc_sqlite.New(db), "sqlite")
	querier := NewDBMediaQuerier(db, "sqlite", dbRouter, logger)

	ctx := context.Background()

	// Get Drive My Car (ID: 376837)
	details, err := querier.GetMediaDetails(ctx, 376837, "movie")
	if err != nil {
		t.Fatalf("Failed to get media details: %v", err)
	}

	t.Logf("Movie: %s", details.Title)
	t.Logf("OriginalLanguage: '%s'", details.OriginalLanguage)
	t.Logf("CountryOfOrigin: '%s'", details.CountryOfOrigin)
	t.Logf("Cast count: %d", len(details.Cast))
	if len(details.Cast) > 0 {
		t.Logf("First cast member: %s", details.Cast[0].Name)
	}
	t.Logf("Directors: %v", details.Directors)

	if details.OriginalLanguage == "" {
		t.Error("Expected OriginalLanguage to be set")
	}
	if details.OriginalLanguage != "ja" {
		t.Errorf("Expected OriginalLanguage 'ja', got '%s'", details.OriginalLanguage)
	}
}
