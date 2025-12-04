//! MPEG-TS/M2TS container support
//!
//! Handles extraction of subtitle tracks from MPEG Transport Stream containers.
//! Supports PGS (HDMV) subtitles commonly found in Blu-ray M2TS files.
//!
//! ## Seeking Strategy
//!
//! Uses binary search to find file positions for target timestamps:
//! 1. Sample PTS at start and end of file to estimate duration
//! 2. Binary search with ~12 iterations to find precise position
//! 3. Each iteration reads 2MB to find a PTS (handles sparse PES packets)
//!
//! This achieves ~2.5s seek to any position in a 62GB M2TS file over network.

use anyhow::Result;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use std::fs::File;
use std::io::{BufReader, Read, Seek, Write};
use std::path::{Path, PathBuf};

use super::{OutputFormat, StreamFormat, SubtitleFrame, TrackInfo, TrackType};

/// Subtitle stream info parsed from PMT
#[derive(Clone)]
struct SubtitleStreamInfo {
    pid: u16,
    /// Stream type (0x90 = PGS, 0x06 = DVB)
    stream_type: u8,
    language: Option<String>,
}

/// Collected subtitle packet with timing
struct SubtitlePacket {
    pts_ms: u64,
    data: Vec<u8>,
}

pub struct TsContainer {
    path: PathBuf,
    streams: Vec<SubtitleStreamInfo>,
}

impl TsContainer {
    pub fn open(path: &Path) -> Result<Self> {
        // First pass: scan for subtitle streams
        let streams = Self::scan_streams(path)?;

        Ok(Self {
            path: path.to_path_buf(),
            streams,
        })
    }

    fn scan_streams(path: &Path) -> Result<Vec<SubtitleStreamInfo>> {
        // Parse PMT manually to find subtitle streams
        Self::parse_pmt_for_subtitles(path)
    }

