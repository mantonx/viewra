//! MPEG-TS/M2TS container support
//!
//! Handles extraction of subtitle tracks from MPEG Transport Stream containers.
//! Supports PGS (HDMV) subtitles commonly found in Blu-ray M2TS files.

use anyhow::{Context, Result};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use mpeg2ts_reader::demultiplex;
use mpeg2ts_reader::packet;
use mpeg2ts_reader::pes;
use mpeg2ts_reader::psi;
use mpeg2ts_reader::StreamType;
use std::cell::RefCell;
use std::collections::HashMap;
use std::fs::File;
use std::io::{BufReader, Read, Write};
use std::path::{Path, PathBuf};
use std::rc::Rc;

use super::{OutputFormat, StreamFormat, SubtitleFrame, TrackInfo, TrackType};

/// Collected subtitle stream info
#[derive(Clone)]
struct SubtitleStreamInfo {
    pid: u16,
    stream_type: StreamType,
    language: Option<String>,
}

/// Collected subtitle packet
struct SubtitlePacket {
    pts_ms: u64,
    data: Vec<u8>,
}

/// Shared state for collecting track info and packets
struct TsState {
    streams: Vec<SubtitleStreamInfo>,
    packets: HashMap<u16, Vec<SubtitlePacket>>,
    target_pid: Option<u16>,
    start_ms: u64,
    end_ms: u64,
}

impl TsState {
    fn new() -> Self {
        Self {
            streams: Vec::new(),
            packets: HashMap::new(),
            target_pid: None,
            start_ms: 0,
            end_ms: 0,
        }
    }
}

// Create the filter switch enum for TS demuxing
mpeg2ts_reader::packet_filter_switch! {
    TsFilterSwitch<TsDemuxContext> {
        Pes: pes::PesPacketFilter<TsDemuxContext, SubtitleStreamConsumer>,
        Pat: demultiplex::PatPacketFilter<TsDemuxContext>,
        Pmt: demultiplex::PmtPacketFilter<TsDemuxContext>,
        Null: demultiplex::NullPacketFilter<TsDemuxContext>,
    }
}

// Create the demux context
mpeg2ts_reader::demux_context!(TsDemuxContext, TsFilterSwitch);

impl TsDemuxContext {
    fn do_construct(&mut self, req: demultiplex::FilterRequest<'_, '_>) -> TsFilterSwitch {
        match req {
            demultiplex::FilterRequest::ByPid(psi::pat::PAT_PID) => {
                TsFilterSwitch::Pat(demultiplex::PatPacketFilter::default())
            }
            demultiplex::FilterRequest::ByPid(mpeg2ts_reader::STUFFING_PID) => {
                TsFilterSwitch::Null(demultiplex::NullPacketFilter::default())
            }
            demultiplex::FilterRequest::ByPid(_) => {
                TsFilterSwitch::Null(demultiplex::NullPacketFilter::default())
            }
            // Handle PGS subtitles (HDMV presentation graphics - 0x90)
            demultiplex::FilterRequest::ByStream {
                stream_type: StreamType(0x90), // PGS
                stream_info,
                ..
            } => SubtitleStreamConsumer::construct(stream_info, StreamType(0x90)),
            // Handle DVB subtitles (private data - 0x06)
            demultiplex::FilterRequest::ByStream {
                stream_type: StreamType(0x06), // DVB subtitles
                stream_info,
                ..
            } => SubtitleStreamConsumer::construct(stream_info, StreamType(0x06)),
            demultiplex::FilterRequest::ByStream { .. } => {
                TsFilterSwitch::Null(demultiplex::NullPacketFilter::default())
            }
            demultiplex::FilterRequest::Pmt {
                pid,
                program_number,
            } => TsFilterSwitch::Pmt(demultiplex::PmtPacketFilter::new(pid, program_number)),
            demultiplex::FilterRequest::Nit { .. } => {
                TsFilterSwitch::Null(demultiplex::NullPacketFilter::default())
            }
        }
    }
}

/// Consumer for subtitle elementary streams
pub struct SubtitleStreamConsumer {
    pid: packet::Pid,
    stream_type: StreamType,
    current_packet: Vec<u8>,
    current_pts: Option<u64>,
    state: Rc<RefCell<TsState>>,
}

