# ViewRA Scaling Guide

**Target Audience**: Deploying ViewRA for multiple users or large media libraries

**Note**: The current ViewRA implementation is optimized for single-user, local deployment. This document outlines strategies for scaling to multi-user or high-traffic scenarios.

---

## Current Architecture (Single-User)

ViewRA currently runs as:
- **Single Go binary** serving HTTP API
- **SQLite or PostgreSQL** database
- **Local filesystem** for media storage
- **No authentication** (single-user assumption)
- **No distributed caching**

**Typical Use Case**: Personal media server (1-5 users, <100k media files)

---

## Horizontal Scaling

### Multi-Server Deployment

To scale beyond a single server:

**Prerequisites**:
- Stateless API servers (no in-memory session storage)
- Shared database (PostgreSQL required)
- Shared file storage (NFS, S3, or similar)
- Load balancer (nginx, HAProxy, or cloud LB)

**Architecture**:
```
                    ┌─────────────┐
                    │ Load        │
         ┌─────────►│ Balancer    │◄──────────┐
         │          └─────────────┘           │
         │                                    │
    ┌────┴────┐                         ┌────┴────┐
    │ ViewRA  │                         │ ViewRA  │
    │ Server 1│                         │ Server N│
    └────┬────┘                         └────┬────┘
         │                                    │
         │         ┌─────────────┐            │
         └────────►│ PostgreSQL  │◄───────────┘
                   │ (Shared DB) │
                   └─────────────┘
                          │
                   ┌──────┴──────┐
                   │ Shared File │
                   │ Storage     │
                   │ (NFS/S3)    │
                   └─────────────┘
```

**Configuration Changes**:
1. Switch from SQLite to PostgreSQL
2. Configure database connection pooling for concurrent access
3. Mount shared filesystem at consistent path on all servers
4. Update CORS settings for load balancer origin

---

## Performance Optimization

### Database Optimization

#### Indexes
```sql
-- High-traffic query indexes
CREATE INDEX idx_media_library_type ON media(library_id, type);
CREATE INDEX idx_media_created_at ON media(created_at DESC);
CREATE INDEX idx_progress_user_media ON watch_progress(user_id, media_id);

-- Full-text search (PostgreSQL)
CREATE INDEX idx_media_title_fts ON media USING GIN(to_tsvector('english', title));
```

#### Connection Pooling
```go
// config/database.go
type DatabaseConfig struct {
    MaxOpenConns    int           // Default: 25
    MaxIdleConns    int           // Default: 5
    ConnMaxLifetime time.Duration // Default: 5m
    ConnMaxIdleTime time.Duration // Default: 1m
}

// For high concurrency (100+ simultaneous users):
cfg := DatabaseConfig{
    MaxOpenConns:    100,
    MaxIdleConns:    25,
    ConnMaxLifetime: 15 * time.Minute,
    ConnMaxIdleTime: 5 * time.Minute,
}
```

#### Read Replicas
For read-heavy workloads (streaming, browsing):
- **Primary**: Handle writes (scan, progress updates, library management)
- **Replicas**: Handle reads (media lists, search, streaming)

```go
type DatabasePool struct {
    primary  *sql.DB  // All writes
    replicas []*sql.DB  // Round-robin reads
}

func (p *DatabasePool) Query(ctx context.Context, query string) (*sql.Rows, error) {
    // Route to read replica
    replica := p.selectReplica()
    return replica.QueryContext(ctx, query)
}

func (p *DatabasePool) Exec(ctx context.Context, query string) (sql.Result, error) {
    // Route to primary
    return p.primary.ExecContext(ctx, query)
}
```

### Caching Layer

#### Redis Integration
For distributed caching across multiple servers:

**Use Cases**:
- Library metadata (rarely changes)
- User watch progress (frequently accessed)
- Search results (expensive queries)
- Thumbnail URLs

**Implementation**:
```go
type CacheRepository struct {
    redis  *redis.Client
    repo   MediaRepository  // Fallback to DB
    ttl    time.Duration
}

func (c *CacheRepository) GetByID(ctx context.Context, id int64) (*Media, error) {
    // Try cache first
    key := fmt.Sprintf("media:%d", id)
    cached, err := c.redis.Get(ctx, key).Result()
    if err == nil {
        var media Media
        json.Unmarshal([]byte(cached), &media)
        return &media, nil
    }

    // Cache miss - fetch from DB
    media, err := c.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }

    // Store in cache
    json, _ := json.Marshal(media)
    c.redis.Set(ctx, key, json, c.ttl)

    return media, nil
}
```

