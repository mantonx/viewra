# Plugin Capability Foundation Plan

## Goal

Build the foundation required for all newly added plugin capabilities and display categories. Create a consistent, extensible architecture that enables future plugin development across notifications, sync, analytics, auth, monitoring, media management, and content domains.

## Current State (Problem)

### Existing Capabilities (Complete End-to-End)

| Capability | Proto | SDK | Manager | Host | % |
|------------|-------|-----|---------|------|---|
| Enricher | ✓ | ✓ | ✓ | ✓ | 95% |
| Provider (AI) | ✓ | ✓ | ✓ | ✓ | 95% |
| Vector Search | ✓ | ✓ | ✓ | ✓ | 90% |
| Trending | ✓ | ✓ | ✓ | ✓ | 90% |

### Partially Implemented Capabilities

| Capability | Proto | SDK | Manager | Host | Gap |
|------------|-------|-----|---------|------|-----|
| Search Provider | ✓ | ⚠️ | ⚠️ | ✗ | No host dispatch logic |
| Widget | ⚠️ | ✓ | ⚠️ | ⚠️ | No widget aggregation |
| User Metadata | ✓ | ✗ | ✓ | ✗ | No SDK wrappers or host server |
| File Parser | ✓ | ✗ | — | ✗ | No SDK wrappers or host server |

### Missing Capabilities (Declared but No Infrastructure)

- **Notifications**: notification_sink, webhook_sender, webhook_receiver
- **Sync**: watch_history, scrobble, list_sync, calendar
- **Analytics**: statistics, reports
- **Playback**: playback_reporting, skip_intro, skip_credits
- **Storage**: backup, storage
- **Auth**: auth_provider, user_sync
- **Monitoring**: metrics, tracing (health_check exists in plugin_core)
- **Transcoding**: transcode
- **Download**: download_client
- **Media Management**: media_requests, library_sync, collection_sync
- **Content**: subtitles, lyrics

### Infrastructure Gaps

1. **Plugin Event Delivery** - Event bus exists but no mechanism to deliver events to plugins via OnEvent()
2. **Capability Routing** - InvokeCapability() dispatch logic incomplete
3. **Dependency Resolution** - No startup ordering by capability requirements
4. **Error Handling** - No retry middleware for retryable errors
5. **Request Tracing** - Request IDs not propagated through plugin calls
6. **Capability Preferences** - No persistence or enforcement
7. **Version Negotiation** - No compatibility checking

## Target State

### Proto Layer
- 11 new proto files for capability domains
- Event types for subscription system
- Consistent request/response patterns

### SDK Layer
- Interface + ServeXxx() helper for each capability
- Host service clients for all host services (including missing ones)
- Error handling middleware with retry support
- Request tracing propagation

### Manager Layer
- Capability getters for all types
- Event bus with subscription management
- Dependency-ordered plugin startup
- Capability preference persistence

### Host Services Layer
- Complete implementations for HostUserMetadata, HostFileParser
- New services: HostPlayback, HostNotification, HostAuth, HostScheduler, HostUsers
- Search provider dispatch
- Widget aggregation

### Application Layer
- Use cases for notification dispatch, sync coordination, external auth

### API Layer
- Webhook endpoints
- Notification management endpoints

## Migration Steps

### Phase 1: Infrastructure Foundations

Address critical gaps that affect all capabilities.

#### 1.1 Plugin Event Delivery

The event bus already exists at `internal/infrastructure/events/bus/` with full pub/sub, filtering, and replay support. Event types are defined in `internal/domain/events/event.go` including playback, user, media, scan, and system events.

**Gap:** Plugins can subscribe to events via `GetSubscriptions()` but there's no mechanism to deliver events to plugins via `OnEvent()`.

**Files to MODIFY:**

1. `internal/infrastructure/plugins/manager/manager.go`
   - Subscribe to the event bus on startup
   - Fan out events to plugins that subscribed via GetSubscriptions()
   - Call `plugin.CoreClient.OnEvent()` for matching events

   ```go
   func (m *Manager) setupEventDelivery(eventBus *bus.Bus) {
       // Subscribe to all events
       sub := eventBus.Subscribe(events.WithBufferSize(500))

       go func() {
           for event := range sub.Events() {
               m.deliverEventToPlugins(event)
           }
       }()
   }

   func (m *Manager) deliverEventToPlugins(event events.Event) {
       m.mu.RLock()
       defer m.mu.RUnlock()

       for _, plugin := range m.plugins {
           // Check if plugin subscribed to this event type
           if !plugin.SubscribesTo(string(event.Type)) {
               continue
           }

           // Convert domain event to proto event
           protoEvent := &pluginv1.Event{
               Type:          string(event.Type),
               Source:        event.Source,
               TimestampUnix: event.Timestamp.Unix(),
               Payload:       marshalEventData(event.Data),
               CorrelationId: event.RequestID,
           }

           // Non-blocking delivery with timeout
           ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
           _, _ = plugin.CoreClient.OnEvent(ctx, protoEvent)
           cancel()
       }
   }
   ```

2. `internal/infrastructure/plugins/types/types.go`
   - Add `Subscriptions []string` field to Instance
   - Add `SubscribesTo(eventType string) bool` method

3. `internal/domain/events/event.go`
   - Add new event types for capabilities that need them:

   ```go
   // Media request events (for media_requests capability)
   EventMediaRequestCreated  EventType = "media.request.created"
   EventMediaRequestApproved EventType = "media.request.approved"
   EventMediaRequestDenied   EventType = "media.request.denied"
   EventMediaRequestFulfilled EventType = "media.request.fulfilled"

   // Library sync events (for library_sync, collection_sync)
   EventLibrarySyncStarted   EventType = "library.sync.started"
   EventLibrarySyncCompleted EventType = "library.sync.completed"
   EventLibrarySyncFailed    EventType = "library.sync.failed"

   // Backup events (for backup capability)
   EventBackupStarted   EventType = "backup.started"
   EventBackupCompleted EventType = "backup.completed"
   EventBackupFailed    EventType = "backup.failed"
   ```

#### 1.2 Capability Routing Completion

**Files to MODIFY:**

1. `internal/infrastructure/plugins/host/plugins.go`
   - Complete `InvokeCapability()` dispatch logic:
     ```go
     func (s *HostPluginsServer) InvokeCapability(ctx context.Context, req *pluginv1.CapabilityInvokeRequest) {
         // 1. Check capability preference for this capability
         // 2. Resolve provider (preferred or first available)
         // 3. Route to ProviderClient.Invoke()
         // 4. Handle retryable errors
     }
     ```

2. `internal/infrastructure/plugins/registry/capability.go`
   - Add preference lookup
   - Add capability→provider resolution with fallback

#### 1.3 Error Handling Middleware

**Files to CREATE:**

1. `pkg/plugin/sdk/middleware/retry.go`
   ```go
   type RetryConfig struct {
       MaxAttempts int
       BackoffBase time.Duration
       BackoffMax  time.Duration
   }

   func WithRetry(config RetryConfig) grpc.UnaryClientInterceptor
   func IsRetryable(err error) bool
   ```

**Files to MODIFY:**

1. `pkg/plugin/sdk/host_services.go`
   - Add retry interceptor to host service clients

#### 1.4 Request Tracing

**Files to MODIFY:**

1. `internal/infrastructure/plugins/manager/manager.go`
   - Generate request ID for each plugin call
   - Pass via context metadata

2. `pkg/plugin/sdk/base.go`
   - Extract request ID from incoming context
   - Propagate to outgoing host service calls

3. `internal/infrastructure/plugins/host/*.go`
   - Log with request ID correlation

#### 1.5 Capability Preferences Persistence

**Files to CREATE:**

1. `migrations/XXXXXX_add_capability_preferences.up.sql`
   ```sql
   CREATE TABLE capability_preferences (
       capability TEXT NOT NULL,
       preferred_plugin TEXT NOT NULL,
       user_id TEXT,  -- NULL = global default
       created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
       PRIMARY KEY (capability, COALESCE(user_id, ''))
   );
   ```

**Files to MODIFY:**

