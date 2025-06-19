export interface DashboardSection {
  id: string;
  plugin_id: string;
  type: string;
  title: string;
  description: string;
  icon: string;
  priority: number;
  config: DashboardSectionConfig;
  manifest: DashboardManifest;
}

export interface DashboardSectionConfig {
  refresh_interval: number;
  supports_realtime: boolean;
  has_nerd_panel: boolean;
  requires_auth: boolean;
  min_refresh_rate: number;
  max_data_points: number;
}

export interface DashboardManifest {
  component_type: string;
  data_endpoints: Record<string, DataEndpoint>;
  actions: DashboardAction[];
  ui_schema: Record<string, unknown>;
}

export interface DataEndpoint {
  path: string;
  method: string;
  data_type: string;
  cache_key?: string;
  headers?: Record<string, string>;
  description: string;
}

export interface DashboardAction {
  id: string;
  label: string;
  icon: string;
  style: string;
  endpoint: string;
  method: string;
  confirm: boolean;
  payload?: Record<string, unknown>;
  shortcut?: string;
}

export interface TimeRange {
  start: Date;
  end: Date;
  step: string;
}

export interface MetricPoint {
  timestamp: Date;
  value: number;
  labels: Record<string, string>;
  metadata?: Record<string, unknown>;
}

export interface DashboardUpdate {
  section_id: string;
  data_type: string;
  data: unknown;
  timestamp: Date;
  event_type: string;
}

// Transcoder-specific types
export interface TranscoderMainData {
  active_sessions: TranscodeSessionSummary[];
  queued_sessions: TranscodeSessionSummary[];
  recent_sessions: TranscodeSessionSummary[];
  engine_status: TranscoderEngineStatus;
  quick_stats: TranscoderQuickStats;
}

export interface TranscoderNerdData {
  encoder_queues: EncoderQueueInfo[];
  hardware_status: HardwareStatusInfo;
  performance_metrics: PerformanceMetrics;
  config_diagnostics: ConfigDiagnostic[];
  system_resources: SystemResourceInfo;
}

export interface TranscodeSessionSummary {
  id: string;
  input_filename: string;
  input_resolution: string;
  output_resolution: string;
  input_codec: string;
  output_codec: string;
  bitrate: string;
  duration: string;
  progress: number;
  transcoder_type: string;
  client_ip: string;
  client_device: string;
  start_time: Date;
  status: string;
  estimated_time_left: string;
  throughput_fps: number;
}

export interface TranscoderEngineStatus {
  type: string;
  status: string;
  version: string;
  max_concurrent: number;
  active_sessions: number;
  queued_sessions: number;
  last_health_check: Date;
  capabilities: string[];
}

export interface TranscoderQuickStats {
  sessions_today: number;
  total_hours_today: number;
  average_speed: number;
  error_rate: number;
  current_throughput: string;
  peak_concurrent: number;
}

export interface EncoderQueueInfo {
  queue_id: string;
  type: string;
  pending: number;
  processing: number;
  max_slots: number;
  avg_wait_time: string;
}

export interface HardwareStatusInfo {
  gpu: GPUInfo;
  encoders: EncoderInfo[];
  memory: MemoryInfo;
  temperature: number;
  power_draw: number;
  utilization_pct: number;
}

export interface GPUInfo {
  name: string;
  driver: string;
  vram_total: number;
  vram_used: number;
  core_clock: number;
  memory_clock: number;
  fan_speed: number;
}

export interface EncoderInfo {
  id: string;
  type: string;
  status: string;
  current_load: number;
  session_count: number;
  max_sessions: number;
}

export interface MemoryInfo {
  system: SystemMemory;
  gpu: GPUMemory;
}

export interface SystemMemory {
  total: number;
  used: number;
  cached: number;
}

export interface GPUMemory {
  total: number;
  used: number;
  free: number;
}

export interface PerformanceMetrics {
  encoding_speed: number;
  quality_score: number;
  compression_ratio: number;
  error_count: number;
  restart_count: number;
  uptime_seconds: number;
}

export interface ConfigDiagnostic {
  category: string;
  level: string;
  message: string;
  setting: string;
  value: string;
  suggestion: string;
}

export interface SystemResourceInfo {
  cpu: CPUInfo;
  memory: MemoryInfo;
  disk: DiskInfo;
  network: NetworkInfo;
}

export interface CPUInfo {
  usage: number;
  cores: number;
  threads: number;
  frequency: number;
}

export interface DiskInfo {
  total_space: number;
  used_space: number;
  io_reads: number;
  io_writes: number;
  io_util: number;
}

export interface NetworkInfo {
  bytes_received: number;
  bytes_sent: number;
  packets_rx: number;
  packets_tx: number;
  bandwidth: number;
} 