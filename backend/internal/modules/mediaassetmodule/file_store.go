package mediaassetmodule

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileStore handles file system operations for media assets
type FileStore struct {
	pathUtil *PathUtil
	mutex    sync.RWMutex
}

// NewFileStore creates a new file store instance
func NewFileStore(pathUtil *PathUtil) *FileStore {
	return &FileStore{
		pathUtil: pathUtil,
	}
}

// GetDefaultFileStore returns a default file store with standard path
func GetDefaultFileStore() *FileStore {
	return NewFileStore(GetDefaultPathUtil())
}

// SaveAsset saves asset data to the filesystem
// Returns the relative path where the asset was saved
func (fs *FileStore) SaveAsset(assetType AssetType, category AssetCategory, hash, mimeType string, data []byte) (string, error) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	// Build paths
	relativePath := fs.pathUtil.BuildRelativePath(assetType, category, hash, mimeType)
	fullPath := fs.pathUtil.GetFullPath(relativePath)

	// Ensure directory exists
	if err := fs.pathUtil.EnsurePathForAsset(assetType, category, hash); err != nil {
		return "", fmt.Errorf("failed to create directory structure: %w", err)
	}

	// Check if file already exists
	if _, err := os.Stat(fullPath); err == nil {
		// File already exists, verify it matches
		existingData, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to read existing file: %w", err)
		}
		
		if len(existingData) == len(data) {
			// Files are the same size, assume they're identical
			return relativePath, nil
		}
		
		// Files differ, this shouldn't happen with proper hashing
		return "", fmt.Errorf("hash collision detected: file exists but content differs")
	}

	// Create temporary file first for atomic write
	tempPath := fullPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Write data to temporary file
	_, err = tempFile.Write(data)
	closeErr := tempFile.Close()
	
	if err != nil {
		os.Remove(tempPath) // Clean up on error
		return "", fmt.Errorf("failed to write data: %w", err)
	}
	
	if closeErr != nil {
		os.Remove(tempPath) // Clean up on error
		return "", fmt.Errorf("failed to close temporary file: %w", closeErr)
	}

	// Atomically move temporary file to final location
	if err := os.Rename(tempPath, fullPath); err != nil {
		os.Remove(tempPath) // Clean up on error
		return "", fmt.Errorf("failed to move file to final location: %w", err)
	}

	return relativePath, nil
}

// GetAssetData retrieves asset data from the filesystem
func (fs *FileStore) GetAssetData(relativePath string) ([]byte, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	fullPath := fs.pathUtil.GetFullPath(relativePath)
	
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("asset file not found: %s", relativePath)
		}
		return nil, fmt.Errorf("failed to read asset file: %w", err)
	}

	return data, nil
}

// RemoveAsset removes an asset from the filesystem
func (fs *FileStore) RemoveAsset(relativePath string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	fullPath := fs.pathUtil.GetFullPath(relativePath)
	
	// Remove the file
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, consider it already removed
			return nil
		}
		return fmt.Errorf("failed to remove asset file: %w", err)
	}

	// Clean up empty directories
	if err := fs.pathUtil.CleanupEmptyDirectories(relativePath); err != nil {
		// Log warning but don't fail the operation
		fmt.Printf("WARNING: Failed to cleanup empty directories: %v\n", err)
	}

	return nil
}

