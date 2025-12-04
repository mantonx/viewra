//! PGS (Presentation Graphic Stream) subtitle decoder
//!
//! Decodes PGS bitmap subtitles to RGBA images using pgs-rs for parsing,
//! with a custom renderer that handles missing palette entries gracefully.
//!
//! MKV stores PGS segments without the SUP format headers. This module handles
//! converting MKV frame data to the SUP format expected by pgs-rs.

use anyhow::{Context, Result};
use pgs_rs::render::DisplaySet;
use std::io::BufWriter;
use std::path::Path;

/// Convert raw MKV PGS frame data to SUP format
///
/// MKV stores PGS segments without the SUP header (PG magic + PTS + DTS).
/// This function wraps each segment with the appropriate header.
///
/// MKV frame data format: [segment_type (1 byte)][length (2 bytes)][data...]
/// SUP format: [PG (2 bytes)][PTS (4 bytes)][DTS (4 bytes)][type (1 byte)][length (2 bytes)][data...]
pub fn mkv_to_sup(frames: &[(u64, Vec<u8>)]) -> Vec<u8> {
    let mut sup_data = Vec::new();

    for (timestamp_ms, frame_data) in frames {
        // Convert ms to 90kHz PTS ticks
        let pts = (*timestamp_ms * 90) as u32;
        let dts = pts; // Usually same as PTS for subtitles

        // Each MKV frame can contain multiple PGS segments
        // Parse the frame to find segment boundaries
        let mut pos = 0;
        while pos < frame_data.len() {
            if pos + 3 > frame_data.len() {
                break;
            }

            let segment_type = frame_data[pos];
            let segment_len = u16::from_be_bytes([frame_data[pos + 1], frame_data[pos + 2]]) as usize;

            if pos + 3 + segment_len > frame_data.len() {
                break;
            }

            // Write SUP header
            sup_data.extend_from_slice(&[0x50, 0x47]); // "PG" magic
            sup_data.extend_from_slice(&pts.to_be_bytes()); // PTS
            sup_data.extend_from_slice(&dts.to_be_bytes()); // DTS

            // Write segment type, length, and data
            sup_data.push(segment_type);
            sup_data.extend_from_slice(&frame_data[pos + 1..pos + 3 + segment_len]);

            pos += 3 + segment_len;
        }
    }

    sup_data
}

/// Render a PGS DisplaySet to RGBA buffer
///
/// This is a custom renderer that handles missing palette entries gracefully
/// by treating them as transparent. The pgs-rs renderer errors on missing entries.
pub fn render_display_set(display_set: &DisplaySet) -> Result<Vec<u8>> {
    let width = display_set.width as usize;
    let height = display_set.height as usize;

    if width == 0 || height == 0 {
        return Ok(Vec::new());
    }

    // Create RGBA buffer (initialized to transparent)
    let mut rgba = vec![0u8; width * height * 4];

    // Get the palette (use palette ID 0 as default, like pgs-rs)
    let palette = match display_set.palettes.get(&0) {
        Some(p) => p,
        None => return Ok(Vec::new()), // No palette = nothing to render
    };

    // Render each composition object
    for comp_obj in display_set.composition_objects {
        let obj = match display_set.objects.get(&comp_obj.id) {
            Some(o) => o,
            None => continue,
        };

        let obj_x = comp_obj.horizontal_position as usize;
        let obj_y = comp_obj.vertical_position as usize;
        let obj_width = obj.width as usize;

        // Decode RLE data and write to buffer
        let mut x = 0usize;
        let mut y = 0usize;

        for rle in &obj.data.0 {
            let color_id = rle.color;
            let count = rle.count as usize;

            // Look up color in palette, treat missing as transparent
            let (r, g, b, a) = if let Some(entry) = palette.entries.get(&color_id) {
                // Convert YCbCr to RGB
                ycbcr_to_rgba(
                    entry.luminance,
                    entry.color_difference_blue,
                    entry.color_difference_red,
                    entry.alpha,
                )
            } else {
                // Missing palette entry = transparent
                (0, 0, 0, 0)
            };

            // Write pixels
            for _ in 0..count {
                if x >= obj_width {
                    x = 0;
                    y += 1;
                }

                let px = obj_x + x;
                let py = obj_y + y;

                if px < width && py < height {
                    let idx = (py * width + px) * 4;
                    rgba[idx] = r;
                    rgba[idx + 1] = g;
                    rgba[idx + 2] = b;
                    rgba[idx + 3] = a;
                }

                x += 1;
            }
        }
    }

    Ok(rgba)
}

/// Convert YCbCr (BT.601) to RGBA
fn ycbcr_to_rgba(y: u8, cb: u8, cr: u8, a: u8) -> (u8, u8, u8, u8) {
    // BT.601 limited range (16-235 for Y, 16-240 for Cb/Cr)
    let y = y as f32;
    let cb = cb as f32 - 128.0;
    let cr = cr as f32 - 128.0;

    // BT.601 conversion matrix
    let r = y + 1.402 * cr;
    let g = y - 0.344136 * cb - 0.714136 * cr;
    let b = y + 1.772 * cb;

    // Clamp to 0-255
    let r = r.clamp(0.0, 255.0) as u8;
    let g = g.clamp(0.0, 255.0) as u8;
    let b = b.clamp(0.0, 255.0) as u8;

    (r, g, b, a)
}

/// Encode RGBA buffer as PNG
#[allow(dead_code)]
pub fn encode_png(rgba: &[u8], width: u32, height: u32) -> Result<Vec<u8>> {
    let mut png_data = Vec::new();

    {
        let mut encoder = png::Encoder::new(&mut png_data, width, height);
        encoder.set_color(png::ColorType::Rgba);
        encoder.set_depth(png::BitDepth::Eight);
        encoder.set_compression(png::Compression::Fast);

        let mut writer = encoder.write_header().context("Failed to write PNG header")?;
        writer
            .write_image_data(rgba)
            .context("Failed to write PNG data")?;
    }

    Ok(png_data)
}

/// Save RGBA buffer as PNG file
pub fn save_png(rgba: &[u8], width: u32, height: u32, path: &Path) -> Result<()> {
    let file = std::fs::File::create(path).context("Failed to create PNG file")?;
    let writer = BufWriter::new(file);

    let mut encoder = png::Encoder::new(writer, width, height);
    encoder.set_color(png::ColorType::Rgba);
    encoder.set_depth(png::BitDepth::Eight);
    encoder.set_compression(png::Compression::Fast);

    let mut png_writer = encoder.write_header().context("Failed to write PNG header")?;
    png_writer
        .write_image_data(rgba)
        .context("Failed to write PNG data")?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_encode_png_empty() {
        // 1x1 transparent pixel
        let rgba = vec![0, 0, 0, 0];
        let result = encode_png(&rgba, 1, 1);
        assert!(result.is_ok());
        let png = result.unwrap();
        // PNG magic bytes
        assert_eq!(&png[0..8], &[0x89, b'P', b'N', b'G', 0x0D, 0x0A, 0x1A, 0x0A]);
    }
}
