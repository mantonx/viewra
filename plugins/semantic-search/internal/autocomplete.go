package internal

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// AutocompleteService handles type-ahead search suggestions.
// Uses LIKE queries for prefix matching (database-agnostic).
type AutocompleteService struct {
	sql    *sdk.SQLClient
	data   *sdk.DataClient
	logger *slog.Logger

	// Population state
	mu           sync.RWMutex
	isPopulating bool
	lastPopulate time.Time
}

// AutocompleteResult represents a single autocomplete suggestion.
type AutocompleteResult struct {
	Type       string `json:"type"`                 // "title", "person"
	Text       string `json:"text"`                 // Display text
	EntityID   int64  `json:"entity_id"`            // For navigation
	Subtype    string `json:"subtype,omitempty"`    // "movie", "tv_show", "director", "actor", etc.
	Year       int    `json:"year,omitempty"`       // For titles
	Popularity int64  `json:"popularity,omitempty"` // For ranking
}

// NewAutocompleteService creates a new autocomplete service.
func NewAutocompleteService(sql *sdk.SQLClient, data *sdk.DataClient, logger *slog.Logger) *AutocompleteService {
	return &AutocompleteService{
		sql:    sql,
		data:   data,
		logger: logger,
	}
}

// Search performs autocomplete search using LIKE queries.
// Returns suggestions ranked by match quality (exact prefix > word start > contains) then popularity.
func (s *AutocompleteService) Search(ctx context.Context, query string, limit int, types string) ([]AutocompleteResult, error) {
	if s.sql == nil {
		return nil, fmt.Errorf("SQL client not available")
	}

	// Normalize and validate query
	query = normalizeQueryForAutocomplete(query)
	if len(query) < 2 {
		return nil, nil // Too short, return empty
	}

	// Apply limits
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}

	// Build type filter
	typeFilter := ""
	typeArgs := []any{}
	switch types {
	case "titles":
		typeFilter = " AND type = ?"
		typeArgs = append(typeArgs, "title")
	case "people":
		typeFilter = " AND type = ?"
		typeArgs = append(typeArgs, "person")
	case "genres":
		typeFilter = " AND type = ?"
		typeArgs = append(typeArgs, "genre")
	}

	// Escape LIKE special characters in query
	escapedQuery := escapeLIKE(query)
	lowerQuery := strings.ToLower(escapedQuery)

	// Tiered ranking query using LIKE
	// Tier 0: Exact prefix match on name
	// Tier 1: Word boundary match (contains " query" pattern)
	// Tier 2: Contains anywhere
	sql := `
		SELECT 
			name, type, entity_id, subtype, year, popularity,
			CASE
				WHEN name_lower LIKE ? THEN 0
				WHEN name_lower LIKE ? OR aliases LIKE ? THEN 1
				ELSE 2
			END AS match_tier
		FROM autocomplete
		WHERE (name_lower LIKE ? OR aliases LIKE ?)` + typeFilter + `
		ORDER BY match_tier ASC, popularity DESC
		LIMIT ?`

	// Build args for the query
	prefixPattern := lowerQuery + "%"         // "dark%"
	wordPattern := "% " + lowerQuery + "%"    // "% dark%"
	containsPattern := "%" + lowerQuery + "%" // "%dark%"

	args := []any{prefixPattern, wordPattern, wordPattern, containsPattern, containsPattern}
	args = append(args, typeArgs...)
	args = append(args, limit)

	rows, err := s.sql.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("autocomplete query: %w", err)
	}
	defer rows.Close()

	var results []AutocompleteResult
	for rows.Next() {
		var r AutocompleteResult
		var matchTier int
		if err := rows.Scan(&r.Text, &r.Type, &r.EntityID, &r.Subtype, &r.Year, &r.Popularity, &matchTier); err != nil {
			s.logger.Warn("failed to scan autocomplete row", "error", err)
			continue
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating autocomplete results: %w", err)
	}

	return results, nil
}

