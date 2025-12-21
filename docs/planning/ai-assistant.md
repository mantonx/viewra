# AI Assistant - Requirements Document

## 1. Overview

A modular AI system for Viewra that provides semantic search, conversational AI, and personalized recommendations for movies, TV shows, and music.

### Goals

- Natural language search across media library
- Personalized recommendations based on watch history, context (weather, time, holidays)
- Conversational AI interface for discovery
- Support for local (Ollama) and cloud (OpenRouter, OpenAI, Anthropic) LLM providers
- Works with both PostgreSQL and SQLite

---

## 2. Architecture

### Design Philosophy: Thin Core, Rich Plugins

The core Viewra application provides **minimal AI infrastructure** - just enough to let plugins do their work. All AI features live in plugins, making them:

- **Optional**: Users only install what they need
- **Independently updatable**: Fix bugs or add features without updating Viewra
- **Substitutable**: Users can swap plugins (e.g., use a different recommendation engine)
- **Resource-conscious**: No AI overhead if no AI plugins installed

### Core Infrastructure (Built into Viewra)

```
┌─────────────────────────────────────────────────────────────────┐
│                         Viewra Host                             │
│                                                                 │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │              Minimal AI Infrastructure (Core)               │ │
│  │                                                             │ │
│  │  • LLM Provider Clients (shared library)                    │ │
│  │    - Ollama, OpenRouter, OpenAI, Anthropic                  │ │
│  │    - Plugins call these via gRPC                            │ │
│  │                                                             │ │
│  │  • Vector Storage (database layer only)                     │ │
│  │    - PostgreSQL: pgvector extension                         │ │
│  │    - SQLite: BLOB + cosine similarity                       │ │
│  │    - Exposed via data:embeddings permission                 │ │
│  │                                                             │ │
│  │  • AI Settings Storage                                      │ │
│  │    - Provider configs, API keys, preferences                │ │
│  │    - Exposed via data:settings permission                   │ │
│  │                                                             │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Plugin Manager                           │ │
│  └──────────────────────────┬─────────────────────────────────┘ │
└─────────────────────────────┼───────────────────────────────────┘
                              │ gRPC
          ┌───────────────────┼───────────────────┐
          │                   │                   │
          ▼                   ▼                   ▼
   ┌─────────────┐     ┌─────────────┐     ┌───────────────────┐
   │  ai-search  │     │   ai-chat   │     │ ai-recommendations│
   │  (plugin)   │     │  (plugin)   │     │     (plugin)      │
   └─────────────┘     └─────────────┘     └───────────────────┘
         │
         │ Provides semantic search
         │ that other plugins use
         ▼
   ┌─────────────────────────────────────────────────────────────┐
   │  ai-search exposes:                                          │
   │  • GET /api/search/semantic                                  │
   │  • GET /api/search/similar/:type/:id                         │
   │  • Indexing (enrichment stage + scheduled reindex)           │
   │  • Index management endpoints                                │
   └─────────────────────────────────────────────────────────────┘

External Services (optional, user-managed):
┌───────────────┐     ┌───────────────┐
│    Qdrant     │     │    Ollama     │
│  (optional)   │     │ (if local LLM)│
└───────────────┘     └───────────────┘
```

### Plugins

| Plugin | Purpose | Dependencies |
|--------|---------|--------------|
| **ai-search** | Semantic search, indexing, similar items | Core (LLM clients, vector storage) |
| **ai-chat** | Conversational interface, natural language queries | ai-search |
| **ai-recommendations** | Personalized, context-aware recommendations | ai-search |

### What Lives Where

| Component | Location | Rationale |
|-----------|----------|-----------|
| LLM provider clients | Core | Shared by all AI plugins, avoids duplication |
| Vector storage (repository) | Core | Database-level, plugins store/retrieve via gRPC |
| AI settings table | Core | Shared config for provider keys, model preferences |
| Embedding generation | ai-search plugin | Plugin-specific logic, not all users need it |
| Semantic search | ai-search plugin | Feature that requires indexing |
| Media indexing | ai-search plugin | Tightly coupled to search |
| Enrichment stage | ai-search plugin | Only runs when plugin installed |
| Chat UI/logic | ai-chat plugin | Standalone feature |
| Recommendations | ai-recommendations plugin | Standalone feature |
| Mood tag generation | ai-search plugin | Part of indexing pipeline |

---

## 3. Hardware Requirements

