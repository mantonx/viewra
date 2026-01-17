# Home Screen Enhancement Plan - Part 3

## Search Enhancement & Plugin File Storage

This document captures research findings for enhancing ViewRA's search capabilities and designing the plugin file storage SDK needed to support Bleve integration.

---

## Search Technology Research

### Current State Analysis

ViewRA's current search architecture:

| Component | Technology | Characteristics |
|-----------|------------|-----------------|
| Semantic Search | Vector embeddings (AI) | Natural language understanding, requires AI provider |
| Fallback Search | SQL `LIKE '%query%'` | Slow, no typo tolerance, case-sensitive |
| Text Search in Vectors | `VectorSearchText()` | Searches embedded text field, limited |

**Problems with current fallback:**
- No fuzzy matching (typos break search)
- No stemming ("running" won't find "run")
- Poor performance on large libraries
- No relevance ranking

### JellySearch Analysis

Researched Meilisearch integration patterns:

**Architecture:**
- Uses Meilisearch as external search engine
- Indexes ~18 fields from SQLite database
- Intercepts `/Items` endpoint via ActionFilter
- Provides typo tolerance, fast prefix search

**Fields Indexed:**
```csharp
Name, OriginalTitle, SortName, Overview, Tagline,
Genres, Studios, Tags, People (actors/directors),
ProductionYear, OfficialRating, PremiereDate
```

**Limitations:**
- Requires external Meilisearch server
- No semantic/natural language understanding
- Simple keyword matching only

### Go Search Libraries Comparison

| Library | Type | Binary Size | Features | Used By |
|---------|------|-------------|----------|---------|
| **Bleve** | Full-text + Vector | ~10MB | FTS, fuzzy, stemming, facets, KNN | Gitea, Grafana, Dendrite |
| sahilm/fuzzy | Fuzzy matching | <1MB | Simple fuzzy, no index | k9s, lazygit |
| lithammer/fuzzysearch | Fuzzy search | <1MB | Levenshtein-based | Various CLIs |
| agnivade/levenshtein | Edit distance | <100KB | Distance calculation only | Libraries |

### Bleve Deep Dive

**Why Bleve is ideal for ViewRA:**

1. **Hybrid Search** - Supports both text search AND vector KNN in one engine
2. **Typo Tolerance** - Built-in fuzzy matching
3. **Stemming** - 30+ language support
4. **Faceted Search** - Filter by genre, year, etc.
5. **Pure Go** - No external dependencies
6. **Embedded** - Runs in-process, no separate server

**Bleve Index Structure:**
```go
mapping := bleve.NewIndexMapping()

// Document mapping for movies
movieMapping := bleve.NewDocumentMapping()
movieMapping.AddFieldMappingsAt("title", bleve.NewTextFieldMapping())
movieMapping.AddFieldMappingsAt("plot", bleve.NewTextFieldMapping())
movieMapping.AddFieldMappingsAt("genres", bleve.NewTextFieldMapping())
movieMapping.AddFieldMappingsAt("year", bleve.NewNumericFieldMapping())
movieMapping.AddFieldMappingsAt("vector", bleve.NewVectorFieldMapping())

mapping.AddDocumentMapping("movie", movieMapping)
```

**Storage Requirements:**
- Uses BoltDB for persistence (embedded key-value store)
- Needs filesystem access for index directory
- Index size: ~50-100 bytes per document + vector storage

---

## Plugin Architecture Analysis

### Current Plugin Dependencies

```
ai-features (root)
    │
    ├── ai-provider-anthropic
    ├── ai-provider-openai
    └── ai-provider-voyage
         │
         └── semantic-search (requires embedding capability)
              │
              └── recommendations (calls semantic-search's FindSimilar)
```

### semantic-search Plugin Responsibilities

1. **Vector Indexing** - Store embeddings for media items
2. **Semantic Search** - Find similar items by meaning
3. **Mood Tags** - Generate mood-based tags
4. **Query Rewriting** - Enhance search queries with AI

### Proposed Bleve Integration

Add Bleve to `semantic-search` for hybrid search:

```
semantic-search plugin
├── Vector Storage (existing) - pgvector/sqlite-vec via host
├── Bleve Index (new) - Local filesystem
└── Search Router - Choose vector vs text vs hybrid
```

**Search Flow:**
```
User Query
    │
    ├─► If has AI provider → Generate embedding → Vector search
    │
    ├─► If no AI provider → Bleve text search (fuzzy, stemmed)
    │
    └─► Hybrid mode → Both, merge results
```

---

## Plugin File Storage SDK Design

### Requirements

1. **Secure Path Validation** - Prevent directory traversal attacks
2. **Convenience Methods** - Common file operations
3. **Quota Monitoring** - Track file storage usage
4. **Backup Compatible** - Store under `./data/` directory

### Current Storage Locations

```
./data/                           # Main data directory (backed up)
├── viewra.db                     # Main database
├── cache/
│   ├── images/
│   └── transcodes/
└── plugins/
    ├── semantic-search/          # Plugin binary
    │   └── semantic-search
    └── storage/                  # Plugin data storage
        └── {plugin-id}/
            └── cache.db          # Plugin's SQLite cache
```

**Current SDK:**
- `Base.DataDir()` - Returns `./data/plugins/storage/{plugin-id}/`
- No path validation helpers
- No file operation convenience methods
- No quota tracking for files

### Files SDK Design

#### New File: `pkg/plugin/sdk/files.go`

```go
// Package sdk provides file utilities for ViewRA plugins.
//
// The Files helper provides safe filesystem operations within the plugin's
// data directory. All paths are validated to prevent directory traversal.
//
// # Usage
//
//	func (p *MyPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) error {
//	    p.files = sdk.NewFiles(req.DataDir)
//	    
//	    // Create a subdirectory for Bleve index
//	    if err := p.files.EnsureDir("bleve-index"); err != nil {
//	        return err
//	    }
//	    
//	    // Get absolute path for libraries that need it
//	    indexPath, err := p.files.AbsPath("bleve-index")
//	    if err != nil {
//	        return err
//	    }
//	    
//	    // Open Bleve index at indexPath
//	}
package sdk

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathEscapesDataDir is returned when a path would escape the data directory.
var ErrPathEscapesDataDir = errors.New("path escapes plugin data directory")

// Files provides safe filesystem operations within a plugin's data directory.
// All operations are sandboxed to prevent access outside the data directory.
type Files struct {
	baseDir string
}

// NewFiles creates a Files helper for the given plugin data directory.
// The baseDir should be the value returned by Base.DataDir().
func NewFiles(baseDir string) *Files {
	return &Files{baseDir: filepath.Clean(baseDir)}
}

// BaseDir returns the plugin's base data directory.
func (f *Files) BaseDir() string {
	return f.baseDir
}

// AbsPath returns the absolute path for a relative path within the data directory.
// Returns ErrPathEscapesDataDir if the resolved path would escape the data directory.
//
// Example:
//
//	absPath, err := files.AbsPath("bleve-index")
//	// absPath = "/data/plugins/storage/my-plugin/bleve-index"
func (f *Files) AbsPath(relPath string) (string, error) {
	if relPath == "" {
		return f.baseDir, nil
	}
	
	// Clean the path and join with base
	cleaned := filepath.Clean(relPath)
	
	// Reject absolute paths
	if filepath.IsAbs(cleaned) {
		return "", ErrPathEscapesDataDir
	}
	
	// Join and clean the full path
	absPath := filepath.Join(f.baseDir, cleaned)
	absPath = filepath.Clean(absPath)
	
	// Ensure the path is within baseDir (handles ".." traversal)
	if !strings.HasPrefix(absPath, f.baseDir+string(os.PathSeparator)) && absPath != f.baseDir {
		return "", ErrPathEscapesDataDir
	}
	
	return absPath, nil
}

// EnsureDir creates a directory within the data directory if it doesn't exist.
// Creates parent directories as needed (like mkdir -p).
func (f *Files) EnsureDir(relPath string) error {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(absPath, 0755)
}

// Exists checks if a path exists within the data directory.
func (f *Files) Exists(relPath string) bool {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return false
	}
	_, err = os.Stat(absPath)
	return err == nil
}

// IsDir checks if a path exists and is a directory.
func (f *Files) IsDir(relPath string) bool {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return false
	}
	info, err := os.Stat(absPath)
	return err == nil && info.IsDir()
}

// Stat returns file info for a path within the data directory.
func (f *Files) Stat(relPath string) (fs.FileInfo, error) {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return nil, err
	}
	return os.Stat(absPath)
}

// ReadFile reads the contents of a file within the data directory.
func (f *Files) ReadFile(relPath string) ([]byte, error) {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(absPath)
}

// WriteFile writes data to a file within the data directory.
// Creates parent directories as needed.
func (f *Files) WriteFile(relPath string, data []byte, perm fs.FileMode) error {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return err
	}
	
	// Ensure parent directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	return os.WriteFile(absPath, data, perm)
}

// Remove removes a file or empty directory.
func (f *Files) Remove(relPath string) error {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return err
	}
	return os.Remove(absPath)
}

// RemoveAll removes a path and all its contents.
func (f *Files) RemoveAll(relPath string) error {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return err
	}
	
	// Safety: don't allow removing the entire base directory
	if absPath == f.baseDir {
		return errors.New("cannot remove base data directory")
	}
	
	return os.RemoveAll(absPath)
}

// ReadDir reads the contents of a directory.
func (f *Files) ReadDir(relPath string) ([]fs.DirEntry, error) {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(absPath)
}

// Open opens a file for reading. Caller must close the file.
func (f *Files) Open(relPath string) (*os.File, error) {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(absPath)
}

// Create creates or truncates a file. Creates parent directories as needed.
// Caller must close the file.
func (f *Files) Create(relPath string) (*os.File, error) {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return nil, err
	}
	
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	
	return os.Create(absPath)
}

// OpenFile opens a file with the specified flags and permissions.
// Creates parent directories as needed for write modes.
func (f *Files) OpenFile(relPath string, flag int, perm fs.FileMode) (*os.File, error) {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return nil, err
	}
	
	if flag&(os.O_CREATE|os.O_WRONLY|os.O_RDWR) != 0 {
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	
	return os.OpenFile(absPath, flag, perm)
}

// Copy copies a file from src to dst within the data directory.
func (f *Files) Copy(srcRelPath, dstRelPath string) error {
	srcAbs, err := f.AbsPath(srcRelPath)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	
	dstAbs, err := f.AbsPath(dstRelPath)
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	
	src, err := os.Open(srcAbs)
	if err != nil {
		return err
	}
	defer src.Close()
	
	srcInfo, err := src.Stat()
	if err != nil {
		return err
	}
	
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
		return err
	}
	
	dst, err := os.OpenFile(dstAbs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dst.Close()
	
	_, err = io.Copy(dst, src)
	return err
}

// DiskUsage calculates the total size of all files in a directory (recursively).
// Returns 0 if the directory doesn't exist.
func (f *Files) DiskUsage(relPath string) (int64, error) {
	absPath, err := f.AbsPath(relPath)
	if err != nil {
		return 0, err
	}
	
	var size int64
	err = filepath.WalkDir(absPath, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return 0, err
	}
	return size, nil
}

// TotalDiskUsage calculates the total size of all files in the data directory.
func (f *Files) TotalDiskUsage() (int64, error) {
	return f.DiskUsage("")
}
```

### Proto Changes

Update `api/proto/plugin/host_services.proto`:

```protobuf
message DatabaseStats {
  int64 size_bytes = 1;      // Total storage (KV + SQL + file)
  int64 quota_bytes = 2;     // Quota limit
  int32 table_count = 3;     // SQL table count (or KV key count)
  
  // Breakdown by storage type (new fields)
  int64 kv_size_bytes = 4;     // Key-value storage size
  int64 sql_size_bytes = 5;    // SQL storage size
  int64 file_size_bytes = 6;   // File storage size in data directory
  int64 vector_size_bytes = 7; // Vector storage size
}
```

### Host Storage Updates

Modify `internal/infrastructure/plugins/host/storage.go`:

```go
// GetDatabaseStats returns storage usage statistics for the plugin.
func (s *StorageServer) GetDatabaseStats(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.DatabaseStats, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	// Get KV store size from database
	kvSize, err := s.querier.PluginKVTotalSize(ctx, pluginID)
	if err != nil {
		kvSize = 0
	}

	// Get plugin's file storage size (recursive directory walk)
	pluginDir := filepath.Join(s.baseDir, pluginID)
	fileSize := calculateDirSize(pluginDir)

	// Get key count
	keyCount, err := s.querier.PluginKVCount(ctx, pluginID)
	if err != nil {
		keyCount = 0
	}

	totalSize := kvSize + fileSize

	return &pluginv1.DatabaseStats{
		SizeBytes:     totalSize,
		QuotaBytes:    s.defaultQuota,
		TableCount:    int32(keyCount),
		KvSizeBytes:   kvSize,
		FileSizeBytes: fileSize,
	}, nil
}

// calculateDirSize recursively calculates directory size.
func calculateDirSize(dir string) int64 {
	var size int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}
```

### SDK StorageClient Enhancement

Add to `pkg/plugin/sdk/host.go`:

```go
// Stats returns storage usage statistics for the plugin.
func (c *StorageClient) Stats(ctx context.Context) (*StorageStats, error) {
	resp, err := c.client.GetDatabaseStats(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, err
	}
	return &StorageStats{
		TotalBytes:  resp.SizeBytes,
		QuotaBytes:  resp.QuotaBytes,
		KVBytes:     resp.KvSizeBytes,
		FileBytes:   resp.FileSizeBytes,
		VectorBytes: resp.VectorSizeBytes,
		KeyCount:    int(resp.TableCount),
	}, nil
}

// StorageStats contains storage usage information.
type StorageStats struct {
	TotalBytes  int64 // Total storage used
	QuotaBytes  int64 // Quota limit (0 = no limit)
	KVBytes     int64 // Key-value storage size
	FileBytes   int64 // File storage size
	VectorBytes int64 // Vector storage size
	KeyCount    int   // Number of KV keys
}
```

---

## Usage Example: semantic-search with Bleve

```go
func (p *Plugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
    p.Base.Init(req.DataDir)
    p.files = sdk.NewFiles(req.DataDir)
    
    // Check storage stats before creating large index
    stats, _ := p.storage.Stats(ctx)
    if stats.QuotaBytes > 0 && stats.TotalBytes > stats.QuotaBytes*90/100 {
        p.Log().Warn("storage near quota", "used", stats.TotalBytes, "quota", stats.QuotaBytes)
    }
    
    // Get absolute path for Bleve (it needs direct filesystem access)
    indexPath, err := p.files.AbsPath("bleve-index")
    if err != nil {
        return nil, fmt.Errorf("invalid index path: %w", err)
    }
    
    // Open or create Bleve index
    if p.files.Exists("bleve-index") {
        p.index, err = bleve.Open(indexPath)
    } else {
        if err := p.files.EnsureDir("bleve-index"); err != nil {
            return nil, err
        }
        mapping := p.buildIndexMapping()
        p.index, err = bleve.New(indexPath, mapping)
    }
    if err != nil {
        return nil, fmt.Errorf("bleve index: %w", err)
    }
    
    return &pluginv1.InitResponse{Success: true}, nil
}

// buildIndexMapping creates the Bleve index mapping for media items
func (p *Plugin) buildIndexMapping() mapping.IndexMapping {
    indexMapping := bleve.NewIndexMapping()
    
    // Movie document mapping
    movieMapping := bleve.NewDocumentMapping()
    
    // Text fields for search
    titleField := bleve.NewTextFieldMapping()
    titleField.Analyzer = "en"  // English analyzer with stemming
    movieMapping.AddFieldMappingsAt("title", titleField)
    
    plotField := bleve.NewTextFieldMapping()
    plotField.Analyzer = "en"
    movieMapping.AddFieldMappingsAt("plot", plotField)
    
    // Keyword fields for filtering
    genreField := bleve.NewKeywordFieldMapping()
    movieMapping.AddFieldMappingsAt("genres", genreField)
    
    // Numeric fields
    yearField := bleve.NewNumericFieldMapping()
    movieMapping.AddFieldMappingsAt("year", yearField)
    
    indexMapping.AddDocumentMapping("movie", movieMapping)
    indexMapping.AddDocumentMapping("tv_show", movieMapping) // Same mapping
    
    return indexMapping
}
```

---

## Implementation Steps

### Phase 1: Files SDK (0.5 day)

1. Create `pkg/plugin/sdk/files.go`
2. Create `pkg/plugin/sdk/files_test.go`
3. Test path validation edge cases

### Phase 2: Storage Stats Enhancement (0.5 day)

1. Update proto with new `DatabaseStats` fields
2. Run `make proto-gen`
3. Update `storage.go` to calculate file sizes
4. Add `Stats()` method to SDK `StorageClient`
5. Test with existing plugins

### Phase 3: Bleve Integration (1-2 days)

1. Add Bleve dependency to semantic-search plugin
2. Create index on plugin startup
3. Index existing media on first run
4. Add text search endpoint alongside vector search
5. Implement hybrid search (text + vector)
6. Handle incremental updates on media changes

### Phase 4: Search API Enhancement (0.5 day)

1. Update search handler to use hybrid search
2. Add fallback chain: semantic -> bleve -> SQL LIKE
3. Return search source in response for debugging

---

## Open Questions

### Quota Enforcement

**Current state:** Quota is reported but not enforced.

**Options:**
- A) Report only (current) - Plugins self-enforce
- B) Soft limit - Warn but allow writes
- C) Hard limit - Reject writes over quota

