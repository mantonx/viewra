#!/bin/bash

echo "🧹 Cleaning up redundant database files..."

# Define the correct database path
CORRECT_DB_PATH="./viewra-data/viewra.db"

# List of potential redundant database file patterns
REDUNDANT_PATTERNS=(
    "./backend/data/*.db"
    "./backend/*.db"  
    "./data/*.db"
    "./database.db"
    "./viewra.db"
    "./**/database.db"
    "./**/test.db"
    "./**/temp.db"
)

# Find and list all database files
echo "📋 Current database files found:"
find . -name "*.db" -type f

echo ""
echo "✅ Correct database file: ${CORRECT_DB_PATH}"

# Remove redundant database files (but preserve the correct one)
for pattern in "${REDUNDANT_PATTERNS[@]}"; do
    for file in $pattern; do
        if [[ -f "$file" && "$file" != "$CORRECT_DB_PATH" ]]; then
            echo "🗑️  Removing redundant database file: $file"
            rm -f "$file"
        fi
    done
done

# Ensure the correct database directory exists
mkdir -p "$(dirname "$CORRECT_DB_PATH")"

echo ""
echo "📋 Remaining database files:"
find . -name "*.db" -type f

echo ""
echo "✅ Database cleanup completed!"
echo "   Only using: ${CORRECT_DB_PATH}" 