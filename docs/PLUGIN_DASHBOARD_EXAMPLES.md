# Plugin Dashboard Examples

This document shows how the dashboard pattern extends beyond transcoding plugins to other plugin types, demonstrating the scalability and flexibility of the system.

## 1. Metadata Enhancement Plugins

### Example: TMDB Metadata Plugin

**Essential Info (Main Panel):**
- Active enrichment jobs (movie/TV lookups)
- Recent completions and failures
- Rate limiting status and API quotas
- Most matched and unmatched content

**Nerd Panel:**
- API response times and cache hit rates
- Rate limiting windows and retry backoff
- Detailed match confidence scores
- API endpoint performance breakdown
- Cache size and eviction statistics

```typescript
// MetadataSection.tsx
interface MetadataMainData {
  active_jobs: EnrichmentJob[];
  recent_completions: CompletedJob[];
  api_status: {
    tmdb_quota_remaining: number;
    last_success: string;
    rate_limit_reset: string;
  };
  success_rate: number;
  pending_queue_size: number;
}

interface MetadataNerdData {
  performance_metrics: {
    average_response_time: number;
    cache_hit_rate: number;
    api_errors_24h: number;
  };
  detailed_queues: {
    movie_queue: EnrichmentJob[];
    tv_queue: EnrichmentJob[];
    person_queue: EnrichmentJob[];
  };
  confidence_distribution: {
    high_confidence: number;
    medium_confidence: number;
    low_confidence: number;
    failed_matches: number;
  };
}
```

### Example: MusicBrainz Audio Plugin

**Essential Info:**
- Album/artist lookup progress
- Audio fingerprinting status
- Unidentified files count

**Nerd Panel:**
- Fingerprint match confidence distribution
- API endpoint latency breakdown
- Database synchronization status

## 2. Storage Management Plugins

### Example: Network Storage Plugin

**Essential Info:**
- Available storage space per mount
- Active file operations (copy/move/scan)
- Mount health status
- Recent errors or disconnections

**Nerd Panel:**
- Detailed I/O statistics (IOPS, throughput)
- Network latency to storage endpoints
- File system fragmentation levels
- Cache utilization and hit rates
- Backup/sync job histories

```typescript
interface StorageMainData {
  mounts: StorageMount[];
  active_operations: FileOperation[];
  space_usage: {
    total_space: number;
    used_space: number;
    available_space: number;
  };
  health_status: 'healthy' | 'warning' | 'critical';
}

interface StorageNerdData {
  io_metrics: {
    read_iops: number;
    write_iops: number;
    read_throughput: number;
    write_throughput: number;
  };
  network_stats: {
    latency_ms: number;
    packet_loss: number;
    connection_uptime: number;
  };
  filesystem_health: {
    fragmentation_level: number;
    bad_sectors: number;
    smart_status: SmartData;
  };
}
```

## 3. Live TV/Streaming Plugins

### Example: HDHomeRun Tuner Plugin

**Essential Info:**
- Active recordings and streams
- Tuner utilization (2/4 tuners busy)
- Signal strength and quality
- Recording schedule conflicts

**Nerd Panel:**
- Detailed signal diagnostics (SNR, BER, UNC)
- Channel scan results and lineup changes
- Transcoding queue for mobile clients
- Network utilization for streaming

```typescript
interface TunerMainData {
  active_recordings: Recording[];
  tuner_status: TunerStatus[];
  upcoming_recordings: ScheduledRecording[];
  signal_quality: 'excellent' | 'good' | 'fair' | 'poor';
  conflicts: ScheduleConflict[];
}

interface TunerNerdData {
  signal_diagnostics: {
    snr_db: number;
    signal_strength_dbm: number;
    bit_error_rate: number;
    uncorrected_blocks: number;
  };
  channel_lineup: {
    total_channels: number;
    hd_channels: number;
    encrypted_channels: number;
    last_scan: string;
  };
  network_streaming: {
    concurrent_streams: number;
    bandwidth_utilization: number;
    client_devices: StreamClient[];
  };
}
```