// AssetExists checks if an asset file exists
func (fs *FileStore) AssetExists(relativePath string) bool {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	fullPath := fs.pathUtil.GetFullPath(relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// GetAssetInfo returns information about an asset file
func (fs *FileStore) GetAssetInfo(relativePath string) (os.FileInfo, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	fullPath := fs.pathUtil.GetFullPath(relativePath)
	return os.Stat(fullPath)
}

// UpdateAsset updates an existing asset with new data
func (fs *FileStore) UpdateAsset(relativePath string, data []byte) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	fullPath := fs.pathUtil.GetFullPath(relativePath)

	// Create temporary file for atomic update
	tempPath := fullPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	// Write new data
	_, err = tempFile.Write(data)
	closeErr := tempFile.Close()
	
	if err != nil {
		os.Remove(tempPath) // Clean up on error
		return fmt.Errorf("failed to write data: %w", err)
	}
	
	if closeErr != nil {
		os.Remove(tempPath) // Clean up on error
		return fmt.Errorf("failed to close temporary file: %w", closeErr)
	}

	// Atomically replace the original file
	if err := os.Rename(tempPath, fullPath); err != nil {
		os.Remove(tempPath) // Clean up on error
		return fmt.Errorf("failed to replace file: %w", err)
	}

	return nil
}

// ValidateAssetIntegrity validates that an asset file matches expected properties
func (fs *FileStore) ValidateAssetIntegrity(relativePath, expectedHash string, expectedSize int64) error {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	fullPath := fs.pathUtil.GetFullPath(relativePath)
	
	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("asset file missing: %s", relativePath)
		}
		return fmt.Errorf("failed to stat asset file: %w", err)
	}

	// Check file size
	if info.Size() != expectedSize {
		return fmt.Errorf("asset file size mismatch: expected %d, got %d", expectedSize, info.Size())
	}

	// Check hash by calculating it from the file
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read asset file for hash validation: %w", err)
	}

	actualHash := CalculateDataHash(data)
	if actualHash != expectedHash {
		return fmt.Errorf("asset file hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// CopyAsset copies an asset from one location to another
func (fs *FileStore) CopyAsset(sourcePath, destPath string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	sourceFullPath := fs.pathUtil.GetFullPath(sourcePath)
	destFullPath := fs.pathUtil.GetFullPath(destPath)

	// Open source file
	sourceFile, err := os.Open(sourceFullPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	destDir := filepath.Dir(destFullPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create destination file
	destFile, err := os.Create(destFullPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy data
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		os.Remove(destFullPath) // Clean up on error
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

// ListAssets returns all assets for a given type and category
func (fs *FileStore) ListAssets(assetType AssetType, category AssetCategory) ([]string, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	// Get all hash prefix directories
	hashDirs, err := fs.pathUtil.ListAssetDirectories(assetType, category)
	if err != nil {
		return nil, fmt.Errorf("failed to list asset directories: %w", err)
	}

	var assets []string
	basePath := filepath.Join(fs.pathUtil.GetRootPath(), string(assetType), string(category))

	for _, hashDir := range hashDirs {
		hashDirPath := filepath.Join(basePath, hashDir)
		
		entries, err := os.ReadDir(hashDirPath)
		if err != nil {
			continue // Skip directories we can't read
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				relativePath := filepath.Join(string(assetType), string(category), hashDir, entry.Name())
				assets = append(assets, relativePath)
			}
		}
	}

	return assets, nil
}

// GetStorageStats returns storage statistics
func (fs *FileStore) GetStorageStats() (*StorageStats, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	stats := &StorageStats{
		AssetsByType: make(map[AssetType]*TypeStats),
	}

	rootPath := fs.pathUtil.GetRootPath()
	
	// Walk through all asset types
	validTypes := []AssetType{AssetTypeMusic, AssetTypeMovie, AssetTypeTV, AssetTypePeople, AssetTypeMeta}
	
	for _, assetType := range validTypes {
		typeStats := &TypeStats{
			AssetsByCategory: make(map[AssetCategory]int64),
		}
		
		typePath := filepath.Join(rootPath, string(assetType))
		if _, err := os.Stat(typePath); os.IsNotExist(err) {
			continue
		}

		// Walk through all files in this type
		err := filepath.Walk(typePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			
			if !info.IsDir() {
				typeStats.TotalFiles++
				typeStats.TotalSize += info.Size()
				stats.TotalFiles++
				stats.TotalSize += info.Size()

				// Extract category from path
				relativePath, err := filepath.Rel(typePath, path)
				if err == nil {
					parts := filepath.SplitList(relativePath)
					if len(parts) > 0 {
						category := AssetCategory(parts[0])
						typeStats.AssetsByCategory[category]++
					}
				}
			}
			return nil
		})
		
		if err != nil {
			return nil, fmt.Errorf("failed to walk type directory %s: %w", assetType, err)
		}
		
		stats.AssetsByType[assetType] = typeStats
	}

	return stats, nil
}

// StorageStats represents filesystem storage statistics
type StorageStats struct {
	TotalFiles    int64                    `json:"total_files"`
	TotalSize     int64                    `json:"total_size"`
	AssetsByType  map[AssetType]*TypeStats `json:"assets_by_type"`
}

// TypeStats represents statistics for a specific asset type
type TypeStats struct {
	TotalFiles       int64                      `json:"total_files"`
	TotalSize        int64                      `json:"total_size"`
	AssetsByCategory map[AssetCategory]int64    `json:"assets_by_category"`
}

// AssetFileInfo contains information about an asset file
type AssetFileInfo struct {
	Path         string    `json:"path"`
	RelativePath string    `json:"relative_path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Exists       bool      `json:"exists"`
}

// Convenience functions using the default file store

// SaveAsset saves asset data to disk using the default file store
func SaveAsset(category AssetCategory, hash string, mimeType string, data []byte) (string, error) {
	return GetDefaultFileStore().SaveAsset(AssetTypeMusic, category, hash, mimeType, data)
}

// GetAsset retrieves asset data from disk using the default file store
func GetAsset(relativePath string) ([]byte, error) {
	return GetDefaultFileStore().GetAssetData(relativePath)
}

// RemoveAsset removes an asset file from disk using the default file store
func RemoveAsset(relativePath string) error {
	return GetDefaultFileStore().RemoveAsset(relativePath)
}

// AssetExists checks if an asset file exists on disk using the default file store
func AssetExists(relativePath string) bool {
	return GetDefaultFileStore().AssetExists(relativePath)
} 