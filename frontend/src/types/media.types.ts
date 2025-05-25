export interface MediaLibrary {
  id: number;
  path: string;
  type: string;
  created_at: string;
  updated_at: string;
}

export interface ScanJob {
  id: string;
  library_id: number;
  status: string;
  files_found: number;
  files_processed: number;
  bytes_processed: number;
  errors: string[];
  started_at: string;
  completed_at?: string;
}

export interface ScanStats {
  active_scans: number;
  total_files_scanned: number;
  total_bytes_scanned: number;
}

export interface LibraryStats {
  [key: number]: {
    total_files: number;
    total_size: number;
    extension_stats: Array<{
      extension: string;
      count: number;
    }>;
    scan_status?: string;
    progress?: number;
    files_found?: number;
    files_processed?: number;
    bytes_processed?: number;
  };
}

export interface ScanConfig {
  parallel_scanning_enabled: boolean;
  worker_count: number;
  batch_size: number;
  channel_buffer_size: number;
  smart_hash_enabled: boolean;
  async_metadata_enabled: boolean;
  metadata_worker_count: number;
}

export interface ScanPerformanceStats {
  id: number;
  library_id: number;
  status: string;
  files_found: number;
  files_processed: number;
  bytes_processed: number;
  duration_seconds: number;
  files_per_second: number;
  mb_per_second: number;
  started_at: string;
  completed_at: string;
}
