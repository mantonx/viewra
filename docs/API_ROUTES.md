# Viewra API Routes Documentation

This document provides a comprehensive reference of all API routes available in the Viewra media management platform.

## Table of Contents
- [Core API Routes](#core-api-routes)
- [Media Management](#media-management)
- [Plugin System](#plugin-system)
- [Admin Routes](#admin-routes)
- [Module-Specific APIs](#module-specific-apis)

## Core API Routes

### Root & Health
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api` | ApiRootHandler | API discovery endpoint |
| GET | `/api/health` | HandleHealthCheck | System health check |
| GET | `/api/db-status` | HandleDBStatus | Database connection status |
| GET | `/api/db-health` | HandleDatabaseHealth | Comprehensive database health check |
| GET | `/api/connection-pool` | HandleConnectionPoolStats | Database connection pool statistics |

### Development
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/dev/load-test-music` | LoadTestMusicData | Load test music data (development only) |

### Events System
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/events/` | GetEvents | List all recorded events |
| GET | `/api/events/by-time` | GetEventsByTimeRange | Get events within a specific time range |
| GET | `/api/events/stats` | GetEventStats | Get statistics about recorded events |
| GET | `/api/events/types` | GetEventTypes | List all unique event types |
| GET | `/api/events/stream` | EventStream | Stream events in real-time (SSE) |
| POST | `/api/events/` | PublishEvent | Publish a new event (for testing/dev) |
| GET | `/api/events/subscriptions` | GetSubscriptions | List active event subscriptions |
| DELETE | `/api/events/:id` | DeleteEvent | Delete a specific event by ID |
| DELETE | `/api/events/` | ClearEvents | Clear all recorded events |

### Configuration Management
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/config/` | GetConfig | Get full configuration (sensitive data redacted) |
| GET | `/api/config/:section` | GetConfigSection | Get specific configuration section |
| PUT | `/api/config/:section` | UpdateConfigSection | Update configuration section |
| POST | `/api/config/reload` | ReloadConfig | Reload configuration from file |
| POST | `/api/config/save` | SaveConfig | Save current configuration to file |
| POST | `/api/config/validate` | ValidateConfig | Validate current configuration |
| GET | `/api/config/defaults` | GetConfigDefaults | Get default configuration values |
| GET | `/api/config/info` | GetConfigInfo | Get configuration system information |

## Media Management

### Media Routes
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/media/` | GetMedia | List all media items |
| GET | `/api/media/:id` | GetMediaByID | Get a specific media item by ID |
| GET | `/api/media/:id/stream` | StreamMedia | Stream a specific media file |
| GET | `/api/media/:id/artwork` | GetArtwork | Get artwork for a media item |
| GET | `/api/media/:id/metadata` | GetMusicMetadata | Get metadata for a music item |
| GET | `/api/media/music` | GetMusicFiles | List all music files |

### Playback Routes
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/media/playback/start` | RecordPlaybackStarted | Record media playback started |
| POST | `/api/media/playback/end` | RecordPlaybackFinished | Record media playback finished |
| POST | `/api/media/playback/progress` | RecordPlaybackProgress | Record media playback progress |

### User Routes
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/users/` | GetUsers | List all users |
| POST | `/api/users/` | CreateUser | Create a new user |
| POST | `/api/users/login` | LoginUser | Login a user |
| POST | `/api/users/logout` | LogoutUser | Logout a user |

## Plugin System

### Core Plugin Management
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/core-plugins/` | ListCorePlugins | List all core plugins |
| GET | `/api/core-plugins/:name` | GetCorePluginInfo | Get information about a specific core plugin |
| POST | `/api/core-plugins/:name/enable` | EnableCorePlugin | Enable a core plugin |
| POST | `/api/core-plugins/:name/disable` | DisableCorePlugin | Disable a core plugin |

### Plugin Routes
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| ANY | `/api/plugins/*path` | HandlePluginRoute | Handle all plugin-specific routes |

## Admin Routes

### Media Libraries
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/admin/media-libraries/` | GetMediaLibraries | List all media libraries |
| POST | `/api/admin/media-libraries/` | CreateMediaLibrary | Create a new media library |
| DELETE | `/api/admin/media-libraries/:id` | DeleteMediaLibrary | Delete a media library |
| GET | `/api/admin/media-libraries/:id/stats` | GetLibraryStats | Get statistics for a media library |
| GET | `/api/admin/media-libraries/:id/files` | GetMediaFiles | List files in a media library |

### Scanner Management
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/admin/scanner/stats` | GetScannerStats | Get scanner statistics |
| GET | `/api/admin/scanner/library-stats` | GetAllLibraryStats | Get statistics for all libraries |
| GET | `/api/admin/scanner/status` | GetScannerStatus | Get current scanner status |
| GET | `/api/admin/scanner/current-jobs` | GetCurrentJobs | List current scanner jobs |
| POST | `/api/admin/scanner/start/:id` | StartLibraryScanByID | Start scanning a media library by ID |
| POST | `/api/admin/scanner/pause/:id` | StopLibraryScan | Pause scanning a media library |
| POST | `/api/admin/scanner/stop/:id` | StopLibraryScan | Stop scanning a media library |
| POST | `/api/admin/scanner/resume/:id` | ResumeLibraryScan | Resume scanning a media library |
| POST | `/api/admin/scanner/cleanup-orphaned` | CleanupOrphanedJobs | Cleanup orphaned scanner jobs |
| POST | `/api/admin/scanner/cleanup-orphaned-assets` | CleanupOrphanedAssets | Cleanup orphaned assets |
| POST | `/api/admin/scanner/cleanup-orphaned-files` | CleanupOrphanedFiles | Cleanup orphaned files |
| DELETE | `/api/admin/scanner/jobs/:id` | DeleteScanJob | Delete a scan job |
| GET | `/api/admin/scanner/monitoring-status` | GetMonitoringStatus | Get file monitoring status |
| POST | `/api/admin/scanner/safe/start/:id` | StartSafeguardedLibraryScan | Start scanning with safeguards |
| POST | `/api/admin/scanner/safe/pause/:id` | PauseSafeguardedLibraryScan | Pause with safeguards |
| POST | `/api/admin/scanner/safe/resume/:id` | ResumeSafeguardedLibraryScan | Resume with safeguards |
| DELETE | `/api/admin/scanner/safe/jobs/:id` | DeleteSafeguardedLibraryScan | Delete with safeguards |
| GET | `/api/admin/scanner/safeguards/status` | GetSafeguardStatus | Get safeguard system status |
| POST | `/api/admin/scanner/emergency/cleanup` | ForceEmergencyCleanup | Emergency cleanup |
| POST | `/api/admin/scanner/force-complete/:id` | ForceCompleteScan | Manually mark scan as completed |
| POST | `/api/admin/scanner/throttle/disable/:jobId` | DisableThrottling | Disable adaptive throttling |
| POST | `/api/admin/scanner/throttle/enable/:jobId` | EnableThrottling | Re-enable adaptive throttling |
| GET | `/api/admin/scanner/throttle/status` | GetAdaptiveThrottleStatus | Get throttling status |
| POST | `/api/admin/scanner/throttle/config` | UpdateThrottleConfig | Update throttling configuration |
| GET | `/api/admin/scanner/throttle/performance/:jobId` | GetThrottlePerformanceHistory | Get throttling performance |
| GET | `/api/admin/scanner/health/:id` | GetScanHealth | Monitor scan health |

### Plugin Management
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/admin/plugins/` | GetPlugins | List all available plugins |
| GET | `/api/admin/plugins/:id` | GetPlugin | Get details for a specific plugin |
| GET | `/api/admin/plugins/:id/health` | GetPluginHealth | Get health status of a plugin |
| GET | `/api/admin/plugins/:id/events` | GetPluginEvents | Get events related to a plugin |
| GET | `/api/admin/plugins/events` | GetAllPluginEvents | Get events for all plugins |
| POST | `/api/admin/plugins/refresh` | RefreshPlugins | Refresh the list of available plugins |
| GET | `/api/admin/plugins/:id/manifest` | GetPluginManifest | Get manifest for a plugin |
| GET | `/api/admin/plugins/admin-pages` | GetPluginAdminPages | List admin pages provided by plugins |
| GET | `/api/admin/plugins/ui-components` | GetPluginUIComponents | List UI components provided by plugins |
| POST | `/api/admin/plugins/:id/enable` | EnablePlugin | Enable a plugin |
| POST | `/api/admin/plugins/:id/disable` | DisablePlugin | Disable a plugin |
| POST | `/api/admin/plugins/:id/install` | InstallPlugin | Install a plugin |
| DELETE | `/api/admin/plugins/:id` | UninstallPlugin | Uninstall a plugin |

## Module-Specific APIs

### Scanner Module (`/api/scanner`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/scanner/status` | getGeneralStatus | Get scanner general status |
| GET | `/api/scanner/config` | getConfig | Get scanner configuration |
| POST | `/api/scanner/config` | setConfig | Set scanner configuration |
| POST | `/api/scanner/scan` | startGeneralScan | Start a general scan |
| GET | `/api/scanner/jobs` | listScanJobs | List all scan jobs |
| POST | `/api/scanner/cancel-all` | cancelAllScans | Cancel all scans |
| GET | `/api/scanner/jobs/:id` | getScanStatus | Get status of a specific scan job |
| DELETE | `/api/scanner/jobs/:id` | cancelScan | Cancel a specific scan job |
| POST | `/api/scanner/resume/:id` | resumeScan | Resume a specific scan |
| GET | `/api/scanner/progress/:id` | getScanProgress | Get real-time scan progress |
| GET | `/api/scanner/monitoring` | getMonitoringStatus | Get file monitoring status |

### Database Module (`/api/database`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/database/health` | getHealth | Database health check |
| GET | `/api/database/status` | getStatus | Database module status |
| GET | `/api/database/stats` | getStats | Comprehensive database statistics |
| GET | `/api/database/connections` | getConnectionStats | Connection pool statistics |
| GET | `/api/database/connections/health` | getConnectionHealth | Connection health check |
| GET | `/api/database/migrations` | getMigrations | Get migration status |
| POST | `/api/database/migrations/execute` | executePendingMigrations | Execute pending migrations |
| POST | `/api/database/migrations/:id/rollback` | rollbackMigration | Rollback a migration |
| GET | `/api/database/models` | getRegisteredModels | Get registered models |
| GET | `/api/database/models/stats` | getModelStats | Get model statistics |
| POST | `/api/database/models/migrate` | autoMigrateModels | Auto-migrate models |
| GET | `/api/database/transactions/stats` | getTransactionStats | Transaction statistics |

### Playback Module (`/api/playback`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/playback/decide` | HandlePlaybackDecision | Make playback decision (direct play vs transcode) |
| POST | `/api/playback/validate` | HandleValidateMedia | Validate media file compatibility |
| GET | `/api/playback/stats` | HandleGetStats | Get playback statistics |
| GET | `/api/playback/health` | HandleHealthCheck | Playback health check |
| GET | `/api/playback/error-recovery/stats` | HandleErrorRecoveryStats | Get error recovery statistics |
| POST | `/api/playback/plugins/refresh` | HandleRefreshPlugins | Refresh plugins |

### Content-Addressable Storage API (`/api/v1/content`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/content/:hash/:file` | ServeContentFile | Serve content files (manifests, segments, etc.) |
| GET | `/api/v1/content/:hash/manifest.mpd` | ServeContentFile | Get DASH manifest |
| GET | `/api/v1/content/:hash/playlist.m3u8` | ServeContentFile | Get HLS playlist |
| GET | `/api/v1/content/:hash/info` | GetContentInfo | Get content metadata |
| GET | `/api/v1/content/by-media/:mediaId` | GetContentByMediaId | List content by media ID |
| GET | `/api/v1/content/stats` | GetContentStats | Get storage statistics |
| POST | `/api/v1/content/cleanup` | CleanupExpiredContent | Cleanup expired content |

**Note**: The content-addressable storage API replaced the deprecated `/api/playback/stream/` endpoints. All streaming content is now served through content hashes, enabling deduplication and CDN-friendly URLs.

### Asset Module (`/api/v1/assets`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/v1/assets/` | createAsset | Create a new asset |
| GET | `/api/v1/assets/:id` | getAsset | Get asset by ID |
| PUT | `/api/v1/assets/:id/preferred` | setPreferredAsset | Set as preferred asset |
| DELETE | `/api/v1/assets/:id` | deleteAsset | Delete an asset |
| GET | `/api/v1/assets/entity/:type/:id` | getAssetsByEntity | Get assets by entity |
| GET | `/api/v1/assets/entity/:type/:id/preferred/:asset_type` | getPreferredAsset | Get preferred asset |
| GET | `/api/v1/assets/entity/:type/:id/preferred/:asset_type/data` | getPreferredAssetData | Get preferred asset data |
| DELETE | `/api/v1/assets/entity/:type/:id` | deleteAssetsByEntity | Delete assets by entity |
| GET | `/api/v1/assets/:id/data` | getAssetData | Get asset binary data |
| GET | `/api/v1/assets/stats` | getAssetStats | Get asset statistics |
| POST | `/api/v1/assets/cleanup` | cleanupOrphanedFiles | Cleanup orphaned files |
| GET | `/api/v1/assets/types` | getValidTypes | Get valid asset types |
| GET | `/api/v1/assets/sources` | getValidSources | Get valid sources |
| GET | `/api/v1/assets/entity-types` | getEntityTypes | Get entity types |

### Events Module V1 (`/api/v1/events`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/events/` | GetEvents | Get events |
| GET | `/api/v1/events/range` | GetEventsByTimeRange | Get events by time range |
| GET | `/api/v1/events/types` | GetEventTypes | Get event types |
| POST | `/api/v1/events/` | PublishEvent | Publish event (admin) |
| DELETE | `/api/v1/events/:id` | DeleteEvent | Delete event |
| POST | `/api/v1/events/clear` | ClearEvents | Clear all events |
| GET | `/api/v1/events/health` | getEventHealth | Event health check |

### Enrichment Module (`/api/enrichment`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/enrichment/status/:mediaFileId` | GetEnrichmentStatusHandler | Get enrichment status |
| POST | `/api/enrichment/apply/:mediaFileId/:fieldName/:sourceName` | ForceApplyEnrichmentHandler | Apply enrichment |
| GET | `/api/enrichment/sources` | GetEnrichmentSourcesHandler | Get enrichment sources |
| PUT | `/api/enrichment/sources/:sourceName` | UpdateEnrichmentSourceHandler | Update enrichment source |
| GET | `/api/enrichment/jobs` | GetEnrichmentJobsHandler | Get enrichment jobs |
| POST | `/api/enrichment/jobs/:mediaFileId` | TriggerEnrichmentJobHandler | Trigger enrichment job |
| GET | `/api/enrichment/progress` | GetOverallProgressHandler | Get overall progress |
| GET | `/api/enrichment/progress/tv-shows` | GetTVShowProgressHandler | Get TV show progress |
| GET | `/api/enrichment/progress/movies` | GetMovieProgressHandler | Get movie progress |
| GET | `/api/enrichment/progress/music` | GetMusicProgressHandler | Get music progress |

### Plugin Module V1 (`/api/v1/plugins`)

#### Core Operations
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/` | handleListAllPlugins | List all plugins |
| GET | `/api/v1/plugins/search` | handleSearchPlugins | Search plugins |
| GET | `/api/v1/plugins/categories` | handleGetPluginCategories | Get plugin categories |
| GET | `/api/v1/plugins/capabilities` | handleGetSystemCapabilities | Get system capabilities |
| GET | `/api/v1/plugins/:id` | handleGetPlugin | Get plugin details |
| PUT | `/api/v1/plugins/:id` | handleUpdatePlugin | Update plugin |
| DELETE | `/api/v1/plugins/:id` | handleUninstallPlugin | Uninstall plugin |
| POST | `/api/v1/plugins/:id/enable` | handleEnablePlugin | Enable plugin |
| POST | `/api/v1/plugins/:id/disable` | handleDisablePlugin | Disable plugin |
| POST | `/api/v1/plugins/:id/restart` | handleRestartPlugin | Restart plugin |
| POST | `/api/v1/plugins/:id/reload` | handleReloadPlugin | Reload plugin |

#### Plugin Health & Monitoring
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/:id/health` | handleGetPluginHealth | Get plugin health |
| GET | `/api/v1/plugins/:id/metrics` | handleGetPluginMetrics | Get plugin metrics |
| GET | `/api/v1/plugins/:id/logs` | handleGetPluginLogs | Get plugin logs |
| POST | `/api/v1/plugins/:id/health/reset` | handleResetPluginHealth | Reset plugin health |

#### Plugin Configuration
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/:id/config` | handleGetPluginConfig | Get plugin config |
| PUT | `/api/v1/plugins/:id/config` | handleUpdatePluginConfig | Update plugin config |
| GET | `/api/v1/plugins/:id/config/schema` | handleGetPluginConfigSchema | Get config schema |
| POST | `/api/v1/plugins/:id/config/validate` | handleValidatePluginConfig | Validate config |
| POST | `/api/v1/plugins/:id/config/reset` | handleResetPluginConfig | Reset config |

#### Plugin Events & History
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/:id/events` | handleGetPluginEvents | Get plugin events |
| GET | `/api/v1/plugins/:id/history` | handleGetPluginHistory | Get plugin history |
| DELETE | `/api/v1/plugins/:id/events` | handleClearPluginEvents | Clear plugin events |

#### Plugin UI & Admin
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/:id/admin-pages` | handleGetPluginAdminPages | Get admin pages |
| GET | `/api/v1/plugins/:id/admin/:pageId/status` | handleGetAdminPageStatus | Get admin page status |
| POST | `/api/v1/plugins/:id/admin/:pageId/actions/:actionId` | handleExecuteAdminPageAction | Execute admin action |
| GET | `/api/v1/plugins/:id/ui-components` | handleGetPluginUIComponents | Get UI components |
| GET | `/api/v1/plugins/:id/assets` | handleGetPluginAssets | Get plugin assets |

#### Plugin Dependencies & Testing
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/:id/dependencies` | handleGetPluginDependencies | Get dependencies |
| GET | `/api/v1/plugins/:id/dependents` | handleGetPluginDependents | Get dependents |
| POST | `/api/v1/plugins/:id/validate-dependencies` | handleValidateDependencies | Validate dependencies |
| POST | `/api/v1/plugins/:id/test` | handleTestPlugin | Test plugin |
| GET | `/api/v1/plugins/:id/test-results` | handleGetTestResults | Get test results |
| POST | `/api/v1/plugins/:id/validate` | handleValidatePlugin | Validate plugin |

#### Core Plugins API (`/api/v1/plugins/core`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/core/` | handleListCorePlugins | List core plugins |
| GET | `/api/v1/plugins/core/:name` | handleGetCorePlugin | Get core plugin |
| POST | `/api/v1/plugins/core/:name/enable` | handleEnableCorePlugin | Enable core plugin |
| POST | `/api/v1/plugins/core/:name/disable` | handleDisableCorePlugin | Disable core plugin |
| GET | `/api/v1/plugins/core/:name/config` | handleGetCorePluginConfig | Get core plugin config |
| PUT | `/api/v1/plugins/core/:name/config` | handleUpdateCorePluginConfig | Update core plugin config |

#### External Plugins API (`/api/v1/plugins/external`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/external/` | handleListExternalPlugins | List external plugins |
| POST | `/api/v1/plugins/external/` | handleInstallPlugin | Install plugin |
| POST | `/api/v1/plugins/external/refresh` | handleRefreshExternalPlugins | Refresh external plugins |
| GET | `/api/v1/plugins/external/:id` | handleGetExternalPlugin | Get external plugin |
| POST | `/api/v1/plugins/external/:id/load` | handleLoadExternalPlugin | Load external plugin |
| POST | `/api/v1/plugins/external/:id/unload` | handleUnloadExternalPlugin | Unload external plugin |
| GET | `/api/v1/plugins/external/:id/manifest` | handleGetPluginManifest | Get plugin manifest |

#### Plugin System Management (`/api/v1/plugins/system`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/system/status` | handleGetSystemStatus | Get system status |
| GET | `/api/v1/plugins/system/stats` | handleGetSystemStats | Get system statistics |
| POST | `/api/v1/plugins/system/refresh` | handleRefreshAllPlugins | Refresh all plugins |
| POST | `/api/v1/plugins/system/cleanup` | handleCleanupSystem | Cleanup system |
| GET | `/api/v1/plugins/system/hot-reload` | handleGetHotReloadStatus | Get hot reload status |
| POST | `/api/v1/plugins/system/hot-reload/enable` | handleEnableHotReload | Enable hot reload |
| POST | `/api/v1/plugins/system/hot-reload/disable` | handleDisableHotReload | Disable hot reload |
| POST | `/api/v1/plugins/system/hot-reload/trigger/:id` | handleTriggerHotReload | Trigger hot reload |
| POST | `/api/v1/plugins/system/bulk/enable` | handleBulkEnable | Bulk enable plugins |
| POST | `/api/v1/plugins/system/bulk/disable` | handleBulkDisable | Bulk disable plugins |
| POST | `/api/v1/plugins/system/bulk/update` | handleBulkUpdate | Bulk update plugins |

#### Plugin Admin Integration (`/api/v1/plugins/admin`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/v1/plugins/admin/pages` | handleGetAllAdminPages | Get all admin pages |
| GET | `/api/v1/plugins/admin/navigation` | handleGetAdminNavigation | Get admin navigation |
| GET | `/api/v1/plugins/admin/permissions` | handleGetPluginPermissions | Get plugin permissions |
| PUT | `/api/v1/plugins/admin/permissions` | handleUpdatePluginPermissions | Update permissions |
| GET | `/api/v1/plugins/admin/settings` | handleGetGlobalPluginSettings | Get global settings |
| PUT | `/api/v1/plugins/admin/settings` | handleUpdateGlobalPluginSettings | Update global settings |

### Transcoding Module (`/api/v1/transcoding`)
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/v1/transcoding/transcode` | StartTranscode | Start a new transcoding session |
| DELETE | `/api/v1/transcoding/transcode/:sessionId` | StopTranscode | Stop a transcoding session |
| GET | `/api/v1/transcoding/progress/:sessionId` | GetProgress | Get real-time transcoding progress |
| GET | `/api/v1/transcoding/sessions` | ListSessions | List all transcoding sessions |
| GET | `/api/v1/transcoding/sessions/:sessionId` | GetSession | Get detailed session information |
| GET | `/api/v1/transcoding/providers` | ListProviders | List available transcoding providers |
| GET | `/api/v1/transcoding/providers/:providerId` | GetProvider | Get provider details |
| GET | `/api/v1/transcoding/providers/:providerId/formats` | GetProviderFormats | Get supported formats for provider |
| GET | `/api/v1/transcoding/pipeline/status` | GetPipelineStatus | Get pipeline provider status |

### Scan Routes
| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/scan/start` | StartDirectoryScan | Start a new directory scan |
| GET | `/api/scan/:id/progress` | GetScanProgress | Get progress of a specific scan job |
| POST | `/api/scan/:id/stop` | StopScan | Stop a specific scan job |
| POST | `/api/scan/:id/resume` | ResumeScan | Resume a specific scan job |
| GET | `/api/scan/:id/results` | GetScanResults | Get results of a completed scan job |
| DELETE | `/api/scan/:id` | DeleteScan | Delete a scan job |
| POST | `/api/scan/:id/pause` | PauseScan | Pause a specific scan job |
| GET | `/api/scan/:id/details` | GetScanDetails | Get detailed information about a scan job |
| POST | `/api/scan/library/:id` | StartLibraryScan | Start scanning a library |
| GET | `/api/library/:id/trickplay-analysis` | AnalyzeTrickplayContent | Analyze trickplay content |
| POST | `/api/library/:id/cleanup` | CleanupLibraryData | Cleanup library data |

## Static Assets
- `/plugins/*` - Static plugin assets served from the plugins directory

## Notes

1. **Authentication**: Most routes require authentication. Check the middleware configuration for specific requirements.
2. **API Versioning**: Some modules use versioned APIs (e.g., `/api/v1/`). Always use the latest version unless compatibility requires otherwise.
3. **Plugin Routes**: The `/api/plugins/*path` route handles all plugin-specific routes dynamically.
4. **WebSocket/SSE**: Some routes like `/api/events/stream` use Server-Sent Events for real-time data.
5. **Development Routes**: Routes under `/api/dev/` are only available in development mode.

## Response Formats

All API endpoints return JSON responses with the following general structure:

### Success Response
```json
{
  "data": {}, // Response data
  "message": "Operation successful",
  "status": "success"
}
```

### Error Response
```json
{
  "error": "Error message",
  "details": {}, // Optional error details
  "status": "error"
}
```

## Rate Limiting

The API implements rate limiting on certain endpoints. Check response headers for rate limit information:
- `X-RateLimit-Limit`: Maximum requests per window
- `X-RateLimit-Remaining`: Remaining requests in current window
- `X-RateLimit-Reset`: Time when the rate limit window resets