| Tier | GPU Examples | VRAM | RAM | Response Time | Concurrent Users | Library Size |
|------|--------------|------|-----|---------------|------------------|--------------|
| **CPU Only** | None / GTX 1050 | - | 16GB+ | 10-30s | 1 | <500 items |
| **Entry GPU** | GTX 1660, RTX 3050, RTX 4060 | 6-8GB | 16GB+ | 3-8s | 1-2 | <5,000 items |
| **Mid GPU** | RTX 3070, RTX 3080 Ti, RTX 4070 | 8-12GB | 32GB+ | 1-3s | 2-4 | <50,000 items |
| **High GPU** | RTX 3090, RTX 4090, A6000 | 16-48GB | 64GB+ | <1s | 5-10+ | 50,000+ items |

### What Each Tier Means

**CPU Only (Minimum):**

- Intel i5-8400 / AMD Ryzen 5 2600 or better
- 16GB RAM minimum, 32GB recommended
- No dedicated GPU, or older GPU (GTX 1050, integrated graphics)
- Usable but slow responses (10-30s)
- Indexing takes minutes to hours (~1-2 items/second)
- Smaller quantized models (Phi-3 3.8B Q4, Llama 3 8B Q4)
- Best for: Small libraries (<500 items), patient users, testing

**Entry GPU (Recommended Minimum):**

- NVIDIA: GTX 1660 (6GB), RTX 2060 (6GB), RTX 3050 (8GB), RTX 4060 (8GB)
- AMD: RX 6600 (8GB), RX 7600 (8GB) — requires ROCm support
- Intel Arc: A750 (8GB), A770 (16GB) — experimental support
- 16GB RAM
- Good experience (3-8s responses) for casual use
- Indexing: ~10-20 items/second
- Models: Llama 3 8B Q5/Q6, Mistral 7B Q4

**Mid GPU (Recommended):**

- NVIDIA: RTX 3070 (8GB), RTX 3080 (10GB), RTX 3080 Ti (12GB), RTX 4070 (12GB), RTX 4070 Ti (12GB)
- AMD: RX 6800 (16GB), RX 7800 XT (16GB)
- 32GB RAM
- Fast responses (1-3s), good multi-user support (2-4 concurrent)
- Indexing: ~50-100 items/second
- Models: Llama 3 8B FP16, Mistral 7B FP16, Qwen2.5 7B

**High GPU (Optimal):**

- NVIDIA: RTX 3090 (24GB), RTX 4080 (16GB), RTX 4090 (24GB), A5000 (24GB), A6000 (48GB)
- AMD: RX 7900 XTX (24GB)
- 64GB RAM
- Near-instant responses (<1s), excellent multi-user support (5-10+)
- Indexing: ~200+ items/second
- Models: Llama 3 70B Q4, Mixtral 8x7B, multiple models simultaneously

---

## 4. LLM Provider Support

| Provider | Type | Models | Notes |
|----------|------|--------|-------|
| **Ollama** | Local | Llama 3.1, Mistral, Phi-3, Qwen2.5, etc. | Free, self-hosted |
| **OpenRouter** | Cloud | Access to many models | Single API for multiple providers |
| **OpenAI** | Cloud | GPT-4o, GPT-4o-mini | Reliable, well-documented |
| **Anthropic** | Cloud | Claude 3.5 Sonnet, Claude 3 Haiku | High quality responses |

### Fallback Behavior

When no LLM provider is configured or limits are reached:

- Semantic search falls back to keyword search
- Shows message explaining why and how to configure
- Recommendations fall back to metadata-based matching

---

## 5. Vector Storage

| Database | Vector Extension | Notes |
|----------|-----------------|-------|
| **PostgreSQL** | `pgvector` | Native, production-ready, requires `CREATE EXTENSION vector` |
| **SQLite** | `sqlite-vss` | Embedded, no extra services |
| **Optional** | Qdrant (external) | For users wanting maximum performance |

### Embedding Configuration

- **User-selectable models**: Users can choose their embedding model
- **Dimension normalization**: All embeddings normalized to fixed dimension for consistency
- **Re-indexing**: Changing embedding model requires full re-index (user warned)

### What Gets Indexed

| Entity | Embedding Text |
|--------|---------------|
| **Movies** | `{title} ({year}). {tagline}. {plot}. Directed by {director}. Starring {cast}. Genres: {genre}. Mood: {mood_tags}.` |
| **TV Shows** | `{title} ({year}). {tagline}. {plot}. Network: {network}. Genres: {genre}. Mood: {mood_tags}.` |
| **TV Episodes** | `{show_title} - S{season}E{episode}: {episode_title}. {plot}. Mood: {mood_tags}.` |
| **Music Artists** | `{name}. {bio}. Genre: {genre}. From {country}.` |
| **Music Albums** | `{title} by {album_artist} ({year}). Genre: {genre}. Type: {release_type}. Mood: {mood_tags}.` |
| **Music Tracks** | `{title} by {artist}. Album: {album}. Genre: {genre}.` |

