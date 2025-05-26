/**
 * Media Library and Scanner Type Definitions
 *
 * This file contains TypeScript interfaces for media library management
 * and scanner functionality. These types match the backend API responses.
 */

/**
 * Represents a media library configuration
 */
export interface MediaLibrary {
  id: number;
  path: string;
  type: string;
  created_at: string;
  updated_at: string;
}

/**
 * Represents a background scan job for processing media files
 *
 * Note: This interface was updated to match the backend data structure:
 * - id is now a number (was string)
 * - error_message is now a single string (was errors array)
 * - Added optional fields that the backend provides
 */
export interface ScanJob {
  id: number; // Unique job identifier (numeric)
  library_id: number; // ID of the library being scanned
  status: string; // Job status: pending, running, completed, failed, paused
  files_found: number; // Total number of files discovered
  files_processed: number; // Number of files processed so far
  bytes_processed: number; // Total bytes processed
  error_message?: string; // Error message if job failed (single string)
  started_at: string; // ISO timestamp when job started
  completed_at?: string; // ISO timestamp when job completed (if finished)
  progress?: number; // Progress percentage (0-100)
  created_at?: string; // ISO timestamp when job was created
  updated_at?: string; // ISO timestamp when job was last updated
  library?: {
    // Associated library information
    id: number;
    path: string;
    type: string;
    created_at: string;
    updated_at: string;
  };
}

/**
 * Overall scanner statistics
 */
export interface ScanStats {
  active_scans: number;
  total_files_scanned: number;
  total_bytes_scanned: number;
}

/**
 * Statistics for individual libraries
 */
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

/**
 * Scanner configuration settings
 */
export interface ScanConfig {
  parallel_scanning_enabled: boolean;
  worker_count: number;
  batch_size: number;
  channel_buffer_size: number;
  smart_hash_enabled: boolean;
  async_metadata_enabled: boolean;
  metadata_worker_count: number;
}

/**
 * Performance statistics for completed scans
 */
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
