package filesystem_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// ExampleWalker_Walk demonstrates how to use the walker and filter together
// to discover media files in a directory structure.
func ExampleWalker_Walk() {
	// Create a temporary test directory structure
	tmpDir := createExampleLibrary()
	defer os.RemoveAll(tmpDir)

	// Create walker and filter instances
	walker := filesystem.NewWalker()
	filter := filesystem.NewFilter()

	// Track discovered files
	var (
		videoFiles []string
		audioFiles []string
		skipped    int
	)

	// Walk the directory
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := walker.Walk(ctx, tmpDir, func(info scanner.FileInfo) error {
		// Skip directories
		if info.IsDir {
			return nil
		}

		// Get file info for filtering
		osInfo, err := os.Stat(info.Path)
		if err != nil {
			return nil
		}

		// Check if file should be processed
		if !filter.ShouldProcess(info.Path, osInfo) {
			skipped++
			return nil
		}

		// Classify by media type
		mediaType := filter.GetMediaType(info.Extension)
		relPath, _ := filepath.Rel(tmpDir, info.Path)

		switch mediaType {
		case scanner.MediaTypeMovie:
			videoFiles = append(videoFiles, relPath)
		case scanner.MediaTypeTrack:
			audioFiles = append(audioFiles, relPath)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print results
	fmt.Println("=== Media Library Scan Results ===")
	fmt.Printf("Video files found: %d\n", len(videoFiles))
	for _, file := range videoFiles {
		fmt.Printf("  - %s\n", file)
	}
	fmt.Printf("\nAudio files found: %d\n", len(audioFiles))
	for _, file := range audioFiles {
		fmt.Printf("  - %s\n", file)
	}
	fmt.Printf("\nFiles skipped (non-media): %d\n", skipped)

	// Output:
	// === Media Library Scan Results ===
	// Video files found: 3
	//   - Movies/Inception (2010)/Inception (2010).mkv
	//   - TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E01.mkv
	//   - TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E02.mp4
	//
	// Audio files found: 2
	//   - Music/Artist Name/Album Name/01 - Track One.mp3
	//   - Music/Artist Name/Album Name/02 - Track Two.flac
	//
	// Files skipped (non-media): 7
}

// Example_mediaTypeDetection demonstrates media type detection
func Example_mediaTypeDetection() {
	filter := filesystem.NewFilter()

	files := []string{
		"movie.mkv",
		"video.mp4",
		"song.mp3",
		"track.flac",
		"poster.jpg",
		"subtitle.srt",
		"readme.txt",
	}

	fmt.Println("=== Media Type Detection ===")
	for _, file := range files {
		ext := filepath.Ext(file)
		mediaType := filter.GetMediaType(ext)
		isMedia := filter.IsMediaFile(file)

		fmt.Printf("%-20s -> type: %-10s media: %v\n",
			file, mediaType, isMedia)
	}

	// Output:
	// === Media Type Detection ===
	// movie.mkv            -> type: movie      media: true
	// video.mp4            -> type: movie      media: true
	// song.mp3             -> type: track      media: true
	// track.flac           -> type: track      media: true
	// poster.jpg           -> type: unknown    media: false
	// subtitle.srt         -> type: unknown    media: false
	// readme.txt           -> type: unknown    media: false
}

// Example_fileFiltering demonstrates file filtering
func Example_fileFiltering() {
	filter := filesystem.NewFilter()

	// Create mock file info
	makeFileInfo := func(name string, isDir bool) os.FileInfo {
		return mockFileInfo{
			name:    name,
			size:    1024 * 1024,
			mode:    0644,
			modTime: time.Now(),
			isDir:   isDir,
		}
	}

	testCases := []struct {
		path string
		info os.FileInfo
	}{
		{"/media/movie.mkv", makeFileInfo("movie.mkv", false)},
		{"/media/song.mp3", makeFileInfo("song.mp3", false)},
		{"/media/poster.jpg", makeFileInfo("poster.jpg", false)},
		{"/media/movie.nfo", makeFileInfo("movie.nfo", false)},
		{"/media/subtitle.srt", makeFileInfo("subtitle.srt", false)},
		{"/media/.hidden.mkv", makeFileInfo(".hidden.mkv", false)},
		{"/media/Thumbs.db", makeFileInfo("Thumbs.db", false)},
		{"/media/Season 01", makeFileInfo("Season 01", true)},
	}

	fmt.Println("=== File Filtering Results ===")
	for _, tc := range testCases {
		shouldProcess := filter.ShouldProcess(tc.path, tc.info)
		status := "SKIP"
		if shouldProcess {
			status = "PROCESS"
		}
		fmt.Printf("%-30s -> %s\n", filepath.Base(tc.path), status)
	}

	// Output:
	// === File Filtering Results ===
	// movie.mkv                      -> PROCESS
	// song.mp3                       -> PROCESS
	// poster.jpg                     -> SKIP
	// movie.nfo                      -> SKIP
	// subtitle.srt                   -> SKIP
	// .hidden.mkv                    -> SKIP
	// Thumbs.db                      -> SKIP
	// Season 01                      -> SKIP
}

// Example_contextCancellation demonstrates context cancellation
func Example_contextCancellation() {
	tmpDir := createExampleLibrary()
	defer os.RemoveAll(tmpDir)

	walker := filesystem.NewWalker()
	filter := filesystem.NewFilter()

	// Create a context that cancels after finding 2 files
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filesFound := 0
	maxFiles := 2

	err := walker.Walk(ctx, tmpDir, func(info scanner.FileInfo) error {
		if info.IsDir {
			return nil
		}

		osInfo, err := os.Stat(info.Path)
		if err != nil {
			return nil
		}

		if filter.ShouldProcess(info.Path, osInfo) {
			filesFound++
			relPath, _ := filepath.Rel(tmpDir, info.Path)
			fmt.Printf("Found file %d: %s\n", filesFound, relPath)

			if filesFound >= maxFiles {
				fmt.Println("Reached limit, cancelling scan...")
				cancel()
			}
		}

		return nil
	})

	if err == context.Canceled {
		fmt.Printf("Scan cancelled after finding %d files\n", filesFound)
	}

	// Output:
	// Found file 1: Movies/Inception (2010)/Inception (2010).mkv
	// Found file 2: Music/Artist Name/Album Name/01 - Track One.mp3
	// Reached limit, cancelling scan...
	// Scan cancelled after finding 2 files
}

// Helper functions

type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

// createExampleLibrary creates a realistic test library structure
func createExampleLibrary() string {
	tmpDir, err := os.MkdirTemp("", "viewra-example-*")
	if err != nil {
		panic(err)
	}

	files := []string{
		// Movies
		"Movies/Inception (2010)/Inception (2010).mkv",
		"Movies/Inception (2010)/poster.jpg",
		"Movies/Inception (2010)/Inception (2010).nfo",

		// TV Shows
		"TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E01.mkv",
		"TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E02.mp4",
		"TV Shows/Breaking Bad/Season 01/Breaking Bad - S01E01.srt",
		"TV Shows/Breaking Bad/poster.jpg",

		// Music
		"Music/Artist Name/Album Name/01 - Track One.mp3",
		"Music/Artist Name/Album Name/02 - Track Two.flac",
		"Music/Artist Name/Album Name/cover.jpg",

		// System files
		".DS_Store",
		"Thumbs.db",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		dir := filepath.Dir(fullPath)

		if err := os.MkdirAll(dir, 0755); err != nil {
			panic(err)
		}

		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			panic(err)
		}
	}

	return tmpDir
}
