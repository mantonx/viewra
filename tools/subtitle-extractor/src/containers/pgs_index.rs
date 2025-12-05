//! PGS frame index for fast seeking
//!
//! This module provides indexing of PGS frame byte positions within MKV containers.
//! Instead of scanning through clusters to find subtitle frames, we can:
//! 1. Build an index once (scanning all clusters)
//! 2. Use the index for O(1) seeking to any frame by byte offset
//!
//! The index is designed for progressive building - you can index the first N minutes
//! of a file, then continue indexing later as the user watches more of the video.

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::fs::File;
use std::io::{BufReader, Read, Seek, SeekFrom};
use std::path::{Path, PathBuf};

use super::cluster_cache::ClusterCache;
use super::ebml_primitives::{read_element_header, read_uint, read_vint, ids};

/// A single PGS frame entry in the index
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgsFrameEntry {
    /// Start time in milliseconds
    pub start_ms: u64,
    /// Byte offset in the file where the frame data starts
    pub offset: u64,
    /// Size of the frame data in bytes
    pub size: u64,
}

/// PGS frame index for a specific track
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgsIndex {
    /// Index format version (for future compatibility)
    pub version: u32,
    /// File size when index was built (to detect file changes)
    pub file_size: u64,
    /// Track number this index is for
    pub track: u64,
    /// How far into the file we've indexed (in milliseconds)
    /// Used for progressive indexing - can continue from here
    pub indexed_through_ms: u64,
    /// All indexed frame entries, sorted by start_ms
    pub frames: Vec<PgsFrameEntry>,
}

impl PgsIndex {
    /// Create a new empty index
    pub fn new(file_size: u64, track: u64) -> Self {
        Self {
            version: 1,
            file_size,
            track,
            indexed_through_ms: 0,
            frames: Vec::new(),
        }
    }

    /// Find frames within a time range using binary search
    #[allow(dead_code)]
    pub fn frames_in_range(&self, start_ms: u64, end_ms: u64) -> Vec<&PgsFrameEntry> {
        // Binary search for start position
        let start_idx = match self.frames.binary_search_by_key(&start_ms, |f| f.start_ms) {
            Ok(idx) => idx,
            Err(idx) => {
                // If not exact match, we might need to include the previous frame
                // (if its duration extends into our range)
                if idx > 0 { idx - 1 } else { idx }
            }
        };

        // Collect frames until we pass end_ms
        self.frames[start_idx..]
            .iter()
            .take_while(|f| end_ms == 0 || f.start_ms < end_ms)
            .collect()
    }

    /// Check if this index covers the requested time range
    #[allow(dead_code)]
    pub fn covers_range(&self, _start_ms: u64, end_ms: u64) -> bool {
        if end_ms == 0 {
            // End=0 means "until end of file" - we can't know if we cover it
            // unless we've indexed the entire file
            false
        } else {
            end_ms <= self.indexed_through_ms
        }
    }
}

/// Builder for creating PGS frame indexes
pub struct PgsIndexBuilder {
    path: PathBuf,
    track: u64,
    file_size: u64,
}

impl PgsIndexBuilder {
    /// Open a file for indexing
    pub fn open(path: &Path, track: u64) -> Result<Self> {
        let file = File::open(path).context("Failed to open file")?;
        let file_size = file.metadata()?.len();

        Ok(Self {
            path: path.to_path_buf(),
            track,
            file_size,
        })
    }

    /// Build index for a time range
    ///
    /// If `start_ms` is non-zero, this continues from a previous partial index.
    /// If `end_ms` is 0, indexes until end of file.
    pub fn build_index(&mut self, start_ms: u64, end_ms: u64) -> Result<PgsIndex> {
        let mut index = PgsIndex::new(self.file_size, self.track);

        // Initialize cluster cache for efficient seeking
        let mut cluster_cache = ClusterCache::open(&self.path)?;

        self.scan_for_frames(&mut index, &mut cluster_cache, start_ms, end_ms)?;

        Ok(index)
    }

