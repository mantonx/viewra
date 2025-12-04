//! Low-level EBML/MKV parsing for building sparse cluster indexes.
//!
//! This module provides minimal EBML parsing to scan MKV files and build
//! a cluster-level index. This bypasses the container's Cues element (which
//! may be malformed) and gives us byte offsets to clusters for fast seeking.
//!
//! The key insight: we don't need per-subtitle-frame indexes. We just need to
//! know where clusters start and their timestamps. Then we can seek to the
//! right cluster and let matroska-demuxer iterate within it.

use anyhow::{Context, Result};
use std::fs::File;
use std::io::{BufReader, Read, Seek, SeekFrom};
use std::path::Path;

// EBML Element IDs (Matroska)
const EBML_HEADER: u32 = 0x1A45DFA3;
const SEGMENT: u32 = 0x18538067;
const SEGMENT_INFO: u32 = 0x1549A966;
const TIMESTAMP_SCALE: u32 = 0x2AD7B1;
const CLUSTER: u32 = 0x1F43B675;
const CLUSTER_TIMESTAMP: u32 = 0xE7;
const SIMPLE_BLOCK: u32 = 0xA3;
const BLOCK_GROUP: u32 = 0xA0;
const BLOCK: u32 = 0xA1;
const BLOCK_DURATION: u32 = 0x9B;

/// A cluster index entry - points to a cluster start position
#[derive(Debug, Clone)]
pub struct ClusterEntry {
    /// Cluster timestamp in milliseconds
    pub timestamp_ms: u64,
    /// Byte offset in the file where this cluster's data starts (after header)
    pub file_offset: u64,
    /// Size of the cluster data
    pub cluster_size: u64,
}

/// A raw frame extracted from the container
#[derive(Debug, Clone, Default)]
pub struct RawFrame {
    /// Track number this frame belongs to
    pub track: u64,
    /// Timestamp in raw units (needs scaling)
    pub timestamp: u64,
    /// Duration in raw units (needs scaling), if available
    pub duration: Option<u64>,
    /// Frame data
    pub data: Vec<u8>,
}

/// Cluster-level index for fast seeking
#[derive(Debug)]
pub struct ClusterIndex {
    /// Sorted list of cluster entries
    pub clusters: Vec<ClusterEntry>,
    /// Timestamp scale (nanoseconds per timestamp unit)
    pub timestamp_scale: u64,
}

impl ClusterIndex {
    /// Find the cluster at or before the given timestamp (for seeking)
    #[allow(dead_code)]
    pub fn find_cluster(&self, timestamp_ms: u64) -> Option<&ClusterEntry> {
        if self.clusters.is_empty() {
            return None;
        }

        match self.clusters.binary_search_by_key(&timestamp_ms, |c| c.timestamp_ms) {
            Ok(idx) => Some(&self.clusters[idx]),
            Err(idx) => {
                if idx == 0 {
                    Some(&self.clusters[0])
                } else {
                    Some(&self.clusters[idx - 1])
                }
            }
        }
    }
}


/// EBML scanner for building sparse cluster indexes
pub struct EbmlScanner {
    reader: BufReader<File>,
    file_size: u64,
}

impl EbmlScanner {
    pub fn open(path: &Path) -> Result<Self> {
        let file = File::open(path).context("Failed to open file")?;
        let file_size = file.metadata()?.len();
        // Use 4MB buffer for network filesystems - minimizes round trips
        let reader = BufReader::with_capacity(4 * 1024 * 1024, file);

        Ok(Self { reader, file_size })
    }

    /// Build a cluster-level index for the file
    ///
    /// This is FAST because we only read cluster headers (element ID + size + timestamp),
    /// not the actual block data. We skip over cluster contents using seek.
    pub fn build_cluster_index(&mut self) -> Result<ClusterIndex> {
        self.reader.seek(SeekFrom::Start(0))?;

        // First, parse header and get timestamp scale
        let timestamp_scale = self.find_timestamp_scale()?;

        // Reset and scan for clusters
        self.reader.seek(SeekFrom::Start(0))?;
        let clusters = self.scan_clusters(timestamp_scale)?;

        Ok(ClusterIndex {
            clusters,
            timestamp_scale,
        })
    }

