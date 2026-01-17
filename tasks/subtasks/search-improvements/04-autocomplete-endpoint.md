# Task 04: Add /autocomplete HTTP Endpoint

## Objective
Add an HTTP endpoint for autocomplete that the frontend can call during type-ahead.

## Requirements
1. Register `/autocomplete` route in plugin
2. Handle GET requests with query parameters
3. Return JSON response with suggestions
4. Include response timing for debugging

## API Specification
**Endpoint**: `GET /api/plugins/semantic-search/autocomplete`

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `q` | string | required | Search query (min 2 chars) |
| `limit` | int | 8 | Max suggestions to return |
| `types` | string | "all" | Filter: "titles", "people", "genres", or "all" |

**Response**:
```json
{
  "suggestions": [
    {"type": "title", "text": "The Lord of the Rings: The Fellowship of the Ring", "entity_id": 123, "year": 2001, "subtype": "movie"},
    {"type": "person", "text": "Peter Jackson", "entity_id": 456, "subtype": "director"}
  ],
  "query": "lord ring",
  "took_ms": 8
}
```

## Files to Modify
- `plugins/semantic-search/internal/plugin.go` - Add route registration and handler

## Implementation Notes
- Minimum query length: 2 characters
- Maximum limit: 20 suggestions
- Return 400 for missing/short query
- Include `took_ms` for performance monitoring
- Use existing `jsonResponse()` helper

## Acceptance Criteria
- [ ] Route registered in GetRoutes()
- [ ] Handler validates query parameters
- [ ] Returns proper JSON response format
- [ ] Includes timing information
- [ ] Error handling for edge cases

## Dependencies
- Task 02 (AutocompleteService must exist)