impl SubtitleStreamConsumer {
    fn construct(stream_info: &psi::pmt::StreamInfo, stream_type: StreamType) -> TsFilterSwitch {
        // We can't access state here without changing the architecture
        // For now, create a dummy consumer that collects nothing
        let filter = pes::PesPacketFilter::new(SubtitleStreamConsumer {
            pid: stream_info.elementary_pid(),
            stream_type,
            current_packet: Vec::new(),
            current_pts: None,
            state: Rc::new(RefCell::new(TsState::new())),
        });
        TsFilterSwitch::Pes(filter)
    }

    fn with_state(
        stream_info: &psi::pmt::StreamInfo,
        stream_type: StreamType,
        state: Rc<RefCell<TsState>>,
    ) -> Self {
        SubtitleStreamConsumer {
            pid: stream_info.elementary_pid(),
            stream_type,
            current_packet: Vec::new(),
            current_pts: None,
            state,
        }
    }
}

impl pes::ElementaryStreamConsumer<TsDemuxContext> for SubtitleStreamConsumer {
    fn start_stream(&mut self, _ctx: &mut TsDemuxContext) {}

    fn begin_packet(&mut self, _ctx: &mut TsDemuxContext, header: pes::PesHeader) {
        self.current_packet.clear();

        if let pes::PesContents::Parsed(Some(parsed)) = header.contents() {
            // Extract PTS
            if let Ok(pts_dts) = parsed.pts_dts() {
                let pts = match pts_dts {
                    pes::PtsDts::PtsOnly(Ok(pts)) => Some(pts.value()),
                    pes::PtsDts::Both { pts: Ok(pts), .. } => Some(pts.value()),
                    _ => None,
                };
                // Convert 90kHz PTS to milliseconds
                self.current_pts = pts.map(|p| p / 90);
            }

            // Start collecting payload
            self.current_packet.extend_from_slice(parsed.payload());
        }
    }

    fn continue_packet(&mut self, _ctx: &mut TsDemuxContext, data: &[u8]) {
        self.current_packet.extend_from_slice(data);
    }

    fn end_packet(&mut self, _ctx: &mut TsDemuxContext) {
        if let Some(pts_ms) = self.current_pts {
            let state = self.state.borrow();
            let target = state.target_pid;
            let start = state.start_ms;
            let end = state.end_ms;
            drop(state);

            let pid_val = u16::from(self.pid);

            // Only collect if this is our target PID and within time range
            if target.map_or(true, |t| t == pid_val) {
                if pts_ms >= start && (end == 0 || pts_ms <= end) {
                    let packet = SubtitlePacket {
                        pts_ms,
                        data: std::mem::take(&mut self.current_packet),
                    };

                    let mut state = self.state.borrow_mut();
                    state
                        .packets
                        .entry(pid_val)
                        .or_insert_with(Vec::new)
                        .push(packet);
                }
            }
        }

        self.current_packet.clear();
        self.current_pts = None;
    }

    fn continuity_error(&mut self, _ctx: &mut TsDemuxContext) {}
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
        let file = File::open(path).context("Failed to open TS file")?;
        let mut reader = BufReader::new(file);
        let mut streams = Vec::new();

        // Read first few MB to find PMT with subtitle streams
        let mut buf = vec![0u8; 188 * 1024];
        let mut total_read = 0;
        const MAX_SCAN: usize = 10 * 1024 * 1024; // 10MB should be enough to find PMT

        let mut ctx = TsDemuxContext::new();
        let mut demux = demultiplex::Demultiplex::new(&mut ctx);

        while total_read < MAX_SCAN {
            match reader.read(&mut buf) {
                Ok(0) => break,
                Ok(n) => {
                    total_read += n;
                    demux.push(&mut ctx, &buf[0..n]);
                }
                Err(e) => return Err(e.into()),
            }
        }

        // Extract stream info from PMT
        // Since we can't easily get this from the demux context,
        // we'll use a simpler approach: parse PMT manually
        streams = Self::parse_pmt_for_subtitles(path)?;

