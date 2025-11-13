# ViewRA API Specification

## Overview

The ViewRA API is a RESTful HTTP API that provides endpoints for managing media libraries, streaming content, and tracking watch progress. All endpoints return JSON responses and follow standard HTTP conventions.

**Base URL**: `http://localhost:3000/api`

**API Version**: v1

**Documentation**: Swagger UI available at `/swagger/index.html`

## Authentication

**Current**: No authentication (single-user mode)

**Future**: JWT-based authentication with Bearer tokens

```http
Authorization: Bearer <token>
```

## Response Format

### Success Response

```json
{
  "id": 1,
  "name": "My Movies",
  "path": "/media/movies",
  "type": "movies",
  "created_at": "2024-11-11T10:30:00Z",
  "updated_at": "2024-11-11T10:30:00Z"
}
```

### Error Response

**Single Error**:

```json
{
  "error": {
    "code": "MEDIA_NOT_FOUND",
    "message": "Media with ID 123 not found"
  }
}
```

**Validation Errors (Multiple Fields)**:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "fields": {
      "name": "name is required",
      "path": "path must be absolute",
      "type": "type must be one of: movies, tv, music"
    }
  }
}
```

### Paginated Response

```json
{
  "data": [...],
  "pagination": {
    "total": 150,
    "page": 1,
    "page_size": 50,
    "total_pages": 3
  }
}
```

**Default page size**: 50 items  
**Query parameters**: `?page=1&page_size=50` (max: 200)

## HTTP Status Codes

- `200 OK` - Request successful
- `201 Created` - Resource created
- `204 No Content` - Success with no response body
- `400 Bad Request` - Invalid request data
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict (duplicate)
- `422 Unprocessable Entity` - Validation error
- `500 Internal Server Error` - Server error

## Common Error Codes

| Code | Description |
|------|-------------|
| `VALIDATION_ERROR` | Input validation failed |
| `LIBRARY_NOT_FOUND` | Library does not exist |
| `MEDIA_NOT_FOUND` | Media does not exist |
| `DUPLICATE_PATH` | Library path already exists |
| `INVALID_PATH` | File path is invalid or inaccessible |
| `SCAN_IN_PROGRESS` | Library scan already running |
| `TRANSCODE_FAILED` | Transcoding job failed |
| `FILE_NOT_FOUND` | Media file not found on disk |
| `ACCESS_DENIED` | Access to path is denied (outside allowed directories) |
| `PERMISSION_DENIED` | Insufficient filesystem permissions |

---

## Endpoints

### Health Check

#### GET /api/health

Health check endpoint.

**Response**: `200 OK`

```json
{
  "status": "ok",
  "message": "Media server is running",
  "version": "1.0.0",
  "uptime": 3600
}
```

---

## Libraries

### List Libraries

#### GET /api/libraries

Get all libraries.

**Query Parameters**:
- `type` (optional) - Filter by type: `movies`, `tv`, `music`
- `page` (optional) - Page number, default: 1
- `page_size` (optional) - Items per page, default: 50, max: 200

**Response**: `200 OK`

```json
[
  {
    "id": 1,
    "name": "My Movies",
    "path": "/media/movies",
    "type": "movies",
    "media_count": 150,
    "created_at": "2024-11-11T10:30:00Z",
    "updated_at": "2024-11-11T10:30:00Z"
  },
  {
    "id": 2,
    "name": "TV Shows",
    "path": "/media/tv",
    "type": "tv",
    "media_count": 45,
    "created_at": "2024-11-11T11:00:00Z",
    "updated_at": "2024-11-11T11:00:00Z"
  }
]
```

---

### Get Library

#### GET /api/libraries/:id

Get a specific library by ID.

**Path Parameters**:
- `id` (required) - Library ID

**Response**: `200 OK`

```json
{
  "id": 1,
  "name": "My Movies",
  "path": "/media/movies",
  "type": "movies",
  "media_count": 150,
  "created_at": "2024-11-11T10:30:00Z",
  "updated_at": "2024-11-11T10:30:00Z"
}
```

**Errors**:
- `404` - Library not found

---

### Create Library

#### POST /api/libraries

Create a new library.

**Request Body**:

```json
{
  "name": "My Movies",
  "path": "/media/movies",
  "type": "movies"
}
```

**Validation**:
- `name` - Required, non-empty string
- `path` - Required, absolute path, must exist
- `type` - Required, one of: `movies`, `tv`, `music`

**Response**: `201 Created`

```json
{
  "id": 1,
  "name": "My Movies",
  "path": "/media/movies",
  "type": "movies",
  "media_count": 0,
  "created_at": "2024-11-11T10:30:00Z",
  "updated_at": "2024-11-11T10:30:00Z"
}
```

**Errors**:
- `400` - Invalid request body
- `409` - Library path already exists
- `422` - Validation error (path doesn't exist, invalid type)

---

### Update Library

#### PUT /api/libraries/:id

Update an existing library.

**Path Parameters**:
- `id` (required) - Library ID

**Request Body**:

```json
{
  "name": "Updated Name",
  "path": "/new/path",
  "type": "movies"
}
```

**Response**: `200 OK`

```json
{
  "id": 1,
  "name": "Updated Name",
  "path": "/new/path",
  "type": "movies",
  "media_count": 150,
  "created_at": "2024-11-11T10:30:00Z",
  "updated_at": "2024-11-11T15:00:00Z"
}
```

**Errors**:
- `404` - Library not found
- `409` - New path conflicts with existing library

---

### Delete Library

#### DELETE /api/libraries/:id

Delete a library and all associated media.

**Path Parameters**:
- `id` (required) - Library ID

**Response**: `204 No Content`

**Errors**:
- `404` - Library not found

---

### Scan Library

#### POST /api/libraries/:id/scan

Trigger a library scan.

**Path Parameters**:
- `id` (required) - Library ID

**Query Parameters**:
- `force` (optional) - Force rescan of all files (default: `false`)

**Response**: `202 Accepted`

```json
{
  "scan_id": "uuid-1234-5678",
  "library_id": 1,
  "status": "started",
  "message": "Library scan started"
}
```

**Errors**:
- `404` - Library not found
- `409` - Scan already in progress

**SSE Progress**: Connect to `/api/libraries/:id/scan/progress` for real-time updates

---

### Get Scan Progress

#### GET /api/libraries/:id/scan/progress

Server-Sent Events (SSE) endpoint for real-time scan progress.

**Path Parameters**:
- `id` (required) - Library ID

**Response**: `text/event-stream`

```
event: progress
data: {"scan_id":"uuid-1234","status":"scanning","progress":25,"files_scanned":50,"total_files":200}