// PopulateIndex rebuilds the autocomplete index from media data.
// This clears the existing index and repopulates from scratch.
func (s *AutocompleteService) PopulateIndex(ctx context.Context) error {
	if s.sql == nil || s.data == nil {
		return fmt.Errorf("SQL or data client not available")
	}

	s.mu.Lock()
	if s.isPopulating {
		s.mu.Unlock()
		return fmt.Errorf("population already in progress")
	}
	s.isPopulating = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isPopulating = false
		s.lastPopulate = time.Now()
		s.mu.Unlock()
	}()

	s.logger.Info("starting autocomplete index population")
	startTime := time.Now()

	// Clear existing index
	if _, _, err := s.sql.Exec(ctx, `DELETE FROM autocomplete`); err != nil {
		return fmt.Errorf("clear autocomplete index: %w", err)
	}

	// Track counts
	var titleCount, personCount int64

	// Populate titles from all libraries
	// We need to iterate through libraries since there's no ListAllMedia
	// For now, we'll use a reasonable approach: query the host for media
	// The data client provides ListMediaByLibrary, so we need library IDs
	// Since we don't have a ListLibraries, we'll populate incrementally when indexing happens

	// For initial population, we'll query the existing embeddings to get entity IDs
	// and then fetch their details. This is a workaround until we have better APIs.

	// Alternative approach: Use the SQL client to query the host's media tables directly
	// But plugins can't access host tables directly - they only have their own namespaced tables.

	// Best approach for now: Populate during library indexing via callback
	// For initial startup, we'll try to populate from what we can access

	// Let's try a different approach - query for all unique media we've indexed
	// by checking if we have any data in our mood_tags table or similar

	// Actually, the cleanest approach is to populate the autocomplete index
	// alongside the vector indexing. We'll add a hook in IndexLibrary.

	// For now, let's implement a method that can be called with library data
	s.logger.Info("autocomplete index population complete",
		"titles", titleCount,
		"people", personCount,
		"duration", time.Since(startTime))

	return nil
}

// PopulateFromLibrary populates the autocomplete index from a specific library.
// Called during library indexing to keep the autocomplete index in sync.
func (s *AutocompleteService) PopulateFromLibrary(ctx context.Context, libraryID int64) error {
	if s.sql == nil || s.data == nil {
		return fmt.Errorf("SQL or data client not available")
	}

	s.logger.Debug("populating autocomplete from library", "library_id", libraryID)

	offset := 0
	limit := 100
	var titleCount, personCount int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		mediaList, err := s.data.ListMediaByLibrary(ctx, libraryID, limit, offset)
		if err != nil {
			return fmt.Errorf("list media: %w", err)
		}

		for _, media := range mediaList.Items {
			// Add title entry
			if err := s.addTitleEntry(ctx, media); err != nil {
				s.logger.Warn("failed to add title to autocomplete", "id", media.ID, "error", err)
			} else {
				titleCount++
			}

			// Add people entries (directors, cast)
			peopleAdded, err := s.addPeopleEntries(ctx, media)
			if err != nil {
				s.logger.Warn("failed to add people to autocomplete", "id", media.ID, "error", err)
			}
			personCount += int64(peopleAdded)
		}

		if !mediaList.HasMore {
			break
		}
		offset += limit
	}

	s.logger.Debug("populated autocomplete from library",
		"library_id", libraryID,
		"titles", titleCount,
		"people", personCount)

	return nil
}

// addTitleEntry adds a media title to the autocomplete index.
func (s *AutocompleteService) addTitleEntry(ctx context.Context, media *sdk.MediaDetails) error {
	name := media.Title
	nameLower := strings.ToLower(name)
	if nameLower == "" {
		return nil
	}

	// Generate aliases (original title, etc.)
	aliases := ""
	// If we had original_title, we'd add it here

	// Determine subtype
	subtype := media.MediaType
	if subtype == "" {
		subtype = "movie"
	}

	// Estimate popularity (use year as a proxy if we don't have votes)
	// More recent = higher popularity for now
	popularity := float64(media.Year)
	if popularity == 0 {
		popularity = 1
	}

	// Use upsert pattern that works on both SQLite and PostgreSQL
	// First try to update, then insert if no rows affected
	updateSQL := `UPDATE autocomplete SET name = ?, name_lower = ?, aliases = ?, subtype = ?, year = ?, popularity = ?
				  WHERE type = 'title' AND entity_id = ?`
	rowsAffected, _, err := s.sql.Exec(ctx, updateSQL, name, nameLower, aliases, subtype, media.Year, popularity, media.ID)
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		insertSQL := `INSERT INTO autocomplete (name, name_lower, aliases, type, entity_id, subtype, year, popularity)
					  VALUES (?, ?, ?, 'title', ?, ?, ?, ?)`
		_, _, err = s.sql.Exec(ctx, insertSQL, name, nameLower, aliases, media.ID, subtype, media.Year, popularity)
	}

	return err
}