### Indexing Behavior

- **Scheduled refresh**: Daily by default (configurable: hourly, daily, weekly)
- **Background processing**: Runs with progress indicator
- **Pausable/Resumable**: Can pause and resume indexing
- **Priority**: New items indexed first, then backfill existing
- **Manual trigger**: Re-index button in settings

### Indexing Service Architecture

The media indexing service lives in the **ai-search plugin** and integrates with Viewra's enrichment pipeline:

```
┌─────────────────────────────────────────────────────────────────────┐
│                      ai-search Plugin                                │
│  (plugins/ai-search/)                                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  MediaIndexingService                                               │
│  ├─ IndexSingle(mediaID, mediaType) → enricher calls this          │
│  ├─ IndexBatch([]mediaID, mediaType) → efficient batching          │
│  ├─ IndexLibrary(libraryID) → full reindex with progress           │
│  └─ GetIndexStatus() → stats (indexed/total per type)              │
│                                                                     │
│  EmbeddingService                                                   │
│  ├─ Embed(texts) → generate embeddings via LLM provider            │
│  └─ Normalize(embedding, targetDim) → dimension normalization      │
│                                                                     │
│  SemanticSearchService                                              │
│  ├─ Search(query, filters) → natural language search               │
│  └─ FindSimilar(entityType, entityID) → similarity search          │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│  Uses from Core (via gRPC):                                         │
│  - LLM provider clients (for embedding generation)                  │
│  - Embedding repository (store/retrieve vectors)                    │
│  - Media repositories (read metadata for indexing)                  │
│  - Event bus (publish: indexing.progress, indexing.complete)        │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              │ Registers:
                              ▼
┌──────────────────────┐    ┌──────────────────────┐
│  AIIndexerEnricher   │    │   Scheduled Task     │
│  (enrichment stage)  │    │   (daily reindex)    │
├──────────────────────┤    ├──────────────────────┤
│ Stage: "ai_index"    │    │ ID: "ai-full-reindex"│
│ Position: 99         │    │ Schedule: 0 3 * * *  │
│ After: tmdb, mb      │    │ (3 AM daily)         │
│ Disabled by default  │    │ Disabled by default  │
└──────────────────────┘    └──────────────────────┘
```

#### Integration with Enrichment Pipeline

The AI indexer runs as the **final enrichment stage** (position 99):

1. **Scanner** discovers/updates media → saves to database
2. **Scanner** calls `EnqueueFirstStage()` → adds to enrichment queue
3. **NFO enricher** (stage 1) → extracts local metadata
4. **Local Images enricher** (stage 2) → extracts embedded artwork
5. **TMDB/MusicBrainz** (stage 3-4) → fetches external metadata
6. **AI Indexer** (stage 99) → generates embedding from complete metadata

This ensures embeddings are generated with **full metadata** (plot, cast, genres) rather than just file names.

#### Text Assembly

For each media type, we build rich searchable text:

**Movies:**
```
Title: The Shawshank Redemption (1994)
Genre: Drama
Plot: Two imprisoned men bond over a number of years...
Cast: Tim Robbins, Morgan Freeman, Bob Gunton
Director: Frank Darabont
Mood: hopeful, inspiring, dramatic, emotional, uplifting
```

**TV Shows:**
```
Title: Breaking Bad (2008)
Genre: Crime, Drama, Thriller  
Plot: A high school chemistry teacher diagnosed with lung cancer...
Cast: Bryan Cranston, Aaron Paul, Anna Gunn
Mood: tense, dark, thrilling, morally complex
```

**TV Episodes:**
```
Show: Breaking Bad
Episode: S5E14 - Ozymandias
Plot: Everyone copes with radically changed circumstances.
Mood: devastating, climactic, intense
```

**Music Albums:**
```
Artist: Pink Floyd
Album: The Dark Side of the Moon (1973)
Genre: Progressive Rock
Mood: atmospheric, introspective, psychedelic
```

#### Mood Tag Generation (Lazy)

Mood tags are generated **separately** from initial indexing:

1. **Initial indexing**: Fast, no LLM calls, embeddings based on existing metadata
2. **Mood tag generation**: Optional scheduled task or manual trigger
   - Uses LLM to analyze plot/metadata
   - Generates 3-5 mood tags per item
   - Stores in `media_mood_tags` table
   - Regenerates embedding with mood tags included

This approach provides:
- Fast initial indexing (no LLM latency)
- User control over when/if to generate mood tags
- Ability to use mood tags without cloud LLM (can run on Ollama)

#### Model Change Handling

When user changes the embedding model in settings:

