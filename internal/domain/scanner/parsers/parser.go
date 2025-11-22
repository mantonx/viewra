package parsers

import (
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// DefaultParser implements the scanner.FilenameParser interface.
// It provides metadata extraction from movie, TV show, and music filenames.
type DefaultParser struct {
	// metadataExtractor is used to extract ID3/tag metadata from audio files
	// This is an optional dependency - if nil, music parser falls back to filename parsing only
	metadataExtractor scanner.MusicMetadataExtractor
}

// NewDefaultParser creates a new instance of the default filename parser.
// For music files, it will only use filename parsing (no ID3 tag extraction).
func NewDefaultParser() scanner.FilenameParser {
	return &DefaultParser{
		metadataExtractor: nil,
	}
}

// NewDefaultParserWithMetadata creates a parser with ID3 metadata extraction support.
// The metadataExtractor should be an instance from the infrastructure layer.
func NewDefaultParserWithMetadata(extractor scanner.MusicMetadataExtractor) scanner.FilenameParser {
	return &DefaultParser{
		metadataExtractor: extractor,
	}
}

// ParseMovie extracts movie metadata from a filename.
// It attempts to extract the title, year, resolution, and quality.
func (p *DefaultParser) ParseMovie(filename string) (*scanner.MovieInfo, error) {
	return ParseMovie(filename)
}

// ParseTVEpisode extracts TV show metadata from a filename or path.
// It supports multiple naming patterns and can extract season info from directory structure.
func (p *DefaultParser) ParseTVEpisode(filename string) (*scanner.TVEpisodeInfo, error) {
	return ParseTVEpisode(filename)
}

// ParseMusic extracts music metadata from a file path.
// It tries ID3 tags first (if extractor is available), then falls back to filename parsing.
func (p *DefaultParser) ParseMusic(path string) (*scanner.MusicInfo, error) {
	return ParseMusic(path, p.metadataExtractor)
}
