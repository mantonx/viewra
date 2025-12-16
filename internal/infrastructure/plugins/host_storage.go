package plugins

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// HostStorageServer implements the HostStorage gRPC service.
// Each plugin gets an isolated namespace for key-value storage.
type HostStorageServer struct {
	pluginv1.UnimplementedHostStorageServer

	// baseDir is the base directory for plugin data (e.g., SQLite databases).
	baseDir string

	// Database access via sqlc
	db       *sql.DB
	dbType   string
	postgres *sqlc_postgres.Queries
	sqlite   *sqlc_sqlite.Queries
	router   *common.QueryRouter

	// Default quota per plugin (100 MB)
	defaultQuota int64

	logger *slog.Logger
}

// HostStorageConfig configures the host storage server.
type HostStorageConfig struct {
	// BaseDir is the base directory for plugin data files.
	BaseDir string

	// DefaultQuota is the default storage quota per plugin in bytes.
	// Defaults to 100 MB if not set.
	DefaultQuota int64
}

// NewHostStorageServer creates a new HostStorageServer.
func NewHostStorageServer(cfg HostStorageConfig, db *sql.DB, driver string, logger *slog.Logger) (*HostStorageServer, error) {
	if cfg.BaseDir == "" {
		return nil, errors.New("base directory is required")
	}

	if err := os.MkdirAll(cfg.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	defaultQuota := cfg.DefaultQuota
	if defaultQuota == 0 {
		defaultQuota = 100 * 1024 * 1024 // 100 MB
	}

	s := &HostStorageServer{
		baseDir:      cfg.BaseDir,
		db:           db,
		dbType:       driver,
		router:       common.NewQueryRouter(driver),
		defaultQuota: defaultQuota,
		logger:       logger,
	}

	if common.IsPostgres(driver) {
		s.postgres = sqlc_postgres.New(db)
	} else {
		s.sqlite = sqlc_sqlite.New(db)
	}

	return s, nil
}

// KVGet retrieves a value from the plugin's key-value store.
func (s *HostStorageServer) KVGet(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.KVValue, error) {
	pluginID := getPluginIDFromContext(ctx)
	s.logger.Debug("KVGet called", "plugin_id", pluginID, "key", req.Key)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Key == "" {
		return nil, errors.New("key is required")
	}

	result, err := s.router.Route(
		func() (any, error) {
			return s.postgres.PluginKVGet(ctx, sqlc_postgres.PluginKVGetParams{
				PluginID: pluginID,
				Key:      req.Key,
			})
		},
		func() (any, error) {
			return s.sqlite.PluginKVGet(ctx, sqlc_sqlite.PluginKVGetParams{
				PluginID: pluginID,
				Key:      req.Key,
			})
		},
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &pluginv1.KVValue{Exists: false}, nil
		}
		s.logger.Error("failed to get key", "plugin", pluginID, "key", req.Key, "error", err)
		return nil, err
	}

	// Extract value based on result type
	var value []byte
	switch r := result.(type) {
	case sqlc_postgres.PluginKVGetRow:
		value = r.Value
	case sqlc_sqlite.PluginKVGetRow:
		value = r.Value
	}

	return &pluginv1.KVValue{
		Value:  value,
		Exists: true,
	}, nil
}

// KVSet stores a value in the plugin's key-value store.
func (s *HostStorageServer) KVSet(ctx context.Context, req *pluginv1.KVEntry) (*pluginv1.Empty, error) {
	pluginID := getPluginIDFromContext(ctx)
	s.logger.Debug("KVSet called", "plugin_id", pluginID, "key", req.Key, "value_size", len(req.Value))
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Key == "" {
		return nil, errors.New("key is required")
	}

	// Calculate expiration time
	var expiresAt sql.NullString
	if req.TtlSeconds > 0 {
		expiry := time.Now().Add(time.Duration(req.TtlSeconds) * time.Second)
		expiresAt = sql.NullString{String: expiry.Format(time.RFC3339), Valid: true}
	}

	_, err := s.router.Route(
		func() (any, error) {
			var pgExpiry sql.NullTime
			if expiresAt.Valid {
				t, _ := time.Parse(time.RFC3339, expiresAt.String)
				pgExpiry = sql.NullTime{Time: t, Valid: true}
			}
			return nil, s.postgres.PluginKVSet(ctx, sqlc_postgres.PluginKVSetParams{
				PluginID:  pluginID,
				Key:       req.Key,
				Value:     req.Value,
				ExpiresAt: pgExpiry,
			})
		},
		func() (any, error) {
			return nil, s.sqlite.PluginKVSet(ctx, sqlc_sqlite.PluginKVSetParams{
				PluginID:  pluginID,
				Key:       req.Key,
				Value:     req.Value,
				ExpiresAt: expiresAt,
			})
		},
	)

	if err != nil {
		s.logger.Error("failed to set key", "plugin", pluginID, "key", req.Key, "error", err)
		return nil, err
	}

	return &pluginv1.Empty{}, nil
}

// KVDelete removes a value from the plugin's key-value store.
func (s *HostStorageServer) KVDelete(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.Empty, error) {
	pluginID := getPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Key == "" {
		return nil, errors.New("key is required")
	}

	_, err := s.router.Route(
		func() (any, error) {
			return nil, s.postgres.PluginKVDelete(ctx, sqlc_postgres.PluginKVDeleteParams{
				PluginID: pluginID,
				Key:      req.Key,
			})
		},
		func() (any, error) {
			return nil, s.sqlite.PluginKVDelete(ctx, sqlc_sqlite.PluginKVDeleteParams{
				PluginID: pluginID,
				Key:      req.Key,
			})
		},
	)

	if err != nil {
		s.logger.Error("failed to delete key", "plugin", pluginID, "key", req.Key, "error", err)
		return nil, err
	}

	return &pluginv1.Empty{}, nil
}