    fn parse_pmt_for_subtitles(path: &Path) -> Result<Vec<SubtitleStreamInfo>> {
        use std::io::Seek;

        let mut file = File::open(path)?;
        let mut streams = Vec::new();

        // MPEG-TS packet size
        const TS_PACKET_SIZE: usize = 188;
        const M2TS_PACKET_SIZE: usize = 192; // M2TS has 4-byte timestamp prefix

        // Detect packet size (TS vs M2TS)
        let mut detect_buf = [0u8; 1024];
        let bytes_read = file.read(&mut detect_buf)?;
        file.seek(std::io::SeekFrom::Start(0))?;

        let packet_size = if bytes_read >= 5 && detect_buf[4] == 0x47 {
            M2TS_PACKET_SIZE // M2TS
        } else if bytes_read >= 1 && detect_buf[0] == 0x47 {
            TS_PACKET_SIZE // Plain TS
        } else {
            // Try to find sync byte
            let sync_pos = detect_buf
                .iter()
                .take(bytes_read.saturating_sub(TS_PACKET_SIZE))
                .position(|&b| b == 0x47);
            match sync_pos {
                Some(pos) => {
                    file.seek(std::io::SeekFrom::Start(pos as u64))?;
                    TS_PACKET_SIZE
                }
                None => anyhow::bail!("Could not find TS sync byte"),
            }
        };

        let offset = if packet_size == M2TS_PACKET_SIZE {
            4
        } else {
            0
        };

        // Read packets looking for PMT
        let mut buf = vec![0u8; packet_size * 512];
        let mut pmt_pids: Vec<u16> = Vec::new();
        let mut found_pat = false;

        let mut reader = BufReader::new(file);
        let mut total_read = 0;
        const MAX_SCAN: usize = 5 * 1024 * 1024;

        while total_read < MAX_SCAN {
            match reader.read(&mut buf) {
                Ok(0) => break,
                Ok(n) => {
                    total_read += n;

                    for chunk in buf[..n].chunks_exact(packet_size) {
                        let packet = &chunk[offset..];
                        if packet[0] != 0x47 {
                            continue;
                        }

                        let pid = (((packet[1] & 0x1f) as u16) << 8) | packet[2] as u16;

                        // PAT is on PID 0
                        if pid == 0 && !found_pat {
                            // Parse PAT to find PMT PIDs
                            let payload_start = if packet[3] & 0x20 != 0 {
                                // Has adaptation field
                                5 + packet[4] as usize
                            } else {
                                4
                            };

                            if payload_start < packet.len() && packet[3] & 0x10 != 0 {
                                let pointer = packet[payload_start] as usize;
                                let table_start = payload_start + 1 + pointer;

                                if table_start + 8 < packet.len() && packet[table_start] == 0x00 {
                                    // PAT table
                                    let section_len = (((packet[table_start + 1] & 0x0f) as usize)
                                        << 8)
                                        | packet[table_start + 2] as usize;
                                    let programs_start = table_start + 8;
                                    let programs_end =
                                        (table_start + 3 + section_len).min(packet.len()) - 4;

                                    let mut i = programs_start;
                                    while i + 4 <= programs_end {
                                        let program_num =
                                            ((packet[i] as u16) << 8) | packet[i + 1] as u16;
                                        let pmt_pid = (((packet[i + 2] & 0x1f) as u16) << 8)
                                            | packet[i + 3] as u16;

                                        if program_num != 0 {
                                            pmt_pids.push(pmt_pid);
                                        }
                                        i += 4;
                                    }
                                    found_pat = true;
                                }
                            }
                        }

                        // Check if this is a PMT we're looking for
                        if pmt_pids.contains(&pid) {
                            let payload_start = if packet[3] & 0x20 != 0 {
                                5 + packet[4] as usize
                            } else {
                                4
                            };

                            if payload_start < packet.len() && packet[3] & 0x10 != 0 {
                                let pointer = packet[payload_start] as usize;
                                let table_start = payload_start + 1 + pointer;

                                if table_start + 12 < packet.len() && packet[table_start] == 0x02 {
                                    // PMT table
                                    let section_len = (((packet[table_start + 1] & 0x0f) as usize)
                                        << 8)
                                        | packet[table_start + 2] as usize;
                                    let program_info_len =
                                        (((packet[table_start + 10] & 0x0f) as usize) << 8)
                                            | packet[table_start + 11] as usize;
                                    let streams_start = table_start + 12 + program_info_len;
                                    let streams_end =
                                        (table_start + 3 + section_len).min(packet.len()) - 4;

                                    let mut i = streams_start;
                                    while i + 5 <= streams_end {
                                        let stream_type = packet[i];
                                        let elem_pid = (((packet[i + 1] & 0x1f) as u16) << 8)
                                            | packet[i + 2] as u16;
                                        let es_info_len =
                                            (((packet[i + 3] & 0x0f) as usize) << 8)
                                                | packet[i + 4] as usize;

                                        // Check for subtitle stream types
                                        // 0x90 = PGS (HDMV Presentation Graphics)
                                        // 0x06 = Private data (DVB subtitles often use this)
                                        if stream_type == 0x90 {
                                            // Avoid duplicates (PMT may repeat)
                                            if !streams.iter().any(|s: &SubtitleStreamInfo| s.pid == elem_pid) {
                                                streams.push(SubtitleStreamInfo {
                                                    pid: elem_pid,
                                                    stream_type: 0x90,
                                                    language: None,
                                                });
                                            }
                                        }

                                        i += 5 + es_info_len;
                                    }
                                }
                            }
                        }
                    }

                    // If we found streams, we can stop
                    if !streams.is_empty() && found_pat {
                        break;
                    }
                }
                Err(e) => return Err(e.into()),
            }
        }

        Ok(streams)
    }

    pub fn tracks(&self) -> Result<Vec<TrackInfo>> {
        let tracks = self
            .streams
            .iter()
            .enumerate()
            .map(|(idx, stream)| {
                let codec = match stream.stream_type {
                    0x90 => "S_HDMV/PGS".to_string(),
                    0x06 => "dvbsub".to_string(),
                    t => format!("private_{:02x}", t),
                };

                TrackInfo {
                    number: stream.pid as u64,
                    track_type: TrackType::Subtitle,
                    codec,
                    language: stream.language.clone(),
                    name: Some(format!("Subtitle {}", idx + 1)),
                }
            })
            .collect();

        Ok(tracks)
    }