    fn find_timestamp_scale(&mut self) -> Result<u64> {
        // Skip EBML header
        let (id, size) = self.read_element_header()?;
        if id != EBML_HEADER {
            anyhow::bail!("Not an EBML file");
        }
        self.skip_bytes(size)?;

        // Find Segment
        let (id, _) = self.read_element_header()?;
        if id != SEGMENT {
            anyhow::bail!("No Segment element found");
        }

        // Default timestamp scale is 1,000,000 ns = 1ms
        let mut timestamp_scale: u64 = 1_000_000;

        // Scan for SegmentInfo
        while self.reader.stream_position()? < self.file_size {
            let (id, size) = match self.read_element_header() {
                Ok(h) => h,
                Err(_) => break,
            };

            if id == SEGMENT_INFO {
                let info_end = self.reader.stream_position()? + size;
                while self.reader.stream_position()? < info_end {
                    let (sub_id, sub_size) = self.read_element_header()?;
                    if sub_id == TIMESTAMP_SCALE {
                        timestamp_scale = self.read_uint(sub_size)?;
                        return Ok(timestamp_scale);
                    } else {
                        self.skip_bytes(sub_size)?;
                    }
                }
                break;
            } else if id == CLUSTER {
                // Past SegmentInfo, use default
                break;
            } else {
                self.skip_bytes(size)?;
            }
        }

        Ok(timestamp_scale)
    }

    /// Scan file for clusters - only reads cluster headers, skips content
    ///
    /// This is the key optimization: instead of reading every block header,
    /// we just read the cluster element header, peek at the timestamp, then
    /// skip the entire cluster content. This reduces I/O dramatically.
    fn scan_clusters(&mut self, timestamp_scale: u64) -> Result<Vec<ClusterEntry>> {
        let mut clusters = Vec::new();

        // Skip EBML header
        let (id, size) = self.read_element_header()?;
        if id != EBML_HEADER {
            anyhow::bail!("Not an EBML file");
        }
        self.skip_bytes(size)?;

        // Find Segment
        let (id, _) = self.read_element_header()?;
        if id != SEGMENT {
            anyhow::bail!("No Segment element found");
        }

        // Scan for clusters
        while self.reader.stream_position()? < self.file_size {
            let _element_start = self.reader.stream_position()?;
            let (id, size) = match self.read_element_header() {
                Ok(h) => h,
                Err(_) => break,
            };

            if id == CLUSTER {
                let cluster_data_start = self.reader.stream_position()?;
                let cluster_end = cluster_data_start + size;

                // Read the cluster timestamp (usually first element in cluster)
                let mut cluster_timestamp_ms: Option<u64> = None;

                // Peek at first few elements to find timestamp
                let peek_limit = cluster_data_start + size.min(64); // First 64 bytes max
                while self.reader.stream_position()? < peek_limit {
                    let (sub_id, sub_size) = match self.read_element_header() {
                        Ok(h) => h,
                        Err(_) => break,
                    };

                    if sub_id == CLUSTER_TIMESTAMP {
                        let raw_ts = self.read_uint(sub_size)?;
                        cluster_timestamp_ms = Some((raw_ts * timestamp_scale) / 1_000_000);
                        break;
                    } else {
                        // Skip this element
                        self.skip_bytes(sub_size)?;
                    }
                }

                // Add cluster to index (use 0 if no timestamp found)
                clusters.push(ClusterEntry {
                    timestamp_ms: cluster_timestamp_ms.unwrap_or(0),
                    file_offset: cluster_data_start,
                    cluster_size: size,
                });

                // Skip rest of cluster content
                self.reader.seek(SeekFrom::Start(cluster_end))?;
            } else {
                // Skip non-cluster elements
                self.skip_bytes(size)?;
            }
        }

        // Sort by timestamp (should already be sorted, but ensure it)
        clusters.sort_by_key(|c| c.timestamp_ms);

        Ok(clusters)
    }

    /// Read EBML element header (ID + size)
    fn read_element_header(&mut self) -> Result<(u32, u64)> {
        let (id, _) = self.read_element_id()?;
        let (size, _) = self.read_vint()?;
        Ok((id, size))
    }

    /// Read variable-length element ID
    fn read_element_id(&mut self) -> Result<(u32, u64)> {
        let first = self.read_byte()?;

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
            id = (id << 8) | (self.read_byte()? as u32);
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

    /// Read variable-length integer (VINT)
    fn read_vint(&mut self) -> Result<(u64, u64)> {
        let first = self.read_byte()?;

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
            value = (value << 8) | (self.read_byte()? as u64);
        }

        Ok((value, len))
    }