// KVList lists keys with an optional prefix.
func (s *HostStorageServer) KVList(ctx context.Context, req *pluginv1.KVListRequest) (*pluginv1.KVKeyList, error) {
	pluginID := getPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	result, err := s.router.Route(
		func() (any, error) {
			return s.postgres.PluginKVList(ctx, sqlc_postgres.PluginKVListParams{
				PluginID: pluginID,
				Column2:  req.Prefix,
				Column3:  sql.NullString{String: req.Prefix, Valid: true},
				Limit:    int32(limit),
			})
		},
		func() (any, error) {
			return s.sqlite.PluginKVList(ctx, sqlc_sqlite.PluginKVListParams{
				PluginID: pluginID,
				Column2:  req.Prefix,
				Column3:  sql.NullString{String: req.Prefix, Valid: true},
				Limit:    int64(limit),
			})
		},
	)

	if err != nil {
		s.logger.Error("failed to list keys", "plugin", pluginID, "prefix", req.Prefix, "error", err)
		return nil, err
	}

	// Extract keys based on result type
	var keys []string
	switch r := result.(type) {
	case []string:
		keys = r
	}

	return &pluginv1.KVKeyList{Keys: keys}, nil
}

// GetDatabasePath returns the path to the plugin's SQLite database.
// Plugins can use this for their own local caching needs.
func (s *HostStorageServer) GetDatabasePath(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.DatabasePath, error) {
	pluginID := getPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	// Create plugin directory if it doesn't exist
	pluginDir := filepath.Join(s.baseDir, pluginID)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}

	dbPath := filepath.Join(pluginDir, "cache.db")
	return &pluginv1.DatabasePath{Path: dbPath}, nil
}

// RegisterSchema is a no-op - plugins manage their own database schemas.
func (s *HostStorageServer) RegisterSchema(ctx context.Context, req *pluginv1.SchemaVersion) (*pluginv1.Empty, error) {
	// Plugins manage their own database schemas.
	// This is kept for API compatibility but does nothing.
	return &pluginv1.Empty{}, nil
}

// GetDatabaseStats returns storage usage statistics for the plugin.
func (s *HostStorageServer) GetDatabaseStats(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.DatabaseStats, error) {
	pluginID := getPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	// Get KV store size from database
	result, err := s.router.Route(
		func() (any, error) {
			return s.postgres.PluginKVTotalSize(ctx, pluginID)
		},
		func() (any, error) {
			return s.sqlite.PluginKVTotalSize(ctx, pluginID)
		},
	)

	var kvSize int64
	if err == nil {
		switch r := result.(type) {
		case int64:
			kvSize = r
		case interface{}:
			// SQLite returns interface{} for COALESCE
			if v, ok := r.(int64); ok {
				kvSize = v
			}
		}
	}

	// Get plugin's local database file size
	dbPath := filepath.Join(s.baseDir, pluginID, "cache.db")
	var fileSize int64
	if info, err := os.Stat(dbPath); err == nil {
		fileSize = info.Size()
	}

	// Get key count
	var keyCount int64
	countResult, err := s.router.Route(
		func() (any, error) {
			return s.postgres.PluginKVCount(ctx, pluginID)
		},
		func() (any, error) {
			return s.sqlite.PluginKVCount(ctx, pluginID)
		},
	)
	if err == nil {
		if c, ok := countResult.(int64); ok {
			keyCount = c
		}
	}

	return &pluginv1.DatabaseStats{
		SizeBytes:  kvSize + fileSize,
		QuotaBytes: s.defaultQuota,
		TableCount: int32(keyCount), // Repurposing as key count
	}, nil
}

// DeletePluginStorage removes all storage for a plugin.
// Called when a plugin is uninstalled.
func (s *HostStorageServer) DeletePluginStorage(ctx context.Context, pluginID string) error {
	// Delete KV entries
	_, err := s.router.Route(
		func() (any, error) {
			return nil, s.postgres.PluginKVDeleteByPlugin(ctx, pluginID)
		},
		func() (any, error) {
			return nil, s.sqlite.PluginKVDeleteByPlugin(ctx, pluginID)
		},
	)
	if err != nil {
		s.logger.Error("failed to delete plugin KV entries", "plugin", pluginID, "error", err)
	}

	// Delete plugin data directory
	pluginDir := filepath.Join(s.baseDir, pluginID)
	if err := os.RemoveAll(pluginDir); err != nil && !os.IsNotExist(err) {
		s.logger.Error("failed to delete plugin directory", "plugin", pluginID, "error", err)
		return err
	}

	s.logger.Info("deleted plugin storage", "plugin", pluginID)
	return nil
}

// CleanupExpiredEntries removes expired KV entries.
// Should be called periodically (e.g., daily).
func (s *HostStorageServer) CleanupExpiredEntries(ctx context.Context) error {
	_, err := s.router.Route(
		func() (any, error) {
			return nil, s.postgres.PluginKVDeleteExpired(ctx)
		},
		func() (any, error) {
			return nil, s.sqlite.PluginKVDeleteExpired(ctx)
		},
	)
	if err != nil {
		s.logger.Error("failed to cleanup expired entries", "error", err)
		return err
	}
	return nil
}

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const pluginIDKey contextKey = "plugin_id"

// ContextWithPluginID returns a context with the plugin ID set.
func ContextWithPluginID(ctx context.Context, pluginID string) context.Context {
	return context.WithValue(ctx, pluginIDKey, pluginID)
}

// getPluginIDFromContext extracts the plugin ID from the context.
func getPluginIDFromContext(ctx context.Context) string {
	if v := ctx.Value(pluginIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
