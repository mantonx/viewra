//! On-demand cluster cache with binary search seeking.
//!
//! This module provides lazy, on-demand cluster indexing that minimizes
//! I/O for network filesystems. Instead of scanning the entire file upfront,
//! it uses binary search with position estimation to find clusters quickly.
//!
//! ## Algorithm
//!
//! 1. **Position Estimation**: Given target_ms, estimate byte position as:
//!    `(target_ms / duration_ms) * file_size`
//!
//! 2. **Binary Search**: Seek to estimated position, find nearest cluster,
//!    refine with 2-3 additional seeks until we bracket the target.
//!
//! 3. **Caching**: Store discovered clusters in a BTreeMap for instant lookup
//!    on subsequent requests.
//!
//! ## Performance
//!
//! - First seek to any position: 3-5 reads (binary search)
//! - Subsequent seeks to cached regions: 0 reads
//! - Sequential playback: clusters discovered naturally, no extra I/O

use anyhow::{Context, Result};
use std::collections::BTreeMap;
use std::fs::File;
use std::io::{BufReader, Read, Seek, SeekFrom};
use std::path::{Path, PathBuf};

// EBML Element IDs
const CLUSTER: u32 = 0x1F43B675;
const CLUSTER_TIMESTAMP: u32 = 0xE7;
const SEGMENT: u32 = 0x18538067;
const SEGMENT_INFO: u32 = 0x1549A966;
const TIMESTAMP_SCALE: u32 = 0x2AD7B1;
const EBML_HEADER: u32 = 0x1A45DFA3;
const DURATION: u32 = 0x4489;

/// A cached cluster entry
#[derive(Debug, Clone)]
pub struct CachedCluster {
    /// Cluster timestamp in milliseconds
    pub timestamp_ms: u64,
    /// Byte offset where cluster element starts (before ID)
    pub element_offset: u64,
    /// Byte offset where cluster data starts (after header)
    pub data_offset: u64,
    /// Size of the cluster data
    pub size: u64,
}

/// On-demand cluster cache with binary search seeking
pub struct ClusterCache {
    path: PathBuf,
    file_size: u64,
    timestamp_scale: u64,
    duration_ms: Option<u64>,
    segment_start: u64,

    /// Cached clusters: timestamp_ms -> cluster info
    /// Using BTreeMap for ordered iteration and range queries
    clusters: BTreeMap<u64, CachedCluster>,

    /// Track if we've fully indexed the file
    #[allow(dead_code)]
    fully_indexed: bool,
}

impl ClusterCache {
    /// Open a file and read just the header info (timestamp scale, duration)
    pub fn open(path: &Path) -> Result<Self> {
        let file = File::open(path).context("Failed to open file")?;
        let file_size = file.metadata()?.len();
        let mut reader = BufReader::with_capacity(64 * 1024, file);

        // Parse header to get timestamp scale and duration
        let (timestamp_scale, duration_ms, segment_start) = Self::parse_header(&mut reader)?;

        Ok(Self {
            path: path.to_path_buf(),
            file_size,
            timestamp_scale,
            duration_ms,
            segment_start,
            clusters: BTreeMap::new(),
            fully_indexed: false,
        })
    }

    /// Find the cluster containing or just before the given timestamp.
    /// Uses binary search with position estimation for O(log n) I/O.
    pub fn find_cluster(&mut self, target_ms: u64) -> Result<Option<CachedCluster>> {
        // Check cache first
        if let Some(cached) = self.find_in_cache(target_ms) {
            return Ok(Some(cached));
        }

        // Binary search for the cluster
        self.binary_search_cluster(target_ms)
    }

    /// Find cluster in cache (no I/O)
    fn find_in_cache(&self, target_ms: u64) -> Option<CachedCluster> {
        // Find the greatest timestamp <= target_ms
        self.clusters
            .range(..=target_ms)
            .next_back()
            .map(|(_, c)| c.clone())
    }