```
┌─────────────────────────────────────────────────────────────┐
│  AI Settings                                                 │
├─────────────────────────────────────────────────────────────┤
│  Embedding Model: [nomic-embed-text ▼]                      │
│                                                             │
│  ⚠️  Changing the embedding model will invalidate existing  │
│     search index. Your library has 1,247 indexed items.     │
│                                                             │
│  What would you like to do?                                 │
│  ○ Reindex now (recommended) - ~15 minutes                  │
│  ○ Reindex overnight (next scheduled run at 3 AM)          │
│  ○ Clear index and reindex manually later                   │
│                                                             │
│  [Cancel]  [Save & Apply]                                   │
└─────────────────────────────────────────────────────────────┘
```

Backend behavior:
1. Detect model change in settings update
2. Store new model config
3. Based on user choice:
   - **Reindex now**: Clear embeddings, trigger immediate full reindex
   - **Reindex overnight**: Mark embeddings as stale, scheduled task will rebuild
   - **Clear and wait**: Delete all embeddings, user triggers manually later

---

## 6. API Endpoints

### Core (Viewra) - Minimal

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /api/search` | GET | Existing keyword search (unchanged) |
| `GET /api/ai/providers` | GET | List available LLM providers |
| `GET /api/ai/providers/:id/models` | GET | List models for a provider |
| `GET /api/settings/ai` | GET | Get AI settings (provider configs) |
| `PUT /api/settings/ai` | PUT | Update AI settings |

### ai-search Plugin

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /api/search/semantic` | GET | Semantic search via embeddings |
| `GET /api/search/semantic/status` | GET | Check if semantic search is ready |
| `GET /api/search/similar/:type/:id` | GET | Find items similar to entity |
| `GET /api/ai/index/status` | GET | Get indexing status and statistics |
| `GET /api/ai/index/estimate` | GET | Estimate cost before indexing |
| `POST /api/ai/index/start` | POST | Trigger manual reindex |
| `POST /api/ai/index/cancel` | POST | Cancel running reindex |
| `DELETE /api/ai/index` | DELETE | Clear all embeddings |
| `POST /api/ai/mood-tags/generate` | POST | Trigger mood tag generation |

**Semantic Search Parameters:**

```
GET /api/search/semantic
    ?q=sci-fi robots 1980s        # Natural language query
    &similar_to=123               # OR find similar to media ID
    &types=movie,tv_show          # Filter by media type
    &limit=20                     # Results limit
```

**Index Status Response:**

```json
{
  "ready": true,
  "indexing": false,
  "stats": {
    "movie": { "indexed": 450, "total": 500 },
    "tv_show": { "indexed": 80, "total": 80 },
    "tv_episode": { "indexed": 2400, "total": 2500 },
    "music_artist": { "indexed": 120, "total": 120 },
    "music_album": { "indexed": 450, "total": 450 },
    "music_track": { "indexed": 5000, "total": 5200 }
  },
  "last_indexed_at": "2024-01-15T03:00:00Z",
  "embedding_model": "nomic-embed-text",
  "embedding_dimensions": 768
}
```

**Cost Estimate Response:**

```json
GET /api/ai/index/estimate?library_id=1

{
  "library_id": 1,
  "library_name": "Movies",
  "items": {
    "movie": 1247,
    "tv_show": 0,
    "tv_episode": 0
  },
  "estimated_tokens": {
    "embeddings": 312000,
    "mood_tags_input": 625000,
    "mood_tags_output": 62000
  },
  "estimated_cost": {
    "embeddings_usd": 0.01,
    "mood_tags_usd": 0.17,
    "total_usd": 0.18,
    "disclaimer": "Approximate estimate based on configured pricing. Actual costs may vary."
  },
  "provider": {
    "embedding": "openai/text-embedding-3-small",
    "llm": "openai/gpt-4o-mini",
    "is_local": false
  }
}
```

For local providers (Ollama):

```json
{
  "estimated_cost": {
    "embeddings_usd": 0,
    "mood_tags_usd": 0,
    "total_usd": 0,
    "disclaimer": "Free - using local processing"
  },
  "provider": {
    "embedding": "ollama/nomic-embed-text",
    "llm": "ollama/llama3.1:8b",
    "is_local": true
  }
}
```

### Cost Estimation

Token estimates per entity type (used for cost calculation):

| Entity Type | Avg Tokens | Rationale |
|-------------|------------|-----------|
| Movie | 250 | Title + year + plot (~150) + cast (5×10) + genres + director |
| TV Show | 200 | Similar to movie, slightly shorter plots |
| TV Episode | 100 | Show reference + episode title + short plot |
| Music Artist | 60 | Name + bio excerpt + genres + country |
| Music Album | 80 | Artist + album + year + genres |
| Music Track | 50 | Track + artist + album + genre |

