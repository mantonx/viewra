# AI Features Decoupling Plan

## Goal

Make AI features (semantic search, embeddings, vector storage) completely opt-in. The core app should have **zero knowledge** of AI concepts. If no AI plugins are installed, the app works exactly as a traditional media server with text-based search.

## Current State (Problem)

The core app currently has deep knowledge of AI concepts:

### Application Layer
- `internal/application/movies/semantic_search.go` - SemanticSearchProvider interface
- `internal/application/movies/search_movies.go` - Use case with semantic search wiring
- `internal/application/search/service.go` - "Fallback" search (implies primary is semantic)

### Infrastructure Layer
- `internal/infrastructure/plugins/semantic_provider.go` - SemanticSearchProvider implementation
- `internal/infrastructure/plugins/host/storage_vector*.go` - 5 files of vector storage code
- `internal/infrastructure/plugins/host/plugins.go` - InvokeVectorSearch method
- `internal/infrastructure/plugins/grpc/plugin.go` - VectorSearchPlugin
- `internal/infrastructure/plugins/types/types.go` - VectorSearchClient field

### API Layer
- `internal/api/handlers/search.go` - Checks for semantic_search capability
- `internal/api/routes/search.go` - Comments reference semantic search

### App Wiring
- `internal/app/container.go` - Wires SemanticSearchProvider after plugin load
- `internal/app/usecases/usecases.go` - WireSemanticSearch method
- `internal/app/services/services.go` - HostPluginsServer for InvokeVectorSearch

## Target State

### Core App
- **No** interfaces, types, or code referencing semantic search, embeddings, vectors
- Search endpoint is a simple text search
- Plugin system provides generic HTTP route registration
- Plugin system provides generic storage (SQL only, no vector-specific APIs)

### Plugins
- semantic-search plugin registers `/api/search` route (overrides core)
- semantic-search plugin manages its own vector storage (via generic SQL or external DB)
- semantic-search plugin does NOT call back into core for media data
- Provider plugins (ollama, voyage, etc.) are called by semantic-search, not core

## Migration Steps

### Phase 1: Remove SemanticSearchProvider from Core

**Files to modify:**

1. `internal/application/movies/search_movies.go`
   - Remove `semanticSearch` field
   - Remove `WithSemanticSearch()` method
   - Remove `executeSemanticSearch()` method
   - Execute() just does text search

2. `internal/application/movies/semantic_search.go`
   - **DELETE entire file**

3. `internal/app/usecases/usecases.go`
   - Remove `WireSemanticSearch()` method

4. `internal/app/container.go`
   - Remove semantic search wiring section (lines ~365-396)

5. `internal/infrastructure/plugins/semantic_provider.go`
   - **DELETE entire file**

### Phase 2: Simplify Search Handler

**Files to modify:**

1. `internal/api/handlers/search.go`
   - Remove capability check for semantic_search
   - Remove httpProxy dependency (or keep for other capabilities)
   - Always use searchService (text search)

2. `internal/api/routes/search.go`
   - Update comments

### Phase 3: Remove Vector Storage from Host

The vector storage is currently built into the host as a service for plugins. Options:

**Option A: Keep as plugin infrastructure (recommended)**
- Vector storage stays in `host/storage_vector*.go`
- It's plugin infrastructure, not core app knowledge
- Plugins that need it can use it
- Core app doesn't call it directly

**Option B: Move to plugin-owned storage**
- semantic-search plugin creates its own vector tables
- Uses sqlite-vec/pgvector directly
- More isolation but duplicates code

Recommend **Option A** - the vector storage is already isolated to plugin infrastructure.

### Phase 4: Remove InvokeVectorSearch

**Files to modify:**

1. `internal/infrastructure/plugins/host/plugins.go`
   - Remove `InvokeVectorSearch()` method
   - Remove `resolveVectorSearchProvider()` method

2. `internal/infrastructure/plugins/types/types.go`
   - Remove `VectorSearchClient` field from Instance

3. `internal/infrastructure/plugins/grpc/plugin.go`
   - Remove `VectorSearchPlugin` type
   - Remove vector search wrapper methods

4. `internal/infrastructure/plugins/grpc/factory.go`
   - Remove `NewVectorSearchGRPCPlugin()`

5. `internal/infrastructure/plugins/manager/loader.go`
   - Remove vector_search capability dispensing

6. `internal/app/services/services.go`
   - Remove `HostPluginsServer` field (or keep if needed for other purposes)

### Phase 5: Plugin Takes Over Search Route

**semantic-search plugin changes:**

1. Register `/api/search` route with higher priority
   - Currently done via `AliasPath: "/api/search"` in route definition
   - This already works - plugin overrides core route

2. Remove DataClient dependency
   - Plugin should NOT fetch media details from core
   - Instead, indexing should be triggered by core with text already built
   - OR plugin stores its own copy of searchable text

3. Handle "no embedding provider" gracefully
   - Return clear error message
   - Don't log spam during initialization

### Phase 6: Clean Up Domain Layer

**Files to modify:**

1. `internal/domain/search/types.go`
   - Remove `Fallback` field from Response (no longer needed)
   - Or keep if useful for other purposes

## Alternative: Minimal Fix

If the full decoupling is too large, a minimal fix addresses just the immediate issue:

1. **Remove IsAvailable() check from plugin Initialize()**
   - Check happens too early anyway

2. **Improve error message when embedding capability missing**
   - Translate `CAPABILITY_ERROR_NOT_FOUND` for "embedding" to user-friendly message

3. **Keep everything else as-is**
   - Accept the coupling for now
   - Document it as technical debt

## Recommendation

Start with **Phase 1** (remove SemanticSearchProvider interface) and **Phase 2** (simplify search handler). This removes the most visible coupling in the application layer.

Phases 3-5 can be deferred - the vector storage infrastructure is already isolated to plugins, it just happens to be hosted by the core app.

Phase 6 is optional cleanup.

## Files Summary

### To DELETE
- `internal/application/movies/semantic_search.go`
- `internal/infrastructure/plugins/semantic_provider.go`

### To MODIFY (significant)
- `internal/application/movies/search_movies.go`
- `internal/api/handlers/search.go`
- `internal/app/container.go`
- `internal/app/usecases/usecases.go`
- `internal/infrastructure/plugins/host/plugins.go`
- `internal/infrastructure/plugins/types/types.go`
- `internal/infrastructure/plugins/grpc/plugin.go`
- `internal/infrastructure/plugins/manager/loader.go`

### To MODIFY (minor)
- `internal/api/routes/search.go` (comments only)
- `internal/app/services/services.go`
- `internal/domain/search/types.go` (optional)
