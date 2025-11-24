# ViewRA - Common Code Patterns

> Quick reference for LLMs generating ViewRA code. Copy-paste these patterns when creating new features.

## Go Patterns

### 1. Domain Entity with Validation

```go
// internal/domain/media/entity.go
package media

import (
    "errors"
    "fmt"
    "time"
)

type Media struct {
    ID          int64
    LibraryID   int64
    Title       string
    FilePath    string
    FileSize    int64
    Duration    int
    Type        MediaType
    VideoCodec  string
    AudioCodec  string
    Width       int
    Height      int
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// Validate checks if the entity is valid
func (m *Media) Validate() error {
    if m.Title == "" {
        return ErrInvalidTitle
    }
    if m.FilePath == "" {
        return ErrInvalidFilePath
    }
    if m.LibraryID == 0 {
        return ErrInvalidLibraryID
    }
    return nil
}

// CalculateAspectRatio returns the aspect ratio string
func (m *Media) CalculateAspectRatio() string {
    if m.Width == 0 || m.Height == 0 {
        return ""
    }
    gcd := gcd(m.Width, m.Height)
    return fmt.Sprintf("%d:%d", m.Width/gcd, m.Height/gcd)
}

// Helper for aspect ratio
func gcd(a, b int) int {
    for b != 0 {
        a, b = b, a%b
    }
    return a
}
```

### 2. Domain Errors

```go
// internal/domain/media/errors.go
package media

import "errors"

var (
    ErrMediaNotFound     = errors.New("media not found")
    ErrInvalidTitle      = errors.New("invalid title")
    ErrInvalidFilePath   = errors.New("invalid file path")
    ErrInvalidLibraryID  = errors.New("invalid library ID")
    ErrDuplicateMedia    = errors.New("media already exists")
    ErrUnsupportedFormat = errors.New("unsupported media format")
)
```

### 3. Domain Repository Interface

```go
// internal/domain/media/repository.go
package media

import "context"

type Repository interface {
    // Create creates a new media entry
    Create(ctx context.Context, media *Media) error

    // GetByID retrieves a media entry by ID
    GetByID(ctx context.Context, id int64) (*Media, error)

    // GetByLibraryID retrieves all media for a library
    GetByLibraryID(ctx context.Context, libraryID int64, limit, offset int) ([]*Media, error)

    // Update updates an existing media entry
    Update(ctx context.Context, media *Media) error

    // Delete deletes a media entry
    Delete(ctx context.Context, id int64) error

    // CountByLibraryID returns the count of media in a library
    CountByLibraryID(ctx context.Context, libraryID int64) (int, error)
}
```

### 4. Repository Implementation

```go
// internal/infrastructure/persistence/media/repository.go
package media

import (
    "context"
    "database/sql"
    "fmt"

    "viewra/internal/domain/media"
    "viewra/internal/infrastructure/database"
    "viewra/internal/infrastructure/database/common"
)

type repository struct {
    queries *database.Queries
}

func NewRepository(db database.DBTX) media.Repository {
    return &repository{
        queries: database.New(db),
    }
}

func (r *repository) Create(ctx context.Context, m *media.Media) error {
    // Map ALL fields - NO empty placeholders
    result, err := r.queries.CreateMedia(ctx, database.CreateMediaParams{
        LibraryID:  m.LibraryID,
        Title:      m.Title,
        FilePath:   m.FilePath,
        FileSize:   m.FileSize,
        Duration:   sql.NullInt64{Int64: int64(m.Duration), Valid: m.Duration > 0},
        Type:       string(m.Type),
        VideoCodec: common.NullString(m.VideoCodec),
        AudioCodec: common.NullString(m.AudioCodec),
        Width:      sql.NullInt64{Int64: int64(m.Width), Valid: m.Width > 0},
        Height:     sql.NullInt64{Int64: int64(m.Height), Valid: m.Height > 0},
    })
    if err != nil {
        return fmt.Errorf("failed to create media: %w", err)
    }

    // Update entity with generated values
    m.ID = result.ID
    m.CreatedAt = result.CreatedAt
    m.UpdatedAt = result.UpdatedAt
    return nil
}

func (r *repository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
    row, err := r.queries.GetMediaByID(ctx, id)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, media.ErrMediaNotFound
        }
        return nil, fmt.Errorf("failed to get media: %w", err)
    }

    return mapRowToMedia(row), nil
}

// mapRowToMedia converts database row to domain entity
func mapRowToMedia(row database.Media) *media.Media {
    return &media.Media{
        ID:         row.ID,
        LibraryID:  row.LibraryID,
        Title:      row.Title,
        FilePath:   row.FilePath,
        FileSize:   row.FileSize,
        Duration:   int(row.Duration.Int64),
        Type:       media.MediaType(row.Type),
        VideoCodec: row.VideoCodec.String,
        AudioCodec: row.AudioCodec.String,
        Width:      int(row.Width.Int64),
        Height:     int(row.Height.Int64),
        CreatedAt:  row.CreatedAt,
        UpdatedAt:  row.UpdatedAt,
    }
}
```

