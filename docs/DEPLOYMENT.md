# Viewra Deployment Guide

## Table of Contents
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Production Architecture](#production-architecture)
- [Deployment Options](#deployment-options)
- [Configuration](#configuration)
- [Storage Requirements](#storage-requirements)
- [Security Considerations](#security-considerations)
- [Monitoring](#monitoring)
- [Backup and Recovery](#backup-and-recovery)
- [Troubleshooting](#troubleshooting)

## Overview

This guide covers deploying Viewra with the new two-stage transcoding pipeline and content-addressable storage system in production environments.

## Prerequisites

### System Requirements

**Minimum Requirements:**
- CPU: 4 cores (8+ recommended)
- RAM: 8GB (16GB+ recommended)
- Storage: 100GB SSD for content store
- Network: 1Gbps connection
- OS: Ubuntu 20.04+ or similar Linux distribution

**For GPU Acceleration:**
- NVIDIA GPU with NVENC support
- CUDA 11.0+ drivers
- nvidia-docker runtime

### Software Dependencies
- Docker 20.10+
- Docker Compose 2.0+
- PostgreSQL 13+ (production database)
- Redis 6+ (optional, for caching)
- Nginx or Traefik (reverse proxy)

## Production Architecture

### Component Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Load Balancer │────▶│   Web Frontend  │────▶│     CDN Edge    │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │                                                │
         ▼                                                ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Backend API    │────▶│ Content Store   │────▶│  Object Storage │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│ Transcoding     │────▶│   PostgreSQL    │
│ Workers         │     └─────────────────┘
└─────────────────┘
```

### Storage Architecture

```
/data/
├── content-store/        # Content-addressable storage
│   ├── ab/c1/abc123/    # Sharded by first 4 chars
│   └── de/f4/def456/
├── media/               # Original media files
├── transcoding/         # Temporary transcoding files
└── plugins/             # Plugin binaries
```

## Deployment Options

### Option 1: Docker Compose (Small-Medium Scale)

Create production docker-compose:

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  backend:
    image: viewra/backend:latest
    environment:
      - DATABASE_URL=postgresql://user:pass@postgres:5432/viewra
      - CONTENT_STORE_PATH=/data/content-store
      - REDIS_URL=redis://redis:6379
      - LOG_LEVEL=info
      - TRANSCODE_MAX_CONCURRENT=6
      - ENABLE_METRICS=true
    volumes:
      - content-store:/data/content-store
      - media-files:/data/media
      - ./viewra-data/plugins:/app/plugins
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          cpus: '2'
          memory: 4G

  frontend:
    image: viewra/frontend:latest
    environment:
      - API_URL=https://api.example.com
      - CDN_URL=https://cdn.example.com
    deploy:
      replicas: 2

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=viewra
      - POSTGRES_USER=viewra
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    deploy:
      placement:
        constraints:
          - node.labels.db == true

  redis:
    image: redis:7-alpine
    command: redis-server --maxmemory 2gb --maxmemory-policy allkeys-lru
    volumes:
      - redis-data:/data

  nginx:
    image: nginx:alpine
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    ports:
      - "80:80"
      - "443:443"
    depends_on:
      - backend
      - frontend

volumes:
  content-store:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /mnt/fast-ssd/content-store
  
  media-files:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /mnt/storage/media
  
  postgres-data:
  redis-data:
```

Deploy with:
```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### Option 2: Kubernetes (Large Scale)

Kubernetes deployment manifests:

```yaml
# viewra-backend-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: viewra-backend
spec:
  replicas: 3
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
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: viewra-secrets
              key: database-url
        - name: CONTENT_STORE_PATH
          value: /data/content-store
        - name: TRANSCODE_MAX_CONCURRENT
          value: "4"
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
        volumeMounts:
        - name: content-store
          mountPath: /data/content-store
        - name: media-files
          mountPath: /data/media
      volumes:
      - name: content-store
        persistentVolumeClaim:
          claimName: content-store-pvc
      - name: media-files
        persistentVolumeClaim:
          claimName: media-files-pvc
---
# viewra-transcode-workers.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: viewra-transcode-workers
spec:
  replicas: 5
  selector:
    matchLabels:
      app: viewra-transcode-worker
  template:
    metadata:
      labels:
        app: viewra-transcode-worker
    spec:
      nodeSelector:
        gpu: "true"  # For GPU nodes
      containers:
      - name: worker
        image: viewra/backend:latest
        command: ["/app/viewra", "worker"]
        env:
        - name: WORKER_MODE
          value: "transcode"
        - name: NVIDIA_VISIBLE_DEVICES
          value: "all"
        resources:
          requests:
            memory: "8Gi"
            cpu: "4"
            nvidia.com/gpu: 1
          limits:
            memory: "16Gi"
            cpu: "8"
            nvidia.com/gpu: 1
```

### Option 3: Cloud Platform Deployment

#### AWS ECS/Fargate

```json
{
  "family": "viewra-backend",
  "taskRoleArn": "arn:aws:iam::123456789:role/viewra-task-role",
  "executionRoleArn": "arn:aws:iam::123456789:role/viewra-execution-role",
  "networkMode": "awsvpc",
  "containerDefinitions": [
    {
      "name": "backend",
      "image": "viewra/backend:latest",
      "memory": 8192,
      "cpu": 4096,
      "environment": [
        {"name": "CONTENT_STORE_PATH", "value": "/data/content-store"},
        {"name": "S3_CONTENT_BUCKET", "value": "viewra-content"},
        {"name": "USE_S3_STORAGE", "value": "true"}
      ],
      "mountPoints": [
        {
          "sourceVolume": "efs-content",
          "containerPath": "/data/content-store"
        }
      ]
    }
  ],
  "volumes": [
    {
      "name": "efs-content",
      "efsVolumeConfiguration": {
        "fileSystemId": "fs-12345678",
        "transitEncryption": "ENABLED"
      }
    }
  ]
}
```

## Configuration

### Environment Variables

```bash
# Core Settings
NODE_ENV=production
LOG_LEVEL=info
API_PORT=8080

# Database
DATABASE_URL=postgresql://user:pass@localhost:5432/viewra
DATABASE_POOL_SIZE=20

# Content Storage
CONTENT_STORE_PATH=/data/content-store
CONTENT_BASE_URL=https://cdn.example.com/content
CONTENT_RETENTION_DAYS=90

# Transcoding
TRANSCODE_MAX_CONCURRENT=8
TRANSCODE_QUALITY_PRESET=high
PIPELINE_ENCODER_TIMEOUT=7200
PIPELINE_PACKAGER_TIMEOUT=1200

# CDN Configuration
CDN_ENABLED=true
CDN_PROVIDER=cloudflare
CDN_API_KEY=${CDN_API_KEY}
CDN_PURGE_ON_UPDATE=true

# Redis Cache
REDIS_URL=redis://localhost:6379
CACHE_TTL=3600

# Monitoring
ENABLE_METRICS=true
METRICS_PORT=9090
SENTRY_DSN=${SENTRY_DSN}

# Security
JWT_SECRET=${JWT_SECRET}
CORS_ORIGINS=https://app.example.com
RATE_LIMIT_ENABLED=true
RATE_LIMIT_MAX=100
```

### Production Configuration File

Create `config/production.yaml`:

```yaml
server:
  port: 8080
  host: 0.0.0.0
  readTimeout: 30s
  writeTimeout: 30s

database:
  driver: postgres
  maxOpenConns: 50
  maxIdleConns: 10
  connMaxLifetime: 1h

storage:
  contentStore:
    path: /data/content-store
    shardingLevel: 2
    cleanupEnabled: true
    retentionDays: 90
  
  tempDir: /data/transcoding
  maxTempSize: 100GB

transcoding:
  maxConcurrent: 8
  workerPoolSize: 4
  jobTimeout: 2h
  
  quality:
    presets:
      low:
        videoBitrate: 1500k
        audioBitrate: 128k
        crf: 28
      medium:
        videoBitrate: 3000k
        audioBitrate: 192k
        crf: 23
      high:
        videoBitrate: 6000k
        audioBitrate: 256k
        crf: 18

pipeline:
  encoder:
    timeout: 7200s
    retries: 3
  packager:
    timeout: 1200s
    segmentDuration: 4
    
cdn:
  enabled: true
  providers:
    - name: cloudflare
      baseUrl: https://cdn.example.com
      apiKey: ${CDN_API_KEY}
      
monitoring:
  metrics:
    enabled: true
    port: 9090
  logging:
    level: info
    format: json
  tracing:
    enabled: true
    endpoint: http://jaeger:14268/api/traces
```

## Storage Requirements

### Capacity Planning

Calculate storage needs:

```
Storage Required = (Original Media Size × 1.5) + (Transcoded Content × Retention Period)

Example for 10TB library:
- Original Media: 10TB
- Transcoded Content (3 quality levels): 10TB × 1.5 = 15TB
- With 30-day retention: 15TB × 2 = 30TB
- Total: 10TB + 30TB = 40TB
```

### Storage Optimization

1. **Content Deduplication**
   - Reduces storage by 60-80% for popular content
   - Automatic via content-addressable storage

2. **Tiered Storage**
   ```bash
   # Hot tier (SSD) - Recent/popular content
   /mnt/ssd/content-store/
   
   # Cold tier (HDD) - Archived content
   /mnt/hdd/content-archive/
   ```

3. **S3-Compatible Storage**
   ```yaml
   storage:
     s3:
       enabled: true
       bucket: viewra-content
       region: us-east-1
       endpoint: https://s3.amazonaws.com
   ```

## Security Considerations

### Network Security

1. **Firewall Rules**
   ```bash
   # Allow only necessary ports
   ufw allow 80/tcp   # HTTP (redirect to HTTPS)
   ufw allow 443/tcp  # HTTPS
   ufw allow 22/tcp   # SSH (restrict source IPs)
   ```

2. **SSL/TLS Configuration**
   ```nginx
   server {
       listen 443 ssl http2;
       ssl_certificate /etc/ssl/certs/viewra.crt;
       ssl_certificate_key /etc/ssl/private/viewra.key;
       ssl_protocols TLSv1.2 TLSv1.3;
       ssl_ciphers HIGH:!aNULL:!MD5;
   }
   ```

### Application Security

1. **API Authentication**
   ```yaml
   security:
     jwt:
       secret: ${JWT_SECRET}
       expiration: 24h
     apiKeys:
       enabled: true
       rateLimit: 1000/hour
   ```

2. **Content Protection**
   ```yaml
   content:
     security:
       signedUrls: true
       urlExpiration: 6h
       tokenAuth: true
   ```

### Data Security

1. **Database Encryption**
   ```sql
   -- Enable encryption at rest
   ALTER DATABASE viewra SET encryption = 'on';
   ```

2. **Backup Encryption**
   ```bash
   # Encrypted backups
   pg_dump viewra | gpg --encrypt -r backup@example.com > backup.sql.gpg
   ```

## Monitoring

### Metrics Collection

1. **Prometheus Configuration**
   ```yaml
   # prometheus.yml
   scrape_configs:
     - job_name: 'viewra-backend'
       static_configs:
         - targets: ['backend:9090']
       metrics_path: '/metrics'
   ```

2. **Key Metrics to Monitor**
   - Content store hit/miss rate
   - Transcoding queue depth
   - Pipeline stage durations
   - Storage utilization
   - API response times

### Logging

1. **Centralized Logging**
   ```yaml
   logging:
     driver: "json-file"
     options:
       max-size: "10m"
       max-file: "3"
       labels: "service,environment"
   ```

2. **Log Aggregation**
   ```yaml
   # fluentd configuration
   <source>
     @type forward
     port 24224
   </source>
   
   <match viewra.**>
     @type elasticsearch
     host elasticsearch
     port 9200
     index_name viewra
   </match>
   ```

### Alerting

```yaml
# alerting-rules.yml
groups:
  - name: viewra
    rules:
      - alert: HighTranscodingQueueDepth
        expr: viewra_transcoding_queue_depth > 50
        for: 5m
        annotations:
          summary: "Transcoding queue depth is high"
          
      - alert: LowContentStoreHitRate
        expr: rate(viewra_content_store_hits[5m]) < 0.7
        for: 10m
        annotations:
          summary: "Content store hit rate below 70%"
          
      - alert: StorageSpaceLow
        expr: viewra_storage_free_bytes < 10737418240  # 10GB
        for: 5m
        annotations:
          summary: "Storage space running low"
```

## Backup and Recovery

### Backup Strategy

1. **Database Backups**
   ```bash
   #!/bin/bash
   # backup.sh
   DATE=$(date +%Y%m%d_%H%M%S)
   BACKUP_DIR=/backups
   
   # Database backup
   pg_dump $DATABASE_URL > $BACKUP_DIR/viewra_db_$DATE.sql
   
   # Content metadata backup
   pg_dump $DATABASE_URL \
     --table=content_metadata \
     --table=transcode_sessions \
     > $BACKUP_DIR/viewra_metadata_$DATE.sql
   
   # Compress and encrypt
   tar czf - $BACKUP_DIR/*.sql | \
     gpg --encrypt -r backup@example.com > \
     $BACKUP_DIR/viewra_backup_$DATE.tar.gz.gpg
   
   # Upload to S3
   aws s3 cp $BACKUP_DIR/viewra_backup_$DATE.tar.gz.gpg \
     s3://viewra-backups/daily/
   ```

2. **Content Store Sync**
   ```bash
   # Sync content store to backup location
   rsync -avz --delete \
     /data/content-store/ \
     backup-server:/backups/content-store/
   ```

### Recovery Procedures

1. **Database Recovery**
   ```bash
   # Restore database
   gunzip < backup.sql.gz | psql $DATABASE_URL
   
   # Verify integrity
   psql $DATABASE_URL -c "SELECT COUNT(*) FROM media_files;"
   ```

2. **Content Store Recovery**
   ```bash
   # Restore content store
   rsync -avz \
     backup-server:/backups/content-store/ \
     /data/content-store/
   
   # Verify content hashes
   find /data/content-store -name "*.mpd" | \
     xargs -I {} sha256sum {} > content_verification.txt
   ```

## Troubleshooting

### Common Issues

1. **Transcoding Failures**
   ```bash
   # Check worker logs
   docker logs viewra-backend --tail 100 | grep ERROR
   
   # Verify plugin availability
   curl http://localhost:8080/api/playback/health
   
   # Check FFmpeg processes
   ps aux | grep ffmpeg
   ```

2. **Content Store Issues**
   ```bash
   # Check permissions
   ls -la /data/content-store/
   
   # Verify storage space
   df -h /data/content-store
   
   # Test write permissions
   touch /data/content-store/test && rm /data/content-store/test
   ```

3. **Performance Issues**
   ```bash
   # Check system resources
   htop
   
   # Monitor I/O
   iotop
   
   # Check database connections
   psql $DATABASE_URL -c "SELECT count(*) FROM pg_stat_activity;"
   ```

### Health Checks

```bash
# Backend health
curl http://localhost:8080/api/health

# Transcoding service health
curl http://localhost:8080/api/playback/health

# Content store status
curl http://localhost:8080/api/content/stats
```

### Emergency Procedures

1. **Stop All Transcoding**
   ```bash
   curl -X POST http://localhost:8080/api/playback/stop-all
   ```

2. **Clear Stuck Sessions**
   ```bash
   curl -X POST http://localhost:8080/api/playback/cleanup?max_age_hours=1
   ```

3. **Emergency Shutdown**
   ```bash
   # Graceful shutdown
   docker-compose stop
   
   # Force shutdown
   docker-compose kill
   ```

## Performance Tuning

### System Optimization

1. **Kernel Parameters**
   ```bash
   # /etc/sysctl.conf
   net.core.rmem_max = 134217728
   net.core.wmem_max = 134217728
   net.ipv4.tcp_rmem = 4096 87380 134217728
   net.ipv4.tcp_wmem = 4096 65536 134217728
   vm.swappiness = 10
   ```

2. **File System Tuning**
   ```bash
   # Mount options for content store
   mount -o noatime,nodiratime /dev/sdb1 /data/content-store
   ```

3. **Database Tuning**
   ```sql
   -- PostgreSQL optimization
   ALTER SYSTEM SET shared_buffers = '4GB';
   ALTER SYSTEM SET effective_cache_size = '12GB';
   ALTER SYSTEM SET work_mem = '32MB';
   ALTER SYSTEM SET maintenance_work_mem = '512MB';
   ```

## Scaling Considerations

### Horizontal Scaling

1. **Backend API**: Add more replicas behind load balancer
2. **Transcoding Workers**: Scale based on queue depth
3. **Content Store**: Use distributed storage (Ceph, GlusterFS)
4. **Database**: Read replicas for queries

### Vertical Scaling

1. **GPU Nodes**: For faster transcoding
2. **High-Memory Nodes**: For 4K/8K content
3. **NVMe Storage**: For content store performance

### Auto-Scaling

```yaml
# HPA for Kubernetes
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: viewra-backend-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: viewra-backend
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Pods
    pods:
      metric:
        name: transcoding_queue_depth
      target:
        type: AverageValue
        averageValue: "30"
```