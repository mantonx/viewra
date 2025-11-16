package scanner

// MusicMetadataExtractor defines the interface for extracting rich metadata from audio files
// This allows the domain layer to depend on an abstraction rather than infrastructure details
type MusicMetadataExtractor interface {
	// ExtractMetadata reads ID3/Vorbis/APE tags from an audio file
	// Returns error if file cannot be read or has no parseable metadata
	ExtractMetadata(filePath string) (*MusicInfo, error)
}
