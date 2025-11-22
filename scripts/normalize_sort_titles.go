package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
	domainCommon "github.com/mantonx/viewra/internal/domain/common"
)

func main() {
	// Open database
	db, err := sql.Open("sqlite3", "data/viewra.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Update movies
	moviesUpdated, err := updateMovies(db)
	if err != nil {
		log.Fatalf("Error updating movies: %v", err)
	}
	fmt.Printf("Updated %d movie sort_title values\n", moviesUpdated)

	// Update TV shows
	showsUpdated, err := updateTVShows(db)
	if err != nil {
		log.Fatalf("Error updating TV shows: %v", err)
	}
	fmt.Printf("Updated %d TV show sort_title values\n", showsUpdated)

	// Update music tracks
	tracksUpdated, err := updateMusicTracks(db)
	if err != nil {
		log.Fatalf("Error updating music tracks: %v", err)
	}
	fmt.Printf("Updated %d music track sort_title values\n", tracksUpdated)

	fmt.Println("Sort title normalization complete!")
}

func updateMovies(db *sql.DB) (int, error) {
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
		_, err := db.Exec(`UPDATE movies SET sort_title = ? WHERE media_id = ?`, sortTitle, mediaID)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

func updateTVShows(db *sql.DB) (int, error) {
	// Get all TV shows
	rows, err := db.Query(`SELECT id, title FROM tv_shows`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

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
		_, err := db.Exec(`UPDATE tv_shows SET sort_title = ? WHERE id = ?`, sortTitle, id)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}

func updateMusicTracks(db *sql.DB) (int, error) {
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
		_, err := db.Exec(`UPDATE music_tracks SET sort_title = ? WHERE media_id = ?`, sortTitle, mediaID)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, rows.Err()
}
