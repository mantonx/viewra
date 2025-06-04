#!/bin/bash

echo "=== Viewra Plugin Health Check ==="

# Function to check if Docker Compose is running
check_docker() {
    if ! docker-compose ps | grep -q "viewra-backend.*Up"; then
        echo "❌ Backend service is not running"
        echo "   Run: docker-compose up -d"
        return 1
    fi
    echo "✅ Backend service is running"
    return 0
}

# Function to check database plugin status
check_database_status() {
    echo -e "\n📊 Database Plugin Status:"
    docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db <<EOF
.headers on
.mode column
SELECT 
    plugin_id,
    name,
    status,
    CASE 
        WHEN enabled_at IS NULL THEN 'never'
        ELSE enabled_at 
    END as enabled_at
FROM plugins 
WHERE type = 'external'
ORDER BY plugin_id;
EOF
}

# Function to check plugin tables
check_plugin_tables() {
    echo -e "\n📋 Plugin Database Tables:"
    TABLES=$(docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db ".tables" | tr ' ' '\n' | grep -E "(tmdb|musicbrainz|music_brainz|tm_db)" | sort)
    
    if [ -z "$TABLES" ]; then
        echo "❌ No plugin tables found"
        echo "   This indicates plugins are not properly loaded"
        return 1
    else
        echo "✅ Plugin tables found:"
        echo "$TABLES" | sed 's/^/   - /'
        return 0
    fi
}

# Function to check enrichment activity
check_enrichment_activity() {
    echo -e "\n🔍 Enrichment Activity:"
    docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db <<EOF
.headers on
.mode column
SELECT 
    plugin,
    COUNT(*) as enrichments,
    MAX(updated_at) as last_activity
FROM media_enrichments 
GROUP BY plugin
ORDER BY plugin;
EOF
}

# Function to check enabled vs running status
check_enabled_vs_running() {
    echo -e "\n⚠️  Status Analysis:"
    
    ENABLED_COUNT=$(docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db "SELECT COUNT(*) FROM plugins WHERE type = 'external' AND status = 'enabled';" | tr -d '\r')
    RUNNING_COUNT=$(docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db "SELECT COUNT(*) FROM plugins WHERE type = 'external' AND status = 'running';" | tr -d '\r')
    
    echo "   Enabled plugins: $ENABLED_COUNT"
    echo "   Running plugins: $RUNNING_COUNT"
    
    if [ "$ENABLED_COUNT" -gt 0 ] && [ "$RUNNING_COUNT" -eq 0 ]; then
        echo "❌ Plugins are enabled but not running!"
        echo "   This is the bug we're fixing"
        return 1
    elif [ "$ENABLED_COUNT" -gt "$RUNNING_COUNT" ]; then
        echo "⚠️  Some enabled plugins are not running"
        echo "   Fix: Restart backend to trigger auto-loading"
        return 2
    else
        echo "✅ Plugin status looks good"
        return 0
    fi
}

# Function to fix plugin loading issues
fix_plugin_loading() {
    echo -e "\n🔧 Applying Plugin Loading Fix:"
    
    # Update enabled plugins to running status to trigger auto-loading
    echo "   Setting enabled plugins to running status..."
    docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db "UPDATE plugins SET status = 'running' WHERE type = 'external' AND status = 'enabled';"
    
    echo "   Restarting backend service..."
    docker-compose restart backend
    
    echo "   Waiting for backend to start..."
    sleep 10
    
    echo "✅ Fix applied - backend restarted"
}

# Function to verify fix was successful
verify_fix() {
    echo -e "\n🎯 Verification:"
    
    # Wait a bit more for plugins to load
    sleep 15
    
    # Check if plugin tables exist now
    if check_plugin_tables; then
        echo "✅ Plugin tables found - plugins loaded successfully"
    else
        echo "❌ Plugin tables still missing"
        return 1
    fi
    
    # Check enrichment activity
    TMDB_ENRICHMENTS=$(docker-compose exec -T backend sqlite3 /app/viewra-data/viewra.db "SELECT COUNT(*) FROM media_enrichments WHERE plugin = 'tmdb_enricher';" 2>/dev/null | tr -d '\r')
    
    if [ "$TMDB_ENRICHMENTS" -gt 0 ]; then
        echo "✅ TMDb plugin is actively creating enrichments ($TMDB_ENRICHMENTS total)"
    else
        echo "⚠️  TMDb plugin not yet creating enrichments (may need more time)"
    fi
}

# Main execution
main() {
    check_docker || exit 1
    
    check_database_status
    
    if ! check_plugin_tables; then
        echo -e "\n🚨 Issue detected: Plugin tables missing"
    fi
    
    check_enrichment_activity
    
    STATUS_RESULT=$(check_enabled_vs_running)
    STATUS_CODE=$?
    
    if [ $STATUS_CODE -eq 1 ]; then
        echo -e "\n🚨 Critical Issue: Plugins enabled but not running"
        echo "   This is exactly the bug we're fixing!"
        
        read -p "Apply automatic fix? (y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            fix_plugin_loading
            verify_fix
        else
            echo "Manual fix: Run 'docker-compose restart backend'"
        fi
        
    elif [ $STATUS_CODE -eq 2 ]; then
        echo -e "\n⚠️  Warning: Some plugins not running"
        echo "   Recommend restarting backend"
        
    else
        echo -e "\n✅ All checks passed - plugins appear healthy"
    fi
    
    echo -e "\n=== Health Check Complete ==="
}

# Run main function
main "$@" 