    /// Read unsigned integer of given size
    fn read_uint(&mut self, size: u64) -> Result<u64> {
        let mut value: u64 = 0;
        for _ in 0..size {
            value = (value << 8) | (self.read_byte()? as u64);
        }
        Ok(value)
    }

    fn read_byte(&mut self) -> Result<u8> {
        let mut buf = [0u8; 1];
        self.reader.read_exact(&mut buf)?;
        Ok(buf[0])
    }

    fn skip_bytes(&mut self, count: u64) -> Result<()> {
        self.reader.seek(SeekFrom::Current(count as i64))?;
        Ok(())
    }
}

/// Frame streamer that can seek to clusters and extract frames
pub struct FrameStreamer {
    reader: BufReader<File>,
    file_size: u64,
    timestamp_scale: u64,
    /// Current cluster timestamp (ms)
    cluster_timestamp_ms: u64,
    /// End of current cluster
    cluster_end: u64,
}

impl FrameStreamer {
    /// Open a file and parse header info
    pub fn open(path: &Path) -> Result<Self> {
        let file = File::open(path).context("Failed to open file")?;
        let file_size = file.metadata()?.len();
        let mut reader = BufReader::with_capacity(256 * 1024, file);

        // Parse timestamp scale from header
        let timestamp_scale = Self::parse_timestamp_scale(&mut reader)?;

        Ok(Self {
            reader,
            file_size,
            timestamp_scale,
            cluster_timestamp_ms: 0,
            cluster_end: 0,
        })
    }

    /// Get timestamp scale
    #[allow(dead_code)]
    pub fn timestamp_scale(&self) -> u64 {
        self.timestamp_scale
    }

    /// Seek to a specific byte offset (cluster data start position)
    pub fn seek_to_cluster(&mut self, cluster_offset: u64, cluster_size: u64) -> Result<()> {
        self.reader.seek(SeekFrom::Start(cluster_offset))?;
        self.cluster_end = cluster_offset + cluster_size;
        self.cluster_timestamp_ms = 0;

        // Read cluster timestamp (should be first element)
        let peek_end = cluster_offset + cluster_size.min(64);
        while self.reader.stream_position()? < peek_end {
            let (id, size) = match Self::read_element_header(&mut self.reader) {
                Ok(v) => v,
                Err(_) => break,
            };

            if id == CLUSTER_TIMESTAMP {
                let raw_ts = Self::read_uint(&mut self.reader, size)?;
                self.cluster_timestamp_ms = (raw_ts * self.timestamp_scale) / 1_000_000;
                break;
            } else {
                self.reader.seek(SeekFrom::Current(size as i64))?;
            }
        }

        Ok(())
    }

    /// Read the next frame from the current cluster, or scan forward to find more clusters
    pub fn next_frame(&mut self, frame: &mut RawFrame) -> Result<bool> {
        loop {
            // Check if we're past the current cluster
            let pos = self.reader.stream_position()?;
            if pos >= self.cluster_end {
                // Try to find next cluster
                if !self.find_next_cluster()? {
                    return Ok(false);
                }
                continue;
            }

            // Read next element in cluster
            let (id, size) = match Self::read_element_header(&mut self.reader) {
                Ok(v) => v,
                Err(_) => {
                    // Try next cluster
                    if !self.find_next_cluster()? {
                        return Ok(false);
                    }
                    continue;
                }
            };

            match id {
                SIMPLE_BLOCK => {
                    // SimpleBlock: track + timecode + flags + data
                    if Self::parse_simple_block(&mut self.reader, size, self.cluster_timestamp_ms, frame)? {
                        return Ok(true);
                    }
                }
                BLOCK_GROUP => {
                    // BlockGroup contains Block + optional BlockDuration
                    let group_end = self.reader.stream_position()? + size;
                    let mut block_data: Option<(u64, Vec<u8>)> = None;
                    let mut block_duration: Option<u64> = None;

                    while self.reader.stream_position()? < group_end {
                        let (sub_id, sub_size) = Self::read_element_header(&mut self.reader)?;

                        match sub_id {
                            BLOCK => {
                                // Read block header (track + timecode + flags)
                                let (track, timecode, data) = self.read_block_data(sub_size)?;
                                block_data = Some((track, data));
                                // Calculate timestamp
                                let timestamp = self.cluster_timestamp_ms as i64 + timecode as i64;
                                frame.timestamp = timestamp.max(0) as u64;
                                frame.track = track;
                            }
                            BLOCK_DURATION => {
                                block_duration = Some(Self::read_uint(&mut self.reader, sub_size)?);
                            }
                            _ => {
                                self.reader.seek(SeekFrom::Current(sub_size as i64))?;
                            }
                        }
                    }

                    if let Some((track, data)) = block_data {
                        frame.track = track;
                        frame.data = data;
                        frame.duration = block_duration;
                        return Ok(true);
                    }
                }
                CLUSTER_TIMESTAMP => {
                    // Update cluster timestamp
                    let raw_ts = Self::read_uint(&mut self.reader, size)?;
                    self.cluster_timestamp_ms = (raw_ts * self.timestamp_scale) / 1_000_000;
                }
                _ => {
                    // Skip unknown elements
                    self.reader.seek(SeekFrom::Current(size as i64))?;
                }
            }
        }
    }

