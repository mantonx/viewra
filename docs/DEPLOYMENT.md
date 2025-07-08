# Deployment Guide

## Overview

This guide covers deploying Viewra in production environments.

## System Requirements

### Minimum Requirements
- **CPU**: 4 cores (8+ recommended for transcoding)
- **RAM**: 8GB (16GB+ for multiple concurrent transcodes)
- **Storage**: 50GB for application + media storage needs
- **OS**: Linux (Ubuntu 20.04+ or RHEL 8+)

### Optional Requirements
- **GPU**: NVIDIA GPU with NVENC for hardware transcoding
- **Network**: 1Gbps for smooth streaming

## Deployment Options

### 1. Docker Compose (Recommended)

#### Production Configuration

Create `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  backend:
    image: viewra/backend:latest
    restart: always
    environment:
      - NODE_ENV=production
      - DATABASE_TYPE=postgres
      - DATABASE_HOST=postgres
      - DATABASE_USER=viewra
      - DATABASE_PASSWORD=${DB_PASSWORD}
      - DATABASE_NAME=viewra
    volumes:
      - ./viewra-data:/app/viewra-data
      - /media:/media:ro
    ports:
      - "8080:8080"
    depends_on:
      - postgres

  frontend:
    image: viewra/frontend:latest
    restart: always
    environment:
      - VITE_API_URL=https://api.yourdomain.com
    ports:
      - "80:80"

  postgres:
    image: postgres:15-alpine
    restart: always
    environment:
      - POSTGRES_USER=viewra
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=viewra
    volumes:
      - postgres-data:/var/lib/postgresql/data

volumes:
  postgres-data:
```

#### Deploy Commands

```bash
# Set environment variables
export DB_PASSWORD=your-secure-password

# Pull latest images
docker-compose -f docker-compose.prod.yml pull

# Start services
docker-compose -f docker-compose.prod.yml up -d

# Check status
docker-compose -f docker-compose.prod.yml ps
```

### 2. Kubernetes

Basic Kubernetes deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: viewra-backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: viewra-backend
  template:
    metadata:
      labels:
        app: viewra-backend
    spec:
      containers:
      - name: backend
        image: viewra/backend:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_TYPE
          value: postgres
        - name: DATABASE_HOST
          value: postgres-service
        volumeMounts:
        - name: data
          mountPath: /app/viewra-data
        - name: media
          mountPath: /media
          readOnly: true
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: viewra-data-pvc
      - name: media
        hostPath:
          path: /mnt/media
```

### 3. Bare Metal

#### Prerequisites

```bash
# Install dependencies
sudo apt update
sudo apt install -y postgresql ffmpeg

# Create user and directories
sudo useradd -r -s /bin/false viewra
sudo mkdir -p /opt/viewra /var/lib/viewra
sudo chown viewra:viewra /opt/viewra /var/lib/viewra
```

#### Installation

```bash
# Download binary
wget https://github.com/viewra/releases/download/latest/viewra-linux-amd64
chmod +x viewra-linux-amd64
sudo mv viewra-linux-amd64 /opt/viewra/viewra

# Create systemd service
sudo tee /etc/systemd/system/viewra.service << EOF
[Unit]
Description=Viewra Media Server
After=network.target postgresql.service

[Service]
Type=simple
User=viewra
WorkingDirectory=/opt/viewra
ExecStart=/opt/viewra/viewra
Restart=on-failure
Environment="DATABASE_TYPE=postgres"
Environment="DATABASE_HOST=localhost"
Environment="DATABASE_USER=viewra"
Environment="DATABASE_NAME=viewra"
Environment="VIEWRA_DATA_DIR=/var/lib/viewra"

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl enable viewra
sudo systemctl start viewra
```

## Configuration

### Environment Variables

```bash
# Database
DATABASE_TYPE=postgres
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=viewra
DATABASE_PASSWORD=secure-password
DATABASE_NAME=viewra

# Storage
VIEWRA_DATA_DIR=/var/lib/viewra
MEDIA_PATHS=/media/movies,/media/tv,/media/music

# Transcoding
TRANSCODE_MAX_CONCURRENT=4
TRANSCODE_HARDWARE_ACCELERATION=auto
TRANSCODE_OUTPUT_DIR=/var/lib/viewra/transcoded

# Performance
SCAN_INTERVAL_HOURS=6
CLEANUP_INTERVAL_HOURS=24
SESSION_TIMEOUT_MINUTES=30

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

### Nginx Reverse Proxy