Default pricing is stored in settings and user-editable:

| Provider | Model | Default Price (per 1M tokens) |
|----------|-------|-------------------------------|
| Ollama | any | $0.00 (local) |
| OpenAI | text-embedding-3-small | $0.02 |
| OpenAI | text-embedding-3-large | $0.13 |
| OpenAI | gpt-4o-mini (input/output) | $0.15 / $0.60 |
| OpenAI | gpt-4o (input/output) | $2.50 / $10.00 |
| Anthropic | claude-3-haiku (input/output) | $0.25 / $1.25 |

**UI Display:**

```
┌─────────────────────────────────────────────────────────────┐
│  Index Library: Movies                                       │
├─────────────────────────────────────────────────────────────┤
│  Items to index: 1,247                                       │
│  Estimated tokens: ~312,000                                  │
│                                                             │
│  Estimated cost (approximate):                              │
│  • Embeddings (text-embedding-3-small): ~$0.01              │
│  • Mood tags (gpt-4o-mini): ~$0.17                          │
│                                                             │
│  ☑ Generate embeddings                                      │
│  ☐ Generate mood tags (+~$0.17)                             │
│                                                             │
│  Using local Ollama? Costs are $0.00                        │
│                                                             │
│  [Cancel]  [Start Indexing]                                 │
└─────────────────────────────────────────────────────────────┘
```

### ai-chat Plugin

| Endpoint | Method | Description |
|----------|--------|-------------|
| `POST /api/ai/chat` | POST | Send message, get conversational response |
| `GET /api/ai/chat/history` | GET | User's conversation history |
| `DELETE /api/ai/chat/history` | DELETE | Clear conversation history |

### ai-recommendations Plugin

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /api/ai/recommendations` | GET | Personalized recommendations |
| `GET /api/ai/similar/{id}` | GET | Items similar to media ID |
| `GET /api/ai/context` | GET | Current context (weather, time, holiday) |

---

## 7. Plugin Specifications

### ai-search Plugin (Foundation)

**plugin.yml:**

```yaml
id: ai-search
name: AI Search
version: 1.0.0
description: Semantic search and media indexing for AI features

categories:
  - ai

permissions:
  - data:media:read        # Read media metadata for indexing
  - data:embeddings:write  # Store embeddings in vector DB
  - data:embeddings:read   # Query embeddings for search
  - ai:llm                 # Generate embeddings via LLM providers
  - enrichment:register    # Register as enrichment stage

provides:
  - ai:search              # Other plugins can depend on this
```

**Features:**

- Semantic search across all media types
- Similar item discovery
- Media indexing (automatic via enrichment + manual reindex)
- Embedding generation with dimension normalization
- Cost estimation for cloud providers

**API Endpoints (registered by plugin):**

- `GET /api/search/semantic` - Natural language search
- `GET /api/search/similar/:type/:id` - Find similar items
- `GET /api/ai/index/status` - Indexing statistics
- `GET /api/ai/index/estimate` - Cost estimation
- `POST /api/ai/index/start` - Trigger reindex
- `POST /api/ai/index/cancel` - Cancel reindex
- `DELETE /api/ai/index` - Clear all embeddings

### ai-chat Plugin

**plugin.yml:**

```yaml
id: ai-chat
name: AI Chat
version: 1.0.0
description: Conversational AI for natural language media queries

categories:
  - ai

dependencies:
  - ai-search              # Requires ai-search for queries

permissions:
  - data:media:read
  - data:users:read
  - ai:llm
  - ai:search              # Use semantic search from ai-search plugin
```

**Features:**

- Natural language queries ("Find sci-fi movies from the 80s with robots")
- Conversational responses with explanations
- Multi-turn conversations with context
- Watch history awareness ("What's next in Breaking Bad?")

### ai-recommendations Plugin

**plugin.yml:**

```yaml
id: ai-recommendations
name: AI Recommendations
version: 1.0.0
description: Personalized, context-aware media recommendations

categories:
  - ai

dependencies:
  - ai-search              # Requires ai-search for similarity

permissions:
  - network                # Weather API
  - data:media:read
  - data:users:read
  - ai:llm
  - ai:search              # Use similarity search from ai-search plugin