    /// Parse a SimpleBlock element (static method)
    fn parse_simple_block(reader: &mut BufReader<File>, size: u64, cluster_ts_ms: u64, frame: &mut RawFrame) -> Result<bool> {
        if size < 4 {
            return Ok(false);
        }

        // Read track number (variable length)
        let (track, track_len) = Self::read_vint_from(reader)?;

        // Read relative timecode (signed 16-bit)
        let mut tc_buf = [0u8; 2];
        reader.read_exact(&mut tc_buf)?;
        let timecode = i16::from_be_bytes(tc_buf);

        // Read flags (1 byte)
        let mut flags = [0u8; 1];
        reader.read_exact(&mut flags)?;

        // Read frame data
        let data_size = size - track_len - 3;
        let mut data = vec![0u8; data_size as usize];
        reader.read_exact(&mut data)?;

        // Calculate absolute timestamp
        let timestamp = cluster_ts_ms as i64 + timecode as i64;

        frame.track = track;
        frame.timestamp = timestamp.max(0) as u64;
        frame.duration = None;
        frame.data = data;

        Ok(true)
    }

    /// Read block data (track, timecode, data) without duration
    fn read_block_data(&mut self, size: u64) -> Result<(u64, i16, Vec<u8>)> {
        if size < 4 {
            anyhow::bail!("Block too small");
        }

        let (track, track_len) = Self::read_vint_from(&mut self.reader)?;

        let mut tc_buf = [0u8; 2];
        self.reader.read_exact(&mut tc_buf)?;
        let timecode = i16::from_be_bytes(tc_buf);

        // Skip flags
        let mut flags = [0u8; 1];
        self.reader.read_exact(&mut flags)?;

        let data_size = size - track_len - 3;
        let mut data = vec![0u8; data_size as usize];
        self.reader.read_exact(&mut data)?;

        Ok((track, timecode, data))
    }

    /// Find the next cluster in the file
    fn find_next_cluster(&mut self) -> Result<bool> {
        // Scan for cluster signature
        let cluster_sig = [0x1F, 0x43, 0xB6, 0x75];
        let mut window = [0u8; 4];

        // Start from current position
        let start_pos = self.reader.stream_position()?;
        if start_pos >= self.file_size {
            return Ok(false);
        }

        if self.reader.read_exact(&mut window).is_err() {
            return Ok(false);
        }

        let scan_limit = 10 * 1024 * 1024; // Scan up to 10MB
        let mut scanned = 4u64;

        loop {
            if window == cluster_sig {
                // Found cluster - back up and read properly
                let cluster_start = start_pos + scanned - 4;
                self.reader.seek(SeekFrom::Start(cluster_start))?;

                // Read cluster header
                let (id, size) = Self::read_element_header(&mut self.reader)?;
                if id == CLUSTER {
                    let data_start = self.reader.stream_position()?;
                    self.cluster_end = data_start + size;

                    // Read cluster timestamp
                    self.cluster_timestamp_ms = 0;
                    let peek_end = data_start + size.min(64);
                    while self.reader.stream_position()? < peek_end {
                        let (sub_id, sub_size) = match Self::read_element_header(&mut self.reader) {
                            Ok(v) => v,
                            Err(_) => break,
                        };

                        if sub_id == CLUSTER_TIMESTAMP {
                            let raw_ts = Self::read_uint(&mut self.reader, sub_size)?;
                            self.cluster_timestamp_ms = (raw_ts * self.timestamp_scale) / 1_000_000;
                            break;
                        } else {
                            self.reader.seek(SeekFrom::Current(sub_size as i64))?;
                        }
                    }

                    return Ok(true);
                }
            }

            if scanned >= scan_limit || start_pos + scanned >= self.file_size {
                return Ok(false);
            }

            // Slide window
            window[0] = window[1];
            window[1] = window[2];
            window[2] = window[3];
            let mut byte = [0u8; 1];
            if self.reader.read_exact(&mut byte).is_err() {
                return Ok(false);
            }
            window[3] = byte[0];
            scanned += 1;
        }
    }

