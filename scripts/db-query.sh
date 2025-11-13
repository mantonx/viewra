#!/bin/bash
# Quick database query script for ViewRA development

DB_PATH="data/viewra.db"

# Color output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

function show_help() {
    echo "ViewRA Database Query Tool"
    echo ""
    echo "Usage: ./scripts/db-query.sh [command]"
    echo ""
    echo "Commands:"
    echo "  tables      - List all tables"
    echo "  libs        - Show libraries"
    echo "  scans       - Show recent scans"
    echo "  media       - Show media files"
    echo "  stats       - Show database statistics"
    echo "  query       - Open interactive SQL shell"
    echo ""
}

function run_query() {
    echo -e "${BLUE}Executing query...${NC}"
    sqlite3 "$DB_PATH" <<EOF
.mode box
.headers on
$1
EOF
}

case "$1" in
    tables)
        run_query "SELECT name, type FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' ORDER BY name;"
        ;;
    libs|libraries)
        run_query "SELECT id, name, type, path, created_at FROM libraries;"
        ;;
    scans)
        run_query "SELECT sj.id, l.name as library, sj.status, ROUND(sj.progress, 1) as progress, sj.files_found, sj.files_processed, datetime(sj.started_at, 'localtime') as started FROM scan_jobs sj JOIN libraries l ON l.id = sj.library_id ORDER BY sj.created_at DESC LIMIT 10;"
        ;;
    media)
        run_query "SELECT m.id, m.file_path, ROUND(m.file_size_bytes / 1024.0 / 1024.0 / 1024.0, 2) as size_gb, m.type, l.name as library FROM media m JOIN libraries l ON l.id = m.library_id ORDER BY m.created_at DESC LIMIT 20;"
        ;;
    stats)
        echo -e "${GREEN}Database Statistics${NC}"
        run_query "SELECT 'Libraries' as metric, COUNT(*) as count FROM libraries UNION ALL SELECT 'Media Items', COUNT(*) FROM media UNION ALL SELECT 'Total Scans', COUNT(*) FROM scan_jobs UNION ALL SELECT 'Active Scans', COUNT(*) FROM scan_jobs WHERE status = 'running';"
        ;;
    query|shell)
        echo -e "${BLUE}Opening SQLite shell (type .quit to exit)${NC}"
        sqlite3 "$DB_PATH" <<EOF
.mode box
.headers on
.prompt '${GREEN}viewra>${NC} '
EOF
        ;;
    ""|help|-h|--help)
        show_help
        ;;
    *)
        echo "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