1. `internal/infrastructure/plugins/registry/capability.go`
   - Load preferences on startup
   - Persist on SetCapabilityPreference()

2. `internal/infrastructure/plugins/host/plugins.go`
   - Apply preferences in capability resolution

#### 1.6 Dependency Resolution

**Files to MODIFY:**

1. `internal/infrastructure/plugins/manager/dependencies.go`
   - Add `resolveStartupOrder()` based on `Requires` field
   - Skip plugins with unsatisfied requirements (log warning)
   - Re-check after each plugin starts (capability now available?)

2. `internal/infrastructure/plugins/manager/manager.go`
   - Use startup order from dependency resolver
   - Health check: warn if required capability unavailable

### Phase 2: Complete Partial Implementations

#### 2.1 Search Provider Execution

**Files to CREATE:**

1. `internal/infrastructure/plugins/host/search_provider.go`
   ```go
   type SearchProviderDispatcher struct {
       registry *registry.SearchProviderRegistry
       manager  *manager.Manager
   }

   func (d *SearchProviderDispatcher) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
       // 1. Resolve search provider from registry
       // 2. Get gRPC client from manager
       // 3. Call SearchProviderClient.Search()
   }
   ```

**Files to MODIFY:**

1. `internal/infrastructure/plugins/grpc/search_provider.go`
   - Complete client wrapper implementation

2. `internal/api/handlers/search.go`
   - Use SearchProviderDispatcher when search_provider capability available

#### 2.2 Widget Aggregation

**Files to CREATE:**

1. `internal/infrastructure/plugins/host/widgets.go`
   ```go
   type WidgetAggregator struct {
       registry *registry.WidgetRegistry
       manager  *manager.Manager
   }

   func (a *WidgetAggregator) GetAllWidgets(ctx context.Context) ([]*Widget, error) {
       // 1. List all registered widget plugins
       // 2. Call each plugin's widget endpoint via HTTP proxy
       // 3. Merge responses
   }
   ```

**Files to MODIFY:**

1. `internal/api/handlers/widgets.go`
   - Use WidgetAggregator for unified widget response

#### 2.3 HostUserMetadata Implementation

**Files to CREATE:**

1. `internal/infrastructure/plugins/host/user_metadata.go`
   ```go
   type HostUserMetadataServer struct {
       repo UserMetadataRepository
   }

   func (s *HostUserMetadataServer) Get(ctx, req) (*pluginv1.UserMetadataResponse, error)
   func (s *HostUserMetadataServer) Set(ctx, req) (*pluginv1.UserMetadataResponse, error)
   func (s *HostUserMetadataServer) Delete(ctx, req) (*pluginv1.UserMetadataResponse, error)
   ```

2. `pkg/plugin/sdk/user_metadata.go`
   ```go
   type UserMetadataClient struct {
       client pluginv1.HostUserMetadataClient
   }

   func (c *UserMetadataClient) Get(ctx, key string) ([]byte, error)
   func (c *UserMetadataClient) Set(ctx, key string, value []byte) error
   ```

**Files to MODIFY:**

1. `internal/infrastructure/plugins/grpc/plugin.go`
   - Add HostUserMetadataPlugin to dispense map

2. `pkg/plugin/sdk/base.go`
   - Add UserMetadata field to HostServices

#### 2.4 HostFileParser Implementation

**Files to CREATE:**

1. `internal/infrastructure/plugins/host/file_parser.go`
   - Implement NFO parsing, image extraction services

2. `pkg/plugin/sdk/file_parser.go`
   - SDK client wrapper

### Phase 3: Proto Definitions (New Capabilities)

Create all proto files at once for API stability.

**Files to CREATE:**

1. `api/proto/plugin/notification.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   // Plugin implements this to receive notifications
   service NotificationSink {
     rpc SendNotification(SendNotificationRequest) returns (SendNotificationResponse);
     rpc GetChannels(GetChannelsRequest) returns (GetChannelsResponse);
     rpc TestChannel(TestChannelRequest) returns (TestChannelResponse);
   }

   // Plugin implements this to send webhooks on events
   service WebhookSender {
     rpc Configure(WebhookSenderConfigRequest) returns (WebhookSenderConfigResponse);
     rpc GetSubscribedEvents(GetSubscribedEventsRequest) returns (GetSubscribedEventsResponse);
   }

   // Plugin implements this to receive incoming webhooks
   service WebhookReceiver {
     rpc HandleWebhook(HandleWebhookRequest) returns (HandleWebhookResponse);
     rpc GetEndpointInfo(GetEndpointInfoRequest) returns (GetEndpointInfoResponse);
   }

   message SendNotificationRequest {
     string channel_id = 1;
     NotificationMessage message = 2;
     NotificationPriority priority = 3;
   }

   message NotificationMessage {
     string title = 1;
     string body = 2;
     string url = 3;
     map<string, string> metadata = 4;
     repeated NotificationAction actions = 5;
   }

   message NotificationAction {
     string id = 1;
     string label = 2;
     string url = 3;
   }

   enum NotificationPriority {
     NORMAL = 0;
     LOW = 1;
     HIGH = 2;
     URGENT = 3;
   }

   message NotificationChannel {
     string id = 1;
     string name = 2;
     string type = 3;  // discord, slack, email, pushover, etc.
     bool enabled = 4;
   }

   message HandleWebhookRequest {
     string method = 1;
     string path = 2;
     map<string, string> headers = 3;
     bytes body = 4;
     string source_ip = 5;
   }

   message HandleWebhookResponse {
     int32 status_code = 1;
     map<string, string> headers = 2;
     bytes body = 3;
   }
   ```

2. `api/proto/plugin/sync.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service WatchHistorySync {
     rpc SyncToExternal(SyncToExternalRequest) returns (SyncToExternalResponse);
     rpc SyncFromExternal(SyncFromExternalRequest) returns (SyncFromExternalResponse);
     rpc GetSyncStatus(GetSyncStatusRequest) returns (GetSyncStatusResponse);
   }

   service Scrobbler {
     rpc NowPlaying(NowPlayingRequest) returns (NowPlayingResponse);
     rpc Scrobble(ScrobbleRequest) returns (ScrobbleResponse);
     rpc GetScrobbleHistory(GetScrobbleHistoryRequest) returns (GetScrobbleHistoryResponse);
   }

   service ListSync {
     rpc SyncPlaylist(SyncPlaylistRequest) returns (SyncPlaylistResponse);
     rpc SyncCollection(SyncCollectionRequest) returns (SyncCollectionResponse);
     rpc GetSyncedLists(GetSyncedListsRequest) returns (GetSyncedListsResponse);
   }

   service CalendarProvider {
     rpc GetUpcomingReleases(GetUpcomingReleasesRequest) returns (GetUpcomingReleasesResponse);
     rpc GetCalendarFeed(GetCalendarFeedRequest) returns (GetCalendarFeedResponse);
   }

   message WatchHistoryEntry {
     string media_id = 1;
     string external_id = 2;
     MediaType media_type = 3;
     int64 watched_at = 4;
     float progress = 5;  // 0.0 - 1.0
     int32 play_count = 6;
   }

   message NowPlayingRequest {
     string media_id = 1;
     MediaIdentifiers identifiers = 2;
     float progress = 3;
     int32 duration_seconds = 4;
   }

   message ScrobbleRequest {
     string media_id = 1;
     MediaIdentifiers identifiers = 2;
     int64 started_at = 3;
     int64 ended_at = 4;
     float completion = 5;  // 0.0 - 1.0
   }

   message MediaIdentifiers {
     string imdb_id = 1;
     string tmdb_id = 2;
     string tvdb_id = 3;
     string musicbrainz_id = 4;
     int32 season = 5;
     int32 episode = 6;
   }

   message CalendarEntry {
     string title = 1;
     MediaType media_type = 2;
     int64 release_date = 3;
     string poster_url = 4;
     MediaIdentifiers identifiers = 5;
   }
   ```