## 4. Monitoring/Analytics Plugins

### Example: System Health Plugin

**Essential Info:**
- CPU, RAM, disk usage
- Active alerts and warnings
- System uptime and temperature
- Service availability status

**Nerd Panel:**
- Detailed performance graphs
- Log analysis and error patterns
- Resource utilization trends
- Predictive capacity warnings

```typescript
interface MonitoringMainData {
  system_overview: {
    cpu_usage: number;
    memory_usage: number;
    disk_usage: number;
    temperature: number;
  };
  active_alerts: Alert[];
  service_status: ServiceStatus[];
  uptime: number;
}

interface MonitoringNerdData {
  detailed_metrics: {
    cpu_per_core: number[];
    memory_breakdown: MemoryStats;
    disk_io_stats: DiskIOStats;
    network_interfaces: NetworkInterface[];
  };
  historical_data: {
    cpu_history_24h: TimeSeriesData;
    memory_history_24h: TimeSeriesData;
    alert_frequency: AlertStats;
  };
  predictive_analysis: {
    capacity_warnings: CapacityPrediction[];
    trend_analysis: TrendData;
  };
}
```

## 5. Network Services Plugins

### Example: DLNA/UPnP Server Plugin

**Essential Info:**
- Connected devices and active streams
- Library sharing status
- Network discovery results

**Nerd Panel:**
- Protocol compliance levels
- Detailed device capabilities
- UPnP event subscription status
- SOAP request/response statistics

## Plugin Implementation Pattern

Each plugin type follows the same pattern:

### 1. Dashboard Section Provider
```go
func (p *MetadataPlugin) GetDashboardSections() []*plugins.DashboardSection {
    return []*plugins.DashboardSection{
        {
            ID:          "metadata_enrichment",
            Title:       "Metadata Enrichment",
            Type:        "metadata",
            Description: "TMDB and MusicBrainz content enrichment",
            Icon:        "database",
            Priority:    5,
        },
    }
}
```

### 2. Data Providers
```go
func (p *MetadataPlugin) GetMainData(ctx context.Context, sectionID string) (interface{}, error) {
    return &MetadataMainData{
        ActiveJobs:     p.getActiveJobs(),
        APIStatus:      p.getAPIStatus(),
        SuccessRate:    p.calculateSuccessRate(),
        PendingQueue:   p.getQueueSize(),
    }, nil
}

func (p *MetadataPlugin) GetNerdData(ctx context.Context, sectionID string) (interface{}, error) {
    return &MetadataNerdData{
        PerformanceMetrics: p.getPerformanceMetrics(),
        DetailedQueues:     p.getDetailedQueues(),
        ConfidenceDistribution: p.getConfidenceStats(),
    }, nil
}
```

### 3. Frontend Section Renderers
```typescript
// MetadataSectionRenderer.tsx
export default function MetadataSectionRenderer({
  data,
  nerdMode,
  onActionExecute
}: SectionRendererProps<MetadataMainData, MetadataNerdData>) {
  return (
    <div className="space-y-4">
      {/* Main info: active jobs, success rate, API status */}
      {!nerdMode && (
        <MetadataMainView data={data.main} />
      )}
      
      {/* Nerd panel: detailed stats, cache metrics, API timing */}
      {nerdMode && (
        <MetadataNerdView data={data.nerd} />
      )}
    </div>
  );
}
```

## Benefits of This Approach

1. **Consistent UX**: Every plugin type follows the same main/nerd panel pattern
2. **Scalable Architecture**: Adding new plugin types requires minimal framework changes
3. **Developer Friendly**: Clear interfaces and examples for plugin developers
4. **Modular UI**: Each plugin type can have specialized renderers while sharing common patterns
5. **Real-time Ready**: All plugin types can implement streaming updates using the same infrastructure
6. **Type Safe**: Full TypeScript coverage with proper interface definitions

This design ensures that whether you're managing transcoding, metadata, storage, or any other service, the admin experience remains consistent while allowing each plugin type to expose its unique capabilities and metrics. 