    /// Binary search for a cluster at the given timestamp
    ///
    /// This uses position-based estimation even without duration info:
    /// 1. Use cached clusters to estimate timestamp-to-position ratio
    /// 2. Or fall back to simple positional binary search
    fn binary_search_cluster(&mut self, target_ms: u64) -> Result<Option<CachedCluster>> {
        let file = File::open(&self.path)?;
        let mut reader = BufReader::with_capacity(64 * 1024, file);

        // Calculate search bounds
        let mut low_pos = self.segment_start;
        let mut high_pos = self.file_size;
        let mut low_ts: u64 = 0;
        let mut high_ts: u64 = self.duration_ms.unwrap_or(u64::MAX);

        // Use cached clusters to narrow bounds
        if let Some((ts, cluster)) = self.clusters.range(..=target_ms).next_back() {
            low_pos = cluster.data_offset + cluster.size; // Start after this cluster
            low_ts = *ts;
        }
        if let Some((ts, cluster)) = self.clusters.range((target_ms + 1)..).next() {
            high_pos = cluster.element_offset;
            high_ts = *ts;
        }

        // Debug: uncomment to trace binary search
        // eprintln!("  Binary search: target={}ms, bounds=[{}..{}] pos=[{}..{}]",
        //     target_ms, low_ts, high_ts, low_pos, high_pos);

        // Binary search with ~5-8 iterations
        const MAX_ITERATIONS: usize = 10;
        let mut best_cluster: Option<CachedCluster> = None;

        for _iteration in 0..MAX_ITERATIONS {
            if high_pos <= low_pos || high_pos - low_pos < 512 * 1024 {
                // Close enough, final refinement
                break;
            }

            // Estimate position using linear interpolation
            let estimate = if high_ts > low_ts && high_ts != u64::MAX {
                // We have timestamp bounds - use linear interpolation
                let ratio = if target_ms <= low_ts {
                    0.0
                } else if target_ms >= high_ts {
                    1.0
                } else {
                    (target_ms - low_ts) as f64 / (high_ts - low_ts) as f64
                };
                low_pos + ((high_pos - low_pos) as f64 * ratio) as u64
            } else {
                // No timestamp bounds - simple midpoint
                (low_pos + high_pos) / 2
            };

            // Find cluster at estimated position
            match Self::find_cluster_near_static(&mut reader, estimate, self.timestamp_scale)? {
                Some(cluster) => {
                    let cluster_ts = cluster.timestamp_ms;
                    let cluster_pos = cluster.element_offset;
                    let cluster_end = cluster.data_offset + cluster.size;

                    // Cache this cluster
                    self.clusters.insert(cluster_ts, cluster.clone());

                    if cluster_ts <= target_ms {
                        // This cluster is at or before target - it's a candidate
                        best_cluster = Some(cluster);
                        // Search in upper half - move past this cluster
                        let new_low_pos = cluster_end;

                        // Avoid getting stuck if we keep finding the same cluster
                        if new_low_pos == low_pos {
                            break;
                        }

                        low_pos = new_low_pos;
                        low_ts = cluster_ts;
                    } else {
                        // This cluster is after target - search in lower half
                        let new_high_pos = cluster_pos;

                        // Avoid getting stuck
                        if new_high_pos == high_pos {
                            break;
                        }

                        high_pos = new_high_pos;
                        high_ts = cluster_ts;
                    }
                }
                None => {
                    // No cluster found at this position, try lower half
                    high_pos = estimate;
                }
            }
        }

        // If we found a good cluster, return it
        if best_cluster.is_some() {
            return Ok(best_cluster);
        }

        // Final fallback: scan the remaining small region
        if high_pos - low_pos < 2 * 1024 * 1024 {
            return self.scan_for_cluster(&mut reader, low_pos, target_ms);
        }

        // Still couldn't find - scan from last known position
        self.scan_for_cluster(&mut reader, self.segment_start, target_ms)
    }