event: progress
data: {"scan_id":"uuid-1234","status":"scanning","progress":50,"files_scanned":100,"total_files":200}

event: complete
data: {"scan_id":"uuid-1234","status":"completed","files_added":45,"files_updated":5,"files_removed":2}

event: error
data: {"scan_id":"uuid-1234","status":"failed","error":"Permission denied"}
```

---

## Filesystem

### Browse Filesystem

#### GET /api/filesystem/browse

Browse server filesystem directories for library path selection.

**Security**: Only returns directories. System directories are restricted. Path traversal is prevented.

**Query Parameters**:
- `path` (optional) - Directory path to browse (default: server-configured base path or user home)

**Response**: `200 OK`

```json
{
  "current_path": "/media/movies",
  "parent": "/media",
  "is_root": false,
  "directories": [
    {
      "name": "action",
      "path": "/media/movies/action",
      "readable": true,
      "writable": false,
      "modified_at": "2024-11-11T10:30:00Z"
    },
    {
      "name": "comedy",
      "path": "/media/movies/comedy",
      "readable": true,
      "writable": false,
      "modified_at": "2024-11-10T15:20:00Z"
    }
  ]
}
```

**Response Fields**:
- `current_path` - Absolute path of current directory
- `parent` - Parent directory path (null if at root/base path)
- `is_root` - Whether at the root of allowed browsing area
- `directories` - Array of subdirectories (files are excluded)
  - `name` - Directory name
  - `path` - Absolute path
  - `readable` - Whether directory is readable
  - `writable` - Whether directory is writable (for library validation)
  - `modified_at` - Last modification time

**Errors**:
- `400` - Invalid path (path traversal attempt, malformed path)
- `403` - Access denied (outside allowed directories, system directory)
- `404` - Directory not found

**Error Examples**:

```json
{
  "error": {
    "code": "INVALID_PATH",
    "message": "Path contains invalid sequences (..)"
  }
}
```

```json
{
  "error": {
    "code": "ACCESS_DENIED",
    "message": "Path is outside allowed directories"
  }
}
```

```json
{
  "error": {
    "code": "PERMISSION_DENIED",
    "message": "Insufficient permissions to read directory"
  }
}
```

**Security Considerations**:
- Only directories within configured allowed base paths are accessible
- Path traversal attempts (`..`, symlinks) are blocked
- System directories (`/etc`, `/sys`, `/proc`, etc.) are blocked
- Hidden directories (starting with `.`) are excluded from results
- Permissions are checked before listing directory contents
- Empty path defaults to safe base directory (not root)

**Allowed Base Paths** (server configuration):
- Default: User home directory
- Common media paths: `/media`, `/mnt`, `/home/*/Videos`, `/home/*/Movies`
- Configurable via server config file or environment variables

**Use Case**:
This endpoint powers the folder browser dialog in the library creation/edit form, allowing users to visually navigate the filesystem instead of manually typing paths.

---

## Media

### List Media

#### GET /api/media

Get all media with filtering and pagination.

**Query Parameters**:
- `library_id` (optional) - Filter by library
- `type` (optional) - Filter by type: `movie`, `tv_episode`, `music_track`
- `search` (optional) - Search in title, artist, album
- `genre` (optional) - Filter by genre
- `year` (optional) - Filter by year
- `watched` (optional) - Filter by watch status: `true`, `false`
- `sort` (optional) - Sort field: `title`, `year`, `created_at`, `last_watched`
- `order` (optional) - Sort order: `asc`, `desc` (default: `asc`)
- `page` (optional) - Page number (default: `1`)
- `page_size` (optional) - Items per page (default: `20`, max: `100`)

**Response**: `200 OK`

```json
{
  "data": [
    {
      "id": 1,
      "library_id": 1,
      "title": "The Matrix",
      "file_path": "/media/movies/The.Matrix.1999.mp4",
      "file_size": 2147483648,
      "duration": 8160,
      "width": 1920,
      "height": 1080,
      "codec": "h264",
      "bit_rate": 5000000,
      "frame_rate": 24.0,
      "thumbnail_path": "/data/thumbnails/movies/1.jpg",
      "type": "movie",
      "has_dash": true,
      "dash_manifest_path": "/data/dash/1/manifest.mpd",
      "transcoding_status": "completed",
      "created_at": "2024-11-11T10:30:00Z",
      "updated_at": "2024-11-11T10:30:00Z",
      "movie": {
        "year": 1999,
        "genre": "Action, Sci-Fi",
        "director": "Wachowski Brothers",
        "rating": "R",
        "plot": "A computer hacker learns..."
      },
      "watch_progress": {
        "position": 3600,
        "watched": false,
        "last_watched": "2024-11-11T20:00:00Z"
      }
    }
  ],
  "pagination": {
    "total": 150,
    "page": 1,
    "page_size": 20,
    "total_pages": 8
  }
}
```

---

### Get Media

#### GET /api/media/:id

Get a specific media item by ID.

**Path Parameters**:
- `id` (required) - Media ID

**Response**: `200 OK`

```json
{
  "id": 1,
  "library_id": 1,
  "title": "The Matrix",
  "file_path": "/media/movies/The.Matrix.1999.mp4",
  "file_size": 2147483648,
  "duration": 8160,
  "width": 1920,
  "height": 1080,
  "codec": "h264",
  "bit_rate": 5000000,
  "frame_rate": 24.0,
  "thumbnail_path": "/data/thumbnails/movies/1.jpg",
  "type": "movie",
  "has_dash": true,
  "dash_manifest_path": "/data/dash/1/manifest.mpd",
  "transcoding_status": "completed",
  "created_at": "2024-11-11T10:30:00Z",
  "updated_at": "2024-11-11T10:30:00Z",
  "movie": {
    "year": 1999,
    "genre": "Action, Sci-Fi",
    "director": "Wachowski Brothers",
    "cast": "Keanu Reeves, Laurence Fishburne",
    "rating": "R",
    "plot": "A computer hacker learns from mysterious rebels about the true nature of his reality and his role in the war against its controllers.",
    "imdb_id": "tt0133093"
  },
  "watch_progress": {
    "position": 3600,
    "duration": 8160,
    "watched": false,
    "last_watched": "2024-11-11T20:00:00Z"
  }
}
```

**Errors**:
- `404` - Media not found

---

### Update Media

#### PUT /api/media/:id

Update media metadata.

**Path Parameters**:
- `id` (required) - Media ID

**Request Body**:

```json
{
  "title": "The Matrix (1999)",
  "movie": {
    "year": 1999,
    "genre": "Action, Sci-Fi",
    "director": "Wachowski Brothers",
    "plot": "Updated plot..."
  }
}
```

**Response**: `200 OK`

```json
{
  "id": 1,
  "title": "The Matrix (1999)",
  "movie": {
    "year": 1999,
    "genre": "Action, Sci-Fi",
    "director": "Wachowski Brothers",
    "plot": "Updated plot..."
  },
  "updated_at": "2024-11-11T16:00:00Z"
}
```

**Errors**:
- `404` - Media not found
- `422` - Validation error

---

### Delete Media

#### DELETE /api/media/:id

Delete a media item from the database.

**Path Parameters**:
- `id` (required) - Media ID

**Query Parameters**:
- `delete_file` (optional) - Also delete file from disk (default: `false`)

**Response**: `204 No Content`

**Errors**:
- `404` - Media not found

---

### Stream Media

#### GET /api/media/:id/stream

Stream media file with HTTP range request support.

**Path Parameters**:
- `id` (required) - Media ID

**Headers**:
- `Range` (optional) - Byte range request (e.g., `bytes=0-1023`)

**Response**: `206 Partial Content` or `200 OK`

**Headers**:
```
Content-Type: video/mp4
Content-Length: 2147483648
Accept-Ranges: bytes
Content-Range: bytes 0-1023/2147483648
```

**Body**: Binary media data

**Errors**:
- `404` - Media not found
- `416` - Range not satisfiable

---

### Get DASH Manifest

#### GET /api/media/:id/manifest.mpd

Get DASH manifest for adaptive streaming.

**Path Parameters**:
- `id` (required) - Media ID

**Response**: `200 OK`

**Headers**:
```
Content-Type: application/dash+xml
```

**Body**: DASH MPD manifest XML

**Errors**:
- `404` - Media or manifest not found
- `202` - Transcoding in progress (returns status)

---

### Get Thumbnail

#### GET /api/media/:id/thumbnail

Get media thumbnail image.

**Path Parameters**:
- `id` (required) - Media ID

**Query Parameters**:
- `size` (optional) - Thumbnail size: `small`, `medium`, `large` (default: `medium`)

**Response**: `200 OK`

**Headers**:
```
Content-Type: image/jpeg
Cache-Control: public, max-age=31536000
```

**Body**: JPEG image data

**Errors**:
- `404` - Media or thumbnail not found

---

### Request Transcode

#### POST /api/media/:id/transcode

Request DASH transcoding for a media file.

**Path Parameters**:
- `id` (required) - Media ID

**Request Body**:

```json
{
  "qualities": ["360p", "720p", "1080p"],
  "priority": "normal"
}
```

**Response**: `202 Accepted`

```json
{
  "media_id": 1,
  "jobs": [
    {
      "id": 1,
      "quality": "360p",
      "status": "queued"
    },
    {
      "id": 2,
      "quality": "720p",
      "status": "queued"
    }
  ],
  "message": "Transcode jobs queued"
}
```

**Errors**:
- `404` - Media not found
- `409` - Transcode already in progress

---

### Get Transcode Status

#### GET /api/media/:id/transcode/status

Get transcoding job status.

**Path Parameters**:
- `id` (required) - Media ID

**Response**: `200 OK`

```json
{
  "media_id": 1,
  "jobs": [
    {
      "id": 1,
      "quality": "360p",
      "status": "completed",
      "progress": 100,
      "completed_at": "2024-11-11T10:35:00Z"
    },
    {
      "id": 2,
      "quality": "720p",
      "status": "processing",
      "progress": 65,
      "started_at": "2024-11-11T10:35:00Z"
    }
  ]
}
```

**SSE Progress**: Connect to `/api/media/:id/transcode/progress` for real-time updates

---

## TV Shows

### List TV Shows

#### GET /api/tv/shows

Get all TV shows.

**Query Parameters**:
- `library_id` (optional) - Filter by library
- `search` (optional) - Search in title
- `genre` (optional) - Filter by genre
- `sort` (optional) - Sort field: `title`, `year`
- `order` (optional) - Sort order: `asc`, `desc`

**Response**: `200 OK`

```json
[
  {
    "id": 1,
    "library_id": 2,
    "title": "Breaking Bad",
    "year": 2008,
    "genre": "Crime, Drama",
    "plot": "A high school chemistry teacher...",
    "network": "AMC",
    "tvdb_id": 81189,
    "episode_count": 62,
    "season_count": 5,
    "created_at": "2024-11-11T10:30:00Z"
  }
]
```

---

### Get TV Show

#### GET /api/tv/shows/:id

Get a specific TV show with all episodes.

**Path Parameters**:
- `id` (required) - Show ID

**Response**: `200 OK`

```json
{
  "id": 1,
  "title": "Breaking Bad",
  "year": 2008,
  "genre": "Crime, Drama",
  "plot": "A high school chemistry teacher...",
  "network": "AMC",
  "tvdb_id": 81189,
  "seasons": [
    {
      "season": 1,
      "episode_count": 7,
      "episodes": [
        {
          "id": 101,
          "media_id": 501,
          "season": 1,
          "episode": 1,
          "episode_title": "Pilot",
          "air_date": "2008-01-20",
          "duration": 3480,
          "file_path": "/media/tv/Breaking.Bad.S01E01.mp4",
          "thumbnail_path": "/data/thumbnails/tv/501.jpg",
          "watch_progress": {
            "watched": true
          }
        }
      ]
    }
  ]
}
```

---

### Get Season Episodes

#### GET /api/tv/shows/:id/seasons/:season

Get all episodes for a specific season.

**Path Parameters**:
- `id` (required) - Show ID
- `season` (required) - Season number

**Response**: `200 OK`

```json
{
  "show_id": 1,
  "season": 1,
  "episodes": [
    {
      "id": 101,
      "media_id": 501,
      "season": 1,
      "episode": 1,
      "episode_title": "Pilot",
      "duration": 3480,
      "watch_progress": {
        "watched": true
      }
    }
  ]
}
```

---

## Music

### List Artists

#### GET /api/music/artists

Get all music artists.

**Query Parameters**:
- `library_id` (optional) - Filter by library
- `search` (optional) - Search in artist name
- `genre` (optional) - Filter by genre

**Response**: `200 OK`

```json
[
  {
    "artist": "Pink Floyd",
    "album_count": 15,
    "track_count": 147,
    "genre": "Progressive Rock"
  }
]
```

---

### List Albums

#### GET /api/music/albums

Get all music albums.

**Query Parameters**:
- `library_id` (optional) - Filter by library
- `artist` (optional) - Filter by artist
- `year` (optional) - Filter by year

**Response**: `200 OK`

```json
[
  {
    "album": "The Dark Side of the Moon",
    "artist": "Pink Floyd",
    "album_artist": "Pink Floyd",
    "year": 1973,
    "genre": "Progressive Rock",
    "track_count": 10
  }
]
```

---

### List Tracks

#### GET /api/music/tracks

Get all music tracks.

**Query Parameters**:
- `library_id` (optional) - Filter by library
- `artist` (optional) - Filter by artist
- `album` (optional) - Filter by album
- `genre` (optional) - Filter by genre

**Response**: `200 OK`

```json
[
  {
    "id": 1001,
    "media_id": 1001,
    "title": "Time",
    "artist": "Pink Floyd",
    "album": "The Dark Side of the Moon",
    "track_number": 4,
    "duration": 413,
    "file_path": "/media/music/Pink Floyd/1973 - The Dark Side of the Moon/04 - Time.flac",
    "watch_progress": null
  }
]
```

---

## Watch Progress

### Get Watch Progress

#### GET /api/progress

Get watch progress for all media.

**Query Parameters**:
- `limit` (optional) - Results per page (default: 50)
- `offset` (optional) - Results offset (default: 0)
- `media_type` (optional) - Filter: `movie`, `tv_episode`, `music_track`

**Response**: `200 OK`

```json
{
  "items": [
    {
      "id": 1,
      "media_id": 1,
      "media_title": "The Matrix",
      "media_type": "movie",
      "position": 3600,
      "duration": 8160,
      "watched": false,
      "progress_percent": 44,
      "last_watched": "2024-11-11T20:00:00Z"
    }
  ],
  "total": 100,
  "limit": 50,
  "offset": 0
}
```

---

### Get Watched Media

#### GET /api/progress/watched

Get all media marked as watched.

**Query Parameters**:
- `limit` (optional) - Results per page (default: 50)
- `offset` (optional) - Results offset (default: 0)
- `media_type` (optional) - Filter: `movie`, `tv_episode`, `music_track`

**Response**: `200 OK`

```json
{
  "items": [
    {
      "id": 1,
      "media_id": 1,
      "media_title": "The Matrix",
      "media_type": "movie",
      "position": 8160,
      "duration": 8160,
      "watched": true,
      "progress_percent": 100,
      "last_watched": "2024-11-11T20:00:00Z"
    }
  ],
  "total": 42,
  "limit": 50,
  "offset": 0
}
```

---

### Get In-Progress Media

#### GET /api/progress/in-progress

Get all media that is partially watched (not completed).

**Query Parameters**:
- `limit` (optional) - Results per page (default: 50)
- `offset` (optional) - Results offset (default: 0)
- `media_type` (optional) - Filter: `movie`, `tv_episode`, `music_track`

**Response**: `200 OK`

```json
{
  "items": [
    {
      "id": 1,
      "media_id": 1,
      "media_title": "The Matrix",
      "media_type": "movie",
      "position": 3600,
      "duration": 8160,
      "watched": false,
      "progress_percent": 44,
      "last_watched": "2024-11-11T20:00:00Z"
    }
  ],
  "total": 12,
  "limit": 50,
  "offset": 0
}
```

---

### Get Progress for Specific Media

#### GET /api/progress/:media_id

Get watch progress for a specific media item.

**Path Parameters**:
- `media_id` (required) - Media ID

**Response**: `200 OK`

```json
{
  "id": 1,
  "media_id": 1,
  "media_title": "The Matrix",
  "media_type": "movie",
  "position": 3600,
  "duration": 8160,
  "watched": false,
  "progress_percent": 44,
  "last_watched": "2024-11-11T20:00:00Z"
}
```

**Response**: `404 NOT FOUND` - No progress found for this media

---

### Delete Progress

#### DELETE /api/progress/:media_id

Delete watch progress for a media item.

**Path Parameters**:
- `media_id` (required) - Media ID

**Response**: `204 NO CONTENT`

---

### Update Watch Progress

#### PUT /api/progress

Update watch progress for a media item.

**Request Body**:

```json
{
  "media_id": 1,
  "position": 3600,
  "duration": 8160
}
```

**Response**: `200 OK`

```json
{
  "id": 1,
  "media_id": 1,
  "position": 3600,
  "duration": 8160,
  "watched": false,
  "last_watched": "2024-11-11T20:30:00Z",
  "updated_at": "2024-11-11T20:30:00Z"
}
```

**Note**: Automatically marks as `watched: true` if position > 90% of duration

---

### Mark as Watched

#### POST /api/progress/mark-watched

Mark media as watched.

**Request Body**:

```json
{
  "media_id": 1
}
```

**Response**: `200 OK`

```json
{
  "id": 1,
  "media_id": 1,
  "watched": true,
  "last_watched": "2024-11-11T20:30:00Z"
}
```

---

### Mark as Unwatched

#### POST /api/progress/mark-unwatched

Mark media as unwatched and reset position.

**Request Body**:

```json
{
  "media_id": 1
}
```

**Response**: `200 OK`

```json
{
  "id": 1,
  "media_id": 1,
  "position": 0,
  "watched": false,
  "updated_at": "2024-11-11T20:35:00Z"
}
```

---

## Real-Time Updates (SSE)

### Scan Progress Stream

#### GET /api/libraries/:id/scan/progress

Server-Sent Events stream for library scan progress.

**Event Types**:
- `progress` - Scan progress update
- `complete` - Scan completed
- `error` - Scan failed

---

### Transcode Progress Stream

#### GET /api/media/:id/transcode/progress

Server-Sent Events stream for transcode job progress.

**Event Types**:
- `progress` - Transcode progress update (with percentage)
- `complete` - Transcode job completed
- `error` - Transcode job failed

---

## Rate Limiting

**Future**: Rate limiting will be applied to prevent abuse.

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 99
X-RateLimit-Reset: 1699714800
```

