mod cluster_cache;
mod ebml;
mod mkv;
mod mp4;
mod ts;

use anyhow::Result;
use std::io::Write;
use std::path::Path;

pub use cluster_cache::ClusterCache;
#[allow(unused_imports)]
pub use ebml::{ClusterIndex, EbmlScanner, FrameStreamer, RawFrame};
pub use mkv::MkvContainer;
pub use mp4::Mp4Container;
pub use ts::TsContainer;

/// Information about a track in a container
#[derive(Debug, Clone, serde::Serialize)]
pub struct TrackInfo {
    pub number: u64,
    #[serde(rename = "type")]
    pub track_type: TrackType,
    pub codec: String,
    pub language: Option<String>,
    pub name: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize)]
#[serde(rename_all = "lowercase")]
pub enum TrackType {
    Video,
    Audio,
    Subtitle,
    Other,
}

/// A subtitle frame with timing information
#[derive(Debug, Clone, serde::Serialize)]
pub struct SubtitleFrame {
    pub start_ms: u64,
    pub end_ms: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data_base64: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
}

#[derive(Debug, Clone, Copy)]
pub enum OutputFormat {
    Raw,
    Srt,
    WebVtt,
}

impl OutputFormat {
    pub fn from_str(s: &str) -> Result<Self> {
        match s {
            "raw" => Ok(Self::Raw),
            "srt" => Ok(Self::Srt),
            "webvtt" => Ok(Self::WebVtt),
            _ => anyhow::bail!("Unknown output format: {}", s),
        }
    }
}

#[derive(Debug, Clone, Copy)]
pub enum StreamFormat {
    Jsonl,
    Raw,
}

impl StreamFormat {
    pub fn from_str(s: &str) -> Result<Self> {
        match s {
            "jsonl" => Ok(Self::Jsonl),
            "raw" => Ok(Self::Raw),
            _ => anyhow::bail!("Unknown stream format: {}", s),
        }
    }
}

/// Enum-based container dispatch (avoids dyn trait issues with generics)
pub enum MediaContainer {
    Mkv(MkvContainer),
    Mp4(Mp4Container),
    Ts(TsContainer),
}

impl MediaContainer {
    /// Open a media file, auto-detecting the container format
    pub fn open(path: &Path) -> Result<Self> {
        let ext = path
            .extension()
            .and_then(|e| e.to_str())
            .map(|e| e.to_lowercase())
            .unwrap_or_default();

        match ext.as_str() {
            "mkv" | "webm" => Ok(Self::Mkv(MkvContainer::open(path)?)),
            "mp4" | "m4v" | "mov" => Ok(Self::Mp4(Mp4Container::open(path)?)),
            "ts" | "m2ts" | "mts" => Ok(Self::Ts(TsContainer::open(path)?)),
            "avi" => anyhow::bail!("AVI support not yet implemented"),
            _ => anyhow::bail!("Unsupported container format: .{}", ext),
        }
    }

    /// List all tracks in the container
    pub fn tracks(&self) -> Result<Vec<TrackInfo>> {
        match self {
            Self::Mkv(c) => c.tracks(),
            Self::Mp4(c) => c.tracks(),
            Self::Ts(c) => c.tracks(),
        }
    }

    /// Extract a complete subtitle track
    pub fn extract_track<W: Write>(
        &mut self,
        track_num: u64,
        format: OutputFormat,
        out: &mut W,
    ) -> Result<()> {
        match self {
            Self::Mkv(c) => c.extract_track(track_num, format, out),
            Self::Mp4(c) => c.extract_track(track_num, format, out),
            Self::Ts(c) => c.extract_track(track_num, format, out),
        }
    }

    /// Stream subtitle frames from a time range
    pub fn stream_track<W: Write>(
        &mut self,
        track_num: u64,
        start_ms: u64,
        end_ms: u64,
        format: StreamFormat,
        out: &mut W,
    ) -> Result<()> {
        match self {
            Self::Mkv(c) => c.stream_track(track_num, start_ms, end_ms, format, out),
            Self::Mp4(c) => c.stream_track(track_num, start_ms, end_ms, format, out),
            Self::Ts(c) => c.stream_track(track_num, start_ms, end_ms, format, out),
        }
    }
}