### 5. SQL Queries (SQLite)

```sql
-- internal/infrastructure/persistence/media/queries/media.sql

-- name: CreateMedia :one
INSERT INTO media (
    library_id,
    title,
    file_path,
    file_size,
    duration,
    type,
    video_codec,
    audio_codec,
    width,
    height
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMediaByID :one
SELECT * FROM media
WHERE id = ?
LIMIT 1;

-- name: GetMediaByLibraryID :many
SELECT * FROM media
WHERE library_id = ?
ORDER BY title ASC
LIMIT ? OFFSET ?;

-- name: UpdateMedia :one
UPDATE media
SET title = ?,
    video_codec = ?,
    audio_codec = ?,
    width = ?,
    height = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteMedia :exec
DELETE FROM media WHERE id = ?;

-- name: CountMediaByLibraryID :one
SELECT COUNT(*) FROM media WHERE library_id = ?;
```

### 6. SQL Queries (PostgreSQL)

```sql
-- internal/infrastructure/persistence/media/queries/media_postgres.sql

-- name: CreateMedia :one
INSERT INTO media (
    library_id,
    title,
    file_path,
    file_size,
    duration,
    type,
    video_codec,
    audio_codec,
    width,
    height
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetMediaByID :one
SELECT * FROM media
WHERE id = $1
LIMIT 1;

-- name: GetMediaByLibraryID :many
SELECT * FROM media
WHERE library_id = $1
ORDER BY title ASC
LIMIT $2 OFFSET $3;

-- name: UpdateMedia :one
UPDATE media
SET title = $2,
    video_codec = $3,
    audio_codec = $4,
    width = $5,
    height = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteMedia :exec
DELETE FROM media WHERE id = $1;

-- name: CountMediaByLibraryID :one
SELECT COUNT(*) FROM media WHERE library_id = $1;
```

### 7. Use Case Pattern

```go
// internal/application/media/create_media.go
package media

import (
    "context"
    "fmt"

    "viewra/internal/domain/media"
)

type CreateMediaRequest struct {
    LibraryID  int64
    Title      string
    FilePath   string
    FileSize   int64
    Duration   int
    VideoCodec string
    AudioCodec string
    Width      int
    Height     int
}

type CreateMediaResponse struct {
    ID        int64
    Title     string
    FilePath  string
    CreatedAt string
}

type CreateMediaUseCase struct {
    repo media.Repository
}

func NewCreateMediaUseCase(repo media.Repository) *CreateMediaUseCase {
    return &CreateMediaUseCase{repo: repo}
}

func (uc *CreateMediaUseCase) Execute(ctx context.Context, req CreateMediaRequest) (*CreateMediaResponse, error) {
    // Create domain entity
    m := &media.Media{
        LibraryID:  req.LibraryID,
        Title:      req.Title,
        FilePath:   req.FilePath,
        FileSize:   req.FileSize,
        Duration:   req.Duration,
        VideoCodec: req.VideoCodec,
        AudioCodec: req.AudioCodec,
        Width:      req.Width,
        Height:     req.Height,
        Type:       media.MediaTypeVideo, // Determine based on extension
    }

    // Validate
    if err := m.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // Save to repository
    if err := uc.repo.Create(ctx, m); err != nil {
        return nil, fmt.Errorf("failed to create media: %w", err)
    }

    // Return response
    return &CreateMediaResponse{
        ID:        m.ID,
        Title:     m.Title,
        FilePath:  m.FilePath,
        CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z"),
    }, nil
}
```

### 8. HTTP Handler Pattern

