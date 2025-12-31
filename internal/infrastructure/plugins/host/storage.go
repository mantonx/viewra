package host

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
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// StorageServer implements the HostStorage gRPC service.
// Each plugin gets an isolated namespace for key-value storage.
type StorageServer struct {
	pluginv1.UnimplementedHostStorageServer

	// baseDir is the base directory for plugin data (e.g., SQLite databases).
	baseDir string

	// Database access via unified querier
	db      *sql.DB
	dbType  string
	querier *unified.Querier

	// Default quota per plugin (100 MB)
	defaultQuota int64

	logger *slog.Logger
}

// StorageConfig configures the host storage server.
type StorageConfig struct {
	// BaseDir is the base directory for plugin data files.
	BaseDir string

	// DefaultQuota is the default storage quota per plugin in bytes.
	// Defaults to 100 MB if not set.
	DefaultQuota int64
}

// NewStorageServer creates a new StorageServer.
func NewStorageServer(cfg StorageConfig, db *sql.DB, driver string, logger *slog.Logger) (*StorageServer, error) {
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

	return &StorageServer{
		baseDir:      cfg.BaseDir,
		db:           db,
		dbType:       driver,
		querier:      unified.NewQuerier(db, driver),
		defaultQuota: defaultQuota,
		logger:       logger,
	}, nil
}

// KVGet retrieves a value from the plugin's key-value store.
func (s *StorageServer) KVGet(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.KVValue, error) {
	pluginID := GetPluginIDFromContext(ctx)
	s.logger.Debug("KVGet called", "plugin_id", pluginID, "key", req.Key)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Key == "" {
		return nil, errors.New("key is required")
	}

	result, err := s.querier.PluginKVGet(ctx, unified.PluginKVGetParams{
		PluginID: pluginID,
		Key:      req.Key,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &pluginv1.KVValue{Exists: false}, nil
		}
		s.logger.Error("failed to get key", "plugin", pluginID, "key", req.Key, "error", err)
		return nil, err
	}

	return &pluginv1.KVValue{
		Value:  result.Value,
		Exists: true,
	}, nil
}

// KVSet stores a value in the plugin's key-value store.
func (s *StorageServer) KVSet(ctx context.Context, req *pluginv1.KVEntry) (*pluginv1.Empty, error) {
	pluginID := GetPluginIDFromContext(ctx)
	s.logger.Debug("KVSet called", "plugin_id", pluginID, "key", req.Key, "value_size", len(req.Value))
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Key == "" {
		return nil, errors.New("key is required")
	}

	// Calculate expiration time
	var expiresAt sql.NullTime
	if req.TtlSeconds > 0 {
		expiry := time.Now().Add(time.Duration(req.TtlSeconds) * time.Second)
		expiresAt = sql.NullTime{Time: expiry, Valid: true}
	}

	err := s.querier.PluginKVSet(ctx, unified.PluginKVSetParams{
		PluginID:  pluginID,
		Key:       req.Key,
		Value:     req.Value,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		s.logger.Error("failed to set key", "plugin", pluginID, "key", req.Key, "error", err)
		return nil, err
	}

	return &pluginv1.Empty{}, nil
}

// KVDelete removes a value from the plugin's key-value store.
func (s *StorageServer) KVDelete(ctx context.Context, req *pluginv1.KVKey) (*pluginv1.Empty, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	if req.Key == "" {
		return nil, errors.New("key is required")
	}

	err := s.querier.PluginKVDelete(ctx, unified.PluginKVDeleteParams{
		PluginID: pluginID,
		Key:      req.Key,
	})
	if err != nil {
		s.logger.Error("failed to delete key", "plugin", pluginID, "key", req.Key, "error", err)
		return nil, err
	}

	return &pluginv1.Empty{}, nil
}

// KVList lists keys with an optional prefix.
func (s *StorageServer) KVList(ctx context.Context, req *pluginv1.KVListRequest) (*pluginv1.KVKeyList, error) {
	pluginID := GetPluginIDFromContext(ctx)
	if pluginID == "" {
		return nil, errors.New("plugin ID not found in context")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	keys, err := s.querier.PluginKVList(ctx, unified.PluginKVListParams{
		PluginID: pluginID,
		Column2:  req.Prefix,
		Column3:  sql.NullString{String: req.Prefix, Valid: true},
		Limit:    int64(limit),
	})
	if err != nil {
		s.logger.Error("failed to list keys", "plugin", pluginID, "prefix", req.Prefix, "error", err)
		return nil, err
	}

	return &pluginv1.KVKeyList{Keys: keys}, nil
}

// GetDatabasePath returns the path to the plugin's SQLite database.
// Plugins can use this for their own local caching needs.
func (s *StorageServer) GetDatabasePath(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.DatabasePath, error) {
	pluginID := GetPluginIDFromContext(ctx)
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
func (s *StorageServer) RegisterSchema(ctx context.Context, req *pluginv1.SchemaVersion) (*pluginv1.Empty, error) {
	// Plugins manage their own database schemas.
	// This is kept for API compatibility but does nothing.
	return &pluginv1.Empty{}, nil
}

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

	// Get plugin's local database file size
	dbPath := filepath.Join(s.baseDir, pluginID, "cache.db")
	var fileSize int64
	if info, err := os.Stat(dbPath); err == nil {
		fileSize = info.Size()
	}

	// Get key count
	keyCount, err := s.querier.PluginKVCount(ctx, pluginID)
	if err != nil {
		keyCount = 0
	}

	return &pluginv1.DatabaseStats{
		SizeBytes:  kvSize + fileSize,
		QuotaBytes: s.defaultQuota,
		TableCount: int32(keyCount), // Repurposing as key count
	}, nil
}

// DeletePluginStorage removes all storage for a plugin.
// Called when a plugin is uninstalled.
func (s *StorageServer) DeletePluginStorage(ctx context.Context, pluginID string) error {
	// Delete KV entries
	err := s.querier.PluginKVDeleteByPlugin(ctx, pluginID)
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
func (s *StorageServer) CleanupExpiredEntries(ctx context.Context) error {
	err := s.querier.PluginKVDeleteExpired(ctx)
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

// GetPluginIDFromContext extracts the plugin ID from the context.
func GetPluginIDFromContext(ctx context.Context) string {
	if v := ctx.Value(pluginIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetDB returns the database connection for advanced operations.
func (s *StorageServer) GetDB() *sql.DB {
	return s.db
}

// GetDBType returns the database type.
func (s *StorageServer) GetDBType() string {
	return s.dbType
}

// GetBaseDir returns the base directory for plugin data.
func (s *StorageServer) GetBaseDir() string {
	return s.baseDir
}