        Ok(streams)
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
            let mut found = false;
            for i in 0..bytes_read.saturating_sub(TS_PACKET_SIZE) {
                if detect_buf[i] == 0x47 {
                    file.seek(std::io::SeekFrom::Start(i as u64))?;
                    found = true;
                    break;
                }
            }
            if !found {
                anyhow::bail!("Could not find TS sync byte");
            }
            TS_PACKET_SIZE
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
                                                    stream_type: StreamType(0x90),
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
                let codec = match stream.stream_type.0 {
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

        // Read and extract subtitle packets
        let packets = self.extract_packets(target_pid, start_ms, end_ms)?;

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
        let mut reader = BufReader::with_capacity(256 * 1024, file);
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

        let mut buf = vec![0u8; packet_size * 1024];
        let mut current_pes: Vec<u8> = Vec::new();
        let mut current_pts: Option<u64> = None;
        let mut consecutive_past_end = 0;

        loop {
            match reader.read(&mut buf) {
                Ok(0) => break,
                Ok(n) => {
                    for chunk in buf[..n].chunks_exact(packet_size) {
                        let packet = &chunk[offset..];
                        if packet[0] != 0x47 {
                            continue;
                        }

                        let pid = (((packet[1] & 0x1f) as u16) << 8) | packet[2] as u16;

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
            if !current_pes.is_empty() {
                if pts >= start_ms && (end_ms == 0 || pts <= end_ms) {
                    packets.push(SubtitlePacket {
                        pts_ms: pts,
                        data: current_pes,
                    });
                }
            }
        }

        Ok(packets)
    }

    /// Estimate file position for a given timestamp using linear interpolation
    ///
    /// This uses a simple approach: sample PTS at start and end of file,
    /// then linearly interpolate to estimate the position for the target time.
    /// We seek a bit before the estimate to ensure we don't miss frames.
    fn estimate_seek_position(
        &self,
        file_size: u64,
        target_ms: u64,
        reader: &mut BufReader<File>,
        packet_size: usize,
        offset: usize,
    ) -> Result<Option<u64>> {
        // Get duration estimate by sampling PTS at start and end
        // Only read small amounts to minimize network I/O
        let start_pts = self.sample_pts_at(reader, 0, packet_size, offset)?;
        // Read from 100KB before end - just need one PES packet with PTS
        let end_pts = self.sample_pts_at(reader, file_size.saturating_sub(100 * 1024), packet_size, offset)?;

        let (start_pts, end_pts) = match (start_pts, end_pts) {
            (Some(s), Some(e)) if e > s => (s, e),
            _ => return Ok(None), // Can't estimate, read from start
        };

        let duration_ms = end_pts - start_pts;
        if duration_ms == 0 {
            return Ok(None);
        }

        // Simple linear interpolation - no binary search to minimize network I/O
        let target_offset = target_ms.saturating_sub(start_pts);
        let ratio = (target_offset as f64 / duration_ms as f64).clamp(0.0, 1.0);
        let estimate = (file_size as f64 * ratio) as u64;

        // Seek a fixed amount before the estimate (50MB max) to ensure we don't miss frames
        // Using a fixed size rather than percentage works better for large files
        let safety_margin = (50 * 1024 * 1024).min(estimate); // 50MB or less
        let seek_pos = estimate.saturating_sub(safety_margin);

        Ok(Some(seek_pos))
    }

    /// Sample PTS from packets near a file position
    fn sample_pts_at(
        &self,
        reader: &mut BufReader<File>,
        pos: u64,
        packet_size: usize,
        offset: usize,
    ) -> Result<Option<u64>> {
        reader.seek(std::io::SeekFrom::Start(pos))?;
        self.sync_to_packet(reader, packet_size, offset)?;

        let mut buf = vec![0u8; packet_size * 512];
        let n = reader.read(&mut buf)?;

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
            if payload.len() >= 14 && payload[0] == 0 && payload[1] == 0 && payload[2] == 1 {
                if (payload[7] & 0x80) != 0 {
                    let pts = ((payload[9] as u64 & 0x0e) << 29)
                        | ((payload[10] as u64) << 22)
                        | ((payload[11] as u64 & 0xfe) << 14)
                        | ((payload[12] as u64) << 7)
                        | ((payload[13] as u64) >> 1);
                    return Ok(Some(pts / 90)); // Convert to ms
                }
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

use std::io::Seek;

trait SeekRelative {
    fn seek_relative(&mut self, offset: i64) -> std::io::Result<()>;
}

impl<R: Read + Seek> SeekRelative for BufReader<R> {
    fn seek_relative(&mut self, offset: i64) -> std::io::Result<()> {
        self.seek(std::io::SeekFrom::Current(offset))?;
        Ok(())
    }
}
