use anyhow::{Context, Result};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use matroska_demuxer::{Frame, MatroskaFile, TrackType as MkvTrackType};
use std::fs::File;
use std::io::{BufReader, Write};
use std::path::{Path, PathBuf};

use super::cluster_cache::ClusterCache;
use super::ebml::{FrameStreamer, RawFrame};
use super::{OutputFormat, StreamFormat, SubtitleFrame, TrackInfo, TrackType};

pub struct MkvContainer {
    path: PathBuf,
    mkv: MatroskaFile<BufReader<File>>,
    timestamp_scale: u64,
    /// Lazy-initialized cluster cache for fallback seeking
    cluster_cache: Option<ClusterCache>,
}

impl MkvContainer {
    pub fn open(path: &Path) -> Result<Self> {
        let file = File::open(path).context("Failed to open file")?;
        let reader = BufReader::new(file);
        let mkv = MatroskaFile::open(reader).context("Failed to parse MKV file")?;
        let timestamp_scale = mkv.info().timestamp_scale().get();

        Ok(Self {
            path: path.to_path_buf(),
            mkv,
            timestamp_scale,
            cluster_cache: None,
        })
    }

    /// Try native seek and verify it landed in the right place
    /// Returns Some(true) if seek worked and is accurate
    /// Returns Some(false) if seek worked but landed in wrong place (broken Cues)
    /// Returns None if seek failed entirely
    fn try_native_seek_verified(&mut self, target_ms: u64, _track_num: u64) -> Option<bool> {
        if self.mkv.seek(target_ms).is_err() {
            return None;
        }

        // Read a few frames to verify seek landed near the target
        // We check ANY track's frame since MKV seeks to cluster positions
        // and all tracks share the same cluster structure
        let mut frame = Frame::default();
        for _ in 0..10 {
            match self.mkv.next_frame(&mut frame) {
                Ok(true) => {
                    // Got a frame - check if it's reasonably close to target
                    let frame_ms = self.timestamp_to_ms(frame.timestamp);

                    // MKV Cues typically point to keyframes, which may be up to a GOP
                    // (typically 2-10 seconds) before the target. Allow generous tolerance:
                    // - Up to 60 seconds before target (some files have sparse keyframes)
                    // - Up to 10 seconds after (minor overshoot is acceptable)
                    let acceptable = frame_ms + 60000 >= target_ms && frame_ms <= target_ms + 10000;

                    if !acceptable {
                        // Cues are broken - seek landed in wrong place
                        return Some(false);
                    }

                    // Seek back to target for actual extraction
                    // (we consumed a frame during verification)
                    let _ = self.mkv.seek(target_ms);
                    return Some(true);
                }
                Ok(false) => {
                    // End of file - seek was to the end, that's valid
                    let _ = self.mkv.seek(target_ms);
                    return Some(true);
                }
                Err(_) => {
                    // Error reading frame
                    return None;
                }
            }
        }

        // Couldn't get any frames - something's wrong
        None
    }

    pub fn tracks(&self) -> Result<Vec<TrackInfo>> {
        let tracks = self
            .mkv
            .tracks()
            .iter()
            .map(|track| {
                let track_type = match track.track_type() {
                    MkvTrackType::Video => TrackType::Video,
                    MkvTrackType::Audio => TrackType::Audio,
                    MkvTrackType::Subtitle => TrackType::Subtitle,
                    _ => TrackType::Other,
                };

                TrackInfo {
                    number: track.track_number().get(),
                    track_type,
                    codec: track.codec_id().to_string(),
                    language: track.language().map(|s| s.to_string()),
                    name: track.name().map(|s| s.to_string()),
                }
            })
            .collect();

        Ok(tracks)
    }

    pub fn extract_track<W: Write>(
        &mut self,
        track_num: u64,
        format: OutputFormat,
        out: &mut W,
    ) -> Result<()> {
        let codec = self.get_track_codec(track_num)?;
        let is_text = Self::is_text_codec(&codec);

        if !is_text && !matches!(format, OutputFormat::Raw) {
            anyhow::bail!(
                "Format conversion only supported for text subtitles. Codec is: {}",
                codec
            );
        }

        if matches!(format, OutputFormat::WebVtt) {
            writeln!(out, "WEBVTT")?;
            writeln!(out)?;
        }

        let mut cue_number = 1u32;
        let mut frame = Frame::default();

        while self.mkv.next_frame(&mut frame)? {
            if frame.track == track_num {
                self.write_frame(out, &frame, &format, &codec, &mut cue_number)?;
            }
        }

        Ok(())
    }