```go
// internal/interfaces/http/media/handler.go
package media

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"

    "viewra/internal/application/media"
    domainMedia "viewra/internal/domain/media"
)

type Handler struct {
    createMedia *media.CreateMediaUseCase
    getMedia    *media.GetMediaUseCase
}

func NewHandler(
    createMedia *media.CreateMediaUseCase,
    getMedia *media.GetMediaUseCase,
) *Handler {
    return &Handler{
        createMedia: createMedia,
        getMedia:    getMedia,
    }
}

// CreateMedia godoc
// @Summary Create new media
// @Tags media
// @Accept json
// @Produce json
// @Param request body media.CreateMediaRequest true "Create media request"
// @Success 201 {object} media.CreateMediaResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media [post]
func (h *Handler) CreateMedia(c *gin.Context) {
    var req media.CreateMediaRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
        return
    }

    resp, err := h.createMedia.Execute(c.Request.Context(), req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
        return
    }

    c.JSON(http.StatusCreated, resp)
}

// GetMedia godoc
// @Summary Get media by ID
// @Tags media
// @Produce json
// @Param id path int true "Media ID"
// @Success 200 {object} media.GetMediaResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media/{id} [get]
func (h *Handler) GetMedia(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid media ID"})
        return
    }

    resp, err := h.getMedia.Execute(c.Request.Context(), id)
    if err != nil {
        if err == domainMedia.ErrMediaNotFound {
            c.JSON(http.StatusNotFound, ErrorResponse{Error: "media not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
        return
    }

    c.JSON(http.StatusOK, resp)
}

type ErrorResponse struct {
    Error string `json:"error"`
}
```

### 9. Integration Test Pattern

```go
// internal/infrastructure/persistence/media/repository_test.go
package media_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "viewra/internal/domain/media"
    mediaPersistence "viewra/internal/infrastructure/persistence/media"
    "viewra/internal/testutil"
)

func TestRepository_Create(t *testing.T) {
    db := testutil.SetupTestDB(t)
    repo := mediaPersistence.NewRepository(db)

    m := &media.Media{
        LibraryID:  1,
        Title:      "Test Movie",
        FilePath:   "/path/to/movie.mkv",
        FileSize:   1024000,
        Duration:   7200,
        Type:       media.MediaTypeVideo,
        VideoCodec: "h264",
        AudioCodec: "aac",
        Width:      1920,
        Height:     1080,
    }

    err := repo.Create(context.Background(), m)
    require.NoError(t, err)
    assert.NotZero(t, m.ID)
    assert.NotZero(t, m.CreatedAt)
    assert.NotZero(t, m.UpdatedAt)
}

func TestRepository_GetByID(t *testing.T) {
    db := testutil.SetupTestDB(t)
    repo := mediaPersistence.NewRepository(db)

    // Create test media
    original := &media.Media{
        LibraryID: 1,
        Title:     "Test Movie",
        FilePath:  "/path/to/movie.mkv",
    }
    err := repo.Create(context.Background(), original)
    require.NoError(t, err)

    // Retrieve it
    retrieved, err := repo.GetByID(context.Background(), original.ID)
    require.NoError(t, err)
    assert.Equal(t, original.ID, retrieved.ID)
    assert.Equal(t, original.Title, retrieved.Title)
    assert.Equal(t, original.FilePath, retrieved.FilePath)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
    db := testutil.SetupTestDB(t)
    repo := mediaPersistence.NewRepository(db)

    _, err := repo.GetByID(context.Background(), 99999)
    assert.ErrorIs(t, err, media.ErrMediaNotFound)
}
```

### 10. Database Migration Pattern

```sql
-- migrations/000005_add_media_table.up.sql

CREATE TABLE IF NOT EXISTS media (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    file_path TEXT NOT NULL UNIQUE,
    file_size INTEGER NOT NULL,
    duration INTEGER,
    type TEXT NOT NULL,
    video_codec TEXT,
    audio_codec TEXT,
    width INTEGER,
    height INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_library_id ON media(library_id);
CREATE INDEX idx_media_file_path ON media(file_path);
CREATE INDEX idx_media_type ON media(type);

-- migrations/000005_add_media_table.down.sql

DROP INDEX IF EXISTS idx_media_type;
DROP INDEX IF EXISTS idx_media_file_path;
DROP INDEX IF EXISTS idx_media_library_id;
DROP TABLE IF EXISTS media;
```

## TypeScript/React Patterns

### 11. Component with Types

```typescript
// MediaCard.types.ts
interface MediaCardProps {
  media: Media
  onSelect?: (id: number) => void
  className?: string
}

interface Media {
  id: number
  title: string
  filePath: string
  duration: number
  thumbnailUrl?: string
}

export type { MediaCardProps, Media }
```

