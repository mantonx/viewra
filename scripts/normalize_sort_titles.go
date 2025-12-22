package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Parse command line flags
	dbType := flag.String("db", "sqlite", "Database type: sqlite or postgres")
	dbPath := flag.String("path", "data/viewra.db", "SQLite database path (ignored for postgres)")
	dbURL := flag.String("url", "", "PostgreSQL connection URL (e.g., postgres://user:pass@host:5432/dbname)")
	flag.Parse()

	// Determine database type from environment if not specified
	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}
	if *dbURL != "" && strings.HasPrefix(*dbURL, "postgres") {
		*dbType = "postgres"
	}

	var db *sql.DB
	var err error
	var placeholder func(int) string

	switch *dbType {
	case "postgres":
		if *dbURL == "" {
			log.Fatal("PostgreSQL requires -url flag or DATABASE_URL environment variable")
		}
		db, err = sql.Open("postgres", *dbURL)
		if err != nil {
			log.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}
		placeholder = func(n int) string { return fmt.Sprintf("$%d", n) }
		fmt.Println("Connected to PostgreSQL database")
	case "sqlite":
		db, err = sql.Open("sqlite3", *dbPath)
		if err != nil {
			log.Fatalf("Failed to open SQLite database: %v", err)
		}
		placeholder = func(n int) string { return "?" }
		fmt.Printf("Opened SQLite database: %s\n", *dbPath)
	default:
		log.Fatalf("Unknown database type: %s (use 'sqlite' or 'postgres')", *dbType)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Update movies
	moviesUpdated, err := updateMovies(db, placeholder)
	if err != nil {
		log.Fatalf("Error updating movies: %v", err)
	}
	fmt.Printf("Updated %d movie sort_title values\n", moviesUpdated)

	// Update TV shows
	showsUpdated, err := updateTVShows(db, placeholder)
	if err != nil {
		log.Fatalf("Error updating TV shows: %v", err)
	}
	fmt.Printf("Updated %d TV show sort_title values\n", showsUpdated)

	// Update music tracks
	tracksUpdated, err := updateMusicTracks(db, placeholder)
	if err != nil {
		log.Fatalf("Error updating music tracks: %v", err)
	}
	fmt.Printf("Updated %d music track sort_title values\n", tracksUpdated)

	// Update albums (optional - table may not exist in older schemas)
	albumsUpdated, err := updateAlbums(db, placeholder)
	if err != nil {
		fmt.Printf("Skipping albums (table may not exist): %v\n", err)
	} else {
		fmt.Printf("Updated %d album sort_title values\n", albumsUpdated)
	}

	// Update artists (optional - table may not exist in older schemas)
	artistsUpdated, err := updateArtists(db, placeholder)
	if err != nil {
		fmt.Printf("Skipping artists (table may not exist): %v\n", err)
	} else {
		fmt.Printf("Updated %d artist sort_title values\n", artistsUpdated)
	}

	fmt.Println("Sort title normalization complete!")
}

func updateMovies(db *sql.DB, placeholder func(int) string) (int, error) {
	// Get all movies with their titles
	rows, err := db.Query(`
		SELECT m.media_id, med.title
		FROM movies m
		JOIN media med ON m.media_id = med.id
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	query := fmt.Sprintf(`UPDATE movies SET sort_title = %s WHERE media_id = %s`, placeholder(1), placeholder(2))

	count := 0
	for rows.Next() {
		var mediaID int64
		var title string
		if err := rows.Scan(&mediaID, &title); err != nil {
			return count, err
		}

		// Normalize the title
		sortTitle := domainCommon.NormalizeSortTitle(title)

		// Update the movie
		_, err := db.Exec(query, sortTitle, mediaID)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

func updateTVShows(db *sql.DB, placeholder func(int) string) (int, error) {
	// Get all TV shows
	rows, err := db.Query(`SELECT id, title FROM tv_shows`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	query := fmt.Sprintf(`UPDATE tv_shows SET sort_title = %s WHERE id = %s`, placeholder(1), placeholder(2))

	count := 0
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return count, err
		}

		// Normalize the title
		sortTitle := domainCommon.NormalizeSortTitle(title)

		// Update the show
		_, err := db.Exec(query, sortTitle, id)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

func updateMusicTracks(db *sql.DB, placeholder func(int) string) (int, error) {
	// Get all music tracks with their titles
	rows, err := db.Query(`
		SELECT mt.media_id, med.title
		FROM music_tracks mt
		JOIN media med ON mt.media_id = med.id
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	query := fmt.Sprintf(`UPDATE music_tracks SET sort_title = %s WHERE media_id = %s`, placeholder(1), placeholder(2))

	count := 0
	for rows.Next() {
		var mediaID int64
		var title string
		if err := rows.Scan(&mediaID, &title); err != nil {
			return count, err
		}

		// Normalize the title
		sortTitle := domainCommon.NormalizeSortTitle(title)

		// Update the track
		_, err := db.Exec(query, sortTitle, mediaID)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

func updateAlbums(db *sql.DB, placeholder func(int) string) (int, error) {
	// Get all albums
	rows, err := db.Query(`SELECT id, title FROM albums`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	query := fmt.Sprintf(`UPDATE albums SET sort_title = %s WHERE id = %s`, placeholder(1), placeholder(2))

	count := 0
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return count, err
		}

		// Normalize the title
		sortTitle := domainCommon.NormalizeSortTitle(title)

		// Update the album
		_, err := db.Exec(query, sortTitle, id)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

func updateArtists(db *sql.DB, placeholder func(int) string) (int, error) {
	// Get all artists
	rows, err := db.Query(`SELECT id, name FROM artists`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	query := fmt.Sprintf(`UPDATE artists SET sort_name = %s WHERE id = %s`, placeholder(1), placeholder(2))

	count := 0
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return count, err
		}

		// Normalize the name (artists use sort_name, not sort_title)
		sortName := domainCommon.NormalizeSortTitle(name)

		// Update the artist
		_, err := db.Exec(query, sortName, id)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}
