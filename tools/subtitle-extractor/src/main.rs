mod containers;
mod pgs;

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};
use std::io;
use std::path::PathBuf;

use containers::{EbmlScanner, MediaContainer, OutputFormat, StreamFormat};

#[derive(Parser)]
#[command(name = "subtitle-extractor")]
#[command(about = "Fast subtitle track extractor for multiple container formats")]
#[command(version)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
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

    /// Build a sparse index for a subtitle track (bypasses broken Cues)
    Index {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,

        /// Track number to index (from 'tracks' command)
        #[arg(short, long)]
        track: u64,
    },

    /// Stream PGS subtitle frames as WebP images (for API integration)
    ///
    /// Outputs JSON lines to stdout, each containing a rendered WebP frame:
    /// {"start_ms": 1234, "end_ms": 5678, "x": 100, "y": 800, "width": 480, "height": 50, "canvas_width": 1920, "canvas_height": 1080, "image_base64": "..."}
    ///
    /// The canvas_width/canvas_height fields indicate the PGS authoring resolution.
    /// For 4K videos with 1080p subtitles, the frontend should scale x/y/width/height
    /// from canvas resolution to video resolution before applying to the display.
    ///
    /// The Go API can pipe this directly to clients or cache individual frames.
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
    },

    /// [Debug] Render PGS subtitles to image files in a directory
    ///
    /// This is primarily for debugging and batch processing. For API integration,
    /// use `stream-pgs` which outputs to stdout.
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

        Commands::Index { file, track } => {
            use std::time::Instant;

            eprintln!("Building cluster index (track {} ignored - clusters contain all tracks)...", track);
            let start_time = Instant::now();

            let mut scanner = EbmlScanner::open(&file)?;
            let index = scanner.build_cluster_index()?;

            let elapsed = start_time.elapsed();
            eprintln!(
                "Index built in {:.2}s: {} clusters",
                elapsed.as_secs_f64(),
                index.clusters.len()
            );

            // Output index as JSON
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

        Commands::StreamPgs {
            file,
            track,
            start,
            end,
            limit,
        } => {
            use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
            use std::io::Write;

            let mut container = MediaContainer::open(&file)?;

            // Stream raw PGS frames from container
            let mut jsonl_data = Vec::new();
            {
                let mut out = std::io::Cursor::new(&mut jsonl_data);
                container.stream_track(track, start, end, StreamFormat::Jsonl, &mut out)?;
            }

            // Parse JSONL to get frames with timestamps
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

            // Convert MKV frames to SUP format for pgs-rs
            let mut sup_data = pgs::mkv_to_sup(&frames);

            // Parse SUP data
            let pgs_data = match pgs_rs::parse::parse_pgs(&mut sup_data) {
                Ok(p) => p,
                Err(e) => {
                    anyhow::bail!("Failed to parse PGS data: {:?}", e);
                }
            };

            // Group into display sets and render
            let display_sets = pgs_rs::render::DisplaySetIterator::new(&pgs_data);
            let mut count = 0;
            let mut prev_pts: Option<u64> = None;
            let mut pending_output: Option<serde_json::Value> = None;
            let stdout = io::stdout();
            let mut out = stdout.lock();

            for display_set in display_sets {
                // Check limit
                if limit > 0 && count >= limit {
                    break;
                }

                // Get timing - PTS is in 90kHz clock ticks, convert to ms
                let pts = display_set.presentation_timestamp as u64 / 90;

                // If we have a pending frame, set its end_ms to current pts
                if let Some(mut pending) = pending_output.take() {
                    pending["end_ms"] = serde_json::json!(pts);
                    writeln!(out, "{}", pending)?;
                }

                // Skip empty display sets (clear commands) - they just mark end times
                if display_set.is_empty() {
                    prev_pts = Some(pts);
                    continue;
                }

                // Render to cropped RGBA with position info
                match pgs::render_display_set(&display_set) {
                    Ok(Some(rendered)) => {
                        // Encode as WebP
                        let webp_data = pgs::encode_webp(&rendered.rgba, rendered.width, rendered.height, 100)?;
                        let image_b64 = BASE64.encode(&webp_data);

                        // Queue this frame (end_ms will be set when we see the next frame)
                        // Include canvas dimensions so frontend can scale correctly for 4K video with 1080p subs
                        pending_output = Some(serde_json::json!({
                            "start_ms": pts,
                            "end_ms": 0,  // Will be updated
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
                    Ok(None) => continue, // Empty render result
                    Err(_) => continue,
                }
            }

            // Output final pending frame with estimated end time
            if let Some(mut pending) = pending_output.take() {
                // Use last pts + 5 seconds as default duration for final subtitle
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

            // Validate image format
            let use_webp = match image_format.to_lowercase().as_str() {
                "webp" => true,
                "png" => false,
                _ => anyhow::bail!("Invalid image format '{}'. Use 'webp' or 'png'.", image_format),
            };

            let ext = if use_webp { "webp" } else { "png" };

            // Create output directory
            std::fs::create_dir_all(&output).context("Failed to create output directory")?;

            eprintln!(
                "Rendering PGS subtitles from track {} to {:?} (format: {})...",
                track, output, ext
            );
            let start_time = Instant::now();

            let mut container = MediaContainer::open(&file)?;

            // Stream frames as JSONL to get timestamps with data
            let mut jsonl_data = Vec::new();
            let mut out = std::io::Cursor::new(&mut jsonl_data);
            container.stream_track(track, start, end, StreamFormat::Jsonl, &mut out)?;

            // Parse JSONL to get frames with timestamps
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

            // Convert MKV frames to SUP format
            let mut sup_data = pgs::mkv_to_sup(&frames);

            // Parse SUP data
            let pgs = match pgs_rs::parse::parse_pgs(&mut sup_data) {
                Ok(p) => p,
                Err(e) => {
                    anyhow::bail!("Failed to parse PGS data: {:?}", e);
                }
            };

            eprintln!("Parsed {} PGS segments", pgs.segments.len());

            // Group into display sets and render
            let display_sets = pgs_rs::render::DisplaySetIterator::new(&pgs);
            let mut count = 0;
            let mut metadata = Vec::new();

            for display_set in display_sets {
                if count >= limit {
                    break;
                }

                // Skip empty display sets (clear commands)
                if display_set.is_empty() {
                    continue;
                }

                // Get timing - PTS is in 90kHz clock ticks
                let pts = display_set.presentation_timestamp as u64 / 90;

                // Render to cropped RGBA using our custom renderer (handles missing palette entries)
                match pgs::render_display_set(&display_set) {
                    Ok(Some(rendered)) => {
                        // Save image in requested format
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
                    Ok(None) => continue, // Empty render result
                    Err(e) => {
                        eprintln!("\nWarning: Failed to render display set: {:?}", e);
                    }
                }
            }

            // Write metadata JSON
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