3. `api/proto/plugin/analytics.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service StatisticsProvider {
     rpc GetViewingStats(GetViewingStatsRequest) returns (GetViewingStatsResponse);
     rpc GetUserStats(GetUserStatsRequest) returns (GetUserStatsResponse);
     rpc GetLibraryStats(GetLibraryStatsRequest) returns (GetLibraryStatsResponse);
   }

   service ReportGenerator {
     rpc GenerateReport(GenerateReportRequest) returns (GenerateReportResponse);
     rpc GetReportTemplates(GetReportTemplatesRequest) returns (GetReportTemplatesResponse);
     rpc ScheduleReport(ScheduleReportRequest) returns (ScheduleReportResponse);
   }

   message ViewingStats {
     int64 total_watch_time_seconds = 1;
     int32 total_plays = 2;
     int32 unique_items = 3;
     repeated GenreBreakdown by_genre = 4;
     repeated TimeBreakdown by_time = 5;
   }

   message GenreBreakdown {
     string genre = 1;
     int64 watch_time_seconds = 2;
     int32 play_count = 3;
   }

   message TimeBreakdown {
     string period = 1;  // "2024-01", "2024-W01", "Monday"
     int64 watch_time_seconds = 2;
     int32 play_count = 3;
   }

   message ReportTemplate {
     string id = 1;
     string name = 2;
     string description = 3;
     repeated ReportParameter parameters = 4;
   }

   message ReportParameter {
     string name = 1;
     string type = 2;  // date_range, user_ids, media_types
     bool required = 3;
   }
   ```

4. `api/proto/plugin/playback.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service PlaybackReporter {
     rpc ReportStart(PlaybackStartRequest) returns (PlaybackResponse);
     rpc ReportProgress(PlaybackProgressRequest) returns (PlaybackResponse);
     rpc ReportStop(PlaybackStopRequest) returns (PlaybackResponse);
     rpc ReportPause(PlaybackPauseRequest) returns (PlaybackResponse);
   }

   service SkipDetector {
     rpc GetIntroTimestamps(GetTimestampsRequest) returns (TimestampsResponse);
     rpc GetCreditsTimestamp(GetTimestampsRequest) returns (TimestampsResponse);
     rpc SubmitTimestamps(SubmitTimestampsRequest) returns (SubmitTimestampsResponse);
     rpc GetDetectionStatus(GetDetectionStatusRequest) returns (DetectionStatusResponse);
   }

   message PlaybackStartRequest {
     string session_id = 1;
     string user_id = 2;
     string media_id = 3;
     string device_id = 4;
     string client_name = 5;
     PlaybackQuality quality = 6;
   }

   message PlaybackProgressRequest {
     string session_id = 1;
     int64 position_seconds = 2;
     bool is_paused = 3;
     PlaybackQuality quality = 4;
   }

   message PlaybackQuality {
     string video_codec = 1;
     string audio_codec = 2;
     int32 bitrate = 3;
     string resolution = 4;
     bool is_transcoded = 5;
   }

   message TimestampsResponse {
     int64 start_seconds = 1;
     int64 end_seconds = 2;
     float confidence = 3;  // 0.0 - 1.0
     string source = 4;     // "detected", "community", "manual"
   }
   ```

5. `api/proto/plugin/backup.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service BackupProvider {
     rpc CreateBackup(CreateBackupRequest) returns (CreateBackupResponse);
     rpc RestoreBackup(RestoreBackupRequest) returns (RestoreBackupResponse);
     rpc ListBackups(ListBackupsRequest) returns (ListBackupsResponse);
     rpc DeleteBackup(DeleteBackupRequest) returns (DeleteBackupResponse);
     rpc GetBackupStatus(GetBackupStatusRequest) returns (GetBackupStatusResponse);
   }

   service ExternalStorage {
     rpc Upload(stream UploadRequest) returns (UploadResponse);
     rpc Download(DownloadRequest) returns (stream DownloadResponse);
     rpc List(ListStorageRequest) returns (ListStorageResponse);
     rpc Delete(DeleteStorageRequest) returns (DeleteStorageResponse);
     rpc GetQuota(GetQuotaRequest) returns (GetQuotaResponse);
   }

   message BackupInfo {
     string id = 1;
     string name = 2;
     int64 created_at = 3;
     int64 size_bytes = 4;
     BackupType type = 5;
     BackupStatus status = 6;
     map<string, string> metadata = 7;
   }

   enum BackupType {
     FULL = 0;
     INCREMENTAL = 1;
     DIFFERENTIAL = 2;
   }

   enum BackupStatus {
     PENDING = 0;
     IN_PROGRESS = 1;
     COMPLETED = 2;
     FAILED = 3;
   }
   ```

6. `api/proto/plugin/auth.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service ExternalAuthProvider {
     rpc GetAuthInfo(GetAuthInfoRequest) returns (GetAuthInfoResponse);
     rpc Authenticate(AuthenticateRequest) returns (AuthenticateResponse);
     rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse);
     rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
     rpc Logout(LogoutRequest) returns (LogoutResponse);
   }

   service UserSyncProvider {
     rpc SyncUsers(SyncUsersRequest) returns (SyncUsersResponse);
     rpc SyncGroups(SyncGroupsRequest) returns (SyncGroupsResponse);
     rpc GetExternalUsers(GetExternalUsersRequest) returns (GetExternalUsersResponse);
     rpc GetExternalGroups(GetExternalGroupsRequest) returns (GetExternalGroupsResponse);
     rpc MapUser(MapUserRequest) returns (MapUserResponse);
   }

   message GetAuthInfoResponse {
     string provider_name = 1;
     AuthType auth_type = 2;
     string authorization_url = 3;  // For OAuth
     repeated string scopes = 4;
   }

   enum AuthType {
     OAUTH2 = 0;
     OIDC = 1;
     LDAP = 2;
     SAML = 3;
     API_KEY = 4;
   }

   message AuthenticateRequest {
     oneof credentials {
       OAuthCallback oauth = 1;
       LDAPCredentials ldap = 2;
       string api_key = 3;
     }
   }

   message OAuthCallback {
     string code = 1;
     string state = 2;
     string redirect_uri = 3;
   }

   message LDAPCredentials {
     string username = 1;
     string password = 2;
   }

   message AuthenticateResponse {
     bool success = 1;
     string user_id = 2;
     string display_name = 3;
     string email = 4;
     repeated string groups = 5;
     string access_token = 6;
     string refresh_token = 7;
     int64 expires_at = 8;
   }

   message ExternalUser {
     string external_id = 1;
     string username = 2;
     string display_name = 3;
     string email = 4;
     repeated string groups = 5;
     bool enabled = 6;
     map<string, string> attributes = 7;
   }
   ```

7. `api/proto/plugin/monitoring.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service MetricsExporter {
     rpc GetMetrics(GetMetricsRequest) returns (GetMetricsResponse);
     rpc GetMetricDefinitions(GetMetricDefinitionsRequest) returns (GetMetricDefinitionsResponse);
     rpc PushMetrics(PushMetricsRequest) returns (PushMetricsResponse);
   }

   service TracingExporter {
     rpc ExportSpans(ExportSpansRequest) returns (ExportSpansResponse);
     rpc GetTracingConfig(GetTracingConfigRequest) returns (GetTracingConfigResponse);
   }

   message MetricDefinition {
     string name = 1;
     MetricType type = 2;
     string description = 3;
     repeated string labels = 4;
     string unit = 5;
   }

   enum MetricType {
     COUNTER = 0;
     GAUGE = 1;
     HISTOGRAM = 2;
     SUMMARY = 3;
   }

   message Metric {
     string name = 1;
     map<string, string> labels = 2;
     double value = 3;
     int64 timestamp = 4;
   }

   message Span {
     string trace_id = 1;
     string span_id = 2;
     string parent_span_id = 3;
     string operation_name = 4;
     int64 start_time = 5;
     int64 end_time = 6;
     map<string, string> tags = 7;
     repeated SpanLog logs = 8;
   }

   message SpanLog {
     int64 timestamp = 1;
     map<string, string> fields = 2;
   }
   ```

