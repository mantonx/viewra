//! PGS (Presentation Graphic Stream) subtitle decoder
//!
//! Decodes PGS bitmap subtitles to RGBA images using pgs-rs for parsing,
//! with a custom renderer that handles missing palette entries gracefully.
//!
//! MKV stores PGS segments without the SUP format headers. This module handles
//! converting MKV frame data to the SUP format expected by pgs-rs.
//!
//! Note: Some MKV files store PGS data with zlib compression. This module
//! automatically detects and decompresses such data.

use anyhow::{Context, Result};
use flate2::read::ZlibDecoder;
use pgs_rs::render::DisplaySet;
use std::io::{BufWriter, Read};
use std::path::Path;

/// Check if data is zlib compressed (starts with zlib header)
fn is_zlib_compressed(data: &[u8]) -> bool {
    if data.len() < 2 {
        return false;
    }
    // Zlib header bytes: 0x78 followed by 0x01, 0x5e, 0x9c, or 0xda
    // 0x78 = deflate with 32K window
    // Second byte: compression level/dict flag
    data[0] == 0x78 && (data[1] == 0x01 || data[1] == 0x5e || data[1] == 0x9c || data[1] == 0xda)
}

/// Decompress zlib data, returning the original data if decompression fails
fn try_decompress_zlib(data: &[u8]) -> Vec<u8> {
    if !is_zlib_compressed(data) {
        return data.to_vec();
    }

    let mut decoder = ZlibDecoder::new(data);
    let mut decompressed = Vec::new();
    match decoder.read_to_end(&mut decompressed) {
        Ok(_) => decompressed,
        Err(_) => data.to_vec(), // Fall back to original if decompression fails
    }
}

/// Convert raw MKV PGS frame data to SUP format
///
/// MKV stores PGS segments without the SUP header (PG magic + PTS + DTS).
/// This function wraps each segment with the appropriate header.
///
/// Note: Some MKV files store PGS data with zlib compression. This function
/// automatically detects and decompresses such data.
///
/// MKV frame data format: [segment_type (1 byte)][length (2 bytes)][data...]
/// SUP format: [PG (2 bytes)][PTS (4 bytes)][DTS (4 bytes)][type (1 byte)][length (2 bytes)][data...]
pub fn mkv_to_sup(frames: &[(u64, Vec<u8>)]) -> Vec<u8> {
    let mut sup_data = Vec::new();

    for (timestamp_ms, frame_data) in frames {
        // Convert ms to 90kHz PTS ticks
        let pts = (*timestamp_ms * 90) as u32;
        let dts = pts; // Usually same as PTS for subtitles

        // Decompress if zlib compressed
        let decompressed = try_decompress_zlib(frame_data);

        // Each MKV frame can contain multiple PGS segments
        // Parse the frame to find segment boundaries
        let mut pos = 0;
        while pos < decompressed.len() {
            if pos + 3 > decompressed.len() {
                break;
            }

            let segment_type = decompressed[pos];
            let segment_len = u16::from_be_bytes([decompressed[pos + 1], decompressed[pos + 2]]) as usize;

            if pos + 3 + segment_len > decompressed.len() {
                break;
            }

            // Write SUP header
            sup_data.extend_from_slice(&[0x50, 0x47]); // "PG" magic
            sup_data.extend_from_slice(&pts.to_be_bytes()); // PTS
            sup_data.extend_from_slice(&dts.to_be_bytes()); // DTS

            // Write segment type, length, and data
            sup_data.push(segment_type);
            sup_data.extend_from_slice(&decompressed[pos + 1..pos + 3 + segment_len]);

            pos += 3 + segment_len;
        }
    }

    sup_data
}

/// Result of rendering a PGS DisplaySet - includes cropped image and position
pub struct RenderedSubtitle {
    /// RGBA pixel data for the cropped subtitle region
    pub rgba: Vec<u8>,
    /// X position of the subtitle in the video frame
    pub x: u32,
    /// Y position of the subtitle in the video frame
    pub y: u32,
    /// Width of the cropped subtitle image
    pub width: u32,
    /// Height of the cropped subtitle image
    pub height: u32,
}