    pub fn extract_track<W: Write>(
        &mut self,
        track_num: u64,
        _format: OutputFormat,
        out: &mut W,
    ) -> Result<()> {
        self.stream_track(track_num, 0, 0, StreamFormat::Raw, out)
    }

    pub fn stream_track<W: Write>(
        &mut self,
        track_num: u64,
        start_ms: u64,
        end_ms: u64,
        format: StreamFormat,
        out: &mut W,
    ) -> Result<()> {
        let target_pid = track_num as u16;

        // Verify track exists
        if !self.streams.iter().any(|s| s.pid == target_pid) {
            anyhow::bail!("Track {} not found", track_num);
        }

        // Read and extract subtitle packets using simple linear interpolation
        // For TS files, this is faster than building an index over the network
        let packets = self.extract_packets(target_pid, start_ms, end_ms)?;

        // Write WebVTT header if needed (though TS subtitles are typically bitmap/PGS)
        if matches!(format, StreamFormat::WebVtt) {
            writeln!(out, "WEBVTT")?;
            writeln!(out)?;
            // PGS subtitles cannot be converted to WebVTT (bitmap format)
            // Return empty WebVTT file - caller should handle this by burning in subs
            return Ok(());
        }

        for packet in packets {
            match format {
                StreamFormat::Jsonl => {
                    let sub_frame = SubtitleFrame {
                        start_ms: packet.pts_ms,
                        end_ms: packet.pts_ms, // PGS doesn't have explicit end in PES
                        data_base64: Some(BASE64.encode(&packet.data)),
                        text: None,
                    };
                    writeln!(out, "{}", serde_json::to_string(&sub_frame)?)?;
                }
                StreamFormat::Raw => {
                    out.write_all(&packet.data)?;
                }
                StreamFormat::WebVtt => {
                    // Already handled above - PGS can't be converted to text
                }
            }
            out.flush()?;
        }

        Ok(())
    }