8. `api/proto/plugin/transcode.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service TranscodeProvider {
     rpc GetProfiles(GetProfilesRequest) returns (GetProfilesResponse);
     rpc GetHardwareCapabilities(GetHardwareCapabilitiesRequest) returns (GetHardwareCapabilitiesResponse);
     rpc RequestTranscode(TranscodeRequest) returns (TranscodeResponse);
     rpc GetTranscodeStatus(GetTranscodeStatusRequest) returns (GetTranscodeStatusResponse);
     rpc CancelTranscode(CancelTranscodeRequest) returns (CancelTranscodeResponse);
   }

   message TranscodeProfile {
     string id = 1;
     string name = 2;
     string description = 3;
     VideoSettings video = 4;
     AudioSettings audio = 5;
     ContainerFormat container = 6;
   }

   message VideoSettings {
     string codec = 1;
     int32 bitrate = 2;
     string resolution = 3;
     int32 framerate = 4;
     string preset = 5;
     bool hardware_accelerated = 6;
   }

   message AudioSettings {
     string codec = 1;
     int32 bitrate = 2;
     int32 channels = 3;
     int32 sample_rate = 4;
   }

   message HardwareCapabilities {
     repeated string encoders = 1;
     repeated string decoders = 2;
     string gpu_vendor = 3;
     string gpu_model = 4;
     int64 vram_bytes = 5;
   }
   ```

9. `api/proto/plugin/download.proto`
   ```protobuf
   syntax = "proto3";
   package plugin.v1;

   service DownloadClient {
     rpc AddDownload(AddDownloadRequest) returns (AddDownloadResponse);
     rpc GetDownloads(GetDownloadsRequest) returns (GetDownloadsResponse);
     rpc GetDownloadStatus(GetDownloadStatusRequest) returns (GetDownloadStatusResponse);
     rpc PauseDownload(PauseDownloadRequest) returns (PauseDownloadResponse);
     rpc ResumeDownload(ResumeDownloadRequest) returns (ResumeDownloadResponse);
     rpc RemoveDownload(RemoveDownloadRequest) returns (RemoveDownloadResponse);
     rpc GetClientInfo(GetClientInfoRequest) returns (GetClientInfoResponse);
   }

   message AddDownloadRequest {
     oneof source {
       string magnet_uri = 1;
       string torrent_url = 2;
       bytes torrent_file = 3;
       string nzb_url = 4;
       bytes nzb_file = 5;
       string direct_url = 6;
     }
     string category = 7;
     string save_path = 8;
     map<string, string> tags = 9;
   }

   message DownloadInfo {
     string id = 1;
     string name = 2;
     DownloadStatus status = 3;
     int64 size_bytes = 4;
     int64 downloaded_bytes = 5;
     float progress = 6;
     int64 download_speed = 7;
     int64 upload_speed = 8;
     int32 seeds = 9;
     int32 peers = 10;
     int64 eta_seconds = 11;
     string save_path = 12;
   }

   enum DownloadStatus {
     QUEUED = 0;
     DOWNLOADING = 1;
     PAUSED = 2;
     COMPLETED = 3;
     ERROR = 4;
     SEEDING = 5;
   }

   message ClientInfo {
     string name = 1;
     string version = 2;
     ClientType type = 3;
     bool connected = 4;
     int64 free_space_bytes = 5;
   }

   enum ClientType {
     TORRENT = 0;
     USENET = 1;
     DIRECT = 2;
   }
   ```

10. `api/proto/plugin/media_management.proto`
    ```protobuf
    syntax = "proto3";
    package plugin.v1;

    service MediaRequestHandler {
      rpc CreateRequest(CreateMediaRequestRequest) returns (CreateMediaRequestResponse);
      rpc GetRequests(GetMediaRequestsRequest) returns (GetMediaRequestsResponse);
      rpc ApproveRequest(ApproveMediaRequestRequest) returns (ApproveMediaRequestResponse);
      rpc DenyRequest(DenyMediaRequestRequest) returns (DenyMediaRequestResponse);
      rpc GetRequestStatus(GetRequestStatusRequest) returns (GetRequestStatusResponse);
    }

    service LibrarySyncProvider {
      rpc SyncLibrary(SyncLibraryRequest) returns (SyncLibraryResponse);
      rpc GetSyncStatus(GetLibrarySyncStatusRequest) returns (GetLibrarySyncStatusResponse);
      rpc GetExternalLibrary(GetExternalLibraryRequest) returns (GetExternalLibraryResponse);
      rpc MapItem(MapItemRequest) returns (MapItemResponse);
    }

    service CollectionSyncProvider {
      rpc SyncCollection(SyncCollectionRequest) returns (SyncCollectionResponse);
      rpc GetCollectionMapping(GetCollectionMappingRequest) returns (GetCollectionMappingResponse);
      rpc CreateExternalCollection(CreateExternalCollectionRequest) returns (CreateExternalCollectionResponse);
    }

    message MediaRequest {
      string id = 1;
      string title = 2;
      MediaType media_type = 3;
      MediaIdentifiers identifiers = 4;
      string requested_by = 5;
      int64 requested_at = 6;
      RequestStatus status = 7;
      string approved_by = 8;
      int64 approved_at = 9;
      string deny_reason = 10;
    }

    enum RequestStatus {
      PENDING = 0;
      APPROVED = 1;
      DENIED = 2;
      AVAILABLE = 3;
    }

    message ExternalLibraryItem {
      string external_id = 1;
      string title = 2;
      MediaType media_type = 3;
      string path = 4;
      QualityProfile quality = 5;
      bool monitored = 6;
    }

    message QualityProfile {
      string id = 1;
      string name = 2;
    }
    ```

11. `api/proto/plugin/content.proto`
    ```protobuf
    syntax = "proto3";
    package plugin.v1;

    service SubtitleProvider {
      rpc SearchSubtitles(SearchSubtitlesRequest) returns (SearchSubtitlesResponse);
      rpc DownloadSubtitle(DownloadSubtitleRequest) returns (DownloadSubtitleResponse);
      rpc GetLanguages(GetLanguagesRequest) returns (GetLanguagesResponse);
      rpc GetProviderInfo(GetProviderInfoRequest) returns (GetProviderInfoResponse);
    }

    service LyricsProvider {
      rpc SearchLyrics(SearchLyricsRequest) returns (SearchLyricsResponse);
      rpc GetLyrics(GetLyricsRequest) returns (GetLyricsResponse);
      rpc GetSyncedLyrics(GetSyncedLyricsRequest) returns (GetSyncedLyricsResponse);
    }

    message SearchSubtitlesRequest {
      string query = 1;
      MediaIdentifiers identifiers = 2;
      string language = 3;
      string file_hash = 4;
      int64 file_size = 5;
      int32 season = 6;
      int32 episode = 7;
    }

    message SubtitleResult {
      string id = 1;
      string provider = 2;
      string language = 3;
      string format = 4;  // srt, ass, vtt
      float score = 5;
      bool hearing_impaired = 6;
      bool foreign_parts_only = 7;
      string release_name = 8;
      int32 download_count = 9;
    }

    message DownloadSubtitleResponse {
      bytes content = 1;
      string format = 2;
      string encoding = 3;
    }

    message LyricsResult {
      string id = 1;
      string provider = 2;
      string track_name = 3;
      string artist_name = 4;
      bool is_synced = 5;
      float score = 6;
    }

    message SyncedLyrics {
      repeated LyricLine lines = 1;
    }

    message LyricLine {
      int64 start_ms = 1;
      int64 end_ms = 2;
      string text = 3;
    }
    ```

**Files to MODIFY:**

1. `api/proto/plugin/plugin_core.proto`
   - Add new broker ID fields to `InitRequest` for new host services:

     ```protobuf
     message InitRequest {
       // ... existing fields 1-12 ...

       // Broker ID for host playback service (0 = not available)
       // Plugin can dial this ID to get a HostPlaybackClient
       uint32 host_playback_broker_id = 13;

       // Broker ID for host users service (0 = not available)
       // Plugin can dial this ID to get a HostUsersClient
       uint32 host_users_broker_id = 14;

       // Broker ID for host notification service (0 = not available)
       // Plugin can dial this ID to get a HostNotificationClient
       uint32 host_notification_broker_id = 15;

       // Broker ID for host scheduler service (0 = not available)
       // Plugin can dial this ID to get a HostSchedulerClient
       uint32 host_scheduler_broker_id = 16;

       // Broker ID for host auth service (0 = not available)
       // Plugin can dial this ID to get a HostAuthClient
       uint32 host_auth_broker_id = 17;
     }
     ```

