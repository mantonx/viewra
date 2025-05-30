package mediaassetmodule

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/config"
)

// PathUtil provides utilities for managing asset file paths
type PathUtil struct {
	rootPath string
}

// NewPathUtil creates a new path utility instance
func NewPathUtil(rootPath string) *PathUtil {
	return &PathUtil{
		rootPath: rootPath,
	}
}

// GetDefaultPathUtil returns a default path utility with standard root path
func GetDefaultPathUtil() *PathUtil {
	// Use the configuration system to get the data directory
	// This respects the VIEWRA_DATA_DIR environment variable
	dataDir := config.GetDataDir()
	artworkPath := filepath.Join(dataDir, "artwork")
	return NewPathUtil(artworkPath)
}

// GetRootPath returns the root path for asset storage
func (pu *PathUtil) GetRootPath() string {
	return pu.rootPath
}

// BuildAssetPath builds the full path for an asset
// Structure: /viewra-data/artwork/{type}/{category}/{hashPrefix}/{hash}.{ext}
func (pu *PathUtil) BuildAssetPath(assetType AssetType, category AssetCategory, hash, mimeType string) string {
	ext := pu.getExtensionFromMimeType(mimeType)
	hashPrefix := pu.getHashPrefix(hash)
	
	relativePath := filepath.Join(
		string(assetType),     // music, movie, tv, people, meta
		string(category),      // album, poster, show, actor, studio, etc.
		hashPrefix,            // first 2 chars of hash
		fmt.Sprintf("%s%s", hash, ext), // full hash + extension
	)
	
	return filepath.Join(pu.rootPath, relativePath)
}

// BuildRelativePath builds the relative path for an asset (without root path)
// Structure: {type}/{category}/{hashPrefix}/{hash}.{ext}
func (pu *PathUtil) BuildRelativePath(assetType AssetType, category AssetCategory, hash, mimeType string) string {
	ext := pu.getExtensionFromMimeType(mimeType)
	hashPrefix := pu.getHashPrefix(hash)
	
	return filepath.Join(
		string(assetType),     // music, movie, tv, people, meta
		string(category),      // album, poster, show, actor, studio, etc.
		hashPrefix,            // first 2 chars of hash
		fmt.Sprintf("%s%s", hash, ext), // full hash + extension
	)
}

// GetFullPath converts a relative path to a full path
func (pu *PathUtil) GetFullPath(relativePath string) string {
	return filepath.Join(pu.rootPath, relativePath)
}

// EnsurePath creates the directory structure for an asset
func (pu *PathUtil) EnsurePath(assetType AssetType, category AssetCategory, hashPrefix string) error {
	dirPath := filepath.Join(
		pu.rootPath,
		string(assetType),
		string(category),
		hashPrefix,
	)
	
	return os.MkdirAll(dirPath, 0755)
}

// EnsurePathForAsset creates the directory structure for a specific asset
func (pu *PathUtil) EnsurePathForAsset(assetType AssetType, category AssetCategory, hash string) error {
	hashPrefix := pu.getHashPrefix(hash)
	return pu.EnsurePath(assetType, category, hashPrefix)
}

// getHashPrefix returns the first 2 characters of a hash for directory organization
func (pu *PathUtil) getHashPrefix(hash string) string {
	if len(hash) < 2 {
		return "00"
	}
	return strings.ToLower(hash[:2])
}

// getExtensionFromMimeType returns the appropriate file extension for a MIME type
func (pu *PathUtil) getExtensionFromMimeType(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	case "image/svg+xml":
		return ".svg"
	case "text/plain":
		return ".txt"
	case "application/x-subrip", "text/srt":
		return ".srt"
	case "text/vtt":
		return ".vtt"
	default:
		// Default to .jpg for unknown image types
		if strings.HasPrefix(mimeType, "image/") {
			return ".jpg"
		}
		// Default to .txt for text types
		if strings.HasPrefix(mimeType, "text/") {
			return ".txt"
		}
		return ".bin" // Generic binary extension
	}
}

// GetTypeAndCategoryFromPath extracts type and category from a relative path
func (pu *PathUtil) GetTypeAndCategoryFromPath(relativePath string) (AssetType, AssetCategory, error) {
	parts := strings.Split(filepath.ToSlash(relativePath), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid path format: %s", relativePath)
	}
	
	assetType := AssetType(parts[0])
	category := AssetCategory(parts[1])
	
	return assetType, category, nil
}

