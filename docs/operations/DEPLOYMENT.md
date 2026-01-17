# Deployment Guide

Options for deploying ViewRA in production.

## Prerequisites

- FFmpeg 6.0+ (7.x/8.x recommended)
- subtitle-extractor binary (from `make build-tools`)
- At least 2GB RAM recommended
- Storage for media, transcodes, and database

---

## Binary Deployment

### Directory Structure

```text
/opt/viewra/
├── viewra                # Main server binary
├── subtitle-extractor    # Subtitle extraction tool
├── plugins/              # Plugin binaries
│   ├── tmdb
│   ├── musicbrainz
│   └── semantic-search
└── data/                 # Runtime data
    ├── viewra.db         # SQLite database (default)
    ├── cache/            # Image cache
    ├── transcodes/       # Transcode output
    └── plugins/storage/  # Plugin data
```

### systemd Service

Create `/etc/systemd/system/viewra.service`:

```ini
[Unit]
Description=ViewRA Media Server
After=network.target

[Service]
Type=simple
User=viewra
Group=viewra
WorkingDirectory=/opt/viewra
ExecStart=/opt/viewra/viewra
Restart=on-failure
RestartSec=5

# Environment
Environment=ENVIRONMENT=production
Environment=DATA_DIR=/opt/viewra/data
Environment=PORT=8080
Environment=JWT_SECRET=your-secret-here

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/viewra/data /media

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable viewra
sudo systemctl start viewra
```

---

## Docker

### Dockerfile

```dockerfile
FROM debian:bookworm-slim

# Install FFmpeg
RUN apt-get update && apt-get install -y \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy binaries
COPY viewra /app/viewra
COPY subtitle-extractor /app/subtitle-extractor
COPY plugins/ /app/plugins/

WORKDIR /app
EXPOSE 8080

ENV DATA_DIR=/data
ENV ENVIRONMENT=production

VOLUME ["/data", "/media"]

CMD ["/app/viewra"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  viewra:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - viewra-data:/data
      - /path/to/media:/media:ro
    environment:
      - ENVIRONMENT=production
      - JWT_SECRET=${JWT_SECRET}
      - DB_DRIVER=sqlite
    restart: unless-stopped

  # Optional: PostgreSQL
  # postgres:
  #   image: postgres:15
  #   volumes:
  #     - postgres-data:/var/lib/postgresql/data
  #   environment:
  #     - POSTGRES_USER=viewra
  #     - POSTGRES_PASSWORD=${DB_PASSWORD}
  #     - POSTGRES_DB=viewra

volumes:
  viewra-data:
  # postgres-data:
```

### Docker Compose with PostgreSQL

```yaml
version: '3.8'

services:
  viewra:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - viewra-data:/data
      - /path/to/media:/media:ro
    environment:
      - ENVIRONMENT=production
      - JWT_SECRET=${JWT_SECRET}
      - DB_DRIVER=postgres
      - DB_HOST=postgres
      - DB_USER=viewra
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=viewra
    depends_on:
      - postgres
    restart: unless-stopped

  postgres:
    image: postgres:15
    volumes:
      - postgres-data:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=viewra
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=viewra
    restart: unless-stopped

volumes:
  viewra-data:
  postgres-data:
```

---

## Kubernetes

### Basic Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: viewra
spec:
  replicas: 1
  selector:
    matchLabels:
      app: viewra
  template:
    metadata:
      labels:
        app: viewra
    spec:
      containers:
        - name: viewra
          image: viewra:latest
          ports:
            - containerPort: 8080
          env:
            - name: ENVIRONMENT
              value: "production"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: viewra-secrets
                  key: jwt-secret
            - name: DB_DRIVER
              value: "postgres"
            - name: DB_HOST
              value: "postgres-service"
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: viewra-secrets
                  key: db-password
          volumeMounts:
            - name: data
              mountPath: /data
            - name: media
              mountPath: /media
              readOnly: true
          resources:
            requests:
              memory: "512Mi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "2000m"
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: viewra-data
        - name: media
          persistentVolumeClaim:
            claimName: media-library
---
apiVersion: v1
kind: Service
metadata:
  name: viewra
spec:
  selector:
    app: viewra
  ports:
    - port: 8080
      targetPort: 8080
```

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: viewra-secrets
type: Opaque
stringData:
  jwt-secret: "your-secure-jwt-secret"
  db-password: "your-db-password"
```

---

## Reverse Proxy

### nginx

```nginx
server {
    listen 80;
    server_name viewra.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name viewra.example.com;

    ssl_certificate /etc/letsencrypt/live/viewra.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/viewra.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support (for SSE)
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Long timeout for streaming
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    # Large file uploads (for future features)
    client_max_body_size 100M;
}
```

### Caddy

```caddyfile
viewra.example.com {
    reverse_proxy localhost:8080
}
```

---

## Hardware Acceleration in Docker

### NVIDIA GPU

```yaml
services:
  viewra:
    # ...
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    environment:
      - VIEWRA_HW_ACCEL=nvenc
```

### Intel QuickSync/VAAPI

```yaml
services:
  viewra:
    # ...
    devices:
      - /dev/dri:/dev/dri
    environment:
      - VIEWRA_HW_ACCEL=qsv  # or vaapi
```

---

## Health Checks

ViewRA exposes `/health` for monitoring:

```bash
curl http://localhost:8080/health
```

Response includes:
- Server status
- Database connectivity
- Memory usage
- Goroutine count

### Docker health check

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 30s
  timeout: 10s
  retries: 3
```

---

## Backup

### SQLite

```bash
# Stop server or use SQLite backup API
sqlite3 /opt/viewra/data/viewra.db ".backup /backup/viewra-$(date +%Y%m%d).db"
```

### PostgreSQL

```bash
pg_dump -h localhost -U viewra viewra > viewra-$(date +%Y%m%d).sql
```

### Important directories to backup

- `{DATA_DIR}/viewra.db` - SQLite database
- `{DATA_DIR}/cache/images/` - Image cache (can be regenerated)
- `{DATA_DIR}/plugins/storage/` - Plugin data (embeddings, etc.)