2. `api/proto/plugin/host_services.proto`
   - Add HostPlayback service:

     ```protobuf
     service HostPlayback {
       rpc GetCurrentSessions(GetCurrentSessionsRequest) returns (GetCurrentSessionsResponse);
       rpc GetSessionInfo(GetSessionInfoRequest) returns (GetSessionInfoResponse);
       rpc SubscribePlaybackEvents(SubscribePlaybackEventsRequest) returns (stream PlaybackEvent);
     }

     message PlaybackSession {
       string session_id = 1;
       string user_id = 2;
       string media_id = 3;
       string device_id = 4;
       string client_name = 5;
       int64 position_seconds = 6;
       int64 duration_seconds = 7;
       bool is_paused = 8;
       int64 started_at = 9;
     }

     message PlaybackEvent {
       string event_type = 1;  // "started", "progress", "paused", "stopped"
       PlaybackSession session = 2;
       int64 timestamp = 3;
     }
     ```

   - Add HostUsers service:

     ```protobuf
     service HostUsers {
       rpc GetUsers(GetUsersRequest) returns (GetUsersResponse);
       rpc GetUser(GetUserRequest) returns (GetUserResponse);
       rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
       rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
       rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
     }
     ```

   - Add HostNotification service (for plugins to trigger notifications):

     ```protobuf
     service HostNotification {
       rpc SendNotification(HostSendNotificationRequest) returns (HostSendNotificationResponse);
       rpc GetNotificationHistory(GetNotificationHistoryRequest) returns (GetNotificationHistoryResponse);
     }
     ```

   - Add HostScheduler service:

     ```protobuf
     service HostScheduler {
       rpc ScheduleTask(ScheduleTaskRequest) returns (ScheduleTaskResponse);
       rpc CancelTask(CancelTaskRequest) returns (CancelTaskResponse);
       rpc GetScheduledTasks(GetScheduledTasksRequest) returns (GetScheduledTasksResponse);
     }

     message ScheduleTaskRequest {
       string task_id = 1;
       string cron_expression = 2;  // e.g., "0 0 * * *" for daily
       string callback_method = 3;  // Method to call on plugin
       bytes payload = 4;           // Data to pass to callback
     }
     ```

   - Add HostAuth service (for plugins to check auth context):

     ```protobuf
     service HostAuth {
       rpc GetCurrentUser(GetCurrentUserRequest) returns (GetCurrentUserResponse);
       rpc ValidatePermission(ValidatePermissionRequest) returns (ValidatePermissionResponse);
       rpc GetUserPermissions(GetUserPermissionsRequest) returns (GetUserPermissionsResponse);
     }
     ```

**Commands:**
- Run `make proto-gen`

### Phase 4: SDK Implementations (MVP Capabilities)

#### 4.1 Notification SDK

**Files to CREATE:**

1. `pkg/plugin/sdk/notification.go`
   ```go
   package sdk

   type NotificationSinkPlugin interface {
       SendNotification(ctx context.Context, req *pluginv1.SendNotificationRequest) (*pluginv1.SendNotificationResponse, error)
       GetChannels(ctx context.Context) (*pluginv1.GetChannelsResponse, error)
       TestChannel(ctx context.Context, req *pluginv1.TestChannelRequest) (*pluginv1.TestChannelResponse, error)
   }

   type WebhookSenderPlugin interface {
       Configure(ctx context.Context, req *pluginv1.WebhookSenderConfigRequest) error
       GetSubscribedEvents(ctx context.Context) ([]string, error)
   }

   type WebhookReceiverPlugin interface {
       HandleWebhook(ctx context.Context, req *pluginv1.HandleWebhookRequest) (*pluginv1.HandleWebhookResponse, error)
       GetEndpointInfo(ctx context.Context) (*pluginv1.GetEndpointInfoResponse, error)
   }

   func ServeNotificationSink(plugin NotificationSinkPlugin) { ... }
   func ServeWebhookSender(plugin WebhookSenderPlugin) { ... }
   func ServeWebhookReceiver(plugin WebhookReceiverPlugin) { ... }
   ```

#### 4.2 Content SDK

**Files to CREATE:**

1. `pkg/plugin/sdk/content.go`
   ```go
   package sdk

   type SubtitlePlugin interface {
       SearchSubtitles(ctx context.Context, req *pluginv1.SearchSubtitlesRequest) (*pluginv1.SearchSubtitlesResponse, error)
       DownloadSubtitle(ctx context.Context, req *pluginv1.DownloadSubtitleRequest) (*pluginv1.DownloadSubtitleResponse, error)
       GetLanguages(ctx context.Context) (*pluginv1.GetLanguagesResponse, error)
   }

   type LyricsPlugin interface {
       SearchLyrics(ctx context.Context, req *pluginv1.SearchLyricsRequest) (*pluginv1.SearchLyricsResponse, error)
       GetLyrics(ctx context.Context, req *pluginv1.GetLyricsRequest) (*pluginv1.GetLyricsResponse, error)
       GetSyncedLyrics(ctx context.Context, req *pluginv1.GetSyncedLyricsRequest) (*pluginv1.GetSyncedLyricsResponse, error)
   }

   func ServeSubtitleProvider(plugin SubtitlePlugin) { ... }
   func ServeLyricsProvider(plugin LyricsPlugin) { ... }
   ```

### Phase 4.3: SDK Host Service Connections

Update existing SDK files to support new host service broker IDs.

**Files to MODIFY:**

1. `pkg/plugin/sdk/enricher.go` and `pkg/plugin/sdk/provider.go`
   - Update `connectHostServices()` to handle new broker IDs:

     ```go
     func (s *enricherCoreServer) connectHostServices(req *pluginv1.InitRequest) {
         // ... existing connections for storage, data, weather, plugins, ratings, progress ...

         // Connect to HostPlayback if available
         if req.GetHostPlaybackBrokerId() > 0 && s.broker != nil {
             conn, err := s.broker.Dial(req.GetHostPlaybackBrokerId())
             if err != nil {
                 s.base.Log().Error("failed to dial host playback service", "error", err)
             } else {
                 s.base.hostServices.Playback = NewPlaybackClient(conn)
                 s.base.Log().Debug("connected to host playback service")
             }
         }

         // Connect to HostUsers if available
         if req.GetHostUsersBrokerId() > 0 && s.broker != nil {
             conn, err := s.broker.Dial(req.GetHostUsersBrokerId())
             if err != nil {
                 s.base.Log().Error("failed to dial host users service", "error", err)
             } else {
                 s.base.hostServices.Users = NewUsersClient(conn)
                 s.base.Log().Debug("connected to host users service")
             }
         }

         // Connect to HostNotification if available
         if req.GetHostNotificationBrokerId() > 0 && s.broker != nil {
             conn, err := s.broker.Dial(req.GetHostNotificationBrokerId())
             if err != nil {
                 s.base.Log().Error("failed to dial host notification service", "error", err)
             } else {
                 s.base.hostServices.Notification = NewNotificationClient(conn)
                 s.base.Log().Debug("connected to host notification service")
             }
         }

         // Connect to HostScheduler if available
         if req.GetHostSchedulerBrokerId() > 0 && s.broker != nil {
             conn, err := s.broker.Dial(req.GetHostSchedulerBrokerId())
             if err != nil {
                 s.base.Log().Error("failed to dial host scheduler service", "error", err)
             } else {
                 s.base.hostServices.Scheduler = NewSchedulerClient(conn)
                 s.base.Log().Debug("connected to host scheduler service")
             }
         }

         // Connect to HostAuth if available
         if req.GetHostAuthBrokerId() > 0 && s.broker != nil {
             conn, err := s.broker.Dial(req.GetHostAuthBrokerId())
             if err != nil {
                 s.base.Log().Error("failed to dial host auth service", "error", err)
             } else {
                 s.base.hostServices.Auth = NewAuthClient(conn)
                 s.base.Log().Debug("connected to host auth service")
             }
         }
     }
     ```