    /// Find the nearest cluster to a byte position (static - no self borrow)
    fn find_cluster_near_static(
        reader: &mut BufReader<File>,
        pos: u64,
        timestamp_scale: u64,
    ) -> Result<Option<CachedCluster>> {
        // Seek to position
        reader.seek(SeekFrom::Start(pos))?;

        // Scan forward for cluster signature (0x1F 0x43 0xB6 0x75)
        let cluster_sig = [0x1F, 0x43, 0xB6, 0x75];
        let mut buf = [0u8; 4];
        let mut window = [0u8; 4];

        // Read initial window
        if reader.read_exact(&mut window).is_err() {
            return Ok(None);
        }

        let scan_limit = 2 * 1024 * 1024; // Scan up to 2MB forward
        let mut scanned = 4u64;

        loop {
            if window == cluster_sig {
                // Found cluster signature
                let element_offset = pos + scanned - 4;
                reader.seek(SeekFrom::Start(element_offset))?;
                return Self::read_cluster_at_static(reader, element_offset, timestamp_scale);
            }

            if scanned >= scan_limit {
                return Ok(None);
            }

            // Slide window
            window[0] = window[1];
            window[1] = window[2];
            window[2] = window[3];
            if reader.read_exact(&mut buf[0..1]).is_err() {
                return Ok(None);
            }
            window[3] = buf[0];
            scanned += 1;
        }
    }

    /// Instance method wrapper for find_cluster_near_static
    #[allow(dead_code)]
    fn find_cluster_near(&self, reader: &mut BufReader<File>, pos: u64) -> Result<Option<CachedCluster>> {
        Self::find_cluster_near_static(reader, pos, self.timestamp_scale)
    }

    /// Read cluster info at a known position (static - no self borrow)
    fn read_cluster_at_static(
        reader: &mut BufReader<File>,
        element_offset: u64,
        timestamp_scale: u64,
    ) -> Result<Option<CachedCluster>> {
        reader.seek(SeekFrom::Start(element_offset))?;

        // Read element ID
        let (id, _) = match Self::read_element_id(reader) {
            Ok(v) => v,
            Err(_) => return Ok(None),
        };

        if id != CLUSTER {
            return Ok(None);
        }

        // Read size
        let (size, _) = match Self::read_vint(reader) {
            Ok(v) => v,
            Err(_) => return Ok(None),
        };

        let data_offset = reader.stream_position()?;

        // Read cluster timestamp (usually first element)
        let mut timestamp_ms = 0u64;
        let peek_end = data_offset + size.min(64);

        while reader.stream_position()? < peek_end {
            let (sub_id, sub_size) = match Self::read_element_header(reader) {
                Ok(v) => v,
                Err(_) => break,
            };

            if sub_id == CLUSTER_TIMESTAMP {
                let raw_ts = Self::read_uint(reader, sub_size)?;
                timestamp_ms = (raw_ts * timestamp_scale) / 1_000_000;
                break;
            } else {
                reader.seek(SeekFrom::Current(sub_size as i64))?;
            }
        }

        Ok(Some(CachedCluster {
            timestamp_ms,
            element_offset,
            data_offset,
            size,
        }))
    }

    /// Scan from a position to find cluster at or before target_ms
    /// Uses pattern scanning for robustness (handles malformed elements)
    fn scan_for_cluster(&mut self, reader: &mut BufReader<File>, start: u64, target_ms: u64) -> Result<Option<CachedCluster>> {
        let mut pos = start;
        let mut best: Option<CachedCluster> = None;
        let timestamp_scale = self.timestamp_scale;
        let file_size = self.file_size;

        // Scan for cluster signatures using pattern matching
        while pos < file_size {
            // Find next cluster using pattern scan (static to avoid borrow issues)
            match Self::find_cluster_near_static(reader, pos, timestamp_scale)? {
                Some(cluster) => {
                    // Cache it
                    self.clusters.insert(cluster.timestamp_ms, cluster.clone());

                    if cluster.timestamp_ms <= target_ms {
                        best = Some(cluster.clone());
                        // Move past this cluster to find the next one
                        pos = cluster.data_offset + cluster.size;
                    } else {
                        // Found cluster after target - return best so far
                        return Ok(best);
                    }
                }
                None => {
                    // No more clusters found
                    break;
                }
            }
        }

        Ok(best)
    }