/// Render a PGS DisplaySet to a cropped RGBA buffer
///
/// This is a custom renderer that handles missing palette entries gracefully
/// by treating them as transparent. The pgs-rs renderer errors on missing entries.
///
/// Returns the cropped subtitle image along with its position in the video frame.
pub fn render_display_set(display_set: &DisplaySet) -> Result<Option<RenderedSubtitle>> {
    // Get the palette (use palette ID 0 as default, like pgs-rs)
    let palette = match display_set.palettes.get(&0) {
        Some(p) => p,
        None => return Ok(None), // No palette = nothing to render
    };

    // First pass: calculate bounding box of all composition objects
    let mut min_x = u32::MAX;
    let mut min_y = u32::MAX;
    let mut max_x = 0u32;
    let mut max_y = 0u32;

    for comp_obj in display_set.composition_objects {
        let obj = match display_set.objects.get(&comp_obj.id) {
            Some(o) => o,
            None => continue,
        };

        let obj_x = comp_obj.horizontal_position as u32;
        let obj_y = comp_obj.vertical_position as u32;
        let obj_w = obj.width as u32;
        let obj_h = obj.height as u32;

        if obj_w > 0 && obj_h > 0 {
            min_x = min_x.min(obj_x);
            min_y = min_y.min(obj_y);
            max_x = max_x.max(obj_x + obj_w);
            max_y = max_y.max(obj_y + obj_h);
        }
    }

    // No valid objects found
    if min_x == u32::MAX || max_x == 0 {
        return Ok(None);
    }

    let crop_width = (max_x - min_x) as usize;
    let crop_height = (max_y - min_y) as usize;

    if crop_width == 0 || crop_height == 0 {
        return Ok(None);
    }

    // Create RGBA buffer for cropped region (initialized to transparent)
    let mut rgba = vec![0u8; crop_width * crop_height * 4];

    // Second pass: render each composition object into the cropped buffer
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

            // Write pixels (adjusted for crop offset)
            for _ in 0..count {
                if x >= obj_width {
                    x = 0;
                    y += 1;
                }

                let px = obj_x + x - min_x as usize;
                let py = obj_y + y - min_y as usize;

                if px < crop_width && py < crop_height {
                    let idx = (py * crop_width + px) * 4;
                    rgba[idx] = r;
                    rgba[idx + 1] = g;
                    rgba[idx + 2] = b;
                    rgba[idx + 3] = a;
                }

                x += 1;
            }
        }
    }

    Ok(Some(RenderedSubtitle {
        rgba,
        x: min_x,
        y: min_y,
        width: crop_width as u32,
        height: crop_height as u32,
    }))
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

/// Encode RGBA buffer as WebP (lossless - best for subtitles)
///
/// Note: The `quality` parameter is currently unused as the `image` crate's
/// WebP encoder only supports lossless encoding. Lossless is ideal for
/// subtitle overlays anyway since it preserves text sharpness.
pub fn encode_webp(rgba: &[u8], width: u32, height: u32, _quality: u8) -> Result<Vec<u8>> {
    use image::codecs::webp::WebPEncoder;
    use image::ExtendedColorType;

    let mut webp_data = Vec::new();

    {
        let encoder = WebPEncoder::new_lossless(&mut webp_data);
        encoder.encode(rgba, width, height, ExtendedColorType::Rgba8)
            .context("Failed to encode WebP")?;
    }

    Ok(webp_data)
}

/// Save RGBA buffer as WebP file
pub fn save_webp(rgba: &[u8], width: u32, height: u32, path: &Path, quality: u8) -> Result<()> {
    let webp_data = encode_webp(rgba, width, height, quality)?;
    std::fs::write(path, webp_data).context("Failed to write WebP file")?;
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