2. `pkg/plugin/sdk/base.go`
   - Add new client fields to HostServices struct:

     ```go
     type HostServices struct {
         // Existing
         Storage  *StorageClient
         Data     *DataClient
         Weather  *WeatherClient
         Plugins  *PluginsClient
         Ratings  *RatingsClient
         Progress *ProgressClient

         // New host services
         Playback     *PlaybackClient
         Users        *UsersClient
         Notification *NotificationClient
         Scheduler    *SchedulerClient
         Auth         *AuthClient
     }
     ```

### Phase 5: Host Service Implementations

#### 5.1 HostPlayback Service

**Files to CREATE:**

1. `internal/infrastructure/plugins/host/playback.go`
   ```go
   type HostPlaybackServer struct {
       sessionStore PlaybackSessionStore
       eventBus     *events.EventBus
   }

   func (s *HostPlaybackServer) GetCurrentPlayback(ctx, req) (*pluginv1.GetCurrentPlaybackResponse, error)
   func (s *HostPlaybackServer) SubscribePlaybackEvents(req, stream) error
   func (s *HostPlaybackServer) ReportProgress(ctx, req) error  // Internal, for playback handler
   ```

2. `pkg/plugin/sdk/host_playback.go`
   ```go
   type PlaybackClient struct {
       client pluginv1.HostPlaybackClient
   }

   func (c *PlaybackClient) GetCurrentSessions() ([]PlaybackSession, error)
   func (c *PlaybackClient) SubscribeEvents(ctx context.Context, handler func(PlaybackEvent)) error
   ```

#### 5.2 HostUsers Service

**Files to CREATE:**

1. `internal/infrastructure/plugins/host/users.go`
   ```go
   type HostUsersServer struct {
       userService application.UserService
   }

   func (s *HostUsersServer) GetUsers(ctx, req) (*pluginv1.GetUsersResponse, error)
   func (s *HostUsersServer) GetUser(ctx, req) (*pluginv1.GetUserResponse, error)
   func (s *HostUsersServer) CreateUser(ctx, req) (*pluginv1.CreateUserResponse, error)  // For user sync
   func (s *HostUsersServer) UpdateUser(ctx, req) (*pluginv1.UpdateUserResponse, error)
   ```

#### 5.3 Notification Dispatch

**Files to CREATE:**

1. `internal/infrastructure/plugins/host/notification_dispatch.go`
   ```go
   type NotificationDispatcher struct {
       manager  *manager.Manager
       eventBus *events.EventBus
   }

   func (d *NotificationDispatcher) SendNotification(ctx context.Context, msg NotificationMessage) []error {
       // 1. Get all notification_sink plugins
       // 2. Fan out to each
       // 3. Collect errors
   }

   func (d *NotificationDispatcher) SubscribeToEvents() {
       // Subscribe to events that should trigger notifications
       // e.g., library.scan.complete, media.request.approved
   }
   ```

### Phase 6: Manager & Registry Updates

**Files to MODIFY:**

1. `internal/infrastructure/plugins/manager/manager.go`
   - Add getters for all new capability types following existing pattern:
     ```go
     func (m *Manager) GetNotificationSinks() []*types.Instance {
         m.mu.RLock()
         defer m.mu.RUnlock()
         var result []*types.Instance
         for _, p := range m.plugins {
             if p.NotificationSinkClient != nil {
                 result = append(result, p)
             }
         }
         return result
     }
     ```
   - Initialize event bus
   - Wire notification dispatcher

2. `internal/infrastructure/plugins/types/types.go`
   - Add new Category constants:
     ```go
     const (
         // ... existing categories ...
         CategoryNotificationSink Category = "notification_sink"
         CategorySync             Category = "sync"
         CategoryAnalytics        Category = "analytics"
         CategoryPlaybackReporter Category = "playback_reporter"
         CategorySkipDetector     Category = "skip_detector"
         CategoryBackup           Category = "backup"
         CategoryExternalStorage  Category = "external_storage"
         CategoryAuthProvider     Category = "auth_provider"
         CategoryUserSync         Category = "user_sync"
         CategoryMetrics          Category = "metrics"
         CategoryTracing          Category = "tracing"
         CategoryTranscode        Category = "transcode"
         CategoryDownloadClient   Category = "download_client"
         CategoryMediaRequest     Category = "media_request"
         CategoryLibrarySync      Category = "library_sync"
         CategorySubtitle         Category = "subtitle"
         CategoryLyrics           Category = "lyrics"
     )
     ```
   - Add client fields for new capability types:
     ```go
     type Instance struct {
         // ... existing fields ...

         // Notification clients
         NotificationSinkClient pluginv1.NotificationSinkClient
         WebhookSenderClient    pluginv1.WebhookSenderClient
         WebhookReceiverClient  pluginv1.WebhookReceiverClient

         // Sync clients
         WatchHistorySyncClient pluginv1.WatchHistorySyncClient
         ScrobblerClient        pluginv1.ScrobblerClient
         ListSyncClient         pluginv1.ListSyncClient
         CalendarClient         pluginv1.CalendarProviderClient

         // Analytics clients
         StatisticsClient       pluginv1.StatisticsProviderClient
         ReportGeneratorClient  pluginv1.ReportGeneratorClient

         // Playback clients
         PlaybackReporterClient pluginv1.PlaybackReporterClient
         SkipDetectorClient     pluginv1.SkipDetectorClient

         // Backup clients
         BackupClient           pluginv1.BackupProviderClient
         ExternalStorageClient  pluginv1.ExternalStorageClient

         // Auth clients
         AuthProviderClient     pluginv1.ExternalAuthProviderClient
         UserSyncClient         pluginv1.UserSyncProviderClient

         // Monitoring clients
         MetricsExporterClient  pluginv1.MetricsExporterClient
         TracingExporterClient  pluginv1.TracingExporterClient

         // Other clients
         TranscodeClient        pluginv1.TranscodeProviderClient
         DownloadClient         pluginv1.DownloadClientClient
         MediaRequestClient     pluginv1.MediaRequestHandlerClient
         LibrarySyncClient      pluginv1.LibrarySyncProviderClient
         CollectionSyncClient   pluginv1.CollectionSyncProviderClient
         SubtitleClient         pluginv1.SubtitleProviderClient
         LyricsClient           pluginv1.LyricsProviderClient
     }
     ```
   - Update `InferCategoriesFromCapabilities()`:
     ```go
     func InferCategoriesFromCapabilities(capabilities []string) []Category {
         var cats []Category
         for _, cap := range capabilities {
             switch cap {
             case "notification_sink":
                 cats = append(cats, CategoryNotificationSink)
             case "webhook_sender", "webhook_receiver":
                 cats = append(cats, CategoryNotificationSink)
             case "watch_history", "scrobble", "list_sync", "calendar":
                 cats = append(cats, CategorySync)
             case "statistics", "reports":
                 cats = append(cats, CategoryAnalytics)
             case "playback_reporting":
                 cats = append(cats, CategoryPlaybackReporter)
             case "skip_intro", "skip_credits":
                 cats = append(cats, CategorySkipDetector)
             case "backup":
                 cats = append(cats, CategoryBackup)
             case "storage":
                 cats = append(cats, CategoryExternalStorage)
             case "auth_provider":
                 cats = append(cats, CategoryAuthProvider)
             case "user_sync":
                 cats = append(cats, CategoryUserSync)
             case "metrics":
                 cats = append(cats, CategoryMetrics)
             case "tracing":
                 cats = append(cats, CategoryTracing)
             case "transcode":
                 cats = append(cats, CategoryTranscode)
             case "download_client":
                 cats = append(cats, CategoryDownloadClient)
             case "media_requests":
                 cats = append(cats, CategoryMediaRequest)
             case "library_sync", "collection_sync":
                 cats = append(cats, CategoryLibrarySync)
             case "subtitles":
                 cats = append(cats, CategorySubtitle)
             case "lyrics":
                 cats = append(cats, CategoryLyrics)
             }
         }
         return unique(cats)
     }
     ```