    fn parse_timestamp_scale(reader: &mut BufReader<File>) -> Result<u64> {
        reader.seek(SeekFrom::Start(0))?;

        // Skip EBML header
        let (id, size) = Self::read_element_header(reader)?;
        if id != EBML_HEADER {
            anyhow::bail!("Not an EBML file");
        }
        reader.seek(SeekFrom::Current(size as i64))?;

        // Find Segment
        let (id, _) = Self::read_element_header(reader)?;
        if id != SEGMENT {
            anyhow::bail!("No Segment element found");
        }

        let file_size = reader.get_ref().metadata()?.len();
        let mut timestamp_scale: u64 = 1_000_000;

        // Scan for SegmentInfo
        while reader.stream_position()? < file_size {
            let (id, size) = match Self::read_element_header(reader) {
                Ok(h) => h,
                Err(_) => break,
            };

            if id == SEGMENT_INFO {
                let info_end = reader.stream_position()? + size;
                while reader.stream_position()? < info_end {
                    let (sub_id, sub_size) = Self::read_element_header(reader)?;
                    if sub_id == TIMESTAMP_SCALE {
                        timestamp_scale = Self::read_uint(reader, sub_size)?;
                        return Ok(timestamp_scale);
                    } else {
                        reader.seek(SeekFrom::Current(sub_size as i64))?;
                    }
                }
                break;
            } else if id == CLUSTER {
                break;
            } else {
                reader.seek(SeekFrom::Current(size as i64))?;
            }
        }

        Ok(timestamp_scale)
    }

    fn read_element_header(reader: &mut BufReader<File>) -> Result<(u32, u64)> {
        let (id, _) = Self::read_element_id(reader)?;
        let (size, _) = Self::read_vint_from(reader)?;
        Ok((id, size))
    }

    fn read_element_id(reader: &mut BufReader<File>) -> Result<(u32, u64)> {
        let mut first = [0u8; 1];
        reader.read_exact(&mut first)?;
        let first = first[0];

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
            let mut b = [0u8; 1];
            reader.read_exact(&mut b)?;
            id = (id << 8) | (b[0] as u32);
        }

        let full_id = match len {
            1 => 0x80 | id,
            2 => 0x4000 | id,
            3 => 0x200000 | id,
            4 => 0x10000000 | id,
            _ => id,
        };

        Ok((full_id, len))
    }

    fn read_vint_from<R: Read>(reader: &mut R) -> Result<(u64, u64)> {
        let mut first = [0u8; 1];
        reader.read_exact(&mut first)?;
        let first = first[0];

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
            let mut b = [0u8; 1];
            reader.read_exact(&mut b)?;
            value = (value << 8) | (b[0] as u64);
        }

        Ok((value, len))
    }

    fn read_uint<R: Read>(reader: &mut R, size: u64) -> Result<u64> {
        let mut value: u64 = 0;
        for _ in 0..size {
            let mut b = [0u8; 1];
            reader.read_exact(&mut b)?;
            value = (value << 8) | (b[0] as u64);
        }
        Ok(value)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cluster_index_binary_search() {
        let index = ClusterIndex {
            timestamp_scale: 1_000_000,
            clusters: vec![
                ClusterEntry { timestamp_ms: 0, file_offset: 100, cluster_size: 50000 },
                ClusterEntry { timestamp_ms: 5000, file_offset: 50100, cluster_size: 60000 },
                ClusterEntry { timestamp_ms: 10000, file_offset: 110100, cluster_size: 70000 },
            ],
        };

        // Exact match
        assert_eq!(index.find_cluster(5000).unwrap().file_offset, 50100);

        // Between clusters - should return earlier one
        assert_eq!(index.find_cluster(7000).unwrap().file_offset, 50100);

        // Before first cluster
        assert_eq!(index.find_cluster(0).unwrap().file_offset, 100);

        // After last cluster
        assert_eq!(index.find_cluster(15000).unwrap().file_offset, 110100);
    }
}