**Cache Invalidation**:
```go
// Invalidate on updates
func (c *CacheRepository) Update(ctx context.Context, media *Media) error {
    // Update DB
    if err := c.repo.Update(ctx, media); err != nil {
        return err
    }

    // Invalidate cache
    key := fmt.Sprintf("media:%d", media.ID)
    c.redis.Del(ctx, key)

    return nil
}
```

### CDN for Static Assets

Offload static content (thumbnails, posters, subtitles) to CDN:

**Architecture**:
```
User Request → CDN → (cache miss) → ViewRA → Object Storage
                ↑                              (S3/MinIO)
                └── (cache hit) ──────────────────┘
```

**Implementation**:
1. Generate signed URLs for private content
2. Set appropriate Cache-Control headers
3. Use CDN origin headers for authentication

---

## Concurrency Patterns

### Worker Pool for File Scanning

For high-concurrency scanning operations:

```go
type Scanner struct {
    numWorkers int
    semaphore  chan struct{}
}

func NewScanner(workers int) *Scanner {
    if workers == 0 {
        workers = runtime.NumCPU()
    }
    return &Scanner{
        numWorkers: workers,
        semaphore:  make(chan struct{}, workers),
    }
}

func (s *Scanner) ScanDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
    files := make(chan string, 100)
    results := make(chan *FileInfo, 100)
    var wg sync.WaitGroup

    // Start workers
    for i := 0; i < s.numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for filePath := range files {
                select {
                case <-ctx.Done():
                    return
                default:
                    info, err := s.processFile(filePath)
                    if err != nil {
                        continue
                    }
                    results <- info
                }
            }
        }()
    }

    // Collect results
    go func() {
        wg.Wait()
        close(results)
    }()

    // Walk directory and send files to workers
    go func() {
        defer close(files)
        filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
            if err != nil || d.IsDir() {
                return err
            }
            files <- p
            return nil
        })
    }()

    // Gather results
    var allResults []*FileInfo
    for result := range results {
        allResults = append(allResults, result)
    }

    return allResults, nil
}
```

**Tuning**:
- **CPU-bound operations** (hashing, FFmpeg): `runtime.NumCPU()`
- **I/O-bound operations** (network, slow disks): `2 * runtime.NumCPU()`
- **Environment variable override**: `SCAN_WORKERS=16`

---

## Multi-User Support

### Authentication

**JWT-based authentication**:

```go
type AuthMiddleware struct {
    jwtSecret []byte
}

func (m *AuthMiddleware) Authenticate(c *gin.Context) {
    token := c.GetHeader("Authorization")
    if token == "" {
        c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
        return
    }

    claims, err := m.verifyToken(token)
    if err != nil {
        c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
        return
    }

    c.Set("user_id", claims.UserID)
    c.Next()
}
```

**Password hashing**:
```go
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(hash), err
}

func VerifyPassword(hash, password string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

### Authorization (RBAC)

**Roles**:
- **Admin**: Full access (manage libraries, users, settings)
- **User**: Access assigned libraries, track progress
- **Guest**: Read-only access

**Implementation**:
```go
type Permission string

const (
    PermissionReadLibrary   Permission = "library:read"
    PermissionWriteLibrary  Permission = "library:write"
    PermissionManageUsers   Permission = "users:manage"
)

type Role struct {
    Name        string
    Permissions []Permission
}

var Roles = map[string]Role{
    "admin": {
        Name: "admin",
        Permissions: []Permission{
            PermissionReadLibrary,
            PermissionWriteLibrary,
            PermissionManageUsers,
        },
    },
    "user": {
        Name: "user",
        Permissions: []Permission{
            PermissionReadLibrary,
        },
    },
}

func (m *AuthMiddleware) RequirePermission(perm Permission) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetInt64("user_id")
        if !m.hasPermission(userID, perm) {
            c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
            return
        }
        c.Next()
    }
}
```

### Rate Limiting

Prevent abuse with per-user rate limits:

```go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters map[int64]*rate.Limiter  // userID -> limiter
    mu       sync.Mutex
    rate     rate.Limit  // requests per second
    burst    int         // burst size
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[int64]*rate.Limiter),
        rate:     rate.Limit(rps),
        burst:    burst,
    }
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetInt64("user_id")

        rl.mu.Lock()
        limiter, exists := rl.limiters[userID]
        if !exists {
            limiter = rate.NewLimiter(rl.rate, rl.burst)
            rl.limiters[userID] = limiter
        }
        rl.mu.Unlock()

        if !limiter.Allow() {
            c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
            return
        }

        c.Next()
    }
}
```

**Recommended Limits**:
- API endpoints: 100 requests/minute per user
- Search: 20 requests/minute per user
- File uploads: 10 requests/minute per user

---

## Message Queue for Distributed Jobs

For long-running operations (transcoding, scanning large libraries):

### Use Cases
- **Transcoding jobs**: DASH/HLS conversion
- **Thumbnail generation**: Extract frames from videos
- **Metadata refresh**: Periodic re-scan of libraries

### Architecture with RabbitMQ/NATS

```
API Server → Message Queue → Worker Pool → Database
                  ↓
            Job Status Updates