    /// Index a region around the current playback position (for prefetching)
    #[allow(dead_code)]
    pub fn index_region(&mut self, center_ms: u64, radius_ms: u64) -> Result<usize> {
        let start_ms = center_ms.saturating_sub(radius_ms);
        let end_ms = center_ms + radius_ms;

        // Find starting cluster
        let start_cluster = self.find_cluster(start_ms)?;

        if start_cluster.is_none() {
            return Ok(0);
        }

        let file = File::open(&self.path)?;
        let mut reader = BufReader::with_capacity(256 * 1024, file);

        let start_offset = start_cluster.unwrap().element_offset;
        reader.seek(SeekFrom::Start(start_offset))?;

        let mut count = 0;

        while reader.stream_position()? < self.file_size {
            let element_offset = reader.stream_position()?;

            let (id, size) = match Self::read_element_header(&mut reader) {
                Ok(v) => v,
                Err(_) => break,
            };

            if id == CLUSTER {
                let data_offset = reader.stream_position()?;

                // Read timestamp
                let mut timestamp_ms = 0u64;
                let peek_end = data_offset + size.min(64);

                while reader.stream_position()? < peek_end {
                    let (sub_id, sub_size) = match Self::read_element_header(&mut reader) {
                        Ok(v) => v,
                        Err(_) => break,
                    };

                    if sub_id == CLUSTER_TIMESTAMP {
                        let raw_ts = Self::read_uint(&mut reader, sub_size)?;
                        timestamp_ms = (raw_ts * self.timestamp_scale) / 1_000_000;
                        break;
                    } else {
                        reader.seek(SeekFrom::Current(sub_size as i64))?;
                    }
                }

                if timestamp_ms > end_ms {
                    break;
                }

                let cluster = CachedCluster {
                    timestamp_ms,
                    element_offset,
                    data_offset,
                    size,
                };

                self.clusters.insert(timestamp_ms, cluster);
                count += 1;

                reader.seek(SeekFrom::Start(data_offset + size))?;
            } else {
                reader.seek(SeekFrom::Current(size as i64))?;
            }
        }

        Ok(count)
    }

    /// Get the byte offset to seek to for a given timestamp
    #[allow(dead_code)]
    pub fn get_seek_offset(&mut self, target_ms: u64) -> Result<Option<u64>> {
        Ok(self.find_cluster(target_ms)?.map(|c| c.data_offset))
    }

    /// Get cache statistics
    #[allow(dead_code)]
    pub fn stats(&self) -> (usize, bool) {
        (self.clusters.len(), self.fully_indexed)
    }

    // --- Header parsing ---

    fn parse_header(reader: &mut BufReader<File>) -> Result<(u64, Option<u64>, u64)> {
        // Skip EBML header
        let (id, size) = Self::read_element_header(reader)?;
        if id != EBML_HEADER {
            anyhow::bail!("Not an EBML file");
        }
        reader.seek(SeekFrom::Current(size as i64))?;

        // Find Segment
        let (id, _seg_size) = Self::read_element_header(reader)?;
        if id != SEGMENT {
            anyhow::bail!("No Segment element found");
        }

        let segment_start = reader.stream_position()?;

        // Default values
        let mut timestamp_scale: u64 = 1_000_000; // 1ms default
        let mut duration_ms: Option<u64> = None;

        // Scan for SegmentInfo (limited range)
        let scan_limit = segment_start + 10 * 1024 * 1024; // First 10MB

        while reader.stream_position()? < scan_limit {
            let (id, size) = match Self::read_element_header(reader) {
                Ok(v) => v,
                Err(_) => break,
            };

            if id == SEGMENT_INFO {
                let info_end = reader.stream_position()? + size;

                while reader.stream_position()? < info_end {
                    let (sub_id, sub_size) = Self::read_element_header(reader)?;

                    match sub_id {
                        TIMESTAMP_SCALE => {
                            timestamp_scale = Self::read_uint(reader, sub_size)?;
                        }
                        DURATION => {
                            // Duration is stored as float
                            let raw = Self::read_float(reader, sub_size)?;
                            duration_ms = Some(((raw * timestamp_scale as f64) / 1_000_000.0) as u64);
                        }
                        _ => {
                            reader.seek(SeekFrom::Current(sub_size as i64))?;
                        }
                    }
                }
                break;
            } else if id == CLUSTER {
                // Passed SegmentInfo, stop
                break;
            } else {
                reader.seek(SeekFrom::Current(size as i64))?;
            }
        }

        Ok((timestamp_scale, duration_ms, segment_start))
    }

    // --- EBML primitives ---

    fn read_element_header(reader: &mut BufReader<File>) -> Result<(u32, u64)> {
        let (id, _) = Self::read_element_id(reader)?;
        let (size, _) = Self::read_vint(reader)?;
        Ok((id, size))
    }