---

## Versioning

**Current Strategy**: No versioning (single `/api/...` namespace)

**Rationale**: 
- Pre-1.0 project with no external users
- Additive changes only (add fields, never remove)
- Keeps development simple and fast

**Future** (1.0+ release):
- URL path versioning: `/api/v1/...`, `/api/v2/...`
- Handler structure ready for easy migration
- Breaking changes trigger new version

---

## Watch Progress Sync

### Client-Side Batching Strategy

**Frequency**:
- Track playback position locally every 1 second (UI updates)
- Send batch update to backend every 10 seconds
- Send immediately on pause/stop/seek/close events

**Endpoint**: `PUT /api/watch/progress`

**Behavior**:
- Maximum 10 seconds of progress lost on crash
- Minimal server load
- Smooth UI updates

**Implementation**:
```javascript
// Track locally
let currentPosition = 0;
setInterval(() => {
  currentPosition = player.getCurrentTime();
}, 1000);

// Sync to backend every 10s
setInterval(() => {
  if (playing) {
    updateProgress(mediaId, currentPosition);
  }
}, 10000);

// Immediate sync on events
player.on('pause', () => updateProgress(mediaId, currentPosition));
player.on('ended', () => markWatched(mediaId));
```

---

## Static File Serving

### Image Strategy