3. `internal/infrastructure/plugins/manager/loader.go`
   - Update `buildPluginMap()` to include new capability dispense keys:
     ```go
     func (m *Manager) buildPluginMap(pluginID string, hostServiceLogger *slog.Logger) map[string]plugin.Plugin {
         pluginMap := map[string]plugin.Plugin{
             // Existing
             "core":              m.pluginFactory.NewPluginCoreGRPCPlugin(),
             "enricher":          m.pluginFactory.NewEnricherGRPCPlugin(),
             "provider":          m.pluginFactory.NewPluginProviderGRPCPlugin(),
             "vector_search":     m.pluginFactory.NewVectorSearchGRPCPlugin(),
             "trending_provider": m.pluginFactory.NewTrendingProviderGRPCPlugin(),

             // New capability plugins
             "notification_sink":   m.pluginFactory.NewNotificationSinkGRPCPlugin(),
             "webhook_sender":      m.pluginFactory.NewWebhookSenderGRPCPlugin(),
             "webhook_receiver":    m.pluginFactory.NewWebhookReceiverGRPCPlugin(),
             "watch_history_sync":  m.pluginFactory.NewWatchHistorySyncGRPCPlugin(),
             "scrobbler":           m.pluginFactory.NewScrobblerGRPCPlugin(),
             "list_sync":           m.pluginFactory.NewListSyncGRPCPlugin(),
             "calendar_provider":   m.pluginFactory.NewCalendarProviderGRPCPlugin(),
             "statistics_provider": m.pluginFactory.NewStatisticsProviderGRPCPlugin(),
             "report_generator":    m.pluginFactory.NewReportGeneratorGRPCPlugin(),
             "playback_reporter":   m.pluginFactory.NewPlaybackReporterGRPCPlugin(),
             "skip_detector":       m.pluginFactory.NewSkipDetectorGRPCPlugin(),
             "backup_provider":     m.pluginFactory.NewBackupProviderGRPCPlugin(),
             "external_storage":    m.pluginFactory.NewExternalStorageGRPCPlugin(),
             "auth_provider":       m.pluginFactory.NewAuthProviderGRPCPlugin(),
             "user_sync_provider":  m.pluginFactory.NewUserSyncProviderGRPCPlugin(),
             "metrics_exporter":    m.pluginFactory.NewMetricsExporterGRPCPlugin(),
             "tracing_exporter":    m.pluginFactory.NewTracingExporterGRPCPlugin(),
             "transcode_provider":  m.pluginFactory.NewTranscodeProviderGRPCPlugin(),
             "download_client":     m.pluginFactory.NewDownloadClientGRPCPlugin(),
             "media_request":       m.pluginFactory.NewMediaRequestHandlerGRPCPlugin(),
             "library_sync":        m.pluginFactory.NewLibrarySyncProviderGRPCPlugin(),
             "collection_sync":     m.pluginFactory.NewCollectionSyncProviderGRPCPlugin(),
             "subtitle_provider":   m.pluginFactory.NewSubtitleProviderGRPCPlugin(),
             "lyrics_provider":     m.pluginFactory.NewLyricsProviderGRPCPlugin(),
         }
         // ... host services ...
     }
     ```
   - Update `createPluginInstance()` to dispense new capability clients:
     ```go
     // Example pattern for each capability - add after existing dispense logic:

     // Notification capabilities
     if containsString(mf.Capabilities, "notification_sink") {
         raw, err := rpcClient.Dispense("notification_sink")
         if err != nil {
             m.logger.Warn("plugin provides notification_sink but dispense failed",
                 "plugin", mf.ID, "error", err)
         } else if client, ok := raw.(pluginv1.NotificationSinkClient); ok {
             instance.NotificationSinkClient = client
             m.logger.Debug("notification_sink client available", "plugin", mf.ID)
         }
     }

     if containsString(mf.Capabilities, "webhook_sender") {
         raw, err := rpcClient.Dispense("webhook_sender")
         if err == nil {
             if client, ok := raw.(pluginv1.WebhookSenderClient); ok {
                 instance.WebhookSenderClient = client
             }
         }
     }

     if containsString(mf.Capabilities, "webhook_receiver") {
         raw, err := rpcClient.Dispense("webhook_receiver")
         if err == nil {
             if client, ok := raw.(pluginv1.WebhookReceiverClient); ok {
                 instance.WebhookReceiverClient = client
             }
         }
     }

     // Sync capabilities
     if containsString(mf.Capabilities, "watch_history") {
         raw, err := rpcClient.Dispense("watch_history_sync")
         if err == nil {
             if client, ok := raw.(pluginv1.WatchHistorySyncClient); ok {
                 instance.WatchHistorySyncClient = client
             }
         }
     }

     if containsString(mf.Capabilities, "scrobble") {
         raw, err := rpcClient.Dispense("scrobbler")
         if err == nil {
             if client, ok := raw.(pluginv1.ScrobblerClient); ok {
                 instance.ScrobblerClient = client
             }
         }
     }

     // ... repeat pattern for all capabilities ...

     // Content capabilities
     if containsString(mf.Capabilities, "subtitles") {
         raw, err := rpcClient.Dispense("subtitle_provider")
         if err == nil {
             if client, ok := raw.(pluginv1.SubtitleProviderClient); ok {
                 instance.SubtitleClient = client
             }
         }
     }

     if containsString(mf.Capabilities, "lyrics") {
         raw, err := rpcClient.Dispense("lyrics_provider")
         if err == nil {
             if client, ok := raw.(pluginv1.LyricsProviderClient); ok {
                 instance.LyricsClient = client
             }
         }
     }
     ```

4. `internal/infrastructure/plugins/grpc/plugin.go`
   - Add gRPC plugin wrappers for each new service. Follow existing pattern:
     ```go
     // NotificationSinkPlugin wraps the NotificationSink gRPC service.
     type NotificationSinkPlugin struct {
         plugin.Plugin
     }

     func (p *NotificationSinkPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
         return nil // Plugin side implements the server
     }

     func (p *NotificationSinkPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
         return pluginv1.NewNotificationSinkClient(c), nil
     }

     // WebhookSenderPlugin wraps the WebhookSender gRPC service.
     type WebhookSenderPlugin struct {
         plugin.Plugin
     }

     func (p *WebhookSenderPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
         return nil
     }

     func (p *WebhookSenderPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
         return pluginv1.NewWebhookSenderClient(c), nil
     }

     // ... repeat for all 20+ new capability types ...
     // WebhookReceiverPlugin, WatchHistorySyncPlugin, ScrobblerPlugin,
     // ListSyncPlugin, CalendarProviderPlugin, StatisticsProviderPlugin,
     // ReportGeneratorPlugin, PlaybackReporterPlugin, SkipDetectorPlugin,
     // BackupProviderPlugin, ExternalStoragePlugin, AuthProviderPlugin,
     // UserSyncProviderPlugin, MetricsExporterPlugin, TracingExporterPlugin,
     // TranscodeProviderPlugin, DownloadClientPlugin, MediaRequestHandlerPlugin,
     // LibrarySyncProviderPlugin, CollectionSyncProviderPlugin,
     // SubtitleProviderPlugin, LyricsProviderPlugin
     ```