    pub fn stream_track<W: Write>(
        &mut self,
        track_num: u64,
        start_ms: u64,
        end_ms: u64,
        format: StreamFormat,
        out: &mut W,
    ) -> Result<()> {
        let codec = self.get_track_codec(track_num)?;
        let is_text = Self::is_text_codec(&codec);

        // Strategy for fast seeking:
        // 1. Try native Cues seek first (O(1) if Cues are valid)
        // 2. Verify the seek landed correctly
        // 3. If Cues work, use demuxer path (fast)
        // 4. If Cues are broken/missing, use binary search cluster cache

        if start_ms > 0 {
            // Try native Cues seek with verification
            match self.try_native_seek_verified(start_ms, track_num) {
                Some(true) => {
                    // Cues work! Use demuxer path (it's already seeked)
                    return self.stream_with_demuxer(track_num, start_ms, end_ms, &codec, is_text, format, out);
                }
                Some(false) => {
                    // Cues are broken - they returned success but wrong position
                    // Fall through to cluster cache
                }
                None => {
                    // Seek failed entirely - fall through to cluster cache
                }
            }
        } else {
            // Starting from beginning - demuxer is fine
            return self.stream_with_demuxer(track_num, start_ms, end_ms, &codec, is_text, format, out);
        }

        // Use our optimized cluster cache + frame streamer
        // This handles files with broken/missing Cues via binary search seeking
        // and also filters by track to skip video/audio block data
        match self.stream_with_cache(track_num, start_ms, end_ms, &codec, is_text, format, out) {
            Ok(()) => return Ok(()),
            Err(e) => {
                // Fall back to demuxer on error
                eprintln!("Optimized path failed ({}), falling back to demuxer", e);
            }
        }

        // Final fallback: Use matroska-demuxer from beginning (slowest)
        self.mkv.seek(0).ok();
        self.stream_with_demuxer(track_num, start_ms, end_ms, &codec, is_text, format, out)
    }

    /// Stream using matroska-demuxer (when Cues work)
    #[allow(clippy::too_many_arguments)]
    fn stream_with_demuxer<W: Write>(
        &mut self,
        track_num: u64,
        start_ms: u64,
        end_ms: u64,
        codec: &str,
        is_text: bool,
        format: StreamFormat,
        out: &mut W,
    ) -> Result<()> {
        // Write WebVTT header if needed
        if matches!(format, StreamFormat::WebVtt) {
            writeln!(out, "WEBVTT")?;
            writeln!(out)?;
        }

        let mut frame = Frame::default();

        while self.mkv.next_frame(&mut frame)? {
            if frame.track != track_num {
                continue;
            }

            let frame_start_ms = self.timestamp_to_ms(frame.timestamp);

            // Stop if we've passed the end time
            if end_ms > 0 && frame_start_ms > end_ms {
                break;
            }

            // Skip frames before start time
            if frame_start_ms < start_ms {
                continue;
            }

            let frame_end_ms = frame_start_ms + self.duration_to_ms(frame.duration);
            self.write_subtitle_frame(out, &frame.data, frame_start_ms, frame_end_ms, codec, is_text, &format)?;
        }

        Ok(())
    }

    /// Stream using cluster cache + track-filtered frame streaming
    ///
    /// This is the fast path for subtitle extraction:
    /// 1. Uses binary search seeking via ClusterCache (3-5 reads to any position)
    /// 2. Filters by track at the EBML level, skipping video/audio block data entirely
    ///
    /// For a 40GB 4K remux, this reduces extraction time from ~15s to <1s.
    #[allow(clippy::too_many_arguments)]
    fn stream_with_cache<W: Write>(
        &mut self,
        track_num: u64,
        start_ms: u64,
        end_ms: u64,
        codec: &str,
        is_text: bool,
        format: StreamFormat,
        out: &mut W,
    ) -> Result<()> {
        // Write WebVTT header if needed
        if matches!(format, StreamFormat::WebVtt) {
            writeln!(out, "WEBVTT")?;
            writeln!(out)?;
        }

        // Initialize cluster cache for binary search seeking
        if self.cluster_cache.is_none() {
            self.cluster_cache = Some(ClusterCache::open(&self.path)?);
        }

        // Find the cluster at or before start_ms using binary search
        let cluster = {
            let cache = self.cluster_cache.as_mut().unwrap();
            cache.find_cluster(start_ms)?
        };

        // Open frame streamer with track filtering and time bounds
        let mut streamer = FrameStreamer::open(&self.path)?;
        streamer.set_target_track(track_num); // Skip non-target track blocks entirely
        streamer.set_end_ms(end_ms); // Stop at cluster level when past end time

        // Seek to cluster if found, otherwise start from beginning
        if let Some(c) = cluster {
            streamer.seek_to_cluster(c.data_offset, c.size)?;
        }

        let mut frame = RawFrame::default();

        while streamer.next_frame(&mut frame)? {
            // Frame timestamps are already in ms from FrameStreamer
            let frame_start_ms = frame.timestamp;

            // Stop if we've passed the end time
            if end_ms > 0 && frame_start_ms > end_ms {
                break;
            }

            // Skip frames before start time
            if frame_start_ms < start_ms {
                continue;
            }

            // Calculate end time from duration (if available)
            let frame_end_ms = frame_start_ms + frame.duration.unwrap_or(0);

            self.write_subtitle_frame(out, &frame.data, frame_start_ms, frame_end_ms, codec, is_text, &format)?;
        }

        Ok(())
    }

