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

        /// Output format: jsonl (JSON lines with timing), raw
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

    /// Render PGS subtitles to PNG files in a directory
    RenderPgs {
        /// Path to the media file
        #[arg(value_name = "FILE")]
        file: PathBuf,

        /// Track number (must be PGS subtitle track)
        #[arg(short, long)]
        track: u64,

        /// Output directory for PNG files
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

        Commands::RenderPgs {
            file,
            track,
            output,
            start,
            end,
            limit,
        } => {
            use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
            use std::time::Instant;

            // Create output directory
            std::fs::create_dir_all(&output).context("Failed to create output directory")?;

            eprintln!("Rendering PGS subtitles from track {} to {:?}...", track, output);
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

                // Get position from first composition object
                let (x, y) = display_set
                    .composition_objects
                    .first()
                    .map(|obj| (obj.horizontal_position, obj.vertical_position))
                    .unwrap_or((0, 0));

                // Render to RGBA using our custom renderer (handles missing palette entries)
                match pgs::render_display_set(&display_set) {
                    Ok(rgba) => {
                        if rgba.is_empty() {
                            continue;
                        }

                        let width = display_set.width as u32;
                        let height = display_set.height as u32;

                        if width == 0 || height == 0 {
                            continue;
                        }

                        // Save PNG
                        let filename = format!("sub_{:06}_{}.png", pts, count);
                        let path = output.join(&filename);

                        pgs::save_png(&rgba, width, height, &path)?;

                        metadata.push(serde_json::json!({
                            "index": count,
                            "start_ms": pts,
                            "end_ms": 0,
                            "width": width,
                            "height": height,
                            "x": x,
                            "y": y,
                            "file": filename,
                        }));

                        count += 1;
                        if count % 10 == 0 {
                            eprint!(".");
                        }
                    }
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