    /// Scan file for PGS frames and record their positions
    fn scan_for_frames(
        &self,
        index: &mut PgsIndex,
        cluster_cache: &mut ClusterCache,
        start_ms: u64,
        end_ms: u64,
    ) -> Result<()> {
        // Get cluster info for seeking
        let start_cluster = cluster_cache.find_cluster(start_ms)?;
        let timestamp_scale = cluster_cache.timestamp_scale();

        // Open file for direct scanning
        let file = File::open(&self.path)?;
        let mut reader = BufReader::with_capacity(512 * 1024, file);

        // Start from cluster or beginning
        let scan_start = start_cluster
            .map(|c| c.data_offset)
            .unwrap_or(0);

        if scan_start > 0 {
            reader.seek(SeekFrom::Start(scan_start))?;
        }

        // Use shared EBML element IDs
        use ids::{CLUSTER, CLUSTER_TIMESTAMP, SIMPLE_BLOCK, BLOCK_GROUP, BLOCK};

        let mut cluster_timestamp_ms: u64 = 0;
        let mut last_timestamp_ms: u64 = 0;

        // Simple state machine for scanning
        loop {
            let pos = reader.stream_position()?;
            if pos >= self.file_size {
                break;
            }

            // Read element header
            let (id, size) = match read_element_header(&mut reader) {
                Ok(v) => v,
                Err(_) => break,
            };

            match id {
                CLUSTER => {
                    // Enter cluster - read its timestamp
                    let _cluster_end = reader.stream_position()? + size;

                    // Find timestamp in cluster header
                    let peek_limit = reader.stream_position()? + size.min(64);
                    while reader.stream_position()? < peek_limit {
                        let (sub_id, sub_size) = match read_element_header(&mut reader) {
                            Ok(v) => v,
                            Err(_) => break,
                        };

                        if sub_id == CLUSTER_TIMESTAMP {
                            // Read raw timestamp and convert to ms using the file's timestamp scale
                            let raw_ts = read_uint(&mut reader, sub_size)?;
                            cluster_timestamp_ms = (raw_ts * timestamp_scale) / 1_000_000;
                            break;
                        } else {
                            reader.seek(SeekFrom::Current(sub_size as i64))?;
                        }
                    }

                    // Check if we've passed end time at cluster level
                    if end_ms > 0 && cluster_timestamp_ms > end_ms {
                        index.indexed_through_ms = last_timestamp_ms;
                        return Ok(());
                    }

                    // Continue scanning within cluster - don't skip
                    // We need to parse blocks to find our track
                }

                SIMPLE_BLOCK => {
                    let block_start = reader.stream_position()?;

                    // Read track number (VINT)
                    let (track_num, _track_len) = read_vint(&mut reader)?;

                    if track_num == self.track {
                        // This is our track! Read timing info
                        let mut tc_buf = [0u8; 2];
                        reader.read_exact(&mut tc_buf)?;
                        let timecode = i16::from_be_bytes(tc_buf);

                        // Calculate absolute timestamp
                        let frame_start_ms = (cluster_timestamp_ms as i64 + timecode as i64).max(0) as u64;

                        // Skip if before our start
                        if frame_start_ms >= start_ms {
                            // Record frame position (at the data start, after track+timecode+flags)
                            let data_offset = block_start; // Point to start of block data
                            let data_size = size; // Full block size

                            index.frames.push(PgsFrameEntry {
                                start_ms: frame_start_ms,
                                offset: data_offset,
                                size: data_size,
                            });

                            last_timestamp_ms = frame_start_ms;
                        }
                    }

                    // Skip rest of block
                    let remaining = size - (reader.stream_position()? - block_start);
                    reader.seek(SeekFrom::Current(remaining as i64))?;
                }

                BLOCK_GROUP => {
                    // Need to parse inside to find Block element
                    let group_end = reader.stream_position()? + size;

                    while reader.stream_position()? < group_end {
                        let (sub_id, sub_size) = match read_element_header(&mut reader) {
                            Ok(v) => v,
                            Err(_) => break,
                        };

                        if sub_id == BLOCK {
                            let block_start = reader.stream_position()?;

                            // Read track number
                            let (track_num, _) = read_vint(&mut reader)?;

                            if track_num == self.track {
                                // Read timecode
                                let mut tc_buf = [0u8; 2];
                                reader.read_exact(&mut tc_buf)?;
                                let timecode = i16::from_be_bytes(tc_buf);

                                let frame_start_ms = (cluster_timestamp_ms as i64 + timecode as i64).max(0) as u64;

                                if frame_start_ms >= start_ms {
                                    index.frames.push(PgsFrameEntry {
                                        start_ms: frame_start_ms,
                                        offset: block_start,
                                        size: sub_size,
                                    });

                                    last_timestamp_ms = frame_start_ms;
                                }
                            }

                            // Skip to end of block
                            let block_end = block_start + sub_size;
                            reader.seek(SeekFrom::Start(block_end))?;
                        } else {
                            reader.seek(SeekFrom::Current(sub_size as i64))?;
                        }
                    }
                }

                _ => {
                    // Skip unknown elements
                    reader.seek(SeekFrom::Current(size as i64))?;
                }
            }
        }

        // Mark how far we indexed
        index.indexed_through_ms = if end_ms > 0 {
            end_ms
        } else {
            last_timestamp_ms
        };

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_index_frames_in_range() {
        let index = PgsIndex {
            version: 1,
            file_size: 1000000,
            track: 5,
            indexed_through_ms: 10000,
            frames: vec![
                PgsFrameEntry { start_ms: 1000, offset: 100, size: 50 },
                PgsFrameEntry { start_ms: 2000, offset: 200, size: 60 },
                PgsFrameEntry { start_ms: 3000, offset: 300, size: 70 },
                PgsFrameEntry { start_ms: 5000, offset: 500, size: 80 },
            ],
        };

        // Range covering middle frames
        let frames = index.frames_in_range(1500, 4000);
        assert_eq!(frames.len(), 3); // Should include 1000, 2000, 3000

        // Range from start
        let frames = index.frames_in_range(0, 2500);
        assert_eq!(frames.len(), 2); // 1000, 2000

        // Range to end (end_ms = 0)
        let frames = index.frames_in_range(2500, 0);
        assert_eq!(frames.len(), 3); // 2000, 3000, 5000 (includes 2000 as previous)
    }

    #[test]
    fn test_index_covers_range() {
        let index = PgsIndex {
            version: 1,
            file_size: 1000000,
            track: 5,
            indexed_through_ms: 10000,
            frames: vec![],
        };

        assert!(index.covers_range(0, 5000));
        assert!(index.covers_range(0, 10000));
        assert!(!index.covers_range(0, 15000));
        assert!(!index.covers_range(0, 0)); // end=0 means "to end of file"
    }
}