```nginx
server {
    listen 443 ssl http2;
    server_name viewra.yourdomain.com;

    ssl_certificate /etc/ssl/certs/viewra.crt;
    ssl_certificate_key /etc/ssl/private/viewra.key;

    # API and backend
    location /api {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Frontend
    location / {
        proxy_pass http://localhost:80;
        proxy_set_header Host $host;
    }

    # WebSocket support
    location /ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Storage Configuration

### Directory Structure

```
/var/lib/viewra/
├── database/          # SQLite database (if not using PostgreSQL)
├── plugins/           # Plugin binaries
├── transcoded/        # Transcoded content
│   └── content/       # Content-addressable storage
└── temp/              # Temporary files
```

### Permissions

```bash
# Set ownership
sudo chown -R viewra:viewra /var/lib/viewra

# Set permissions
sudo chmod 750 /var/lib/viewra
sudo chmod 755 /var/lib/viewra/transcoded/content
```

## Performance Tuning

### Database Optimization

PostgreSQL configuration:
```sql
-- Increase connection pool
ALTER SYSTEM SET max_connections = 200;

-- Optimize for SSD
ALTER SYSTEM SET random_page_cost = 1.1;

-- Increase shared buffers
ALTER SYSTEM SET shared_buffers = '2GB';
```

### Transcoding Optimization

```bash
# Enable hardware acceleration
TRANSCODE_HARDWARE_ACCELERATION=auto

# Limit concurrent transcodes based on CPU
TRANSCODE_MAX_CONCURRENT=4  # Set to CPU cores

# Use faster preset for real-time
TRANSCODE_SPEED_PRESET=fast
```

## Monitoring

### Health Checks

```bash
# API health
curl https://viewra.yourdomain.com/api/health

# Database health
curl https://viewra.yourdomain.com/api/db-health

# Transcoding status
curl https://viewra.yourdomain.com/api/v1/transcoding/stats
```

### Prometheus Metrics

Enable metrics endpoint:
```bash
ENABLE_METRICS=true
METRICS_PORT=9090
```

Prometheus configuration:
```yaml
scrape_configs:
  - job_name: 'viewra'
    static_configs:
      - targets: ['localhost:9090']
```

### Logging

Configure structured logging:
```bash
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUT=/var/log/viewra/viewra.log
```

Log rotation:
```bash
/var/log/viewra/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
}
```

## Backup

### Database Backup

```bash
# PostgreSQL
pg_dump -U viewra viewra > backup-$(date +%Y%m%d).sql

# SQLite
sqlite3 /var/lib/viewra/database/viewra.db ".backup backup-$(date +%Y%m%d).db"
```

### Media Backup

Transcoded content can be regenerated, so focus on:
- Original media files
- Database backups
- Configuration files
- Plugin configurations

### Automated Backups

```bash
#!/bin/bash
# /etc/cron.daily/viewra-backup

BACKUP_DIR=/backup/viewra
DATE=$(date +%Y%m%d)

# Database
pg_dump -U viewra viewra > $BACKUP_DIR/db-$DATE.sql

# Configuration
tar -czf $BACKUP_DIR/config-$DATE.tar.gz /opt/viewra/config

# Cleanup old backups
find $BACKUP_DIR -name "*.sql" -mtime +7 -delete
find $BACKUP_DIR -name "*.tar.gz" -mtime +7 -delete
```

## Security

### Basic Security

1. **Use HTTPS**: Always use SSL/TLS in production
2. **Firewall**: Only expose necessary ports
3. **Updates**: Keep system and dependencies updated
4. **Passwords**: Use strong passwords for database

### Firewall Rules

```bash
# Allow HTTPS
sudo ufw allow 443/tcp

# Allow SSH (restrict source IP)
sudo ufw allow from YOUR_IP to any port 22

# Enable firewall
sudo ufw enable
```

### File Permissions

```bash
# Restrict configuration files
chmod 600 /opt/viewra/config/*
chown viewra:viewra /opt/viewra/config/*

# Secure media directories
chmod 755 /media
chmod 644 /media/**/*
```

## Troubleshooting

### Common Issues

#### Database Connection Failed
```bash
# Check PostgreSQL status
sudo systemctl status postgresql

# Check connection
psql -U viewra -h localhost -d viewra

# Check logs
sudo journalctl -u viewra -n 100
```

#### Transcoding Failures
```bash
# Check FFmpeg
ffmpeg -version

# Check available space
df -h /var/lib/viewra

# Check permissions
ls -la /var/lib/viewra/transcoded
```

#### High Memory Usage
```bash
# Check memory
free -h

# Limit transcoding
TRANSCODE_MAX_CONCURRENT=2

# Adjust Go garbage collection
GOGC=50
```

### Debug Mode

Enable debug logging:
```bash
LOG_LEVEL=debug
DEBUG_MODE=true
```

### Support

- GitHub Issues: https://github.com/viewra/viewra/issues
- Documentation: https://docs.viewra.app
- Community: https://discord.gg/viewra