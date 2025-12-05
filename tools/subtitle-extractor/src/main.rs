mod containers;
mod pgs;

use anyhow::{Context, Result};
use clap::{Parser, Subcommand, ValueEnum};
use std::io;
use std::path::PathBuf;

use containers::{MediaContainer, OutputFormat, StreamFormat};

#[derive(Parser)]
#[command(name = "subtitle-extractor")]
#[command(about = "Fast subtitle track extractor for multiple container formats")]
#[command(version)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Clone, ValueEnum)]
enum IndexType {
    /// Cluster-level index (for general MKV seeking)
    Cluster,
    /// PGS frame-level index (for fast PGS subtitle seeking)
    Pgs,
}

#[derive(Subcommand)]
enum Commands {
    /// List all tracks in a media file (outputs JSON)
    Tracks {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,
    },

    /// Extract a subtitle track to stdout
    Extract {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,

        /// Track number to extract (from 'tracks' command)
        #[arg(short, long)]
        track: u64,

        /// Output format (raw, srt, webvtt)
        #[arg(short, long, default_value = "raw")]
        format: String,
    },

    /// Stream subtitle frames from a time range (uses seeking when available)
    Stream {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,

        /// Track number to extract
        #[arg(short, long)]
        track: u64,

        /// Start time in milliseconds
        #[arg(long, default_value = "0")]
        start: u64,

        /// End time in milliseconds (0 = until end)
        #[arg(long, default_value = "0")]
        end: u64,

        /// Output format: jsonl (JSON lines with timing), raw, webvtt
        #[arg(short, long, default_value = "jsonl")]
        format: String,
    },

    /// Build an index for fast seeking
    ///
    /// Index types:
    /// - cluster: Cluster-level index for general MKV seeking
    /// - pgs: Frame-level index for fast PGS subtitle seeking
    Index {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,

        /// Track number to index
        #[arg(short, long)]
        track: u64,

        /// Type of index to build
        #[arg(long, value_enum, default_value = "pgs")]
        r#type: IndexType,

        /// Start time in milliseconds (for progressive indexing, pgs only)
        #[arg(long, default_value = "0")]
        start: u64,

        /// End time in milliseconds (0 = until end, pgs only)
        #[arg(long, default_value = "0")]
        end: u64,
    },

    /// Stream PGS subtitle frames as WebP images
    ///
    /// Outputs JSON lines to stdout, each containing a rendered WebP frame:
    /// {"start_ms": 1234, "end_ms": 5678, "x": 100, "y": 800, "width": 480, "height": 50, "canvas_width": 1920, "canvas_height": 1080, "image_base64": "..."}
    ///
    /// If --index is provided, uses fast indexed extraction (O(1) seeking).
    /// Otherwise, scans through the file to find frames.
    #[command(name = "stream-pgs")]
    StreamPgs {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,

        /// Track number (must be PGS subtitle track)
        #[arg(short, long)]
        track: u64,

        /// Start time in milliseconds
        #[arg(long, default_value = "0")]
        start: u64,

        /// End time in milliseconds (0 = until end)
        #[arg(long, default_value = "0")]
        end: u64,

        /// Maximum number of frames to output (0 = unlimited)
        #[arg(long, default_value = "0")]
        limit: usize,

        /// Path to pre-built index file for fast seeking
        #[arg(long)]
        index: Option<PathBuf>,
    },

    /// [Debug] Render PGS subtitles to image files in a directory
    #[command(name = "debug-render-pgs")]
    DebugRenderPgs {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,

        /// Track number (must be PGS subtitle track)
        #[arg(short, long)]
        track: u64,

        /// Output directory for image files
        #[arg(short, long)]
        output: PathBuf,

        /// Start time in milliseconds
        #[arg(long, default_value = "0")]
        start: u64,

        /// End time in milliseconds (0 = until end)
        #[arg(long, default_value = "0")]
        end: u64,

        /// Maximum number of frames to render
        #[arg(long, default_value = "100")]
        limit: usize,

        /// Output image format: webp (default, smaller files) or png
        #[arg(long, default_value = "webp")]
        image_format: String,

        /// WebP quality (1-100, only for webp format; 100 = lossless)
        #[arg(long, default_value = "100")]
        quality: u8,
    },
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Commands::Tracks { file } => {
            let container = MediaContainer::open(&file)?;
            let tracks = container.tracks()?;
            let json = serde_json::to_string_pretty(&tracks)?;
            println!("{}", json);
        }

