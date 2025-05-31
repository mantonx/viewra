#!/bin/bash

# Connection Pool Optimization Script for Viewra Media Server
# This script analyzes your system and suggests optimal connection pool settings

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔍 Analyzing system for optimal database connection pool settings...${NC}\n"

# Detect database type
DB_TYPE=${DATABASE_TYPE:-sqlite}
echo -e "Database Type: ${GREEN}$DB_TYPE${NC}"

# Get system information
CPU_CORES=$(nproc 2>/dev/null || echo "4")
TOTAL_RAM_KB=$(grep MemTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo "4194304")
TOTAL_RAM_GB=$((TOTAL_RAM_KB / 1024 / 1024))

echo -e "CPU Cores: ${GREEN}$CPU_CORES${NC}"
echo -e "Total RAM: ${GREEN}${TOTAL_RAM_GB}GB${NC}"

# Check if running in container
IS_CONTAINER=false
if [ -f /.dockerenv ] || grep -q docker /proc/1/cgroup 2>/dev/null; then
    IS_CONTAINER=true
    echo -e "Environment: ${YELLOW}Container${NC}"
else
    echo -e "Environment: ${GREEN}Host${NC}"
fi

# Analyze current connection pool stats if server is running
echo -e "\n${BLUE}📊 Checking current connection pool usage...${NC}"

BACKEND_URL=${BACKEND_URL:-http://localhost:8080}
CURRENT_STATS=$(curl -s "$BACKEND_URL/api/connection-pool" 2>/dev/null || echo "")

if [ -n "$CURRENT_STATS" ]; then
    echo -e "${GREEN}✅ Backend is running, analyzing current usage...${NC}"
    
    # Parse current stats (requires jq)
    if command -v jq >/dev/null 2>&1; then
        OPEN_CONNS=$(echo "$CURRENT_STATS" | jq -r '.connection_pool.open_connections // 0')
        MAX_CONNS=$(echo "$CURRENT_STATS" | jq -r '.connection_pool.max_open_connections // 0')
        IN_USE=$(echo "$CURRENT_STATS" | jq -r '.connection_pool.in_use // 0')
        WAIT_COUNT=$(echo "$CURRENT_STATS" | jq -r '.connection_pool.wait_count // 0')
        HEALTH_STATUS=$(echo "$CURRENT_STATS" | jq -r '.health_status // "unknown"')
        
        echo -e "Current Open Connections: ${GREEN}$OPEN_CONNS${NC}/$MAX_CONNS"
        echo -e "Connections In Use: ${GREEN}$IN_USE${NC}"
        echo -e "Connection Waits: ${GREEN}$WAIT_COUNT${NC}"
        echo -e "Health Status: ${GREEN}$HEALTH_STATUS${NC}"
        
        if [ "$WAIT_COUNT" -gt "0" ]; then
            echo -e "${YELLOW}⚠️  Warning: Connection waits detected - pool may be undersized${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  jq not available, showing raw stats:${NC}"
        echo "$CURRENT_STATS"
    fi
else
    echo -e "${YELLOW}ℹ️  Backend not running or not accessible${NC}"
fi

echo -e "\n${BLUE}🎯 Calculating optimal settings...${NC}"

# Calculate recommendations based on database type and system resources
case $DB_TYPE in
    "postgres")
        # PostgreSQL can handle more connections efficiently
        BASE_MAX_OPEN=$((CPU_CORES * 8))
        BASE_MAX_IDLE=$((CPU_CORES * 2))
        MAX_OPEN_CONNS=$((BASE_MAX_OPEN > 100 ? 100 : BASE_MAX_OPEN))
        MAX_IDLE_CONNS=$((BASE_MAX_IDLE > 25 ? 25 : BASE_MAX_IDLE))
        CONN_MAX_LIFETIME="2h"
        CONN_MAX_IDLE_TIME="30m"
        ;;
    "sqlite")
        # SQLite is more limited but we can still optimize
        if [ "$IS_CONTAINER" = true ]; then
            MAX_OPEN_CONNS=15
            MAX_IDLE_CONNS=3
        else
            MAX_OPEN_CONNS=$((CPU_CORES * 3))
            MAX_IDLE_CONNS=$((CPU_CORES / 2))
            # Cap SQLite connections
            MAX_OPEN_CONNS=$((MAX_OPEN_CONNS > 25 ? 25 : MAX_OPEN_CONNS))
            MAX_IDLE_CONNS=$((MAX_IDLE_CONNS > 5 ? 5 : MAX_IDLE_CONNS))
        fi
        CONN_MAX_LIFETIME="1h"
        CONN_MAX_IDLE_TIME="15m"
        ;;
    *)
        # Conservative defaults
        MAX_OPEN_CONNS=10
        MAX_IDLE_CONNS=2
        CONN_MAX_LIFETIME="1h"
        CONN_MAX_IDLE_TIME="10m"
        ;;
