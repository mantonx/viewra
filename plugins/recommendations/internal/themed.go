package internal

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// ThemedRecommendations generates context-aware themed recommendation rows.
type ThemedRecommendations struct {
	data   *sdk.DataClient
	logger interface {
		Debug(msg string, args ...any)
	}
}

// NewThemedRecommendations creates a new themed recommendations service.
func NewThemedRecommendations(data *sdk.DataClient, logger interface{ Debug(msg string, args ...any) }) *ThemedRecommendations {
	return &ThemedRecommendations{
		data:   data,
		logger: logger,
	}
}

// ThemedRow represents a themed recommendation row with metadata.
type ThemedRow struct {
	ID       string
	Title    string
	Subtitle string
	Query    ThemedQuery
	Priority int // Higher = more important
}

// ThemedQuery defines how to query for themed content.
type ThemedQuery struct {
	Genres       []string // Match any of these genres
	Directors    []string // Match any of these directors
	MinRating    float64  // Minimum rating threshold
	MaxRating    float64  // Maximum rating (for hidden gems)
	MinVotes     int      // Minimum vote count
	MaxVotes     int      // Maximum vote count (for hidden gems)
	YearFrom     int      // Release year range start
	YearTo       int      // Release year range end
	Keywords     []string // Title/plot keywords to match
	ExcludeGenre []string // Genres to exclude
}

// GetActiveThemedRows returns themed rows relevant to the current context.
func (t *ThemedRecommendations) GetActiveThemedRows(ctx context.Context) []ThemedRow {
	now := time.Now()
	hour := now.Hour()
	month := now.Month()
	weekday := now.Weekday()

	var rows []ThemedRow

	// 1. Time-of-day based rows
	rows = append(rows, t.getTimeOfDayRow(hour, weekday)...)

	// 2. Seasonal/Holiday rows
	rows = append(rows, t.getSeasonalRows(month, now.Day())...)

	// 3. Curated collections (always available, rotated)
	rows = append(rows, t.getCuratedRows(now)...)

	return rows
}

// getTimeOfDayRow returns mood-based rows for the current time.
func (t *ThemedRecommendations) getTimeOfDayRow(hour int, weekday time.Weekday) []ThemedRow {
	var rows []ThemedRow

	switch {
	case hour >= 22 || hour < 5: // Late night (10pm - 5am)
		rows = append(rows, ThemedRow{
			ID:       "late-night",
			Title:    "Late Night Thrillers",
			Subtitle: "Perfect for night owls",
			Priority: 75,
			Query: ThemedQuery{
				Genres:    []string{"Thriller", "Horror", "Mystery"},
				MinRating: 6.5,
				MinVotes:  1000,
			},
		})
	case hour >= 6 && hour < 12: // Morning (6am - 12pm)
		rows = append(rows, ThemedRow{
			ID:       "morning-picks",
			Title:    "Morning Inspiration",
			Subtitle: "Start your day right",
			Priority: 70,
			Query: ThemedQuery{
				Genres:    []string{"Documentary", "Drama", "Biography"},
				MinRating: 7.0,
				MinVotes:  5000,
			},
		})
	case hour >= 17 && hour < 22: // Evening (5pm - 10pm)
		rows = append(rows, ThemedRow{
			ID:       "evening-watch",
			Title:    "Evening Entertainment",
			Subtitle: "Unwind after your day",
			Priority: 70,
			Query: ThemedQuery{
				Genres:    []string{"Comedy", "Action", "Adventure"},
				MinRating: 6.5,
				MinVotes:  5000,
			},
		})
	}

	// Weekend special
	if weekday == time.Saturday || weekday == time.Sunday {
		rows = append(rows, ThemedRow{
			ID:       "weekend-epics",
			Title:    "Weekend Epics",
			Subtitle: "Big movies for your free time",
			Priority: 72,
			Query: ThemedQuery{
				Genres:    []string{"Adventure", "Fantasy", "Science Fiction", "War"},
				MinRating: 7.0,
				MinVotes:  10000,
			},
		})
	}

	return rows
}