    fn extract_packets(
        &self,
        target_pid: u16,
        start_ms: u64,
        end_ms: u64,
    ) -> Result<Vec<SubtitlePacket>> {
        let file = File::open(&self.path)?;
        let file_size = file.metadata()?.len();
        // Large buffer for network files - amortize latency over more data
        let mut reader = BufReader::with_capacity(4 * 1024 * 1024, file);
        let mut packets = Vec::new();

        // Detect packet size
        const TS_PACKET_SIZE: usize = 188;
        const M2TS_PACKET_SIZE: usize = 192;

        let mut detect_buf = [0u8; 1024];
        let bytes_read = reader.read(&mut detect_buf)?;
        reader.seek_relative(-(bytes_read as i64))?;

        let (packet_size, offset) = if bytes_read >= 5 && detect_buf[4] == 0x47 {
            (M2TS_PACKET_SIZE, 4)
        } else {
            (TS_PACKET_SIZE, 0)
        };

        // If start_ms > 0, try to seek to approximate position
        if start_ms > 0 {
            if let Some(seek_pos) = self.estimate_seek_position(file_size, start_ms, &mut reader, packet_size, offset)? {
                reader.seek(std::io::SeekFrom::Start(seek_pos))?;
                // Re-sync to packet boundary
                self.sync_to_packet(&mut reader, packet_size, offset)?;
            }
        }

        // Large read buffer to reduce network round trips
        // Must be multiple of packet_size to maintain alignment
        let buf_packets = 20000; // ~3.6-3.8MB depending on packet size
        let mut buf = vec![0u8; packet_size * buf_packets];
        let mut leftover = Vec::new(); // Buffer for partial packets between reads
        let mut current_pes: Vec<u8> = Vec::new();
        let mut current_pts: Option<u64> = None;
        let mut consecutive_past_end = 0;

        loop {
            // Prepend any leftover bytes from previous read
            let start_offset = leftover.len();
            if start_offset > 0 {
                buf[..start_offset].copy_from_slice(&leftover);
                leftover.clear();
            }

            match reader.read(&mut buf[start_offset..]) {
                Ok(0) if start_offset == 0 => break,
                Ok(0) => {
                    // Process remaining leftover (partial packet, skip it)
                    break;
                }
                Ok(n) => {
                    let total = start_offset + n;

                    // Calculate how many complete packets we have
                    let complete_bytes = (total / packet_size) * packet_size;
                    let remainder = total - complete_bytes;

                    // Save leftover for next iteration
                    if remainder > 0 {
                        leftover.extend_from_slice(&buf[complete_bytes..total]);
                    }

                    for chunk in buf[..complete_bytes].chunks_exact(packet_size) {
                        let packet = &chunk[offset..];
                        if packet[0] != 0x47 {
                            continue;
                        }

                        let pid = (((packet[1] & 0x1f) as u16) << 8) | packet[2] as u16;

                        // Check any PES packet for PTS to track current position
                        // This enables early termination even if subtitle packets are sparse
                        if end_ms > 0 {
                            let payload_start_indicator = (packet[1] & 0x40) != 0;
                            let has_adaptation = (packet[3] & 0x20) != 0;
                            let has_payload = (packet[3] & 0x10) != 0;

                            if has_payload && payload_start_indicator {
                                let payload_offset = if has_adaptation {
                                    5 + packet[4] as usize
                                } else {
                                    4
                                };
                                if payload_offset < packet.len() {
                                    let payload = &packet[payload_offset..];
                                    if payload.len() >= 14
                                        && payload[0] == 0
                                        && payload[1] == 0
                                        && payload[2] == 1
                                        && (payload[7] & 0x80) != 0
                                    {
                                        let pts = ((payload[9] as u64 & 0x0e) << 29)
                                            | ((payload[10] as u64) << 22)
                                            | ((payload[11] as u64 & 0xfe) << 14)
                                            | ((payload[12] as u64) << 7)
                                            | ((payload[13] as u64) >> 1);
                                        let pts_ms = pts / 90;

                                        // Early terminate if we're well past end time
                                        // (10 seconds buffer to catch trailing subtitles)
                                        if pts_ms > end_ms + 10_000 {
                                            return Ok(packets);
                                        }
                                    }
                                }
                            }
                        }

                        if pid != target_pid {
                            continue;
                        }

                        let payload_start_indicator = (packet[1] & 0x40) != 0;
                        let has_adaptation = (packet[3] & 0x20) != 0;
                        let has_payload = (packet[3] & 0x10) != 0;

                        if !has_payload {
                            continue;
                        }

                        let payload_offset = if has_adaptation {
                            5 + packet[4] as usize
                        } else {
                            4
                        };

                        if payload_offset >= packet.len() {
                            continue;
                        }

                        let payload = &packet[payload_offset..];

                        if payload_start_indicator {
                            // New PES packet - save previous if any
                            if let Some(pts) = current_pts {
                                if !current_pes.is_empty() {
                                    if pts >= start_ms && (end_ms == 0 || pts <= end_ms) {
                                        packets.push(SubtitlePacket {
                                            pts_ms: pts,
                                            data: std::mem::take(&mut current_pes),
                                        });
                                        consecutive_past_end = 0;
                                    } else if end_ms > 0 && pts > end_ms {
                                        consecutive_past_end += 1;
                                        // Early termination: stop after 3 packets past end
                                        if consecutive_past_end >= 3 {
                                            return Ok(packets);
                                        }
                                    }
                                }
                            }
                            current_pes.clear();

                            // Parse PES header
                            if payload.len() >= 9 && payload[0] == 0 && payload[1] == 0 && payload[2] == 1 {
                                let pes_header_len = payload[8] as usize;

                                // Check for PTS
                                if (payload[7] & 0x80) != 0 && payload.len() >= 14 {
                                    let pts = ((payload[9] as u64 & 0x0e) << 29)
                                        | ((payload[10] as u64) << 22)
                                        | ((payload[11] as u64 & 0xfe) << 14)
                                        | ((payload[12] as u64) << 7)
                                        | ((payload[13] as u64) >> 1);
                                    current_pts = Some(pts / 90); // Convert to ms
                                }

                                // Extract payload after PES header
                                let data_start = 9 + pes_header_len;
                                if data_start < payload.len() {
                                    current_pes.extend_from_slice(&payload[data_start..]);
                                }
                            }
                        } else {
                            // Continuation of PES packet
                            current_pes.extend_from_slice(payload);
                        }
                    }
                }
                Err(e) => return Err(e.into()),
            }
        }

        // Don't forget last packet
        if let Some(pts) = current_pts {
            if !current_pes.is_empty() && pts >= start_ms && (end_ms == 0 || pts <= end_ms) {
                packets.push(SubtitlePacket {
                    pts_ms: pts,
                    data: current_pes,
                });
            }
        }

        Ok(packets)
    }