```

**Features:**

- "Similar to X" recommendations
- Watch history-based suggestions
- Weather-aware recommendations
- Holiday/seasonal recommendations (based on user's country/region)
- Time-of-day awareness

**Context Services:**

- Weather: OpenWeatherMap API (user provides API key)
- Holidays: Based on user's country/region setting

---

## 8. Contextual Recommendation Factors

### Phase 1 Factors (Initial Release)

| Factor | Description | Data Source |
|--------|-------------|-------------|
| **Weather** | Cozy movies when raining, light content in good weather | Weather API + user location |
| **Holidays** | Seasonal content (Christmas, Halloween, etc.) | Holiday calendar + user country |
| **Time of day** | Light content late night, energetic content in evening | System clock + user timezone |
| **Day of week** | Different recommendations for weekends vs weekdays | System clock |
| **Weekend vs weekday** | Longer content on weekends, shorter on weekdays | System clock |
| **Watch history** | Based on what user has watched and enjoyed | `watch_progress` table |
| **Runtime awareness** | Time of day + explicit user input ("I have 30 minutes") | Clock + user query |
| **Mood/vibe** | LLM-generated tags + embedding similarity | See Mood Tagging below |

### Mood Tagging

Mood tags are generated during indexing and stored for fast filtering:

**Generation:**

- LLM analyzes plot, genre, and metadata during indexing
- Generates 3-5 mood tags per item
- Examples: `[hopeful, emotional, inspiring, serious]`, `[funny, lighthearted, nostalgic]`

**Storage:**

- Stored as searchable fields in database
- Also included in embedding text for semantic matching
- Dual approach: explicit tags for filtering + embeddings for nuance

**Example tags:**

| Category | Tags |
|----------|------|
| **Emotional tone** | uplifting, dark, tense, relaxing, heartwarming, melancholic |
| **Energy level** | slow-paced, fast-paced, intense, calm, meditative |
| **Social context** | family-friendly, date-night, solo-watching, party-background |
| **Themes** | thought-provoking, escapist, nostalgic, inspiring, suspenseful |

### Deferred Factors (Future Phases)

| Factor | Description | Why Deferred |
|--------|-------------|--------------|
| **Completion rate** | Avoid content similar to abandoned items | Needs behavior analysis |
| **Binge patterns** | Recommend series to bingers, movies to non-bingers | Needs pattern detection |
| **"Who's watching"** | Filter by audience (kids, adults, group) | UX complexity |
| **Group recommendations** | Find overlap between multiple users | Multi-user complexity |
| **Cultural moments** | Award season, actor birthdays, anniversaries | External data sources |

---

## 9. Settings UI

```
Settings
├── General
├── Libraries
├── Users
├── Playback
└── AI Services (visible when ai-chat or ai-recommendations enabled)
    ├── LLM Provider
    │   ├── Provider: [Ollama | OpenRouter | OpenAI | Anthropic]
    │   ├── Model: [dropdown based on provider]
    │   ├── API Key: [for cloud providers] (encrypted storage)
    │   └── Base URL: [for Ollama/custom endpoints]
    │
    ├── Embedding
    │   ├── Provider: [Ollama | OpenAI]
    │   ├── Model: [nomic-embed-text, bge-base, text-embedding-3-small]
    │   └── Note: Changing model requires re-indexing
    │
    ├── Vector Database
    │   ├── Type: [Built-in | External Qdrant]
    │   ├── URL: [if external]
    │   └── Status: [Connected | Indexing... | Error]
    │
    ├── Privacy
    │   ├── Privacy Mode: [On | Off]
    │   │   └── (When on: only embeddings sent, not raw text)
    │   └── Warning: "Plot summaries and titles are sent to external APIs"
    │
    ├── Cost Controls
    │   ├── Daily token limit per user: [10,000 | 50,000 | 100,000 | Unlimited]
    │   ├── Monthly budget cap: [$5 | $20 | $50 | Unlimited]
    │   ├── Current month usage: $X.XX / $XX.XX
    │   └── Warn at: [80%] of budget
    │
    ├── Weather (if ai-recommendations enabled)
    │   ├── Provider: [OpenWeatherMap]
    │   └── API Key: [required for weather features]
    │
    ├── Conversation History
    │   ├── Retention: [1 day | 1 week | 2 weeks | 1 month | Forever]
    │   └── Default: 2 weeks
    │
    └── Indexing
        ├── Schedule: [Hourly | Daily | Weekly]
        ├── Status: [Indexed 1,234 of 5,678 items]
        ├── Last indexed: [2 hours ago]
        └── [Re-index Library] button
