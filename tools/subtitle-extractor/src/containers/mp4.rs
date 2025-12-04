//! MP4/M4V/MOV container support
//!
//! Handles extraction of subtitle tracks from MP4-based containers.
//! Supports tx3g (mov_text) and other subtitle formats.

use anyhow::{Context, Result};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use mp4::{Mp4Reader, TrackType as Mp4TrackType};
use std::fs::File;
use std::io::{BufReader, Write};
use std::path::{Path, PathBuf};

use super::{format_vtt_timestamp, OutputFormat, StreamFormat, SubtitleFrame, TrackInfo, TrackType};

pub struct Mp4Container {
    path: PathBuf,
    mp4: Mp4Reader<BufReader<File>>,
}

impl Mp4Container {
    pub fn open(path: &Path) -> Result<Self> {
        let file = File::open(path).context("Failed to open MP4 file")?;
        let size = file.metadata()?.len();
        let reader = BufReader::new(file);
        let mp4 = Mp4Reader::read_header(reader, size).context("Failed to parse MP4 file")?;

        Ok(Self {
            path: path.to_path_buf(),
            mp4,
        })
    }

    pub fn tracks(&self) -> Result<Vec<TrackInfo>> {
        let mut tracks = Vec::new();

        for track in self.mp4.tracks().values() {
            // Track type can fail for unsupported codecs
            let track_type = match track.track_type() {
                Ok(Mp4TrackType::Video) => TrackType::Video,
                Ok(Mp4TrackType::Audio) => TrackType::Audio,
                Ok(Mp4TrackType::Subtitle) => TrackType::Subtitle,
                Err(_) => TrackType::Other, // Unsupported codec
            };

            // Get codec from media type (may fail for unsupported codecs)
            let codec = match track.media_type() {
                Ok(mp4::MediaType::H264) => "avc1".to_string(),
                Ok(mp4::MediaType::H265) => "hev1".to_string(),
                Ok(mp4::MediaType::VP9) => "vp09".to_string(),
                Ok(mp4::MediaType::AAC) => "mp4a".to_string(),
                Ok(mp4::MediaType::TTXT) => "tx3g".to_string(),
                Err(_) => "unknown".to_string(),
            };

            // Get language from handler
            let language = track.language().to_string();
            let language = if language.is_empty() || language == "und" {
                None
            } else {
                Some(language)
            };

            tracks.push(TrackInfo {
                number: track.track_id() as u64,
                track_type,
                codec,
                language,
                name: None, // MP4 doesn't typically have track names
            });
        }

        Ok(tracks)
    }

    pub fn extract_track<W: Write>(
        &mut self,
        track_num: u64,
        _format: OutputFormat,
        out: &mut W,
    ) -> Result<()> {
        // For now, just stream all frames
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
        let track_id = track_num as u32;

        // Get track info to determine codec
        let track = self
            .mp4
            .tracks()
            .get(&track_id)
            .context("Track not found")?;

        let codec = match track.media_type()? {
            mp4::MediaType::TTXT => "tx3g",
            other => anyhow::bail!("Unsupported subtitle codec: {:?}", other),
        };

        let is_text = codec == "tx3g";

        // Get timescale for timestamp conversion
        let timescale = track.timescale();

        // Re-open file for reading samples (mp4 crate doesn't support seeking well)
        let file = File::open(&self.path)?;
        let size = file.metadata()?.len();
        let reader = BufReader::new(file);
        let mut mp4 = Mp4Reader::read_header(reader, size)?;

        // Read all samples for the track
        let sample_count = mp4.sample_count(track_id)?;

        // Write WebVTT header if needed
        if matches!(format, StreamFormat::WebVtt) {
            writeln!(out, "WEBVTT")?;
            writeln!(out)?;
        }

        for sample_id in 1..=sample_count {
            // Read sample data (includes timing info)
            let sample = match mp4.read_sample(track_id, sample_id)? {
                Some(s) => s,
                None => continue,
            };

            // Convert sample timing to milliseconds
            let sample_offset_ms = (sample.start_time * 1000) / timescale as u64;
            let duration_ms = (sample.duration as u64 * 1000) / timescale as u64;

            // Check time bounds
            if end_ms > 0 && sample_offset_ms > end_ms {
                break;
            }

            if sample_offset_ms < start_ms {
                continue;
            }

            let frame_end_ms = sample_offset_ms + duration_ms;

            match format {
                StreamFormat::Jsonl => {
                    let sub_frame = if is_text {
                        // tx3g format: 2 bytes length + UTF-8 text
                        let text = Self::extract_tx3g_text(&sample.bytes);

                        SubtitleFrame {
                            start_ms: sample_offset_ms,
                            end_ms: frame_end_ms,
                            data_base64: None,
                            text: Some(text),
                        }
                    } else {
                        SubtitleFrame {
                            start_ms: sample_offset_ms,
                            end_ms: frame_end_ms,
                            data_base64: Some(BASE64.encode(&sample.bytes)),
                            text: None,
                        }
                    };
                    writeln!(out, "{}", serde_json::to_string(&sub_frame)?)?;
                }
                StreamFormat::Raw => {
                    out.write_all(&sample.bytes)?;
                }
                StreamFormat::WebVtt => {
                    if is_text {
                        let text = Self::extract_tx3g_text(&sample.bytes);
                        if !text.is_empty() {
                            let start_time = format_vtt_timestamp(sample_offset_ms);
                            let end_time = format_vtt_timestamp(frame_end_ms);
                            writeln!(out, "{} --> {}", start_time, end_time)?;
                            writeln!(out, "{}", text)?;
                            writeln!(out)?;
                        }
                    }
                    // Skip non-text subtitles for WebVTT format
                }
            }
            out.flush()?;
        }

        Ok(())
    }

    /// Extract text from tx3g format (2 bytes length + UTF-8 text)
    fn extract_tx3g_text(data: &[u8]) -> String {
        if data.len() >= 2 {
            let text_len = u16::from_be_bytes([data[0], data[1]]) as usize;
            if data.len() >= 2 + text_len {
                String::from_utf8_lossy(&data[2..2 + text_len]).to_string()
            } else {
                String::from_utf8_lossy(&data[2..]).to_string()
            }
        } else {
            String::new()
        }
    }
}
