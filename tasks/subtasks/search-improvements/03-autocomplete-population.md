# Task 03: Populate FTS5 Index on Startup and Library Scan

## Objective
Populate the FTS5 autocomplete index with data from the host database on plugin startup and after library scans.

## Requirements
1. Populate titles (movies, TV shows) with original_title as alias
2. Populate people (directors, actors, writers) with generated aliases
3. Implement alias generation for common nickname patterns
4. Trigger repopulation after library scan completion

## Alias Generation Rules
```go
// "Steven Spielberg" generates:
// - "s spielberg" (first initial + last)
// - "steven s" (first + last initial)
// - "stevenspielberg" (no space)
// - "ss" (initials)

func generateAliases(name string) string
func normalizeName(name string) string  // lowercase, strip punctuation, remove suffixes
```

## Population Sources
1. **Titles**: From `sdk.DataClient.ListMediaByLibrary()`
   - name: title
   - aliases: original_title (if different)
   - type: "title"
   - subtype: "movie" or "tv_show"
   - year: release year
   - popularity: rating_votes

2. **People**: From credits in media details
   - name: person name
   - aliases: generated patterns
   - type: "person"
   - subtype: primary role (director > actor > writer)
   - popularity: credit count

## Files to Modify
- `plugins/semantic-search/internal/autocomplete.go` - Add population methods
- `plugins/semantic-search/internal/plugin.go` - Call population on init

## Implementation Notes
- Clear and rebuild index on each population (simpler than incremental)
- Use transactions for bulk inserts
- Log population progress and timing
- Handle missing data gracefully

## Acceptance Criteria
- [ ] Titles populated with original_title aliases
- [ ] People populated with generated aliases
- [ ] Alias generation handles edge cases (Jr., III, etc.)
- [ ] Population runs on plugin startup
- [ ] Population triggered after library scan
- [ ] Performance: <5 seconds for 10k items

## Dependencies
- Task 02 (AutocompleteService must exist)