```

---

## 10. User Interface

### Chat Interface

- **Location**: Floating button/overlay (like Intercom)
- **Behavior**: Persistent across pages, collapsible
- **Features**: Message history, typing indicator, suggested prompts

### Recommendations Display

Recommendations appear in multiple locations:

| Location | Context |
|----------|---------|
| **Home screen** | Widget with personalized picks |
| **Dedicated page** | `/recommendations` with full recommendations |
| **Video player** | "Up Next" when video finishes |
| **Movie pages** | "Similar Movies" section |
| **TV Show pages** | "Similar Shows" section |

### Onboarding

- **Guided setup wizard** when AI features first enabled
- Steps:
  1. Choose LLM provider (local vs cloud)
  2. Configure API keys (if cloud)
  3. Privacy warning acknowledgment
  4. Set cost limits (if cloud)
  5. Start initial indexing
  6. Optional: Configure weather for context-aware recommendations

---

## 11. Rate Limiting & Cost Controls

| Control | Description |
|---------|-------------|
| **Request rate limit** | Max requests per minute per user (configurable) |
| **Daily token limit** | Max tokens per user per day (prevents runaway costs) |
| **Monthly budget cap** | Optional: disable cloud LLM when budget reached |
| **Cost tracking** | Display estimated cost in settings UI |
| **Model recommendations** | Suggest cheaper models for budget-conscious users |
| **Usage warnings** | Alert at configurable percentage of budget |

**Fallback when limits reached:**

- Semantic search continues (uses embeddings, not LLM)
- Chat returns: "Daily AI limit reached. Try again tomorrow or switch to a local model."
- Recommendations fall back to metadata-based matching (no LLM explanations)

---

## 12. Privacy & Security

### Data Sent to Cloud Providers

When using cloud LLM providers (OpenAI, Anthropic, OpenRouter):

- Media titles and plots are sent for search/recommendations
- User queries are sent for processing
- Watch history context may be included

**Warning displayed during setup:**

> "When using cloud AI providers, media information (titles, plots, descriptions) and your queries are sent to external servers. Enable Privacy Mode to minimize data sent."

### Privacy Mode

When enabled:

- Only pre-computed embeddings are used for search
- Raw text (plots, descriptions) not sent to cloud LLM
- Recommendations based on embedding similarity only
- Chat responses may be less contextual

### API Key Storage

- All API keys encrypted at rest in database
- Keys never logged or exposed in UI after entry
- Option to use environment variables instead

---

## 13. Plugin Updates

- **Notification**: Users notified when plugin updates available
- **User control**: User decides when to update
- **No auto-update**: Updates require explicit user action
- **Changelog**: Show what changed before updating

---

## 14. Multi-language Support

- Embeddings support non-English queries
- Model selection considers multilingual capability
- UI language preference informs response language
- Recommended models for non-English: `bge-m3`, `multilingual-e5-large`

---

## 15. User Stories

| Story | Component | Example |
|-------|-----------|---------|
| Semantic search | Core | "Find sci-fi movies from the 80s with robots" |
| Similar content | ai-recommendations | "Movies like Inception" |
| Watch history query | ai-chat | "What's the next episode of Breaking Bad I haven't seen?" |
| Mood-based | ai-chat | "What should I watch tonight? I'm in the mood for something funny" |
| Weather-aware | ai-recommendations | "It's raining and cold, what should I watch?" |
| Holiday-themed | ai-recommendations | "Suggest a Christmas movie" |
| Music discovery | ai-chat | "Recommend an album similar to Dark Side of the Moon" |
| Metadata query | ai-chat | "Show me movies directed by Christopher Nolan" |
| Runtime query | ai-chat | "I have 30 minutes, what can I watch?" |
| Weekend browse | ai-recommendations | "It's Saturday, show me something epic" |
| Mood search | ai-chat | "Something uplifting but not cheesy" |

---

## 16. Schema Changes (Deferred)

The following fields should be added to the `users` table when implementation begins:

| Field | Type | Description |
|-------|------|-------------|
| `country` | TEXT | ISO country code (for holidays) |
| `timezone` | TEXT | IANA timezone (e.g., "America/New_York") |
| `city` | TEXT | City name (for weather) |
| `latitude` | REAL | Optional, for precise weather |
| `longitude` | REAL | Optional, for precise weather |

**New table: `media_mood_tags`**

| Field | Type | Description |
|-------|------|-------------|
| `id` | INTEGER | Primary key |
| `media_id` | INTEGER | FK to media table |
| `tag` | TEXT | Mood tag (e.g., "uplifting", "tense") |
| `confidence` | REAL | LLM confidence score (0-1) |
| `created_at` | TIMESTAMP | When tag was generated |

**Indexes:**

- `idx_media_mood_tags_media_id` on `media_id`
- `idx_media_mood_tags_tag` on `tag`

---

## 17. Implementation Priority

### Phase 1: Core AI Infrastructure ✓ (Completed)

Minimal infrastructure in Viewra core:

- [x] LLM provider clients (Ollama, OpenRouter, OpenAI, Anthropic)
- [x] Vector storage repository (pgvector for PostgreSQL, BLOB for SQLite)
- [x] Database migrations for `embeddings` and `ai_settings` tables
- [x] Domain layer types and interfaces
- [x] gRPC service definitions for plugin access

**What was removed from core (moved to ai-search plugin):**
- ~~Embedding service~~ → ai-search plugin
- ~~Semantic search service~~ → ai-search plugin
- ~~Media indexing service~~ → ai-search plugin
- ~~Search API endpoints~~ → ai-search plugin

### Phase 2: ai-search Plugin (Current)

The foundational AI plugin that other AI plugins depend on:

- [ ] Embedding generation with dimension normalization
- [ ] Semantic search (`Search()`, `FindSimilar()`)
- [ ] Media indexing service
  - `IndexSingle()` for enricher integration
  - `IndexBatch()` for efficient bulk processing
  - `IndexLibrary()` for full reindex with progress
  - Text builders for movies, TV, music
- [ ] Enrichment pipeline stage (`ai_index`, position 99)
- [ ] Scheduled task `ai-full-reindex` (daily, disabled by default)
- [ ] API endpoints:
  - `GET /api/search/semantic`
  - `GET /api/search/similar/:type/:id`
  - `GET /api/ai/index/status`
  - `GET /api/ai/index/estimate`
  - `POST /api/ai/index/start`
  - `POST /api/ai/index/cancel`
  - `DELETE /api/ai/index`
- [ ] Optional: Mood tag generation via LLM
- [ ] Settings UI integration (index status, reindex trigger)

### Phase 3: ai-recommendations Plugin

Depends on ai-search for similarity queries:

- [ ] Similar-to recommendations (calls ai-search `FindSimilar()`)
- [ ] Watch history-based recommendations
- [ ] Context services (weather, holidays, time)
- [ ] Personalized recommendation API
- [ ] UI integration (home, movie/TV pages, video player)
- [ ] Dedicated recommendations page

### Phase 4: ai-chat Plugin

Depends on ai-search for natural language queries:

- [ ] Chat service with floating overlay UI
- [ ] Conversation state management
- [ ] Natural language query processing (calls ai-search)
- [ ] Conversational response generation (uses LLM)
- [ ] Multi-turn conversation support
- [ ] History retention with configurable cleanup

### Phase 5: Settings UI & Setup Wizard

Frontend work (can be done in parallel with plugins):

- [ ] AI Settings page
- [ ] LLM provider configuration
- [ ] Embedding model selection with reindex warning
- [ ] Index status display with progress
- [ ] Privacy controls and warnings
- [ ] First-time AI setup wizard

---

## 18. Deployment

### Docker Compose (Initial)

**Minimal (semantic search only, local Ollama):**

```yaml
services:
  viewra:
    image: viewra/viewra:latest
    environment:
      - AI_EMBEDDING_PROVIDER=ollama
      - AI_EMBEDDING_MODEL=nomic-embed-text
      - OLLAMA_HOST=http://ollama:11434
    depends_on:
      - ollama

  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama_data:/root/.ollama
    deploy:
      resources:
        reservations:
          devices:
            - capabilities: [gpu]