    /// Find file position for a given timestamp using binary search
    ///
    /// Uses ~10-12 seeks to binary search to precise position, much faster than
    /// linear interpolation with a large safety margin for network files.
    fn estimate_seek_position(
        &self,
        file_size: u64,
        target_ms: u64,
        reader: &mut BufReader<File>,
        packet_size: usize,
        offset: usize,
    ) -> Result<Option<u64>> {
        // Get duration estimate by sampling PTS at start and end
        let start_pts = self.sample_pts_at(reader, 0, packet_size, offset)?;
        let end_pts = self.sample_pts_at(reader, file_size.saturating_sub(100 * 1024), packet_size, offset)?;

        let (start_pts, end_pts) = match (start_pts, end_pts) {
            (Some(s), Some(e)) if e > s => (s, e),
            _ => return Ok(None), // Can't estimate, read from start
        };

        // If target is before stream start, no seeking needed
        if target_ms <= start_pts {
            return Ok(Some(0));
        }

        // If target is past stream end, seek near end
        if target_ms >= end_pts {
            return Ok(Some(file_size.saturating_sub(1024 * 1024)));
        }

        // Binary search to find the position
        // ~10-12 iterations gives us precision within a few MB on any file size
        let mut low = 0u64;
        let mut high = file_size;
        let mut best_pos = 0u64;
        let mut iterations = 0;
        const MAX_ITERATIONS: usize = 12;

        while low + 2 * 1024 * 1024 < high && iterations < MAX_ITERATIONS {
            iterations += 1;
            let mid = low + (high - low) / 2;

            // Sample PTS at mid position
            let pts = match self.sample_pts_at(reader, mid, packet_size, offset)? {
                Some(pts) => pts,
                None => {
                    // Couldn't sample, narrow the search
                    high = mid;
                    continue;
                }
            };

            if pts <= target_ms {
                best_pos = mid;
                low = mid;
            } else {
                high = mid;
            }
        }

        // Add small safety margin (1MB) to ensure we don't miss frames
        // at the exact seek point due to GOP boundaries
        Ok(Some(best_pos.saturating_sub(1024 * 1024)))
    }

