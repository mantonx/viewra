-- ViewRA Development Queries
-- Use with SQLite VS Code Extension (Ctrl+Shift+Q to run)

-- ============================================
-- 📊 DASHBOARD - Quick Overview
-- ============================================
SELECT
    'Libraries' as metric,
    COUNT(*) as count
FROM libraries
UNION ALL
SELECT 'Media Items', COUNT(*) FROM media
UNION ALL
SELECT 'Total Scans', COUNT(*) FROM scan_jobs
UNION ALL
SELECT 'Active Scans', COUNT(*) FROM scan_jobs WHERE status = 'running';

-- ============================================
-- 📚 LIBRARIES - Detailed View
-- ============================================
SELECT
    l.id,
    l.name,
    l.type,
    l.path,
    COUNT(m.id) as media_count,
    datetime(l.created_at, 'localtime') as created_at
FROM libraries l
LEFT JOIN media m ON m.library_id = l.id
GROUP BY l.id
ORDER BY l.created_at DESC;

-- ============================================
-- 🔍 SCAN JOBS - Recent Activity
-- ============================================
SELECT
    sj.id,
    l.name as library,
    sj.status,
    ROUND(sj.progress, 1) || '%' as progress,
    sj.files_found,
    sj.files_processed,
    sj.error_count,
    datetime(sj.started_at, 'localtime') as started,
    CASE
        WHEN sj.completed_at IS NOT NULL
        THEN ROUND((julianday(sj.completed_at) - julianday(sj.started_at)) * 86400, 1) || 's'
        ELSE '⏳ Running'
    END as duration,
    sj.error_message
FROM scan_jobs sj
JOIN libraries l ON l.id = sj.library_id
ORDER BY sj.created_at DESC
LIMIT 10;

-- ============================================
-- 📈 SCAN STATISTICS - Performance Metrics
-- ============================================
SELECT
    l.name as library,
    COUNT(*) as total_scans,
    SUM(CASE WHEN sj.status = 'completed' THEN 1 ELSE 0 END) as completed,
    SUM(CASE WHEN sj.status = 'failed' THEN 1 ELSE 0 END) as failed,
    SUM(CASE WHEN sj.status = 'running' THEN 1 ELSE 0 END) as running,
    AVG(CASE
        WHEN sj.completed_at IS NOT NULL
        THEN (julianday(sj.completed_at) - julianday(sj.started_at)) * 86400
    END) as avg_duration_seconds,
    SUM(sj.files_processed) as total_files_processed
FROM scan_jobs sj
JOIN libraries l ON l.id = sj.library_id
GROUP BY l.id
ORDER BY total_scans DESC;

-- ============================================
-- 🎬 MEDIA FILES - Latest Additions
-- ============================================
SELECT
    m.id,
    m.file_path,
    ROUND(m.file_size_bytes / 1024.0 / 1024.0 / 1024.0, 2) as size_gb,
    m.type,
    l.name as library,
    datetime(m.created_at, 'localtime') as added_at
FROM media m
JOIN libraries l ON l.id = m.library_id
ORDER BY m.created_at DESC
LIMIT 20;

-- ============================================
-- 🐛 ERRORS - Failed Scans Investigation
-- ============================================
SELECT
    sj.id,
    l.name as library,
    sj.error_message,
    sj.files_processed,
    sj.error_count,
    datetime(sj.started_at, 'localtime') as started,
    datetime(sj.completed_at, 'localtime') as completed
FROM scan_jobs sj
JOIN libraries l ON l.id = sj.library_id
WHERE sj.status = 'failed'
ORDER BY sj.created_at DESC;

-- ============================================
-- 🔧 DATABASE SCHEMA - View Table Structures
-- ============================================
SELECT
    name as table_name,
    sql as create_statement
FROM sqlite_master
WHERE type = 'table'
AND name NOT LIKE 'sqlite_%'
ORDER BY name;

-- ============================================
-- 📦 TABLE SIZES - Row Counts
-- ============================================
SELECT
    (SELECT COUNT(*) FROM libraries) as libraries,
    (SELECT COUNT(*) FROM media) as media,
    (SELECT COUNT(*) FROM scan_jobs) as scan_jobs,
    (SELECT COUNT(*) FROM movies) as movies,
    (SELECT COUNT(*) FROM tv_shows) as tv_shows,
    (SELECT COUNT(*) FROM tv_seasons) as tv_seasons,
    (SELECT COUNT(*) FROM tv_episodes) as tv_episodes,
    (SELECT COUNT(*) FROM music_tracks) as music_tracks;