**Recommendation:** Option B for now - log warnings when quota exceeded but don't block.

### Bleve Index Location

**Options:**
- A) `./data/plugins/storage/{plugin-id}/bleve-index/` (with other plugin data)
- B) `./data/cache/search-index/` (separate cache location)

**Recommendation:** Option A - Keep with plugin data for easier backup/restore.

### Index Rebuild Strategy

**Options:**
- A) Full rebuild on plugin restart
- B) Incremental updates via event subscription
- C) Periodic sync with database

**Recommendation:** Option B - Subscribe to media events for incremental updates, with full rebuild available as admin action.

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `pkg/plugin/sdk/files.go` | Files SDK helper |
| `pkg/plugin/sdk/files_test.go` | Unit tests |
| `plugins/semantic-search/internal/bleve/` | Bleve integration |

### Modified Files

| File | Changes |
|------|---------|
| `api/proto/plugin/host_services.proto` | Add storage breakdown fields |
| `internal/infrastructure/plugins/host/storage.go` | Calculate file sizes |
| `pkg/plugin/sdk/host.go` | Add `Stats()` method |
| `plugins/semantic-search/plugin.go` | Initialize Bleve index |

---

## Summary

This part covers:

1. **Search Technology Research**
   - Analyzed JellySearch (Meilisearch plugin)
   - Compared Go search libraries
   - Recommended Bleve for hybrid text+vector search

2. **Plugin File Storage SDK**
   - Designed `Files` helper with path validation
   - Added convenience methods for common operations
   - Enhanced `DatabaseStats` with storage breakdown

3. **Bleve Integration Plan**
   - Store index in plugin data directory
   - Hybrid search: vector + text + SQL fallback
   - Incremental updates via media events

**Total estimated implementation time: 3-4 days**