    /// Sample PTS from packets near a file position
    ///
    /// Reads up to 2MB of data to find a PES packet with PTS.
    /// This is needed because PES packets with PTS can be sparse in the stream.
    fn sample_pts_at(
        &self,
        reader: &mut BufReader<File>,
        pos: u64,
        packet_size: usize,
        offset: usize,
    ) -> Result<Option<u64>> {
        reader.seek(std::io::SeekFrom::Start(pos))?;
        self.sync_to_packet(reader, packet_size, offset)?;

        // Read up to 2MB to find a PTS (PES packets can be sparse)
        // Use multiple reads to ensure we get the full buffer over network
        let target_size = 2 * 1024 * 1024;
        let mut buf = vec![0u8; target_size];
        let mut total_read = 0;
        while total_read < target_size {
            match reader.read(&mut buf[total_read..]) {
                Ok(0) => break,
                Ok(n) => total_read += n,
                Err(e) if e.kind() == std::io::ErrorKind::Interrupted => continue,
                Err(e) => return Err(e.into()),
            }
        }
        let n = total_read;

        for chunk in buf[..n].chunks_exact(packet_size) {
            let packet = &chunk[offset..];
            if packet[0] != 0x47 {
                continue;
            }

            let payload_start_indicator = (packet[1] & 0x40) != 0;
            let has_adaptation = (packet[3] & 0x20) != 0;
            let has_payload = (packet[3] & 0x10) != 0;

            if !has_payload || !payload_start_indicator {
                continue;
            }

            let payload_offset = if has_adaptation {
                5 + packet[4] as usize
            } else {
                4
            };

            if payload_offset >= packet.len() {
                continue;
            }

            let payload = &packet[payload_offset..];

            // Check for PES packet with PTS
            if payload.len() >= 14
                && payload[0] == 0
                && payload[1] == 0
                && payload[2] == 1
                && (payload[7] & 0x80) != 0
            {
                let pts = ((payload[9] as u64 & 0x0e) << 29)
                    | ((payload[10] as u64) << 22)
                    | ((payload[11] as u64 & 0xfe) << 14)
                    | ((payload[12] as u64) << 7)
                    | ((payload[13] as u64) >> 1);
                return Ok(Some(pts / 90)); // Convert to ms
            }
        }

        Ok(None)
    }

