package querier

import (
	"context"
	"strings"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseCastString parses a cast string (format varies by source).
// Returns simplified cast member info.
func parseCastString(s string) []CastMemberInfo {
	if s == "" {
		return nil
	}
	// Cast is typically stored as comma-separated names
	names := splitAndTrim(s)
	cast := make([]CastMemberInfo, len(names))
	for i, name := range names {
		cast[i] = CastMemberInfo{
			Name:  name,
			Order: i,
		}
	}
	return cast
}

// normalizeLanguageCode normalizes various language code formats to ISO 639-1.
// Handles 3-letter codes (e.g., "kor" -> "ko") and common variations.
func normalizeLanguageCode(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		return ""
	}

	// Map 3-letter (ISO 639-2) to 2-letter (ISO 639-1)
	iso3to1 := map[string]string{
		"kor": "ko", "korean": "ko",
		"jpn": "ja", "japanese": "ja",
		"eng": "en", "english": "en",
		"chi": "zh", "zho": "zh", "cmn": "zh", "chinese": "zh",
		"spa": "es", "spanish": "es",
		"fre": "fr", "fra": "fr", "french": "fr",
		"ger": "de", "deu": "de", "german": "de",
		"ita": "it", "italian": "it",
		"por": "pt", "portuguese": "pt",
		"rus": "ru", "russian": "ru",
		"hin": "hi", "hindi": "hi",
		"ara": "ar", "arabic": "ar",
		"tha": "th", "thai": "th",
		"vie": "vi", "vietnamese": "vi",
		"ind": "id", "indonesian": "id",
		"swe": "sv", "swedish": "sv",
		"nor": "no", "norwegian": "no",
		"dan": "da", "danish": "da",
		"fin": "fi", "finnish": "fi",
		"dut": "nl", "nld": "nl", "dutch": "nl",
		"pol": "pl", "polish": "pl",
		"tur": "tr", "turkish": "tr",
		"gre": "el", "ell": "el", "greek": "el",
		"heb": "he", "hebrew": "he",
		"per": "fa", "fas": "fa", "persian": "fa",
		"tam": "ta", "tamil": "ta",
		"tel": "te", "telugu": "te",
		"mal": "ml", "malayalam": "ml",
		"kan": "kn", "kannada": "kn",
		"mar": "mr", "marathi": "mr",
		"ben": "bn", "bengali": "bn",
		"pun": "pa", "pan": "pa", "punjabi": "pa",
		"guj": "gu", "gujarati": "gu",
	}

	if iso1, ok := iso3to1[lang]; ok {
		return iso1
	}

	// If already 2 chars, assume it's ISO 639-1
	if len(lang) == 2 {
		return lang
	}

	return ""
}

// getCreditsForEntity fetches credits from the credits table with proper billing order.
// Returns cast and crew (directors, writers, producers) separately for embedding generation.
func (q *DBMediaQuerier) getCreditsForEntity(ctx context.Context, mediaType string, entityID int64) (cast []CastMemberInfo, directors, writers, producers []string) {
	// Fetch cast
	if q.router.IsPostgresDB() {
		castRows, err := q.postgres.GetCreditsForEntityByType(ctx, sqlc_postgres.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "cast",
		})
		if err == nil {
			for _, row := range castRows {
				cast = append(cast, CastMemberInfo{
					Name:      row.PersonName,
					Character: row.CharacterName.String,
					Order:     int(row.BillingOrder.Int64),
				})
			}
		}

		// Fetch directors
		directorRows, err := q.postgres.GetCreditsForEntityByType(ctx, sqlc_postgres.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "director",
		})
		if err == nil {
			for _, row := range directorRows {
				directors = append(directors, row.PersonName)
			}
		}

		// Fetch writers
		writerRows, err := q.postgres.GetCreditsForEntityByType(ctx, sqlc_postgres.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "writer",
		})
		if err == nil {
			for _, row := range writerRows {
				writers = append(writers, row.PersonName)
			}
		}

		// Fetch producers
		producerRows, err := q.postgres.GetCreditsForEntityByType(ctx, sqlc_postgres.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "producer",
		})
		if err == nil {
			for _, row := range producerRows {
				producers = append(producers, row.PersonName)
			}
		}
	} else {
		castRows, err := q.sqlite.GetCreditsForEntityByType(ctx, sqlc_sqlite.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "cast",
		})
		if err == nil {
			for _, row := range castRows {
				cast = append(cast, CastMemberInfo{
					Name:      row.PersonName,
					Character: row.CharacterName.String,
					Order:     int(row.BillingOrder.Int64),
				})
			}
		}

		// Fetch directors
		directorRows, err := q.sqlite.GetCreditsForEntityByType(ctx, sqlc_sqlite.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "director",
		})
		if err == nil {
			for _, row := range directorRows {
				directors = append(directors, row.PersonName)
			}
		}

		// Fetch writers
		writerRows, err := q.sqlite.GetCreditsForEntityByType(ctx, sqlc_sqlite.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "writer",
		})
		if err == nil {
			for _, row := range writerRows {
				writers = append(writers, row.PersonName)
			}
		}

		// Fetch producers
		producerRows, err := q.sqlite.GetCreditsForEntityByType(ctx, sqlc_sqlite.GetCreditsForEntityByTypeParams{
			MediaType:  mediaType,
			EntityID:   entityID,
			CreditType: "producer",
		})
		if err == nil {
			for _, row := range producerRows {
				producers = append(producers, row.PersonName)
			}
		}
	}

	return cast, directors, writers, producers
}

