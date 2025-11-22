-- SQL script to inspect music track metadata in the database
-- This helps diagnose why certain fields might not be populated

-- Show music tracks with their media table fields
SELECT
    m.id,
    m.title,
    m.file_path,
    m.type,
    -- Technical metadata fields
    m.audio_codec,
    m.bit_rate,
    m.duration,
    m.container_format,
    m.file_size,
    m.file_hash,
    -- Music-specific fields from music_tracks table
    mt.artist,
    mt.album,
    mt.track_number,
    mt.year
FROM media m
LEFT JOIN music_tracks mt ON m.id = mt.media_id
WHERE m.type = 'music_track'
ORDER BY m.id DESC
LIMIT 20;

-- Count tracks with missing technical metadata
SELECT
    COUNT(*) as total_tracks,
    SUM(CASE WHEN m.audio_codec IS NULL OR m.audio_codec = '' THEN 1 ELSE 0 END) as missing_audio_codec,
    SUM(CASE WHEN m.bit_rate IS NULL OR m.bit_rate = 0 THEN 1 ELSE 0 END) as missing_bitrate,
    SUM(CASE WHEN m.duration IS NULL OR m.duration = 0 THEN 1 ELSE 0 END) as missing_duration,
    SUM(CASE WHEN m.container_format IS NULL OR m.container_format = '' THEN 1 ELSE 0 END) as missing_container,
    SUM(CASE WHEN m.file_size IS NULL OR m.file_size = 0 THEN 1 ELSE 0 END) as missing_file_size
FROM media m
WHERE m.type = 'music_track';

-- Show example of a single track with all fields
SELECT
    'Media table fields:' as section,
    m.*
FROM media m
WHERE m.type = 'music_track'
LIMIT 1;

SELECT
    'Music tracks table fields:' as section,
    mt.*
FROM music_tracks mt
LIMIT 1;