// CleanupEmptyDirectories removes empty directories up the path hierarchy
func (pu *PathUtil) CleanupEmptyDirectories(relativePath string) error {
	fullPath := pu.GetFullPath(relativePath)
	dir := filepath.Dir(fullPath)
	
	// Walk up the directory tree and remove empty directories
	for {
		// Don't try to remove the root asset directory
		if dir == pu.rootPath || dir == "." || dir == "/" {
			break
		}
		
		// Check if directory is empty
		entries, err := os.ReadDir(dir)
		if err != nil {
			// Directory doesn't exist or can't be read, continue up
			dir = filepath.Dir(dir)
			continue
		}
		
		// If directory is empty, remove it
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil {
				// If we can't remove it, stop trying
				break
			}
			fmt.Printf("INFO: Removing empty directory: %s\n", dir)
		} else {
			// Directory is not empty, stop
			break
		}
		
		// Move up one level
		dir = filepath.Dir(dir)
	}
	
	return nil
}

// ListAssetDirectories returns all asset directories for a given type and category
func (pu *PathUtil) ListAssetDirectories(assetType AssetType, category AssetCategory) ([]string, error) {
	basePath := filepath.Join(pu.rootPath, string(assetType), string(category))
	
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return []string{}, nil
	}
	
	var directories []string
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		}
	}
	
	return directories, nil
}

// ValidatePathStructure checks if a relative path follows the expected structure
func (pu *PathUtil) ValidatePathStructure(relativePath string) error {
	parts := strings.Split(filepath.ToSlash(relativePath), "/")
	
	// Expected structure: {type}/{category}/{hashPrefix}/{hash}.{ext}
	if len(parts) != 4 {
		return fmt.Errorf("invalid path structure: expected 4 parts, got %d in %s", len(parts), relativePath)
	}
	
	// Validate type
	assetType := AssetType(parts[0])
	if !pu.isValidAssetType(assetType) {
		return fmt.Errorf("invalid asset type: %s", parts[0])
	}
	
	// Validate category
	category := AssetCategory(parts[1])
	if !pu.isValidCategory(assetType, category) {
		return fmt.Errorf("invalid category %s for type %s", parts[1], parts[0])
	}
	
	// Validate hash prefix (should be 2 lowercase hex chars)
	hashPrefix := parts[2]
	if len(hashPrefix) != 2 {
		return fmt.Errorf("invalid hash prefix: expected 2 characters, got %s", hashPrefix)
	}
	
	// Validate filename has an extension
	filename := parts[3]
	if !strings.Contains(filename, ".") {
		return fmt.Errorf("filename missing extension: %s", filename)
	}
	
	return nil
}

// isValidAssetType checks if the asset type is valid
func (pu *PathUtil) isValidAssetType(assetType AssetType) bool {
	validTypes := []AssetType{
		AssetTypeMusic,
		AssetTypeMovie,
		AssetTypeTV,
		AssetTypePeople,
		AssetTypeMeta,
	}
	
	for _, validType := range validTypes {
		if assetType == validType {
			return true
		}
	}
	return false
}

// isValidCategory checks if the category is valid for the given type
func (pu *PathUtil) isValidCategory(assetType AssetType, category AssetCategory) bool {
	validCategories := map[AssetType][]AssetCategory{
		AssetTypeMusic: {
			CategoryAlbum, CategoryArtist, CategoryTrack, CategoryLabel, CategoryGenre,
		},
		AssetTypeMovie: {
			CategoryPoster, CategoryBackdrop, CategoryLogo, CategoryCollection,
		},
		AssetTypeTV: {
			CategoryShow, CategorySeason, CategoryEpisode, CategoryBackdrop,
		},
		AssetTypePeople: {
			CategoryActor, CategoryDirector, CategoryCrew,
		},
		AssetTypeMeta: {
			CategoryStudio, CategoryNetwork, CategoryRating,
		},
	}
	
	categories, exists := validCategories[assetType]
	if !exists {
		return false
	}
	
	for _, validCategory := range categories {
		if category == validCategory {
			return true
		}
	}
	return false
} 