    /// Write a subtitle frame to output
    #[allow(clippy::too_many_arguments)]
    fn write_subtitle_frame<W: Write>(
        &self,
        out: &mut W,
        data: &[u8],
        start_ms: u64,
        end_ms: u64,
        codec: &str,
        is_text: bool,
        format: &StreamFormat,
    ) -> Result<()> {
        match format {
            StreamFormat::Jsonl => {
                let sub_frame = if is_text {
                    let text = extract_text_content(data, codec)?;
                    SubtitleFrame {
                        start_ms,
                        end_ms,
                        data_base64: None,
                        text: Some(text),
                    }
                } else {
                    SubtitleFrame {
                        start_ms,
                        end_ms,
                        data_base64: Some(BASE64.encode(data)),
                        text: None,
                    }
                };
                writeln!(out, "{}", serde_json::to_string(&sub_frame)?)?;
            }
            StreamFormat::Raw => {
                out.write_all(data)?;
            }
            StreamFormat::WebVtt => {
                if is_text {
                    let text = extract_text_content(data, codec)?;
                    let start_time = format_timestamp(start_ms, true);
                    let end_time = format_timestamp(end_ms, true);
                    writeln!(out, "{} --> {}", start_time, end_time)?;
                    writeln!(out, "{}", text)?;
                    writeln!(out)?;
                }
                // Skip non-text subtitles for WebVTT format
            }
        }
        out.flush()?;
        Ok(())
    }

    fn get_track_codec(&self, track_num: u64) -> Result<String> {
        self.mkv
            .tracks()
            .iter()
            .find(|t| t.track_number().get() == track_num)
            .map(|t| t.codec_id().to_string())
            .context("Track not found")
    }

    fn is_text_codec(codec: &str) -> bool {
        matches!(
            codec,
            "S_TEXT/UTF8" | "S_TEXT/ASS" | "S_TEXT/SSA" | "S_TEXT/WEBVTT"
        )
    }

    fn timestamp_to_ms(&self, timestamp: u64) -> u64 {
        (timestamp * self.timestamp_scale) / 1_000_000
    }

    fn duration_to_ms(&self, duration: Option<u64>) -> u64 {
        duration
            .map(|d| (d * self.timestamp_scale) / 1_000_000)
            .unwrap_or(0)
    }

    fn write_frame<W: Write>(
        &self,
        out: &mut W,
        frame: &Frame,
        format: &OutputFormat,
        codec: &str,
        cue_number: &mut u32,
    ) -> Result<()> {
        match format {
            OutputFormat::Raw => {
                out.write_all(&frame.data)?;
                if codec.starts_with("S_TEXT") {
                    writeln!(out)?;
                }
            }
            OutputFormat::Srt | OutputFormat::WebVtt => {
                let start_ms = self.timestamp_to_ms(frame.timestamp);
                let duration_ms = self.duration_to_ms(frame.duration);
                let end_ms = start_ms + duration_ms;

                let use_dot = matches!(format, OutputFormat::WebVtt);
                let start_time = format_timestamp(start_ms, use_dot);
                let end_time = format_timestamp(end_ms, use_dot);

                let text = extract_text_content(&frame.data, codec)?;

                if matches!(format, OutputFormat::Srt) {
                    writeln!(out, "{}", cue_number)?;
                }
                writeln!(out, "{} --> {}", start_time, end_time)?;
                writeln!(out, "{}", text)?;
                writeln!(out)?;

                *cue_number += 1;
            }
        }
        Ok(())
    }
}

fn format_timestamp(ms: u64, use_dot: bool) -> String {
    let hours = ms / 3_600_000;
    let minutes = (ms % 3_600_000) / 60_000;
    let seconds = (ms % 60_000) / 1_000;
    let millis = ms % 1_000;

    let sep = if use_dot { '.' } else { ',' };
    format!(
        "{:02}:{:02}:{:02}{}{:03}",
        hours, minutes, seconds, sep, millis
    )
}

fn extract_text_content(data: &[u8], codec: &str) -> Result<String> {
    let text = std::str::from_utf8(data).context("Invalid UTF-8 in subtitle")?;

    match codec {
        "S_TEXT/ASS" | "S_TEXT/SSA" => {
            // MKV stores ASS events without timing (timing is in the MKV container)
            // Format: ReadOrder,Layer,Style,Name,MarginL,MarginR,MarginV,Effect,Text
            // That's 8 fields before the text, so we skip 8 commas
            let text_portion = if let Some(pos) = text.match_indices(',').nth(7) {
                &text[pos.0 + 1..]
            } else {
                text
            };

            Ok(strip_ass_tags(text_portion))
        }
        _ => Ok(text.to_string()),
    }
}

fn strip_ass_tags(text: &str) -> String {
    let mut result = String::with_capacity(text.len());
    let mut chars = text.chars().peekable();

    while let Some(c) = chars.next() {
        if c == '{' && chars.peek() == Some(&'\\') {
            // Skip until closing brace
            for c2 in chars.by_ref() {
                if c2 == '}' {
                    break;
                }
            }
        } else {
            result.push(c);
        }
    }

    // Convert ASS newlines to actual newlines
    result.replace("\\N", "\n").replace("\\n", "\n")
}