    fn read_element_id(reader: &mut BufReader<File>) -> Result<(u32, u64)> {
        let first = Self::read_byte(reader)?;

        let (len, mask) = if first & 0x80 != 0 {
            (1, 0x7F)
        } else if first & 0x40 != 0 {
            (2, 0x3F)
        } else if first & 0x20 != 0 {
            (3, 0x1F)
        } else if first & 0x10 != 0 {
            (4, 0x0F)
        } else {
            anyhow::bail!("Invalid EBML element ID");
        };

        let mut id = (first & mask) as u32;
        for _ in 1..len {
            id = (id << 8) | (Self::read_byte(reader)? as u32);
        }

        // Reconstruct full ID with length marker
        let full_id = match len {
            1 => 0x80 | id,
            2 => 0x4000 | id,
            3 => 0x200000 | id,
            4 => 0x10000000 | id,
            _ => id,
        };

        Ok((full_id, len))
    }

    fn read_vint(reader: &mut BufReader<File>) -> Result<(u64, u64)> {
        let first = Self::read_byte(reader)?;

        let (len, mask) = if first & 0x80 != 0 {
            (1, 0x7F)
        } else if first & 0x40 != 0 {
            (2, 0x3F)
        } else if first & 0x20 != 0 {
            (3, 0x1F)
        } else if first & 0x10 != 0 {
            (4, 0x0F)
        } else if first & 0x08 != 0 {
            (5, 0x07)
        } else if first & 0x04 != 0 {
            (6, 0x03)
        } else if first & 0x02 != 0 {
            (7, 0x01)
        } else if first & 0x01 != 0 {
            (8, 0x00)
        } else {
            anyhow::bail!("Invalid VINT");
        };

        let mut value = (first & mask) as u64;
        for _ in 1..len {
            value = (value << 8) | (Self::read_byte(reader)? as u64);
        }

        Ok((value, len))
    }

    fn read_uint(reader: &mut BufReader<File>, size: u64) -> Result<u64> {
        let mut value: u64 = 0;
        for _ in 0..size {
            value = (value << 8) | (Self::read_byte(reader)? as u64);
        }
        Ok(value)
    }

    fn read_float(reader: &mut BufReader<File>, size: u64) -> Result<f64> {
        match size {
            4 => {
                let mut buf = [0u8; 4];
                reader.read_exact(&mut buf)?;
                Ok(f32::from_be_bytes(buf) as f64)
            }
            8 => {
                let mut buf = [0u8; 8];
                reader.read_exact(&mut buf)?;
                Ok(f64::from_be_bytes(buf))
            }
            _ => anyhow::bail!("Invalid float size: {}", size),
        }
    }

    fn read_byte(reader: &mut BufReader<File>) -> Result<u8> {
        let mut buf = [0u8; 1];
        reader.read_exact(&mut buf)?;
        Ok(buf[0])
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cache_lookup() {
        // Create a cache with some pre-populated clusters
        let mut cache = ClusterCache {
            path: PathBuf::from("/nonexistent"),
            file_size: 1_000_000_000,
            timestamp_scale: 1_000_000,
            duration_ms: Some(7200000), // 2 hours
            segment_start: 1000,
            clusters: BTreeMap::new(),
            fully_indexed: false,
        };

        // Add some clusters
        cache.clusters.insert(0, CachedCluster {
            timestamp_ms: 0,
            element_offset: 1000,
            data_offset: 1010,
            size: 50000,
        });
        cache.clusters.insert(30000, CachedCluster {
            timestamp_ms: 30000,
            element_offset: 51010,
            data_offset: 51020,
            size: 60000,
        });
        cache.clusters.insert(60000, CachedCluster {
            timestamp_ms: 60000,
            element_offset: 111020,
            data_offset: 111030,
            size: 55000,
        });

        // Test exact match
        assert_eq!(cache.find_in_cache(30000).unwrap().timestamp_ms, 30000);

        // Test between clusters
        assert_eq!(cache.find_in_cache(45000).unwrap().timestamp_ms, 30000);

        // Test before first
        assert_eq!(cache.find_in_cache(0).unwrap().timestamp_ms, 0);

        // Test after last
        assert_eq!(cache.find_in_cache(90000).unwrap().timestamp_ms, 60000);
    }
}