**Format**: All images served as WebP
- Thumbnails: Generated from FFmpeg, converted to WebP
- Posters/Backdrops: Downloaded from TMDb, converted to WebP
- Superior compression, wide browser support

**Endpoints**:
- `GET /api/media/:id/thumbnail` - Returns WebP thumbnail
- `GET /api/media/:id/poster` - Returns WebP poster
- `GET /api/media/:id/backdrop` - Returns WebP backdrop

**Caching Headers**:
```http
Cache-Control: public, max-age=31536000, immutable
ETag: "<file-hash>"
```

**Storage**:
- `data/thumbnails/<media_id>.webp`
- `data/posters/<media_id>.webp`
- `data/backdrops/<media_id>.webp`

---

## Error Handling

### Frontend Error Display

**Inline Validation Errors** (forms):
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "fields": {
      "name": "name is required",
      "path": "path must be absolute"
    }
  }
}
```
Display field errors inline under form inputs.

**Toast Notifications** (actions):
- System errors (network, server)
- Success messages (library created, scan started)
- Action confirmations

**Error Boundaries** (crashes):
- React error boundaries catch component crashes
- Display error UI with retry button
- Log errors for debugging

---

## Performance & Optimization

### Eager vs Lazy Loading

**List Endpoints** (e.g., `GET /api/media`):
- Return minimal data (id, title, thumbnail, basic metadata)
- Optimized for displaying grids/lists
- Fast response times

**Detail Endpoints** (e.g., `GET /api/media/:id`):
- Eager-load related data (genres, cast, audio tracks, subtitles)
- Complete information for detail views
- Single query for all related data

**Future Enhancement** (Phase 6):
- Allow clients to specify includes: `?include=genres,cast,crew`
- Flexible data loading based on needs

---

## CORS

Cross-Origin Resource Sharing is enabled for configured origins.

**Allowed Origins** (configurable):
- `http://localhost:5173` (Vite dev server)
- `http://localhost:3000` (Production embedded frontend)

**Allowed Methods**:
- `GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`

**Allowed Headers**:
- `Content-Type`, `Authorization`, `Range`

---

## WebSocket Support

**Future**: WebSocket support for real-time notifications.

Currently using Server-Sent Events (SSE) for one-way server→client updates.