volumes:
  ollama_data:
```

**Full stack with plugins:**

```yaml
services:
  viewra:
    image: viewra/viewra:latest
    environment:
      - AI_LLM_PROVIDER=ollama
      - AI_LLM_MODEL=llama3.1:8b
      - AI_EMBEDDING_PROVIDER=ollama
      - AI_EMBEDDING_MODEL=nomic-embed-text
      - OLLAMA_HOST=http://ollama:11434
    depends_on:
      - ollama

  ai-chat:
    image: viewra/ai-chat:latest
    environment:
      - VIEWRA_HOST=viewra:50051
    depends_on:
      - viewra

  ai-recommendations:
    image: viewra/ai-recommendations:latest
    environment:
      - VIEWRA_HOST=viewra:50051
      - WEATHER_API_KEY=${WEATHER_API_KEY}
    depends_on:
      - viewra

  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama_data:/root/.ollama
    deploy:
      resources:
        reservations:
          devices:
            - capabilities: [gpu]

volumes:
  ollama_data:
```

### Kubernetes (Future)

- Helm chart with configurable resource limits
- GPU node selector for Ollama
- Horizontal scaling for embedding service

---

## 19. Success Metrics

| Metric | Target |
|--------|--------|
| Semantic search latency (p95) | <500ms |
| Chat response latency (p95) | <3s (Mid GPU tier) |
| Indexing throughput | >50 items/sec (Mid GPU tier) |
| User satisfaction | Qualitative feedback from friends/family |
| Cost per query (cloud) | <$0.01 average |