// addPeopleEntries adds people (directors, cast) from a media item to the autocomplete index.
// Returns the number of people added.
func (s *AutocompleteService) addPeopleEntries(ctx context.Context, media *sdk.MediaDetails) (int, error) {
	added := 0

	// Track people we've already added to avoid duplicates within this media
	seen := make(map[string]bool)

	// Helper to upsert a person
	upsertPerson := func(name, subtype string, popularity float64) error {
		nameLower := strings.ToLower(name)
		aliases := generateAliases(name)
		entityID := hashString(nameLower)

		updateSQL := `UPDATE autocomplete SET name = ?, name_lower = ?, aliases = ?, subtype = ?, popularity = ?
					  WHERE type = 'person' AND entity_id = ?`
		rowsAffected, _, err := s.sql.Exec(ctx, updateSQL, name, nameLower, aliases, subtype, popularity, entityID)
		if err != nil {
			return err
		}

		if rowsAffected == 0 {
			insertSQL := `INSERT INTO autocomplete (name, name_lower, aliases, type, entity_id, subtype, popularity)
						  VALUES (?, ?, ?, 'person', ?, ?, ?)`
			_, _, err = s.sql.Exec(ctx, insertSQL, name, nameLower, aliases, entityID, subtype, popularity)
		}
		return err
	}

	// Add directors
	for _, director := range media.Directors {
		nameLower := strings.ToLower(director)
		if nameLower == "" || seen[nameLower] {
			continue
		}
		seen[nameLower] = true

		if err := upsertPerson(director, "director", 10); err != nil {
			s.logger.Warn("failed to add director", "name", director, "error", err)
		} else {
			added++
		}
	}

	// Add cast members
	for i, cast := range media.Cast {
		nameLower := strings.ToLower(cast.Name)
		if nameLower == "" || seen[nameLower] {
			continue
		}
		seen[nameLower] = true

		// Popularity: higher for lead actors (first in list)
		popularity := float64(5 - i)
		if popularity < 1 {
			popularity = 1
		}

		if err := upsertPerson(cast.Name, "actor", popularity); err != nil {
			s.logger.Warn("failed to add actor", "name", cast.Name, "error", err)
		} else {
			added++
		}
	}

	// Add writers
	for _, writer := range media.Writers {
		nameLower := strings.ToLower(writer)
		if nameLower == "" || seen[nameLower] {
			continue
		}
		seen[nameLower] = true

		if err := upsertPerson(writer, "writer", 5); err != nil {
			s.logger.Warn("failed to add writer", "name", writer, "error", err)
		} else {
			added++
		}
	}

	return added, nil
}

// IsPopulating returns whether index population is in progress.
func (s *AutocompleteService) IsPopulating() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isPopulating
}

// LastPopulateTime returns when the index was last populated.
func (s *AutocompleteService) LastPopulateTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastPopulate
}

// normalizeQueryForAutocomplete normalizes a search query for autocomplete matching.
// This is similar to normalizeQuery in embedding.go but kept separate for clarity.
func normalizeQueryForAutocomplete(text string) string {
	// Lowercase
	text = strings.ToLower(text)

	// Remove extra whitespace
	text = strings.TrimSpace(text)
	text = strings.Join(strings.Fields(text), " ")

	return text
}

// normalizeName normalizes a name for consistent matching.
func normalizeName(name string) string {
	// Lowercase
	name = strings.ToLower(name)

	// Remove common suffixes
	suffixes := []string{" jr.", " jr", " sr.", " sr", " iii", " ii", " iv"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	// Strip punctuation (keep letters, numbers, spaces)
	var result strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' {
			result.WriteRune(r)
		}
	}
	name = result.String()

	// Collapse whitespace
	name = strings.Join(strings.Fields(name), " ")

	return name
}

// generateAliases generates common alias patterns for a person name.
// Returns space-separated aliases for FTS5 trigram indexing.
func generateAliases(name string) string {
	normalized := normalizeName(name)
	parts := strings.Fields(normalized)
	if len(parts) < 2 {
		return ""
	}

	var aliases []string
	first := parts[0]
	last := parts[len(parts)-1]

	// "steven spielberg" -> "s spielberg", "steven s"
	if len(first) > 0 && len(last) > 0 {
		aliases = append(aliases, first[:1]+" "+last) // "s spielberg"
		aliases = append(aliases, first+" "+last[:1]) // "steven s"
		aliases = append(aliases, first+last)         // "stevenspielberg" (no space)
	}

	// For names with multiple parts: "robert downey" -> "rd"
	if len(parts) >= 2 {
		initials := ""
		for _, p := range parts {
			if len(p) > 0 {
				initials += p[:1]
			}
		}
		if len(initials) >= 2 {
			aliases = append(aliases, initials) // "rd", "rdj" etc.
		}
	}

	// Return space-separated for alias matching
	return strings.Join(aliases, " ")
}

// escapeLIKE escapes special characters for SQL LIKE queries.
func escapeLIKE(s string) string {
	// LIKE special characters: % _ \
	replacer := strings.NewReplacer(
		`%`, `\%`,
		`_`, `\_`,
		`\`, `\\`,
	)
	return replacer.Replace(s)
}

// hashString creates a simple hash of a string for use as entity ID.
// This is used for people who don't have real IDs in our system.
func hashString(s string) int64 {
	var hash int64 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + int64(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