```typescript
// MediaCard.tsx
import { Card } from '@/components/ui/card'
import type { MediaCardProps } from './MediaCard.types'

const MediaCard = ({ media, onSelect, className }: MediaCardProps) => {
  const handleClick = () => {
    onSelect?.(media.id)
  }

  const formatDuration = (seconds: number) => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return `${hours}h ${minutes}m`
  }

  return (
    <Card className={className} onClick={handleClick}>
      <img src={media.thumbnailUrl} alt={media.title} />
      <h3>{media.title}</h3>
      <p>{formatDuration(media.duration)}</p>
    </Card>
  )
}

export { MediaCard }
export type { MediaCardProps } from './MediaCard.types'
```

### 12. TanStack Query Hook

```typescript
// useMedia.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'

const useMediaList = (libraryId: number, page = 1, limit = 20) => {
  return useQuery({
    queryKey: ['media', libraryId, page, limit],
    queryFn: () => api.getMediaByLibrary(libraryId, { page, limit }),
  })
}

const useMedia = (id: number) => {
  return useQuery({
    queryKey: ['media', id],
    queryFn: () => api.getMedia(id),
    enabled: id > 0,
  })
}

const useCreateMedia = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: api.createMedia,
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['media'] })
    },
  })
}

export { useMediaList, useMedia, useCreateMedia }
```

### 13. TanStack Router Route

```typescript
// routes/_layout/media/$mediaId.tsx
import { createFileRoute } from '@tanstack/react-router'
import { MediaDetail } from '@/components/media/MediaDetail'
import { useMedia } from '@/lib/hooks/useMedia'

const MediaDetailRoute = () => {
  const { mediaId } = Route.useParams()
  const { data: media, isLoading, error } = useMedia(Number(mediaId))

  if (isLoading) {
    return <div>Loading...</div>
  }

  if (error || !media) {
    return <div>Media not found</div>
  }

  return <MediaDetail media={media} />
}

export const Route = createFileRoute('/_layout/media/$mediaId')({
  component: MediaDetailRoute,
})
```

### 14. API Client Pattern

```typescript
// lib/api/media.ts
import { apiClient } from './client'

interface CreateMediaRequest {
  libraryId: number
  title: string
  filePath: string
}

interface Media {
  id: number
  title: string
  filePath: string
  duration: number
}

const getMedia = async (id: number): Promise<Media> => {
  const response = await apiClient.get(`/api/media/${id}`)
  return response.data
}

const getMediaByLibrary = async (
  libraryId: number,
  params: { page: number; limit: number }
): Promise<Media[]> => {
  const response = await apiClient.get(`/api/media`, {
    params: { library_id: libraryId, ...params },
  })
  return response.data
}

const createMedia = async (data: CreateMediaRequest): Promise<Media> => {
  const response = await apiClient.post('/api/media', data)
  return response.data
}

export { getMedia, getMediaByLibrary, createMedia }
export type { Media, CreateMediaRequest }
```

## Common Utilities

### 15. Null String Helper

```go
// internal/infrastructure/database/common/helpers.go
package common

import "database/sql"

// NullString creates a sql.NullString from a string
func NullString(s string) sql.NullString {
    if s == "" {
        return sql.NullString{Valid: false}
    }
    return sql.NullString{String: s, Valid: true}
}

// NullInt64 creates a sql.NullInt64 from an int64
func NullInt64(i int64) sql.NullInt64 {
    if i == 0 {
        return sql.NullInt64{Valid: false}
    }
    return sql.NullInt64{Int64: i, Valid: true}
}
```

### 16. File Utilities

```go
// internal/pkg/fileutil/hash.go
package fileutil

import (
    "crypto/sha256"
    "fmt"
    "io"
    "os"
)

// HashFile returns the SHA256 hash of a file
func HashFile(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", fmt.Errorf("failed to open file: %w", err)
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", fmt.Errorf("failed to hash file: %w", err)
    }

    return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// FileExists checks if a file exists
func FileExists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}

// FileSize returns the size of a file in bytes
func FileSize(path string) (int64, error) {
    info, err := os.Stat(path)
    if err != nil {
        return 0, fmt.Errorf("failed to stat file: %w", err)
    }
    return info.Size(), nil
}
```

## Remember

- **Domain**: Pure Go, no external imports
- **Repository**: Map ALL fields, no placeholders
- **SQL**: Write for both SQLite and PostgreSQL
- **TypeScript**: Arrow functions, exports at end
- **Tests**: Integration tests with real DB
- **Errors**: Always wrap with context

---

**Last Updated**: 2025-11-24
