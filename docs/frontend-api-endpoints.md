# Frontend API Endpoints Documentation

This document lists all API endpoints currently used by the frontend, organized by functionality.

## Media Endpoints
```
GET  /api/media/files                       - Get media files with pagination
GET  /api/media/files/{id}                  - Get specific media file by ID
GET  /api/media/files/{id}/metadata         - Get metadata for a media file
GET  /api/media/files/{id}/stream           - Stream media file
GET  /api/media/files/{id}/album-artwork    - Get album artwork for media file
GET  /api/media/files/{id}/album-id         - Get album ID for media file
GET  /api/media/                            - Search media files (with limit param)
GET  /api/media/tv-shows                    - Get TV shows with pagination
GET  /api/media/music                       - Get music files with pagination
```

## Playback/Transcoding Endpoints
```
POST   /api/playback/decide                 - Get playback decision for media
POST   /api/playback/start                  - Start transcoding session
DELETE /api/playback/session/{sessionId}    - Stop transcoding session
POST   /api/playback/seek-ahead             - Request seek-ahead transcoding
GET    /api/playback/session/{sessionId}    - Get session status
DELETE /api/playback/sessions/all           - Stop all sessions
```

## Content-Addressable Storage (v1)
```
GET  /api/v1/content/{contentHash}/manifest.mpd    - Get DASH manifest
GET  /api/v1/content/{contentHash}/playlist.m3u8   - Get HLS playlist
GET  /api/v1/content/{contentHash}/{filename}      - Get content file (segments, etc.)
GET  /api/v1/content/{contentHash}/info            - Get content metadata and status
```

## Asset Management (v1)
```
GET  /api/v1/assets/{assetId}/data                                           - Get asset data by ID
GET  /api/v1/assets/entity/{entityType}/{entityId}/preferred/{assetType}     - Get preferred asset metadata
GET  /api/v1/assets/entity/{entityType}/{entityId}/preferred/{assetType}/data - Get preferred asset data
GET  /api/v1/assets/entity/{entityType}/{entityId}                          - Get all assets for entity
```

## TV Show Endpoints
```
GET  /api/tv/shows                                          - Get all TV shows
GET  /api/tv/shows/{id}                                     - Get TV show by ID
GET  /api/tv/shows/{id}/seasons                             - Get seasons for TV show
GET  /api/tv/shows/{showId}/seasons/{seasonId}/episodes     - Get episodes for season
GET  /api/tv/episodes/{episodeId}                           - Get episode by ID
```

## Movie Endpoints
```
GET  /api/movies                - Get all movies
GET  /api/movies/{id}           - Get movie by ID
```

## Search Endpoints
```
GET  /api/search                - Search across all media types
GET  /api/search/tv             - Search TV shows
GET  /api/search/movies         - Search movies
```

