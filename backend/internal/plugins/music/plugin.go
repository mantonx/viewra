package music

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/plugins"
)

// MusicPlugin implements the CoreMediaPlugin interface for music files
type MusicPlugin struct {
	name               string
	supportedExts      []string
	priority           int
	enabled            bool
}

// NewMusicPlugin creates a new music plugin instance
func NewMusicPlugin() plugins.CoreMediaPlugin {
	return &MusicPlugin{
		name:     "MusicCorePlugin",
		priority: 100,
		enabled:  true,
		supportedExts: []string{
			".mp3", ".flac", ".wav", ".m4a", ".aac", 
			".ogg", ".wma", ".opus", ".aiff", ".ape", ".wv",
		},
	}
}

// GetName returns the plugin name
func (p *MusicPlugin) GetName() string {
	return p.name
}

// GetVersion returns the plugin version
func (p *MusicPlugin) GetVersion() string {
	return "1.0.0"
}

// GetDescription returns the plugin description
func (p *MusicPlugin) GetDescription() string {
	return "Core music file processor with metadata extraction and artwork support"
}

// IsEnabled returns whether the plugin is enabled
func (p *MusicPlugin) IsEnabled() bool {
	return p.enabled
}

// Enable enables the plugin
func (p *MusicPlugin) Enable() error {
	p.enabled = true
	return nil
}

// Disable disables the plugin
func (p *MusicPlugin) Disable() error {
	p.enabled = false
	return nil
}

// GetMediaType returns the media type this plugin handles
func (p *MusicPlugin) GetMediaType() string {
	return "music"
}

// GetSupportedExtensions returns the file extensions this plugin supports
func (p *MusicPlugin) GetSupportedExtensions() []string {
	return p.supportedExts
}

// GetPriority returns the plugin priority
func (p *MusicPlugin) GetPriority() int {
	return p.priority
}

// Initialize performs any setup needed for the plugin
func (p *MusicPlugin) Initialize() error {
	fmt.Printf("DEBUG: Initializing Music Core Plugin v%s\n", p.GetVersion())
	fmt.Printf("DEBUG: Music plugin supports %d file types: %v\n", len(p.supportedExts), p.supportedExts)
	return nil
}

// Shutdown performs any cleanup needed when the plugin is disabled
func (p *MusicPlugin) Shutdown() error {
	fmt.Printf("DEBUG: Shutting down Music Core Plugin\n")
	p.enabled = false
	return nil
}

// Match determines if this plugin can handle the given file (legacy compatibility)
func (p *MusicPlugin) Match(path string, info os.FileInfo) bool {
	if !p.enabled {
		return false
	}
	
	// Skip directories
	if info.IsDir() {
		return false
	}
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	for _, supportedExt := range p.supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	
	return false
}

// HandleMediaFile processes a media file (legacy compatibility)
func (p *MusicPlugin) HandleMediaFile(path string, info os.FileInfo) error {
	// This is a legacy method - for now just return success
	// The new architecture uses HandleFile which returns MediaItem + MediaAssets
	if !p.enabled {
		return fmt.Errorf("music plugin is disabled")
	}
	
	// Basic validation
	if !p.Match(path, info) {
		return fmt.Errorf("unsupported file type")
	}
	
	return nil
}

// HandleFile processes a music file and returns MediaItem + assets
func (p *MusicPlugin) HandleFile(path string, info os.FileInfo, ctx plugins.MediaContext) (*plugins.MediaItem, []plugins.MediaAsset, error) {
	if !p.enabled {
		return nil, nil, fmt.Errorf("music plugin is disabled")
	}

	// Check if we support this file extension
	ext := strings.ToLower(filepath.Ext(path))
	supported := false
	for _, supportedExt := range p.supportedExts {
		if ext == supportedExt {
			supported = true
			break
		}
	}
	
	if !supported {
		return nil, nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Extract metadata using the tag reader
	tagReader := NewTagReader()
	musicMeta, err := tagReader.ReadMetadata(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract music metadata: %w", err)
	}

	fmt.Printf("DEBUG: Music metadata extracted for %s: Artist=%s, Title=%s, Album=%s\n", 
		path, musicMeta.Artist, musicMeta.Title, musicMeta.Album)

	// Create MediaItem
	mediaItem := &plugins.MediaItem{
		MediaFile: ctx.MediaFile,
		Metadata:  musicMeta,
		Type:      "music",
	}

	// Create MediaAssets for artwork if present
	var assets []plugins.MediaAsset
	if musicMeta.HasArtwork && len(musicMeta.ArtworkData) > 0 && musicMeta.ArtworkExt != "" {
		artworkAsset := plugins.MediaAsset{
			Type:        "artwork",
			Data:        musicMeta.ArtworkData,
			Path:        path,
			Extension:   musicMeta.ArtworkExt,
			MediaFileID: ctx.MediaFile.ID,
			MimeType:    getMimeTypeFromExt(musicMeta.ArtworkExt),
			Size:        int64(len(musicMeta.ArtworkData)),
		}
		assets = append(assets, artworkAsset)
		
		fmt.Printf("DEBUG: Created artwork asset for %s: %s (%d bytes)\n", 
			path, musicMeta.ArtworkExt, len(musicMeta.ArtworkData))
	}

	fmt.Printf("DEBUG: Plugin %s processed %s → MediaItem + %d assets\n", 
		p.GetName(), path, len(assets))

	return mediaItem, assets, nil
}

// getMimeTypeFromExt returns the MIME type for a file extension
func getMimeTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
} 