esac

# Adjust for memory constraints
if [ "$TOTAL_RAM_GB" -lt "4" ]; then
    echo -e "${YELLOW}⚠️  Low memory system detected, reducing connection limits${NC}"
    MAX_OPEN_CONNS=$((MAX_OPEN_CONNS / 2))
    MAX_IDLE_CONNS=$((MAX_IDLE_CONNS / 2))
fi

# Ensure minimums
MAX_OPEN_CONNS=$((MAX_OPEN_CONNS < 5 ? 5 : MAX_OPEN_CONNS))
MAX_IDLE_CONNS=$((MAX_IDLE_CONNS < 1 ? 1 : MAX_IDLE_CONNS))

echo -e "\n${GREEN}🎯 Recommended Connection Pool Settings:${NC}"
echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "DB_MAX_OPEN_CONNS=${GREEN}$MAX_OPEN_CONNS${NC}"
echo -e "DB_MAX_IDLE_CONNS=${GREEN}$MAX_IDLE_CONNS${NC}"
echo -e "DB_CONN_MAX_LIFETIME=${GREEN}$CONN_MAX_LIFETIME${NC}"
echo -e "DB_CONN_MAX_IDLE_TIME=${GREEN}$CONN_MAX_IDLE_TIME${NC}"

# Generate environment variables
echo -e "\n${BLUE}📝 Environment Variables to Add:${NC}"
echo "export DB_MAX_OPEN_CONNS=$MAX_OPEN_CONNS"
echo "export DB_MAX_IDLE_CONNS=$MAX_IDLE_CONNS"
echo "export DB_CONN_MAX_LIFETIME=$CONN_MAX_LIFETIME"
echo "export DB_CONN_MAX_IDLE_TIME=$CONN_MAX_IDLE_TIME"

# Generate Docker Compose environment section
echo -e "\n${BLUE}🐳 Docker Compose Environment Section:${NC}"
cat << EOF
    environment:
      - DB_MAX_OPEN_CONNS=$MAX_OPEN_CONNS
      - DB_MAX_IDLE_CONNS=$MAX_IDLE_CONNS
      - DB_CONN_MAX_LIFETIME=$CONN_MAX_LIFETIME
      - DB_CONN_MAX_IDLE_TIME=$CONN_MAX_IDLE_TIME
EOF

# Additional recommendations
echo -e "\n${BLUE}💡 Additional Recommendations:${NC}"
echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$DB_TYPE" = "sqlite" ]; then
    echo -e "• SQLite detected - consider ${GREEN}Write-Ahead Logging (WAL)${NC} mode for better concurrency"
    echo -e "• For heavy scan workloads, consider upgrading to ${GREEN}PostgreSQL${NC}"
    echo -e "• Monitor ${GREEN}/api/connection-pool${NC} during scans for bottlenecks"
fi

if [ "$IS_CONTAINER" = true ]; then
    echo -e "• Container detected - ensure adequate memory limits"
    echo -e "• Consider persistent volume for database files"
fi

if [ "$CPU_CORES" -gt "8" ]; then
    echo -e "• High-CPU system - consider increasing scanner worker count"
    echo -e "• Monitor CPU utilization during scans"
fi

echo -e "• Use ${GREEN}/api/db-health${NC} endpoint to monitor connection pool health"
echo -e "• Monitor connection waits and high utilization warnings"
echo -e "• Restart backend after applying new connection pool settings"

echo -e "\n${GREEN}✅ Analysis complete! Apply the recommended settings and restart your backend.${NC}"

# Check if we should apply settings automatically
if [ "$1" = "--apply" ] && [ -f ".env" ]; then
    echo -e "\n${BLUE}🔧 Applying settings to .env file...${NC}"
    
    # Backup existing .env
    cp .env .env.backup.$(date +%s)
    
    # Update or add the settings
    sed -i '/^DB_MAX_OPEN_CONNS=/d' .env 2>/dev/null || true
    sed -i '/^DB_MAX_IDLE_CONNS=/d' .env 2>/dev/null || true
    sed -i '/^DB_CONN_MAX_LIFETIME=/d' .env 2>/dev/null || true
    sed -i '/^DB_CONN_MAX_IDLE_TIME=/d' .env 2>/dev/null || true
    
    echo "DB_MAX_OPEN_CONNS=$MAX_OPEN_CONNS" >> .env
    echo "DB_MAX_IDLE_CONNS=$MAX_IDLE_CONNS" >> .env
    echo "DB_CONN_MAX_LIFETIME=$CONN_MAX_LIFETIME" >> .env
    echo "DB_CONN_MAX_IDLE_TIME=$CONN_MAX_IDLE_TIME" >> .env
    
    echo -e "${GREEN}✅ Settings applied to .env file!${NC}"
    echo -e "${YELLOW}🔄 Please restart your backend to apply the new settings.${NC}"
fi 