## Plugin Management (v1)
```
GET    /api/v1/plugins/                              - List all plugins with pagination
GET    /api/v1/plugins/search                        - Search plugins
GET    /api/v1/plugins/{id}                          - Get specific plugin
POST   /api/v1/plugins/{id}/enable                   - Enable plugin
POST   /api/v1/plugins/{id}/disable                  - Disable plugin
POST   /api/v1/plugins/{id}/restart                  - Restart plugin
POST   /api/v1/plugins/{id}/reload                   - Reload plugin
GET    /api/v1/plugins/{id}/config                   - Get plugin configuration
PUT    /api/v1/plugins/{id}/config                   - Update plugin configuration
GET    /api/v1/plugins/{id}/config/schema            - Get plugin config schema
POST   /api/v1/plugins/{id}/config/validate          - Validate plugin config
POST   /api/v1/plugins/{id}/config/reset             - Reset plugin config to defaults
GET    /api/v1/plugins/{id}/health                   - Get plugin health status
GET    /api/v1/plugins/{id}/metrics                  - Get plugin metrics
POST   /api/v1/plugins/{id}/health/reset             - Reset plugin health status
GET    /api/v1/plugins/{id}/admin-pages              - Get plugin admin pages
GET    /api/v1/plugins/admin/pages                   - Get all admin pages
GET    /api/v1/plugins/admin/navigation              - Get admin navigation structure
GET    /api/v1/plugins/system/status                 - Get system status
GET    /api/v1/plugins/system/stats                  - Get system statistics
POST   /api/v1/plugins/system/refresh                - Refresh all plugins
POST   /api/v1/plugins/system/cleanup                - System cleanup
GET    /api/v1/plugins/system/hot-reload             - Get hot reload status
POST   /api/v1/plugins/system/hot-reload/enable      - Enable hot reload
POST   /api/v1/plugins/system/hot-reload/disable     - Disable hot reload
POST   /api/v1/plugins/system/hot-reload/trigger/{id} - Trigger hot reload for plugin
GET    /api/v1/plugins/core/                          - List core plugins
GET    /api/v1/plugins/core/{name}                   - Get core plugin
POST   /api/v1/plugins/core/{name}/enable             - Enable core plugin
POST   /api/v1/plugins/core/{name}/disable            - Disable core plugin
GET    /api/v1/plugins/external/                      - List external plugins
GET    /api/v1/plugins/external/{id}                  - Get external plugin
POST   /api/v1/plugins/external/{id}/load             - Load external plugin
POST   /api/v1/plugins/external/{id}/unload           - Unload external plugin
POST   /api/v1/plugins/external/refresh               - Refresh external plugins
POST   /api/v1/plugins/system/bulk/enable             - Bulk enable plugins
POST   /api/v1/plugins/system/bulk/disable            - Bulk disable plugins
GET    /api/v1/plugins/categories                     - Get plugin categories
GET    /api/v1/plugins/capabilities                   - Get system capabilities
POST   /api/v1/plugins/external/                      - Install plugin
DELETE /api/v1/plugins/{id}                           - Uninstall plugin
```

## Admin/Management Endpoints
```
GET    /api/admin/media-libraries/               - List media libraries
POST   /api/admin/media-libraries/               - Add new media library
DELETE /api/admin/media-libraries/{id}           - Remove media library
GET    /api/admin/scanner/stats                  - Get scanner statistics
GET    /api/admin/scanner/library-stats          - Get library statistics
GET    /api/admin/scanner/current-jobs           - Get current scan jobs
GET    /api/admin/scanner/progress/{jobId}       - Get detailed scan progress
POST   /api/admin/scanner/start/{libraryId}      - Start scan for library
POST   /api/admin/scanner/pause/{libraryId}      - Pause scan for library
POST   /api/admin/scanner/resume/{libraryId}     - Resume scan for library
```

## Scanner/Monitoring Endpoints
```
GET  /api/scanner/monitoring                     - Get file monitoring status
```

## Dashboard Endpoints (v1)
```
GET  /api/v1/dashboard/sections                                      - Get dashboard sections
GET  /api/v1/dashboard/sections/{sectionId}/data/nerd                - Get nerd data for section
POST /api/v1/dashboard/sections/{sectionId}/actions/{actionId}      - Execute dashboard action
```

## System/Status Endpoints
```
GET  /api/health                - Health check endpoint
GET  /api/status                - System status
GET  /api/db-status             - Database status
GET  /api/users/                - Get users list
```

## Event Streaming
```
GET  /api/events/stream         - Server-sent events stream (with type filters)
```

## Notes

1. Many endpoints support query parameters for pagination (`limit`, `offset`, `page`), sorting (`sort`, `order`), and filtering (`search`, `filter`)
2. The API uses both `/api` and `/api/v1` prefixes, indicating a versioned API structure
3. Asset endpoints use a content-addressable system with entity types and UUIDs
4. The transcoding/playback system uses session-based management
5. Plugin management has comprehensive CRUD operations and configuration management
6. WebSocket connections are used for real-time dashboard updates