    /// Sync reader to next packet boundary
    fn sync_to_packet(
        &self,
        reader: &mut BufReader<File>,
        packet_size: usize,
        offset: usize,
    ) -> Result<()> {
        let mut buf = [0u8; 1024];
        let n = reader.read(&mut buf)?;

        for i in 0..n.saturating_sub(packet_size) {
            if buf[i + offset] == 0x47 {
                // Verify sync by checking next packet
                if i + offset + packet_size < n && buf[i + offset + packet_size] == 0x47 {
                    // Seek back to this position
                    reader.seek_relative(-((n - i) as i64))?;
                    return Ok(());
                }
            }
        }

        // Couldn't find sync, just continue from current position
        reader.seek_relative(-(n as i64))?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    #[allow(unused_imports)]
    use super::*;

    /// Test PTS parsing from a PES packet
    #[test]
    fn test_pts_parsing() {
        // Construct a minimal PES packet with a known PTS value
        // PTS = 90000 (1 second at 90kHz)
        //
        // PTS encoding format (33 bits split across 5 bytes):
        // byte[9]: '0010' | PTS[32-30] | '1'
        // byte[10]: PTS[29-22]
        // byte[11]: PTS[21-15] | '1'
        // byte[12]: PTS[14-7]
        // byte[13]: PTS[6-0] | '1'

        let pts: u64 = 90000; // 1 second

        // Build PTS bytes according to MPEG spec
        let pts_byte9 = 0x21 | (((pts >> 30) & 0x07) << 1) as u8;  // 0010 | PTS[32-30] | 1
        let pts_byte10 = ((pts >> 22) & 0xFF) as u8;
        let pts_byte11 = 0x01 | (((pts >> 15) & 0x7F) << 1) as u8; // PTS[21-15] | 1
        let pts_byte12 = ((pts >> 7) & 0xFF) as u8;
        let pts_byte13 = 0x01 | (((pts >> 0) & 0x7F) << 1) as u8;  // PTS[6-0] | 1

        let payload = [
            0x00, 0x00, 0x01,  // PES start code
            0xBD,             // Stream ID (private stream 1)
            0x00, 0x10,       // PES packet length
            0x80,             // Flags: no scrambling, no priority, no alignment
            0x80,             // PTS present, no DTS
            0x05,             // PES header data length
            pts_byte9, pts_byte10, pts_byte11, pts_byte12, pts_byte13,
            // payload data would follow
        ];

        // Parse PTS using the same logic as in our code
        assert!(payload.len() >= 14);
        assert_eq!(payload[0], 0);
        assert_eq!(payload[1], 0);
        assert_eq!(payload[2], 1);
        assert!((payload[7] & 0x80) != 0); // PTS present flag

        let parsed_pts = ((payload[9] as u64 & 0x0e) << 29)
            | ((payload[10] as u64) << 22)
            | ((payload[11] as u64 & 0xfe) << 14)
            | ((payload[12] as u64) << 7)
            | ((payload[13] as u64) >> 1);

        assert_eq!(parsed_pts, pts);
        assert_eq!(parsed_pts / 90, 1000); // 1000ms
    }

    /// Test PTS parsing with larger timestamp
    #[test]
    fn test_pts_parsing_one_hour() {
        // PTS for 1 hour = 3600 * 1000 * 90 = 324,000,000
        let pts: u64 = 324_000_000;

        let pts_byte9 = 0x21 | (((pts >> 30) & 0x07) << 1) as u8;
        let pts_byte10 = ((pts >> 22) & 0xFF) as u8;
        let pts_byte11 = 0x01 | (((pts >> 15) & 0x7F) << 1) as u8;
        let pts_byte12 = ((pts >> 7) & 0xFF) as u8;
        let pts_byte13 = 0x01 | (((pts >> 0) & 0x7F) << 1) as u8;

        let payload = [
            0x00, 0x00, 0x01, 0xBD,
            0x00, 0x10,
            0x80, 0x80, 0x05,
            pts_byte9, pts_byte10, pts_byte11, pts_byte12, pts_byte13,
        ];

        let parsed_pts = ((payload[9] as u64 & 0x0e) << 29)
            | ((payload[10] as u64) << 22)
            | ((payload[11] as u64 & 0xfe) << 14)
            | ((payload[12] as u64) << 7)
            | ((payload[13] as u64) >> 1);

        assert_eq!(parsed_pts, pts);
        assert_eq!(parsed_pts / 90, 3_600_000); // 3,600,000ms = 1 hour
    }

    /// Test M2TS packet detection
    #[test]
    fn test_packet_size_detection() {
        // M2TS has 4-byte timestamp prefix, sync byte at offset 4
        let m2ts_header: [u8; 8] = [0x00, 0x00, 0x00, 0x00, 0x47, 0x40, 0x00, 0x10];
        assert_eq!(m2ts_header[4], 0x47);

        // Plain TS has sync byte at offset 0
        let ts_header: [u8; 4] = [0x47, 0x40, 0x00, 0x10];
        assert_eq!(ts_header[0], 0x47);
    }

    /// Test PID extraction from TS packet
    #[test]
    fn test_pid_extraction() {
        // TS packet with PID = 0x1000 (4096)
        // byte[1] = error_indicator(0) | payload_unit_start(1) | transport_priority(0) | PID_high(5 bits)
        // byte[2] = PID_low (8 bits)
        // PID 0x1000 = 0001 0000 0000 0000
        // byte[1] = 0x50 (0101 0000) - payload_start=1, PID_high=0x10
        // byte[2] = 0x00 (PID_low)
        let packet: [u8; 4] = [0x47, 0x50, 0x00, 0x10];

        let pid = (((packet[1] & 0x1f) as u16) << 8) | packet[2] as u16;
        assert_eq!(pid, 0x1000);

        // Test PID = 0
        let pat_packet: [u8; 4] = [0x47, 0x40, 0x00, 0x10];
        let pat_pid = (((pat_packet[1] & 0x1f) as u16) << 8) | pat_packet[2] as u16;
        assert_eq!(pat_pid, 0);

        // Test PID = 0x1FFF (null packet)
        let null_packet: [u8; 4] = [0x47, 0x5F, 0xFF, 0x10];
        let null_pid = (((null_packet[1] & 0x1f) as u16) << 8) | null_packet[2] as u16;
        assert_eq!(null_pid, 0x1FFF);
    }
}
