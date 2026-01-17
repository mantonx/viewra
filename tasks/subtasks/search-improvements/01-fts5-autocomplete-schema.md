# Task 01: Create FTS5 Autocomplete Schema and Migration

## Objective
Create the FTS5 virtual table schema for autocomplete functionality in the semantic-search plugin's SQLite database.

## Requirements
1. Add migration to create `autocomplete_fts` FTS5 virtual table with trigram tokenizer
2. Schema must support titles, people, and genres with metadata
3. Follow existing migration pattern in `plugin.go`

## Schema Design
```sql
CREATE VIRTUAL TABLE autocomplete_fts USING fts5(
    name,                    -- Title or person name (searchable)
    aliases,                 -- Alternative names, space-separated (searchable)
    type UNINDEXED,          -- 'title', 'person', 'genre'
    entity_id UNINDEXED,     -- Movie/TV ID or person ID
    subtype UNINDEXED,       -- For titles: 'movie', 'tv_show'; For people: 'director', 'actor'
    year UNINDEXED,          -- For titles
    popularity UNINDEXED,    -- For ranking (rating_votes or credit_count)
    tokenize='trigram'
);
```

## Files to Modify
- `plugins/semantic-search/internal/plugin.go` - Add migration version 2
- `plugins/semantic-search/internal/schema.go` - Add schema constants (if needed)

## Implementation Notes
- Add as migration version 2 (version 1 is `mood_tags` table)
- Use `tokenize='trigram'` for partial matching support
- UNINDEXED columns are metadata only, not searchable

## Acceptance Criteria
- [ ] Migration runs successfully on plugin initialization
- [ ] FTS5 table created with correct schema
- [ ] Trigram tokenizer enabled for partial matching
- [ ] Existing `mood_tags` migration still works

## Dependencies
None - this is the foundation task.