        Commands::Extract {
            file,
            track,
            format,
        } => {
            let mut container = MediaContainer::open(&file)?;
            let format = OutputFormat::from_str(&format)?;
            let stdout = io::stdout();
            let mut out = stdout.lock();
            container.extract_track(track, format, &mut out)?;
        }

        Commands::Stream {
            file,
            track,
            start,
            end,
            format,
        } => {
            let mut container = MediaContainer::open(&file)?;
            let format = StreamFormat::from_str(&format)?;
            let stdout = io::stdout();
            let mut out = stdout.lock();
            container.stream_track(track, start, end, format, &mut out)?;
        }

        Commands::Index {
            file,
            track,
            r#type,
            start,
            end,
        } => {
            match r#type {
                IndexType::Cluster => {
                    use containers::EbmlScanner;
                    use std::time::Instant;

                    eprintln!("Building cluster index...");
                    let start_time = Instant::now();

                    let mut scanner = EbmlScanner::open(&file)?;
                    let index = scanner.build_cluster_index()?;

                    let elapsed = start_time.elapsed();
                    eprintln!(
                        "Index built in {:.2}s: {} clusters",
                        elapsed.as_secs_f64(),
                        index.clusters.len()
                    );

                    #[derive(serde::Serialize)]
                    struct IndexOutput {
                        timestamp_scale: u64,
                        cluster_count: usize,
                        clusters: Vec<ClusterOutput>,
                    }

                    #[derive(serde::Serialize)]
                    struct ClusterOutput {
                        timestamp_ms: u64,
                        file_offset: u64,
                        cluster_size: u64,
                    }

                    let output = IndexOutput {
                        timestamp_scale: index.timestamp_scale,
                        cluster_count: index.clusters.len(),
                        clusters: index
                            .clusters
                            .iter()
                            .map(|c| ClusterOutput {
                                timestamp_ms: c.timestamp_ms,
                                file_offset: c.file_offset,
                                cluster_size: c.cluster_size,
                            })
                            .collect(),
                    };

                    println!("{}", serde_json::to_string_pretty(&output)?);
                }

                IndexType::Pgs => {
                    use containers::PgsIndexBuilder;

                    let mut builder = PgsIndexBuilder::open(&file, track)?;
                    let index = builder.build_index(start, end)?;
                    println!("{}", serde_json::to_string(&index)?);
                }
            }
        }

        Commands::StreamPgs {
            file,
            track,
            start,
            end,
            limit,
            index,
        } => {
            use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
            use std::io::Write;

            // Collect frames - either from index or by scanning
            let mkv_frames: Vec<(u64, Vec<u8>)> = if let Some(index_path) = index {
                // Fast path: use pre-built index
                use containers::PgsIndex;

                let index_data = std::fs::read_to_string(&index_path)
                    .context("Failed to read index file")?;
                let idx: PgsIndex = serde_json::from_str(&index_data)
                    .context("Failed to parse index file")?;

                if idx.track != track {
                    anyhow::bail!(
                        "Index is for track {}, but track {} was requested",
                        idx.track,
                        track
                    );
                }

                if start > idx.indexed_through_ms {
                    anyhow::bail!(
                        "Index only covers up to {}ms, requested start is {}ms",
                        idx.indexed_through_ms,
                        start
                    );
                }

                let frames_in_range: Vec<_> = idx.frames.iter()
                    .filter(|f| f.start_ms >= start && (end == 0 || f.start_ms < end))
                    .collect();

                if frames_in_range.is_empty() {
                    return Ok(());
                }

                let mut file_handle = std::fs::File::open(&file)?;
                let mut frames = Vec::new();

                for frame_entry in &frames_in_range {
                    use std::io::{Read, Seek, SeekFrom};
                    file_handle.seek(SeekFrom::Start(frame_entry.offset))?;

                    let mut block_data = vec![0u8; frame_entry.size as usize];
                    file_handle.read_exact(&mut block_data)?;

                    if let Some(payload) = extract_block_payload(&block_data) {
                        frames.push((frame_entry.start_ms, payload));
                    }
                }

                frames
            } else {
                // Slow path: scan through file
                let mut container = MediaContainer::open(&file)?;

                let mut jsonl_data = Vec::new();
                {
                    let mut out = std::io::Cursor::new(&mut jsonl_data);
                    container.stream_track(track, start, end, StreamFormat::Jsonl, &mut out)?;
                }

                let jsonl_str = String::from_utf8_lossy(&jsonl_data);
                let mut frames = Vec::new();

                for line in jsonl_str.lines() {
                    if line.is_empty() {
                        continue;
                    }
                    let frame: serde_json::Value = serde_json::from_str(line)?;
                    let start_ms = frame["start_ms"].as_u64().unwrap_or(0);
                    if let Some(data_b64) = frame["data_base64"].as_str() {
                        if let Ok(data) = BASE64.decode(data_b64) {
                            frames.push((start_ms, data));
                        }
                    }
                }

                frames
            };

            // Convert to SUP and render
            let mut sup_data = pgs::mkv_to_sup(&mkv_frames);
            let pgs_data = match pgs_rs::parse::parse_pgs(&mut sup_data) {
                Ok(p) => p,
                Err(e) => {
                    anyhow::bail!("Failed to parse PGS data: {:?}", e);
                }
            };

            let display_sets = pgs_rs::render::DisplaySetIterator::new(&pgs_data);
            let mut count = 0;
            let mut prev_pts: Option<u64> = None;
            let mut pending_output: Option<serde_json::Value> = None;
            let stdout = io::stdout();
            let mut out = stdout.lock();

            for display_set in display_sets {
                if limit > 0 && count >= limit {
                    break;
                }

                let pts = display_set.presentation_timestamp as u64 / 90;

                if let Some(mut pending) = pending_output.take() {
                    pending["end_ms"] = serde_json::json!(pts);
                    writeln!(out, "{}", pending)?;
                }

                if display_set.is_empty() {
                    prev_pts = Some(pts);
                    continue;
                }

                match pgs::render_display_set(&display_set) {
                    Ok(Some(rendered)) => {
                        let webp_data = pgs::encode_webp(&rendered.rgba, rendered.width, rendered.height, 100)?;
                        let image_b64 = BASE64.encode(&webp_data);

                        pending_output = Some(serde_json::json!({
                            "start_ms": pts,
                            "end_ms": 0,
                            "x": rendered.x,
                            "y": rendered.y,
                            "width": rendered.width,
                            "height": rendered.height,
                            "canvas_width": display_set.width,
                            "canvas_height": display_set.height,
                            "image_base64": image_b64,
                        }));

                        count += 1;
                        prev_pts = Some(pts);
                    }
                    Ok(None) | Err(_) => continue,
                }
            }

            if let Some(mut pending) = pending_output.take() {
                let end_ms = prev_pts.unwrap_or(0) + 5000;
                pending["end_ms"] = serde_json::json!(end_ms);
                writeln!(out, "{}", pending)?;
            }
        }

        Commands::DebugRenderPgs {
            file,
            track,
            output,
            start,
            end,
            limit,
            image_format,
            quality,
        } => {
            use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
            use std::time::Instant;

            let use_webp = match image_format.to_lowercase().as_str() {
                "webp" => true,
                "png" => false,
                _ => anyhow::bail!("Invalid image format '{}'. Use 'webp' or 'png'.", image_format),
            };

            let ext = if use_webp { "webp" } else { "png" };
            std::fs::create_dir_all(&output).context("Failed to create output directory")?;

            eprintln!(
                "Rendering PGS subtitles from track {} to {:?} (format: {})...",
                track, output, ext
            );
            let start_time = Instant::now();

            let mut container = MediaContainer::open(&file)?;

            let mut jsonl_data = Vec::new();
            let mut out_cursor = std::io::Cursor::new(&mut jsonl_data);
            container.stream_track(track, start, end, StreamFormat::Jsonl, &mut out_cursor)?;

            let jsonl_str = String::from_utf8_lossy(&jsonl_data);
            let mut frames: Vec<(u64, Vec<u8>)> = Vec::new();

            for line in jsonl_str.lines() {
                if line.is_empty() {
                    continue;
                }
                let frame: serde_json::Value = serde_json::from_str(line)?;
                let start_ms = frame["start_ms"].as_u64().unwrap_or(0);
                if let Some(data_b64) = frame["data_base64"].as_str() {
                    if let Ok(data) = BASE64.decode(data_b64) {
                        frames.push((start_ms, data));
                    }
                }
            }

            eprintln!("Collected {} MKV frames", frames.len());

            let mut sup_data = pgs::mkv_to_sup(&frames);
            let pgs = match pgs_rs::parse::parse_pgs(&mut sup_data) {
                Ok(p) => p,
                Err(e) => {
                    anyhow::bail!("Failed to parse PGS data: {:?}", e);
                }
            };

            eprintln!("Parsed {} PGS segments", pgs.segments.len());

            let display_sets = pgs_rs::render::DisplaySetIterator::new(&pgs);
            let mut count = 0;
            let mut metadata = Vec::new();

            for display_set in display_sets {
                if count >= limit {
                    break;
                }

                if display_set.is_empty() {
                    continue;
                }

                let pts = display_set.presentation_timestamp as u64 / 90;

                match pgs::render_display_set(&display_set) {
                    Ok(Some(rendered)) => {
                        let filename = format!("sub_{:06}_{}.{}", pts, count, ext);
                        let path = output.join(&filename);

                        if use_webp {
                            pgs::save_webp(&rendered.rgba, rendered.width, rendered.height, &path, quality)?;
                        } else {
                            pgs::save_png(&rendered.rgba, rendered.width, rendered.height, &path)?;
                        }

                        metadata.push(serde_json::json!({
                            "index": count,
                            "start_ms": pts,
                            "end_ms": 0,
                            "width": rendered.width,
                            "height": rendered.height,
                            "x": rendered.x,
                            "y": rendered.y,
                            "canvas_width": display_set.width,
                            "canvas_height": display_set.height,
                            "file": filename,
                        }));

                        count += 1;
                        if count % 10 == 0 {
                            eprint!(".");
                        }
                    }
                    Ok(None) => continue,
                    Err(e) => {
                        eprintln!("\nWarning: Failed to render display set: {:?}", e);
                    }
                }
            }

            let metadata_path = output.join("metadata.json");
            let metadata_json = serde_json::to_string_pretty(&metadata)?;
            std::fs::write(&metadata_path, metadata_json)?;

            let elapsed = start_time.elapsed();
            eprintln!(
                "\nRendered {} PGS frames in {:.2}s",
                count,
                elapsed.as_secs_f64()
            );
            eprintln!("Output: {:?}", output);
            eprintln!("Metadata: {:?}", metadata_path);
        }
    }

    Ok(())
}

/// Extract payload from MKV block data (skip track VINT + timecode + flags)
fn extract_block_payload(block: &[u8]) -> Option<Vec<u8>> {
    if block.len() < 4 {
        return None;
    }

    let first = block[0];
    let vint_len = if first & 0x80 != 0 { 1 }
        else if first & 0x40 != 0 { 2 }
        else if first & 0x20 != 0 { 3 }
        else if first & 0x10 != 0 { 4 }
        else { return None; };

    let header_len = vint_len + 2 + 1; // track VINT + timecode (2) + flags (1)
    if block.len() <= header_len {
        return None;
    }

    Some(block[header_len..].to_vec())
}