// getSeasonalRows returns rows for holidays and seasons.
func (t *ThemedRecommendations) getSeasonalRows(month time.Month, day int) []ThemedRow {
	var rows []ThemedRow

	switch month {
	case time.December:
		// Holiday season
		if day >= 1 && day <= 31 {
			rows = append(rows, ThemedRow{
				ID:       "holiday-classics",
				Title:    "Holiday Classics",
				Subtitle: "Festive favorites",
				Priority: 85,
				Query: ThemedQuery{
					Keywords:  []string{"Christmas", "Holiday", "Santa"},
					Genres:    []string{"Family", "Comedy", "Fantasy"},
					MinRating: 6.0,
				},
			})
		}
	case time.October:
		// Halloween season
		if day >= 15 {
			rows = append(rows, ThemedRow{
				ID:       "halloween-horror",
				Title:    "Halloween Horrors",
				Subtitle: "Spooky season picks",
				Priority: 85,
				Query: ThemedQuery{
					Genres:    []string{"Horror", "Thriller"},
					MinRating: 6.0,
					MinVotes:  1000,
				},
			})
		}
	case time.February:
		// Valentine's / Romance
		if day >= 7 && day <= 14 {
			rows = append(rows, ThemedRow{
				ID:       "romantic-picks",
				Title:    "Romantic Picks",
				Subtitle: "Love is in the air",
				Priority: 80,
				Query: ThemedQuery{
					Genres:    []string{"Romance"},
					MinRating: 6.5,
					MinVotes:  5000,
				},
			})
		}
	case time.June, time.July, time.August:
		// Summer blockbusters
		rows = append(rows, ThemedRow{
			ID:       "summer-blockbusters",
			Title:    "Summer Blockbusters",
			Subtitle: "Big screen excitement",
			Priority: 75,
			Query: ThemedQuery{
				Genres:    []string{"Action", "Adventure", "Science Fiction"},
				MinRating: 6.5,
				MinVotes:  50000,
			},
		})
	case time.January, time.March:
		// Awards season
		rows = append(rows, ThemedRow{
			ID:       "award-winners",
			Title:    "Award Season Favorites",
			Subtitle: "Critically acclaimed",
			Priority: 78,
			Query: ThemedQuery{
				MinRating: 7.5,
				MinVotes:  20000,
			},
		})
	}

	return rows
}

// getCuratedRows returns rotating curated collections.
func (t *ThemedRecommendations) getCuratedRows(now time.Time) []ThemedRow {
	// Use day of year to rotate through collections
	dayOfYear := now.YearDay()

	// All possible curated rows
	curatedOptions := []ThemedRow{
		{
			ID:       "hidden-gems",
			Title:    "Hidden Gems",
			Subtitle: "Overlooked masterpieces",
			Priority: 65,
			Query: ThemedQuery{
				MinRating: 7.0,
				MaxVotes:  5000,
				MinVotes:  100,
			},
		},
		{
			ID:       "director-scorsese",
			Title:    "Martin Scorsese Collection",
			Subtitle: "A master of cinema",
			Priority: 60,
			Query: ThemedQuery{
				Directors: []string{"Martin Scorsese"},
			},
		},
		{
			ID:       "director-spielberg",
			Title:    "Steven Spielberg Collection",
			Subtitle: "Blockbuster visionary",
			Priority: 60,
			Query: ThemedQuery{
				Directors: []string{"Steven Spielberg"},
			},
		},
		{
			ID:       "director-kubrick",
			Title:    "Stanley Kubrick Collection",
			Subtitle: "Cinematic perfectionist",
			Priority: 60,
			Query: ThemedQuery{
				Directors: []string{"Stanley Kubrick"},
			},
		},
		{
			ID:       "director-hitchcock",
			Title:    "Alfred Hitchcock Collection",
			Subtitle: "The master of suspense",
			Priority: 60,
			Query: ThemedQuery{
				Directors: []string{"Alfred Hitchcock"},
			},
		},
		{
			ID:       "director-tarantino",
			Title:    "Quentin Tarantino Collection",
			Subtitle: "Bold and unforgettable",
			Priority: 60,
			Query: ThemedQuery{
				Directors: []string{"Quentin Tarantino"},
			},
		},
		{
			ID:       "director-nolan",
			Title:    "Christopher Nolan Collection",
			Subtitle: "Mind-bending cinema",
			Priority: 60,
			Query: ThemedQuery{
				Directors: []string{"Christopher Nolan"},
			},
		},
		{
			ID:       "director-coen",
			Title:    "Coen Brothers Collection",
			Subtitle: "Dark humor and wit",
			Priority: 60,
			Query: ThemedQuery{
				Directors: []string{"Joel Coen", "Ethan Coen"},
			},
		},
		{
			ID:       "classic-cinema",
			Title:    "Classic Cinema",
			Subtitle: "Timeless films from the golden age",
			Priority: 62,
			Query: ThemedQuery{
				YearFrom:  1930,
				YearTo:    1969,
				MinRating: 7.5,
				MinVotes:  5000,
			},
		},
		{
			ID:       "modern-classics",
			Title:    "Modern Classics",
			Subtitle: "Defining films of recent decades",
			Priority: 63,
			Query: ThemedQuery{
				YearFrom:  1990,
				YearTo:    2010,
				MinRating: 7.8,
				MinVotes:  50000,
			},
		},
		{
			ID:       "mind-benders",
			Title:    "Mind-Bending Films",
			Subtitle: "Prepare to think",
			Priority: 64,
			Query: ThemedQuery{
				Genres:    []string{"Science Fiction", "Mystery", "Thriller"},
				MinRating: 7.0,
				MinVotes:  10000,
			},
		},
		{
			ID:       "feel-good",
			Title:    "Feel Good Movies",
			Subtitle: "Guaranteed to lift your spirits",
			Priority: 66,
			Query: ThemedQuery{
				Genres:       []string{"Comedy", "Family", "Animation"},
				MinRating:    7.0,
				MinVotes:     10000,
				ExcludeGenre: []string{"Horror", "Thriller"},
			},
		},
		{
			ID:       "documentary-deep-dive",
			Title:    "Documentary Deep Dive",
			Subtitle: "True stories that captivate",
			Priority: 58,
			Query: ThemedQuery{
				Genres:    []string{"Documentary"},
				MinRating: 7.5,
				MinVotes:  1000,
			},
		},
		{
			ID:       "foreign-favorites",
			Title:    "International Cinema",
			Subtitle: "The best from around the world",
			Priority: 57,
			Query: ThemedQuery{
				MinRating: 7.5,
				MinVotes:  20000,
				// Would need language filter - for now rely on high ratings
			},
		},
	}

	// Pick 2 curated rows based on day rotation
	var selected []ThemedRow

	// Always include hidden gems (popular feature)
	selected = append(selected, curatedOptions[0])

	// Rotate through director spotlights (indices 1-7)
	directorIndex := 1 + (dayOfYear % 7)
	selected = append(selected, curatedOptions[directorIndex])

	// Rotate through other curated rows (indices 8-13)
	otherIndex := 8 + ((dayOfYear / 7) % 6)
	if otherIndex < len(curatedOptions) {
		selected = append(selected, curatedOptions[otherIndex])
	}

	return selected
}

