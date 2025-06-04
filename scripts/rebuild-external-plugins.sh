#!/bin/bash

# Script to properly rebuild external plugins as standalone executables
set -e

echo "🔨 Rebuilding External Plugins for ViewRA"
echo "========================================"

# Function to rebuild a plugin as executable
rebuild_plugin() {
    local plugin_name="$1"
    local plugin_dir="backend/data/plugins/$plugin_name"
    
    if [ ! -d "$plugin_dir" ]; then
        echo "❌ Plugin directory not found: $plugin_dir"
        return 1
    fi
    
    if [ ! -f "$plugin_dir/main.go" ]; then
        echo "❌ main.go not found in: $plugin_dir"
        return 1
    fi
    
    echo "🔨 Building plugin: $plugin_name"
    
    # Build inside container with CGO enabled for SQLite support
    if docker-compose exec backend sh -c "cd /app/data/plugins/$plugin_name && CGO_ENABLED=1 go build -o $plugin_name main.go"; then
        echo "✅ Successfully built: $plugin_name"
        
        # Verify executable exists
        if docker-compose exec backend test -f "/app/data/plugins/$plugin_name/$plugin_name"; then
            echo "✅ Executable verified: $plugin_name"
            return 0
        else
            echo "❌ Executable not found after build: $plugin_name"
            return 1
        fi
    else
        echo "❌ Failed to build: $plugin_name"
        return 1
    fi
}

# Set plugins to enabled status
enable_plugin() {
    local plugin_id="$1"
    echo "📝 Setting $plugin_id to enabled status"
    docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db "UPDATE plugins SET status = 'enabled', updated_at = datetime('now') WHERE plugin_id = '$plugin_id';"
}

# Main execution
echo ""
echo "🏗️  Building external plugins..."
echo ""

# List of external plugins to build
EXTERNAL_PLUGINS=("tmdb_enricher" "musicbrainz_enricher")

BUILD_SUCCESS=true

for plugin in "${EXTERNAL_PLUGINS[@]}"; do
    if rebuild_plugin "$plugin"; then
        enable_plugin "$plugin"
    else
        BUILD_SUCCESS=false
    fi
    echo ""
done

if [ "$BUILD_SUCCESS" = true ]; then
    echo "🎉 All plugins built successfully!"
    echo ""
    echo "🔄 Restarting backend to load plugins..."
    docker-compose restart backend
    
    echo ""
    echo "⏳ Waiting for plugins to load..."
    sleep 8
    
    echo ""
    echo "📊 Plugin Status:"
    docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db "SELECT plugin_id, status, enabled_at FROM plugins WHERE plugin_id LIKE '%enricher' ORDER BY plugin_id;"
    
    echo ""
    echo "✅ Plugin rebuild and deployment complete!"
else
    echo "❌ Some plugins failed to build. Check the logs above."
    exit 1
fi 