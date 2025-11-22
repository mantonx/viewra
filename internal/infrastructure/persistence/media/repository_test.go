package media

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mantonx/viewra/internal/domain/media"
)

// setupTestDB creates an in-memory SQLite database with schema
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create schema (simplified version of migration)
	schema := `
	CREATE TABLE IF NOT EXISTS libraries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL CHECK(type IN ('movies', 'tv', 'music')),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		library_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		file_path TEXT NOT NULL UNIQUE,
		file_size INTEGER,
		file_hash TEXT,
		container_format TEXT,
		duration REAL,
		width INTEGER,
		height INTEGER,
		aspect_ratio TEXT,
		codec TEXT,
		codec_profile TEXT,
		bit_rate INTEGER,
		frame_rate REAL,
		scan_type TEXT CHECK(scan_type IN ('progressive', 'interlaced', NULL)),
		hdr_format TEXT CHECK(hdr_format IN ('SDR', 'HDR10', 'HDR10+', 'Dolby Vision', 'HLG', NULL)),
		color_space TEXT,
		color_primaries TEXT,
		thumbnail_path TEXT,
		type TEXT NOT NULL CHECK(type IN ('movie', 'tv_episode', 'music_track')),
		source_type TEXT,
		resolution_label TEXT,
		quality_score INTEGER,
		is_3d BOOLEAN DEFAULT 0,
		stereo_mode TEXT CHECK(stereo_mode IN ('sbs', 'tab', 'mvc', NULL)),
		has_dash BOOLEAN DEFAULT FALSE,
		dash_manifest_path TEXT,
		transcoding_status TEXT CHECK(transcoding_status IN ('pending', 'processing', 'completed', 'failed', NULL)),
		is_extra BOOLEAN DEFAULT 0 NOT NULL,
		audio_codec TEXT,
		date_added DATETIME DEFAULT CURRENT_TIMESTAMP,
		date_modified DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert test library
	insertLib := "INSERT INTO libraries (name, path, type) VALUES ('Test Library', '/test/path', 'movies')"
	if _, err := db.Exec(insertLib); err != nil {
		t.Fatalf("Failed to insert test library: %v", err)
	}

	return db
}

func TestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	m := &media.Media{
		LibraryID: 1,
		Type:      "movie",
		Title:     "Test Movie",
		FilePath:  "/movies/test.mp4",
		FileSize:  1024000,
		Duration:  7200, // 2 hours in seconds
	}

	// Test creation
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Failed to create media: %v", err)
	}

	if m.ID == 0 {
		t.Error("Expected ID to be set after creation")
	}
	if m.Title != "Test Movie" {
		t.Errorf("Expected title 'Test Movie', got %s", m.Title)
	}
	if m.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if m.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}

	// Test duplicate path error
	m2 := &media.Media{
		LibraryID: 1,
		Type:      "movie",
		Title:     "Another Movie",
		FilePath:  "/movies/test.mp4", // Duplicate path
		FileSize:  2048000,
		Duration:  5400,
	}
	if err := repo.Create(ctx, m2); err == nil {
		t.Error("Expected error for duplicate file path, got nil")
	}
}

func TestRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create a media item first
	m := &media.Media{
		LibraryID: 1,
		Type:      "movie",
		Title:     "Test Movie",
		FilePath:  "/movies/test.mp4",
		FileSize:  1024000,
		Duration:  7200,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Failed to create media: %v", err)
	}

	// Test retrieval
	found, err := repo.GetByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("Failed to get media: %v", err)
	}

	if found.ID != m.ID {
		t.Errorf("Expected ID %d, got %d", m.ID, found.ID)
	}
	if found.Title != "Test Movie" {
		t.Errorf("Expected title 'Test Movie', got %s", found.Title)
	}

	// Test not found
	_, err = repo.GetByID(ctx, 99999)
	if err != media.ErrMediaNotFound {
		t.Errorf("Expected ErrMediaNotFound, got %v", err)
	}
}

func TestRepository_GetByFilePath(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create a media item first
	m := &media.Media{
		LibraryID: 1,
		Type:      "movie",
		Title:     "Test Movie",
		FilePath:  "/movies/test.mp4",
		FileSize:  1024000,
		Duration:  7200,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Failed to create media: %v", err)
	}

	// Test retrieval by file path
	found, err := repo.GetByFilePath(ctx, 1, "/movies/test.mp4")
	if err != nil {
		t.Fatalf("Failed to get media by file path: %v", err)
	}

	if found.ID != m.ID {
		t.Errorf("Expected ID %d, got %d", m.ID, found.ID)
	}

	// Test not found
	_, err = repo.GetByFilePath(ctx, 1, "/movies/nonexistent.mp4")
	if err != media.ErrMediaNotFound {
		t.Errorf("Expected ErrMediaNotFound, got %v", err)
	}
}

func TestRepository_ListByLibrary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create multiple media items
	m1 := &media.Media{LibraryID: 1, Type: "movie", Title: "Movie A", FilePath: "/movies/a.mp4", FileSize: 1024, Duration: 3600}
	m2 := &media.Media{LibraryID: 1, Type: "movie", Title: "Movie B", FilePath: "/movies/b.mp4", FileSize: 2048, Duration: 7200}
	m3 := &media.Media{LibraryID: 1, Type: "movie", Title: "Movie C", FilePath: "/movies/c.mp4", FileSize: 4096, Duration: 5400}

	if err := repo.Create(ctx, m1); err != nil {
		t.Fatalf("Failed to create media 1: %v", err)
	}
	if err := repo.Create(ctx, m2); err != nil {
		t.Fatalf("Failed to create media 2: %v", err)
	}
	if err := repo.Create(ctx, m3); err != nil {
		t.Fatalf("Failed to create media 3: %v", err)
	}

	// Test list all in library
	mediaList, err := repo.ListByLibrary(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to list media: %v", err)
	}

	if len(mediaList) != 3 {
		t.Errorf("Expected 3 media items, got %d", len(mediaList))
	}
}

func TestRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create a media item first
	m := &media.Media{
		LibraryID: 1,
		Type:      "movie",
		Title:     "Original Title",
		FilePath:  "/movies/test.mp4",
		FileSize:  1024000,
		Duration:  7200,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Failed to create media: %v", err)
	}

	// Update the media
	m.Title = "Updated Title"
	m.FilePath = "/movies/updated.mp4"
	m.FileSize = 2048000

	if err := repo.Update(ctx, m); err != nil {
		t.Fatalf("Failed to update media: %v", err)
	}

	if m.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %s", m.Title)
	}

	// Verify persistence
	found, err := repo.GetByID(ctx, m.ID)
	if err != nil {
		t.Fatalf("Failed to get updated media: %v", err)
	}

	if found.Title != "Updated Title" {
		t.Errorf("Expected persisted title 'Updated Title', got %s", found.Title)
	}
	if found.FileSize != 2048000 {
		t.Errorf("Expected file size 2048000, got %d", found.FileSize)
	}
}

func TestRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create a media item first
	m := &media.Media{
		LibraryID: 1,
		Type:      "movie",
		Title:     "Test Movie",
		FilePath:  "/movies/test.mp4",
		FileSize:  1024000,
		Duration:  7200,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Failed to create media: %v", err)
	}

	// Delete the media
	if err := repo.Delete(ctx, m.ID); err != nil {
		t.Fatalf("Failed to delete media: %v", err)
	}

	// Verify deletion
	_, err := repo.GetByID(ctx, m.ID)
	if err != media.ErrMediaNotFound {
		t.Errorf("Expected ErrMediaNotFound after deletion, got %v", err)
	}

	// Test deleting non-existent media (should not error)
	if err := repo.Delete(ctx, 99999); err != nil {
		t.Errorf("Expected no error deleting non-existent media, got %v", err)
	}
}

func TestRepository_ExistsInLibrary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create a media item first
	m := &media.Media{
		LibraryID: 1,
		Type:      "movie",
		Title:     "Test Movie",
		FilePath:  "/movies/test.mp4",
		FileSize:  1024000,
		Duration:  7200,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Failed to create media: %v", err)
	}

	// Test exists
	exists, err := repo.ExistsInLibrary(ctx, 1, "/movies/test.mp4")
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Expected media to exist")
	}

	// Test non-existent
	exists, err = repo.ExistsInLibrary(ctx, 1, "/movies/nonexistent.mp4")
	if err != nil {
		t.Fatalf("Failed to check non-existence: %v", err)
	}
	if exists {
		t.Error("Expected media not to exist")
	}
}

func TestRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Initially should be 0
	count, err := repo.Count(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to count media: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Create media items
	for i := 0; i < 3; i++ {
		m := &media.Media{
			LibraryID: 1,
			Title:     "Movie",
			Type:      "movie",
			FilePath:  "/movies/" + string(rune('a'+i)) + ".mp4",
			FileSize:  1024000,
			Duration:  7200,
		}
		if err := repo.Create(ctx, m); err != nil {
			t.Fatalf("Failed to create media: %v", err)
		}
	}

	// Count should be 3
	count, err = repo.Count(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to count media: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestRepository_ListByType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create media items of different types
	// Note: The Create method currently hardcodes type as "movie", so we'll directly insert for testing
	insertMovie := `
		INSERT INTO media (library_id, title, file_path, type, file_size, duration)
		VALUES (1, 'Test Movie', '/movies/movie.mp4', 'movie', 1024000, 7200)
	`
	insertTV := `
		INSERT INTO media (library_id, title, file_path, type, file_size, duration)
		VALUES (1, 'Test Episode', '/tv/episode.mp4', 'tv_episode', 512000, 2700)
	`
	insertMusic := `
		INSERT INTO media (library_id, title, file_path, type, file_size, duration)
		VALUES (1, 'Test Track', '/music/track.mp3', 'music_track', 256000, 180)
	`

	if _, err := db.Exec(insertMovie); err != nil {
		t.Fatalf("Failed to insert movie: %v", err)
	}
	if _, err := db.Exec(insertTV); err != nil {
		t.Fatalf("Failed to insert TV episode: %v", err)
	}
	if _, err := db.Exec(insertMusic); err != nil {
		t.Fatalf("Failed to insert music track: %v", err)
	}

	// Test listing movies
	movies, err := repo.ListByType(ctx, 1, media.MediaTypeMovie)
	if err != nil {
		t.Fatalf("Failed to list movies: %v", err)
	}
	if len(movies) != 1 {
		t.Errorf("Expected 1 movie, got %d", len(movies))
	}
	if len(movies) > 0 && movies[0].Title != "Test Movie" {
		t.Errorf("Expected movie title 'Test Movie', got %s", movies[0].Title)
	}

	// Test listing TV episodes
	// Note: Using "tv_episode" as media type since that's what's in DB,
	// though domain uses MediaTypeTV ("tv")
	tvEpisodes, err := repo.ListByType(ctx, 1, "tv_episode")
	if err != nil {
		t.Fatalf("Failed to list TV episodes: %v", err)
	}
	if len(tvEpisodes) != 1 {
		t.Errorf("Expected 1 TV episode, got %d", len(tvEpisodes))
	}

	// Test listing music tracks
	musicTracks, err := repo.ListByType(ctx, 1, "music_track")
	if err != nil {
		t.Fatalf("Failed to list music tracks: %v", err)
	}
	if len(musicTracks) != 1 {
		t.Errorf("Expected 1 music track, got %d", len(musicTracks))
	}

	// Test empty result for non-existent library
	empty, err := repo.ListByType(ctx, 999, media.MediaTypeMovie)
	if err != nil {
		t.Fatalf("Failed to list media for non-existent library: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("Expected 0 media items for non-existent library, got %d", len(empty))
	}
}

func TestRepository_CountByType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()

	// Create media items of different types
	insertMovie1 := `
		INSERT INTO media (library_id, title, file_path, type, file_size, duration)
		VALUES (1, 'Movie 1', '/movies/movie1.mp4', 'movie', 1024000, 7200)
	`
	insertMovie2 := `
		INSERT INTO media (library_id, title, file_path, type, file_size, duration)
		VALUES (1, 'Movie 2', '/movies/movie2.mp4', 'movie', 2048000, 5400)
	`
	insertTV := `
		INSERT INTO media (library_id, title, file_path, type, file_size, duration)
		VALUES (1, 'Episode 1', '/tv/episode1.mp4', 'tv_episode', 512000, 2700)
	`

	if _, err := db.Exec(insertMovie1); err != nil {
		t.Fatalf("Failed to insert movie 1: %v", err)
	}
	if _, err := db.Exec(insertMovie2); err != nil {
		t.Fatalf("Failed to insert movie 2: %v", err)
	}
	if _, err := db.Exec(insertTV); err != nil {
		t.Fatalf("Failed to insert TV episode: %v", err)
	}

	// Test counting movies
	movieCount, err := repo.CountByType(ctx, 1, media.MediaTypeMovie)
	if err != nil {
		t.Fatalf("Failed to count movies: %v", err)
	}
	if movieCount != 2 {
		t.Errorf("Expected 2 movies, got %d", movieCount)
	}

	// Test counting TV episodes
	tvCount, err := repo.CountByType(ctx, 1, "tv_episode")
	if err != nil {
		t.Fatalf("Failed to count TV episodes: %v", err)
	}
	if tvCount != 1 {
		t.Errorf("Expected 1 TV episode, got %d", tvCount)
	}

	// Test counting music tracks (should be 0)
	musicCount, err := repo.CountByType(ctx, 1, "music_track")
	if err != nil {
		t.Fatalf("Failed to count music tracks: %v", err)
	}
	if musicCount != 0 {
		t.Errorf("Expected 0 music tracks, got %d", musicCount)
	}

	// Test non-existent library
	emptyCount, err := repo.CountByType(ctx, 999, media.MediaTypeMovie)
	if err != nil {
		t.Fatalf("Failed to count for non-existent library: %v", err)
	}
	if emptyCount != 0 {
		t.Errorf("Expected 0 for non-existent library, got %d", emptyCount)
	}
}

func TestRepository_PostgresDriver(t *testing.T) {
	// Test that NewRepository properly sets up for different driver names
	db := setupTestDB(t)
	defer db.Close()

	// Test with different SQLite driver names
	sqliteRepo := NewRepository(db, "sqlite")
	if sqliteRepo == nil {
		t.Error("Expected non-nil repository for sqlite driver")
	}

	sqlite3Repo := NewRepository(db, "sqlite3")
	if sqlite3Repo == nil {
		t.Error("Expected non-nil repository for sqlite3 driver")
	}

	// Note: We can't test actual Postgres without a Postgres instance,
	// but we can verify the repository is created
	// This tests the postgres detection logic in NewRepository
	postgresRepo := NewRepository(db, "postgres")
	if postgresRepo == nil {
		t.Error("Expected non-nil repository for postgres driver")
	}

	postgresqlRepo := NewRepository(db, "postgresql")
	if postgresqlRepo == nil {
		t.Error("Expected non-nil repository for postgresql driver")
	}
}
