//! Shared EBML parsing primitives.
//!
//! This module provides low-level EBML parsing functions used across multiple
//! container parsing modules (ebml.rs, cluster_cache.rs, pgs_index.rs).
//!
//! EBML (Extensible Binary Meta Language) is the binary format underlying
//! Matroska/WebM containers.

use anyhow::{bail, Result};
use std::io::Read;

/// Read an EBML element header (ID + size).
///
/// Returns (element_id, data_size).
pub fn read_element_header<R: Read>(reader: &mut R) -> Result<(u32, u64)> {
    let (id, _) = read_element_id(reader)?;
    let (size, _) = read_vint(reader)?;
    Ok((id, size))
}

/// Read a variable-length EBML element ID.
///
/// Returns (full_id, byte_length).
/// The full_id includes the length marker bits for proper element identification.
pub fn read_element_id<R: Read>(reader: &mut R) -> Result<(u32, u64)> {
    let mut first = [0u8; 1];
    reader.read_exact(&mut first)?;
    let first = first[0];

    let (len, mask) = if first & 0x80 != 0 {
        (1, 0x7F)
    } else if first & 0x40 != 0 {
        (2, 0x3F)
    } else if first & 0x20 != 0 {
        (3, 0x1F)
    } else if first & 0x10 != 0 {
        (4, 0x0F)
    } else {
        bail!("Invalid EBML element ID");
    };

    let mut id = (first & mask) as u32;
    for _ in 1..len {
        let mut b = [0u8; 1];
        reader.read_exact(&mut b)?;
        id = (id << 8) | (b[0] as u32);
    }

    // Reconstruct full ID with length marker
    let full_id = match len {
        1 => 0x80 | id,
        2 => 0x4000 | id,
        3 => 0x200000 | id,
        4 => 0x10000000 | id,
        _ => id,
    };

    Ok((full_id, len))
}

/// Read a variable-length integer (VINT).
///
/// Returns (value, byte_length).
/// VINTs are used for element sizes and some element values in EBML.
pub fn read_vint<R: Read>(reader: &mut R) -> Result<(u64, u64)> {
    let mut first = [0u8; 1];
    reader.read_exact(&mut first)?;
    let first = first[0];

    let (len, mask) = if first & 0x80 != 0 {
        (1, 0x7F)
    } else if first & 0x40 != 0 {
        (2, 0x3F)
    } else if first & 0x20 != 0 {
        (3, 0x1F)
    } else if first & 0x10 != 0 {
        (4, 0x0F)
    } else if first & 0x08 != 0 {
        (5, 0x07)
    } else if first & 0x04 != 0 {
        (6, 0x03)
    } else if first & 0x02 != 0 {
        (7, 0x01)
    } else if first & 0x01 != 0 {
        (8, 0x00)
    } else {
        bail!("Invalid VINT");
    };

    let mut value = (first & mask) as u64;
    for _ in 1..len {
        let mut b = [0u8; 1];
        reader.read_exact(&mut b)?;
        value = (value << 8) | (b[0] as u64);
    }

    Ok((value, len))
}

/// Read an unsigned integer of the given byte size.
pub fn read_uint<R: Read>(reader: &mut R, size: u64) -> Result<u64> {
    let mut value: u64 = 0;
    for _ in 0..size {
        let mut b = [0u8; 1];
        reader.read_exact(&mut b)?;
        value = (value << 8) | (b[0] as u64);
    }
    Ok(value)
}

/// Read a floating-point number (4 or 8 bytes).
pub fn read_float<R: Read>(reader: &mut R, size: u64) -> Result<f64> {
    match size {
        4 => {
            let mut buf = [0u8; 4];
            reader.read_exact(&mut buf)?;
            Ok(f32::from_be_bytes(buf) as f64)
        }
        8 => {
            let mut buf = [0u8; 8];
            reader.read_exact(&mut buf)?;
            Ok(f64::from_be_bytes(buf))
        }
        _ => bail!("Invalid float size: {}", size),
    }
}

// Common EBML Element IDs (Matroska)
pub mod ids {
    pub const EBML_HEADER: u32 = 0x1A45DFA3;
    pub const SEGMENT: u32 = 0x18538067;
    pub const SEGMENT_INFO: u32 = 0x1549A966;
    pub const TIMESTAMP_SCALE: u32 = 0x2AD7B1;
    pub const DURATION: u32 = 0x4489;
    pub const CLUSTER: u32 = 0x1F43B675;
    pub const CLUSTER_TIMESTAMP: u32 = 0xE7;
    pub const SIMPLE_BLOCK: u32 = 0xA3;
    pub const BLOCK_GROUP: u32 = 0xA0;
    pub const BLOCK: u32 = 0xA1;
    pub const BLOCK_DURATION: u32 = 0x9B;
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn test_read_vint_1_byte() {
        // 0x81 = 1000 0001 -> value = 1
        let mut cursor = Cursor::new(vec![0x81]);
        let (value, len) = read_vint(&mut cursor).unwrap();
        assert_eq!(value, 1);
        assert_eq!(len, 1);
    }

    #[test]
    fn test_read_vint_2_bytes() {
        // 0x40 0x01 = 0100 0000 0000 0001 -> value = 1
        let mut cursor = Cursor::new(vec![0x40, 0x01]);
        let (value, len) = read_vint(&mut cursor).unwrap();
        assert_eq!(value, 1);
        assert_eq!(len, 2);
    }

    #[test]
    fn test_read_element_id_1_byte() {
        // 0xE7 = Cluster Timestamp ID
        let mut cursor = Cursor::new(vec![0xE7]);
        let (id, len) = read_element_id(&mut cursor).unwrap();
        assert_eq!(id, 0xE7);
        assert_eq!(len, 1);
    }

    #[test]
    fn test_read_element_id_4_bytes() {
        // Cluster ID: 0x1F 0x43 0xB6 0x75
        let mut cursor = Cursor::new(vec![0x1F, 0x43, 0xB6, 0x75]);
        let (id, len) = read_element_id(&mut cursor).unwrap();
        assert_eq!(id, 0x1F43B675);
        assert_eq!(len, 4);
    }

    #[test]
    fn test_read_uint() {
        let mut cursor = Cursor::new(vec![0x01, 0x02, 0x03]);
        let value = read_uint(&mut cursor, 3).unwrap();
        assert_eq!(value, 0x010203);
    }
}