```

**Implementation**:
```go
type JobQueue interface {
    Publish(ctx context.Context, job Job) error
    Subscribe(ctx context.Context, handler func(Job) error) error
}

type TranscodeJob struct {
    MediaID     int64
    Quality     string
    OutputPath  string
}

// Publisher (API server)
func (h *MediaHandler) StartTranscode(c *gin.Context) {
    job := TranscodeJob{
        MediaID: req.MediaID,
        Quality: req.Quality,
    }

    if err := h.queue.Publish(c.Request.Context(), job); err != nil {
        c.JSON(500, gin.H{"error": "failed to queue job"})
        return
    }

    c.JSON(202, gin.H{"status": "queued"})
}

// Worker
func (w *Worker) Start(ctx context.Context) {
    w.queue.Subscribe(ctx, func(job Job) error {
        switch j := job.(type) {
        case TranscodeJob:
            return w.handleTranscode(ctx, j)
        }
        return nil
    })
}
```

---

## Microservices (Future)

**When to consider**: 100k+ media files, 1000+ concurrent users

**Potential Services**:
- **API Gateway**: Authentication, routing
- **Library Service**: Library management, scanning
- **Media Service**: Media metadata, search
- **Transcode Service**: Video processing
- **Progress Service**: Watch tracking
- **Streaming Service**: HTTP range requests

**Trade-offs**:
- ✅ Independent scaling (scale transcode workers separately)
- ✅ Technology diversity (use Rust for transcode, Go for API)
- ❌ Operational complexity (deploy 6+ services)
- ❌ Network latency (inter-service communication)
- ❌ Distributed transactions (eventual consistency)

**Recommendation**: Only split services when a single server cannot handle the load.

---

## Monitoring & Observability

### Metrics
- **Request rate**: Requests per second per endpoint
- **Response time**: p50, p95, p99 latencies
- **Error rate**: 4xx/5xx responses
- **Database**: Query latency, connection pool usage
- **Scanning**: Files processed per second, scan duration

### Logging
```go
logger.Info("request completed",
    "method", req.Method,
    "path", req.Path,
    "status", resp.Status,
    "duration_ms", elapsed.Milliseconds(),
    "user_id", userID,
)
```

### Health Checks
```go
func (s *Server) HealthCheck(c *gin.Context) {
    health := gin.H{
        "status": "healthy",
        "checks": gin.H{
            "database": s.checkDatabase(),
            "storage":  s.checkStorage(),
        },
    }
    c.JSON(200, health)
}
```

---

## Deployment Strategies

### Docker Compose (Small Scale)
```yaml
version: '3.8'
services:
  viewra:
    image: viewra:latest
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgresql://viewra@postgres/viewra
      STORAGE_PATH: /media
    volumes:
      - /path/to/media:/media:ro
    depends_on:
      - postgres

  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: viewra
      POSTGRES_DB: viewra
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

### Kubernetes (Large Scale)
- **HorizontalPodAutoscaler**: Auto-scale API pods based on CPU/memory
- **StatefulSet**: PostgreSQL with persistent volumes
- **PersistentVolumeClaim**: Shared media storage (NFS/Ceph)
- **Ingress**: Load balancing and TLS termination

---

## Summary

**Current (Single-User)**: One server, SQLite, local files → Perfect for personal use

**Medium Scale (10-50 users)**: One server, PostgreSQL, optional Redis → Add caching, connection pooling

**Large Scale (100+ users)**: Multiple servers, PostgreSQL (with replicas), Redis, CDN → Horizontal scaling, load balancing

**Enterprise (1000+ users)**: Microservices, message queues, Kubernetes → Only if truly needed

**Recommendation**: Start simple. Scale only when measurements show you need to.

---

**See Also**:
- [ARCHITECTURE.md](./ARCHITECTURE.md) - Core application architecture
- [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md) - Database design
- [API_SPECIFICATION.md](./API_SPECIFICATION.md) - API endpoints

**Last Updated**: 2025-11-12