// GetThemedRecommendations fetches items for a specific themed row.
func (t *ThemedRecommendations) GetThemedRecommendations(ctx context.Context, row ThemedRow, exclude map[int64]bool, limit int) ([]sdk.MediaItem, error) {
	if t.data == nil {
		return nil, fmt.Errorf("data client not available")
	}

	// Build exclude list
	excludeIDs := make([]int64, 0, len(exclude))
	for id := range exclude {
		excludeIDs = append(excludeIDs, id)
	}

	var items []sdk.MediaItem
	seen := make(map[int64]bool)

	// Note: Director-based filtering is not currently supported by the SDK.
	// Director collections will fall through to genre-based querying.
	// TODO: Add ListMediaByDirector to SDK when needed.

	// If we have genres, query by genre
	if len(row.Query.Genres) > 0 {
		for _, genre := range row.Query.Genres {
			if len(items) >= limit {
				break
			}
			genreItems, err := t.data.ListMediaByGenre(ctx, "movie", genre, 0, excludeIDs, limit)
			if err != nil {
				t.logger.Debug("genre query failed", "genre", genre, "error", err)
				continue
			}

			for _, item := range genreItems {
				if seen[item.ID] || exclude[item.ID] {
					continue
				}
				// Apply additional filters
				if !t.matchesQuery(item, row.Query) {
					continue
				}
				seen[item.ID] = true
				reason := row.Subtitle
				if reason == "" {
					reason = fmt.Sprintf("%s pick", genre)
				}
				items = append(items, sdk.MediaItem{
					EntityType: "movie",
					EntityID:   item.ID,
					Title:      item.Title,
					Year:       item.Year,
					Reason:     reason,
				})
				if len(items) >= limit {
					break
				}
			}
		}
		// Shuffle for variety within genres
		rand.Shuffle(len(items), func(i, j int) {
			items[i], items[j] = items[j], items[i]
		})
		if len(items) > limit {
			items = items[:limit]
		}
		return items, nil
	}

	// Generic query (hidden gems, classics, etc.) - query by common genres
	commonGenres := []string{"Drama", "Comedy", "Action", "Thriller", "Science Fiction", "Documentary"}
	for _, genre := range commonGenres {
		if len(items) >= limit*2 { // Get extra for filtering
			break
		}
		genreItems, err := t.data.ListMediaByGenre(ctx, "movie", genre, 0, excludeIDs, limit)
		if err != nil {
			continue
		}
		for _, item := range genreItems {
			if seen[item.ID] || exclude[item.ID] {
				continue
			}
			if !t.matchesQuery(item, row.Query) {
				continue
			}
			seen[item.ID] = true
			items = append(items, sdk.MediaItem{
				EntityType: "movie",
				EntityID:   item.ID,
				Title:      item.Title,
				Year:       item.Year,
				Reason:     row.Subtitle,
			})
		}
	}

	// Shuffle and limit
	rand.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// matchesQuery checks if a media item matches the query filters.
func (t *ThemedRecommendations) matchesQuery(item *sdk.Media, q ThemedQuery) bool {
	// Note: We don't have direct access to rating/votes from Media
	// The query already orders by popularity, so top results should match
	// Year filtering
	if q.YearFrom > 0 && item.Year < q.YearFrom {
		return false
	}
	if q.YearTo > 0 && item.Year > q.YearTo {
		return false
	}
	return true
}
