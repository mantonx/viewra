# Task 02: Implement AutocompleteService with Tiered Ranking

## Objective
Create an AutocompleteService that queries the FTS5 table with tiered ranking for optimal results.

## Requirements
1. Create `AutocompleteService` struct with search methods
2. Implement tiered ranking: exact prefix > token-start > trigram
3. Support filtering by type (titles, people, genres, all)
4. Return entity IDs for direct navigation

## Tiered Ranking Logic
| Tier | Match Type | Example Query | Example Match |
|------|------------|---------------|---------------|
| 0 | Exact prefix on name | "alien" | "Alien", "Aliens" |
| 1 | Token-start matches | "lord ring" | "The Lord of the Rings" |
| 2 | Trigram contains | "ien" | "Alien" |

## API Design
```go
type AutocompleteService struct {
    sql    *sdk.SQLClient
    logger *slog.Logger
}

type AutocompleteResult struct {
    Type       string `json:"type"`        // "title", "person", "genre"
    Text       string `json:"text"`        // Display text
    EntityID   int64  `json:"entity_id"`   // For navigation
    Subtype    string `json:"subtype"`     // "movie", "director", etc.
    Year       int    `json:"year,omitempty"`
    Popularity int64  `json:"popularity"`
}

func (s *AutocompleteService) Search(ctx context.Context, query string, limit int, types string) ([]AutocompleteResult, error)
```

## Files to Create
- `plugins/semantic-search/internal/autocomplete.go`

## Implementation Notes
- Use `buildAutocompleteQuery()` to construct FTS5 queries
- Normalize query: lowercase, collapse whitespace
- Query construction: "lord ring" → "lord* ring*" for prefix matching
- Sort by match_tier ASC, then popularity DESC

## Acceptance Criteria
- [ ] AutocompleteService created with Search method
- [ ] Tiered ranking implemented correctly
- [ ] Type filtering works (titles, people, all)
- [ ] Results include entity_id for navigation
- [ ] Query normalization handles edge cases

## Dependencies
- Task 01 (FTS5 schema must exist)