// getPrimaryAudioLanguage fetches the primary audio track language for a media item.
// This is used as a fallback when original_language is not set in metadata.
// Returns the normalized ISO 639-1 code (e.g., "ko", "ja", "en") or empty string.
func (q *DBMediaQuerier) getPrimaryAudioLanguage(ctx context.Context, mediaID int64) string {
	// Use SQLC-generated query to get all audio tracks, then find primary language
	if q.router.IsPostgresDB() {
		tracks, err := q.postgres.GetAudioTracksByMediaID(ctx, mediaID)
		if err != nil || len(tracks) == 0 {
			return ""
		}
		// Find the default track, or use the first one
		for _, track := range tracks {
			if common.NullInt64ToBool(track.IsDefault) && track.Language.Valid && track.Language.String != "" {
				return normalizeLanguageCode(track.Language.String)
			}
		}
		// Fallback to first track with language
		for _, track := range tracks {
			if track.Language.Valid && track.Language.String != "" {
				return normalizeLanguageCode(track.Language.String)
			}
		}
	} else {
		tracks, err := q.sqlite.GetAudioTracksByMediaID(ctx, mediaID)
		if err != nil || len(tracks) == 0 {
			return ""
		}
		// Find the default track, or use the first one
		for _, track := range tracks {
			if common.NullInt64ToBool(track.IsDefault) && track.Language.Valid && track.Language.String != "" {
				return normalizeLanguageCode(track.Language.String)
			}
		}
		// Fallback to first track with language
		for _, track := range tracks {
			if track.Language.Valid && track.Language.String != "" {
				return normalizeLanguageCode(track.Language.String)
			}
		}
	}
	return ""
}

// getStudiosForEntity fetches studio names for a media entity.
func (q *DBMediaQuerier) getStudiosForEntity(ctx context.Context, mediaType string, entityID int64) []string {
	var studios []string

	if q.router.IsPostgresDB() {
		rows, err := q.postgres.GetStudiosForEntity(ctx, sqlc_postgres.GetStudiosForEntityParams{
			MediaType: mediaType,
			EntityID:  entityID,
		})
		if err == nil {
			for _, row := range rows {
				studios = append(studios, row.Name)
			}
		}
	} else {
		rows, err := q.sqlite.GetStudiosForEntity(ctx, sqlc_sqlite.GetStudiosForEntityParams{
			MediaType: mediaType,
			EntityID:  entityID,
		})
		if err == nil {
			for _, row := range rows {
				studios = append(studios, row.Name)
			}
		}
	}

	return studios
}

// getLocationKeywordsForEntity fetches location-related keywords for a media entity.
// These keywords indicate where the story is set (e.g., "new york city", "paris", "tokyo").
func (q *DBMediaQuerier) getLocationKeywordsForEntity(ctx context.Context, mediaType string, entityID int64) []string {
	var keywords []string

	if q.router.IsPostgresDB() {
		rows, err := q.postgres.GetLocationKeywordsByEntity(ctx, sqlc_postgres.GetLocationKeywordsByEntityParams{
			MediaType: mediaType,
			EntityID:  entityID,
		})
		if err == nil {
			for _, row := range rows {
				keywords = append(keywords, row.Keyword)
			}
		}
	} else {
		rows, err := q.sqlite.GetLocationKeywordsByEntity(ctx, sqlc_sqlite.GetLocationKeywordsByEntityParams{
			MediaType: mediaType,
			EntityID:  entityID,
		})
		if err == nil {
			for _, row := range rows {
				keywords = append(keywords, row.Keyword)
			}
		}
	}

	return keywords
}

// getThemeKeywordsForEntity fetches non-location keywords for a media entity.
// These include themes, moods, plot elements, character types, etc.
func (q *DBMediaQuerier) getThemeKeywordsForEntity(ctx context.Context, mediaType string, entityID int64) []string {
	var keywords []string

	if q.router.IsPostgresDB() {
		rows, err := q.postgres.GetThemeKeywordsByEntity(ctx, sqlc_postgres.GetThemeKeywordsByEntityParams{
			MediaType: mediaType,
			EntityID:  entityID,
		})
		if err == nil {
			for _, row := range rows {
				keywords = append(keywords, row.Keyword)
			}
		}
	} else {
		rows, err := q.sqlite.GetThemeKeywordsByEntity(ctx, sqlc_sqlite.GetThemeKeywordsByEntityParams{
			MediaType: mediaType,
			EntityID:  entityID,
		})
		if err == nil {
			for _, row := range rows {
				keywords = append(keywords, row.Keyword)
			}
		}
	}

	return keywords
}