5. `internal/infrastructure/plugins/grpc/factory.go`
   - Add factory interface methods and implementations:
     ```go
     type PluginFactory interface {
         // ... existing methods ...

         // New capability plugin factories
         NewNotificationSinkGRPCPlugin() plugin.Plugin
         NewWebhookSenderGRPCPlugin() plugin.Plugin
         NewWebhookReceiverGRPCPlugin() plugin.Plugin
         NewWatchHistorySyncGRPCPlugin() plugin.Plugin
         NewScrobblerGRPCPlugin() plugin.Plugin
         NewListSyncGRPCPlugin() plugin.Plugin
         NewCalendarProviderGRPCPlugin() plugin.Plugin
         NewStatisticsProviderGRPCPlugin() plugin.Plugin
         NewReportGeneratorGRPCPlugin() plugin.Plugin
         NewPlaybackReporterGRPCPlugin() plugin.Plugin
         NewSkipDetectorGRPCPlugin() plugin.Plugin
         NewBackupProviderGRPCPlugin() plugin.Plugin
         NewExternalStorageGRPCPlugin() plugin.Plugin
         NewAuthProviderGRPCPlugin() plugin.Plugin
         NewUserSyncProviderGRPCPlugin() plugin.Plugin
         NewMetricsExporterGRPCPlugin() plugin.Plugin
         NewTracingExporterGRPCPlugin() plugin.Plugin
         NewTranscodeProviderGRPCPlugin() plugin.Plugin
         NewDownloadClientGRPCPlugin() plugin.Plugin
         NewMediaRequestHandlerGRPCPlugin() plugin.Plugin
         NewLibrarySyncProviderGRPCPlugin() plugin.Plugin
         NewCollectionSyncProviderGRPCPlugin() plugin.Plugin
         NewSubtitleProviderGRPCPlugin() plugin.Plugin
         NewLyricsProviderGRPCPlugin() plugin.Plugin
     }

     // Implement each factory method:
     func (f *defaultPluginFactory) NewNotificationSinkGRPCPlugin() plugin.Plugin {
         return &NotificationSinkPlugin{}
     }
     // ... etc for all capability types ...
     ```

### Phase 7: API Layer

**Files to CREATE:**

1. `internal/api/handlers/webhooks.go`
   ```go
   // POST /api/webhooks/incoming/:plugin
   func (h *Handler) HandleIncomingWebhook(w http.ResponseWriter, r *http.Request) {
       pluginID := chi.URLParam(r, "plugin")
       // Forward to WebhookReceiverClient
   }

   // GET /api/webhooks/endpoints
   func (h *Handler) ListWebhookEndpoints(w http.ResponseWriter, r *http.Request) {
       // List all plugins with webhook_receiver capability and their endpoints
   }
   ```

2. `internal/api/handlers/notifications.go`
   ```go
   // GET /api/notifications/channels
   func (h *Handler) ListNotificationChannels(w http.ResponseWriter, r *http.Request) {
       // Aggregate channels from all notification_sink plugins
   }

   // POST /api/notifications/test
   func (h *Handler) TestNotificationChannel(w http.ResponseWriter, r *http.Request) {
       // Send test notification to specific channel
   }
   ```

**Files to MODIFY:**

1. `internal/api/routes/routes.go`
   - Add webhook and notification routes

### Phase 8: Testing Infrastructure

**Files to CREATE:**

1. `pkg/plugin/sdk/testing/mock_host.go`
   ```go
   type MockHostServices struct {
       Storage      *MockStorageClient
       Data         *MockDataClient
       Playback     *MockPlaybackClient
       Notification *MockNotificationDispatcher
       // ... etc
   }

   func NewMockHostServices() *MockHostServices
   func (m *MockHostServices) InjectIntoContext(ctx context.Context) context.Context
   ```

2. `pkg/plugin/sdk/testing/plugin_harness.go`
   ```go
   type PluginTestHarness struct {
       hosts *MockHostServices
   }

   func NewTestHarness() *PluginTestHarness
   func (h *PluginTestHarness) ServePlugin(plugin interface{}) *grpc.ClientConn
   func (h *PluginTestHarness) SimulateEvent(event Event)
   ```

## Files Summary

### To CREATE (Infrastructure)

- `internal/infrastructure/plugins/host/search_provider.go`
- `internal/infrastructure/plugins/host/widgets.go`
- `internal/infrastructure/plugins/host/user_metadata.go`
- `internal/infrastructure/plugins/host/file_parser.go`
- `internal/infrastructure/plugins/host/playback.go`
- `internal/infrastructure/plugins/host/users.go`
- `internal/infrastructure/plugins/host/notification_dispatch.go`
- `pkg/plugin/sdk/middleware/retry.go`
- `pkg/plugin/sdk/user_metadata.go`
- `pkg/plugin/sdk/file_parser.go`
- `pkg/plugin/sdk/host_playback.go`
- `migrations/XXXXXX_add_capability_preferences.up.sql`

### To CREATE (Proto)

- `api/proto/plugin/notification.proto`
- `api/proto/plugin/sync.proto`
- `api/proto/plugin/analytics.proto`
- `api/proto/plugin/playback.proto`
- `api/proto/plugin/backup.proto`
- `api/proto/plugin/auth.proto`
- `api/proto/plugin/monitoring.proto`
- `api/proto/plugin/transcode.proto`
- `api/proto/plugin/download.proto`
- `api/proto/plugin/media_management.proto`
- `api/proto/plugin/content.proto`

### To CREATE (SDK)

- `pkg/plugin/sdk/notification.go`
- `pkg/plugin/sdk/content.go`
- `pkg/plugin/sdk/sync.go`
- `pkg/plugin/sdk/analytics.go`
- `pkg/plugin/sdk/playback.go`
- `pkg/plugin/sdk/backup.go`
- `pkg/plugin/sdk/auth.go`
- `pkg/plugin/sdk/monitoring.go`
- `pkg/plugin/sdk/transcode.go`
- `pkg/plugin/sdk/download.go`
- `pkg/plugin/sdk/media_management.go`

### To CREATE (API)

- `internal/api/handlers/webhooks.go`
- `internal/api/handlers/notifications.go`

### To CREATE (Testing)

- `pkg/plugin/sdk/testing/mock_host.go`
- `pkg/plugin/sdk/testing/plugin_harness.go`

### To MODIFY

- `api/proto/plugin/plugin_core.proto` - event types, new host service broker IDs in InitRequest
- `api/proto/plugin/host_services.proto` - new host services (HostPlayback, HostUsers, HostNotification, HostScheduler, HostAuth)
- `internal/infrastructure/plugins/host/plugins.go` - capability routing
- `internal/infrastructure/plugins/registry/capability.go` - preferences
- `internal/infrastructure/plugins/manager/manager.go` - event bus, getters for all new capabilities
- `internal/infrastructure/plugins/manager/loader.go` - buildPluginMap() entries, createPluginInstance() dispense logic
- `internal/infrastructure/plugins/manager/dependencies.go` - startup order
- `internal/infrastructure/plugins/types/types.go` - client fields (20+), category constants, InferCategoriesFromCapabilities()
- `internal/infrastructure/plugins/grpc/plugin.go` - 22 new gRPC plugin wrappers
- `internal/infrastructure/plugins/grpc/factory.go` - 22 new factory methods
- `pkg/plugin/sdk/base.go` - request tracing, new host services
- `pkg/plugin/sdk/host_services.go` - retry middleware, new host service broker connections
- `pkg/plugin/sdk/enricher.go` - connectHostServices() for new broker IDs
- `pkg/plugin/sdk/provider.go` - connectHostServices() for new broker IDs
- `internal/api/routes/routes.go` - new routes

## Recommendation

**Phase 1 (Infrastructure)** is critical - plugin event delivery, capability routing, and error handling affect everything else. Note: Event bus already exists - just need to wire plugin delivery.

**Phase 2 (Partial Implementations)** completes existing half-done work.

**Phases 3-4 (Proto + SDK)** can start in parallel with Phase 1.

**Phase 5-7** depend on earlier phases.

**Phase 8 (Testing)** should be developed alongside each phase.

## Existing Infrastructure (No Work Needed)

The following infrastructure already exists and should NOT be recreated:

- **Event Bus**: `internal/infrastructure/events/bus/` - Full pub/sub with filtering, replay, ring buffer
- **Event Types**: `internal/domain/events/event.go` - Playback, user, media, scan, plugin, transcode events
- **Publisher Interface**: `internal/domain/events/publisher.go` - Used throughout the codebase
- **Capability Registry**: `internal/infrastructure/plugins/registry/` - Basic capability→plugin mapping
- **HostPlugins Service**: `internal/infrastructure/plugins/host/plugins.go` - InvokeCapability, preferences (in-memory)

## Verification

After each phase:

1. `make proto-gen` succeeds
2. `make build` succeeds
3. `make test` passes
4. For capability phases: example plugin loads and functions correctly
5. For infrastructure phases: existing plugins continue to work
