package scanner

import (
	"testing"
	"time"
)

func TestProgressGetPercentage(t *testing.T) {
	tests := []struct {
		name           string
		filesFound     int64
		filesProcessed int64
		expected       float64
	}{
		{"zero files", 0, 0, 0.0},
		{"no progress", 100, 0, 0.0},
		{"half progress", 100, 50, 50.0},
		{"full progress", 100, 100, 100.0},
		{"over 100% capped", 100, 150, 100.0},
		{"single file complete", 1, 1, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Progress{
				FilesFound:     tt.filesFound,
				FilesProcessed: tt.filesProcessed,
			}
			result := p.GetPercentage()
			if result != tt.expected {
				t.Errorf("Expected %.1f%%, got %.1f%%", tt.expected, result)
			}
		})
	}
}

func TestScanStatusConstants(t *testing.T) {
	statuses := []struct {
		status   ScanStatus
		expected string
	}{
		{ScanStatusPending, "pending"},
		{ScanStatusRunning, "running"},
		{ScanStatusPaused, "paused"},
		{ScanStatusCompleted, "completed"},
		{ScanStatusFailed, "failed"},
	}

	for _, tt := range statuses {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestMediaTypeConstants(t *testing.T) {
	types := []struct {
		mediaType MediaType
		expected  string
	}{
		{MediaTypeMovie, "movie"},
		{MediaTypeEpisode, "episode"},
		{MediaTypeTrack, "track"},
		{MediaTypeUnknown, "unknown"},
	}

	for _, tt := range types {
		t.Run(string(tt.mediaType), func(t *testing.T) {
			if string(tt.mediaType) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.mediaType)
			}
		})
	}
}

func TestScanJobCreation(t *testing.T) {
	now := time.Now()
	job := ScanJob{
		ID:             1,
		LibraryID:      100,
		Status:         ScanStatusRunning,
		Progress:       50.5,
		FilesFound:     1000,
		FilesProcessed: 505,
		BytesProcessed: 1024 * 1024 * 100, // 100MB
		ErrorCount:     5,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if job.ID != 1 {
		t.Errorf("Expected ID=1, got %d", job.ID)
	}
	if job.Status != ScanStatusRunning {
		t.Errorf("Expected status=running, got %s", job.Status)
	}
	if job.Progress != 50.5 {
		t.Errorf("Expected progress=50.5, got %.1f", job.Progress)
	}
}

func TestFileInfoCreation(t *testing.T) {
	now := time.Now()
	info := FileInfo{
		Path:      "/path/to/movie.mkv",
		Size:      1024 * 1024 * 500, // 500MB
		ModTime:   now,
		Extension: ".mkv",
		IsDir:     false,
	}

	if info.Path != "/path/to/movie.mkv" {
		t.Errorf("Expected path=/path/to/movie.mkv, got %s", info.Path)
	}
	if info.Extension != ".mkv" {
		t.Errorf("Expected extension=.mkv, got %s", info.Extension)
	}
	if info.IsDir {
		t.Error("Expected IsDir=false")
	}
}

func TestScanResultCreation(t *testing.T) {
	year := 2020
	season := 1
	episode := 5
	track := 3

	result := ScanResult{
		FilePath:       "/path/to/show.mkv",
		MediaType:      MediaTypeEpisode,
		Title:          "Episode Title",
		Year:           &year,
		SeasonNumber:   &season,
		EpisodeNumber:  &episode,
		Artist:         "",
		Album:          "",
		TrackNumber:    &track,
		Duration:       3600,
		Hash:           "abc123",
		Error:          nil,
		BytesProcessed: 1024,
	}

	if result.MediaType != MediaTypeEpisode {
		t.Errorf("Expected MediaType=episode, got %s", result.MediaType)
	}
	if result.Year == nil || *result.Year != 2020 {
		t.Errorf("Expected Year=2020, got %v", result.Year)
	}
	if result.SeasonNumber == nil || *result.SeasonNumber != 1 {
		t.Errorf("Expected SeasonNumber=1, got %v", result.SeasonNumber)
	}
	if result.EpisodeNumber == nil || *result.EpisodeNumber != 5 {
		t.Errorf("Expected EpisodeNumber=5, got %v", result.EpisodeNumber)
	}
}

func TestMovieInfoCreation(t *testing.T) {
	info := MovieInfo{
		Title:      "Inception",
		Year:       2010,
		Resolution: "1080p",
		Quality:    "BluRay",
	}

	if info.Title != "Inception" {
		t.Errorf("Expected Title=Inception, got %s", info.Title)
	}
	if info.Year != 2010 {
		t.Errorf("Expected Year=2010, got %d", info.Year)
	}
	if info.Resolution != "1080p" {
		t.Errorf("Expected Resolution=1080p, got %s", info.Resolution)
	}
	if info.Quality != "BluRay" {
		t.Errorf("Expected Quality=BluRay, got %s", info.Quality)
	}
}

func TestTVEpisodeInfoCreation(t *testing.T) {
	info := TVEpisodeInfo{
		ShowName:     "Breaking Bad",
		Season:       1,
		Episode:      1,
		EpisodeEnd:   0,
		EpisodeTitle: "Pilot",
		Year:         2008,
	}

	if info.ShowName != "Breaking Bad" {
		t.Errorf("Expected ShowName=Breaking Bad, got %s", info.ShowName)
	}
	if info.Season != 1 {
		t.Errorf("Expected Season=1, got %d", info.Season)
	}
	if info.Episode != 1 {
		t.Errorf("Expected Episode=1, got %d", info.Episode)
	}
	if info.EpisodeTitle != "Pilot" {
		t.Errorf("Expected EpisodeTitle=Pilot, got %s", info.EpisodeTitle)
	}
}

func TestMusicInfoCreation(t *testing.T) {
	info := MusicInfo{
		Title:        "Bohemian Rhapsody",
		Artist:       "Queen",
		Album:        "A Night at the Opera",
		AlbumArtist:  "Queen",
		TrackNumber:  11,
		Year:         1975,
		Genre:        "Rock",
		Duration:     354,
	}

	if info.Title != "Bohemian Rhapsody" {
		t.Errorf("Expected Title=Bohemian Rhapsody, got %s", info.Title)
	}
	if info.Artist != "Queen" {
		t.Errorf("Expected Artist=Queen, got %s", info.Artist)
	}
	if info.Album != "A Night at the Opera" {
		t.Errorf("Expected Album=A Night at the Opera, got %s", info.Album)
	}
	if info.TrackNumber != 11 {
		t.Errorf("Expected TrackNumber=11, got %d", info.TrackNumber)
	}
	if info.Year != 1975 {
		t.Errorf("Expected Year=1975, got %d", info.Year)
	}
	if info.Duration != 354 {
		t.Errorf("Expected Duration=354, got %d", info.Duration)
	}
}

func TestProgressInitialization(t *testing.T) {
	start := time.Now()
	p := Progress{
		FilesFound:     100,
		FilesProcessed: 50,
		BytesProcessed: 1024 * 1024 * 50,
		ErrorCount:     2,
		StartTime:      start,
		LastUpdate:     time.Now(),
	}

	if p.FilesFound != 100 {
		t.Errorf("Expected FilesFound=100, got %d", p.FilesFound)
	}
	if p.FilesProcessed != 50 {
		t.Errorf("Expected FilesProcessed=50, got %d", p.FilesProcessed)
	}
	if p.ErrorCount != 2 {
		t.Errorf("Expected ErrorCount=2, got %d", p.ErrorCount)
	}
	if p.StartTime.After(time.Now()) {
		t.Error("StartTime should be in the past")
	}
}

func TestProgressGetPercentageWithEstimate(t *testing.T) {
	tests := []struct {
		name           string
		filesFound     int64
		filesProcessed int64
		estimatedTotal int64
		discoveryDone  bool
		expected       float64
	}{
		{
			name:           "discovery not done, uses estimate",
			filesFound:     50,
			filesProcessed: 25,
			estimatedTotal: 100,
			discoveryDone:  false,
			expected:       25.0, // 25/100
		},
		{
			name:           "discovery done, uses found",
			filesFound:     100,
			filesProcessed: 50,
			estimatedTotal: 200, // ignored
			discoveryDone:  true,
			expected:       50.0, // 50/100
		},
		{
			name:           "estimate less than found, uses found",
			filesFound:     100,
			filesProcessed: 50,
			estimatedTotal: 50, // less than found
			discoveryDone:  false,
			expected:       50.0, // 50/100
		},
		{
			name:           "no estimate, uses found",
			filesFound:     100,
			filesProcessed: 50,
			estimatedTotal: 0,
			discoveryDone:  false,
			expected:       50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Progress{
				FilesFound:     tt.filesFound,
				FilesProcessed: tt.filesProcessed,
				EstimatedTotal: tt.estimatedTotal,
				DiscoveryDone:  tt.discoveryDone,
			}
			result := p.GetPercentage()
			if result != tt.expected {
				t.Errorf("GetPercentage() = %.1f, want %.1f", result, tt.expected)
			}
		})
	}
}

func TestScanPhaseConstants(t *testing.T) {
	phases := []struct {
		phase    ScanPhase
		expected string
	}{
		{ScanPhaseDiscovering, "discovering"},
		{ScanPhaseProcessing, "processing"},
		{ScanPhaseCompleted, "completed"},
	}

	for _, tt := range phases {
		t.Run(string(tt.phase), func(t *testing.T) {
			if string(tt.phase) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.phase)
			}
		})
	}
}

func TestHashingStrategyConstants(t *testing.T) {
	strategies := []struct {
		strategy HashingStrategy
		expected string
	}{
		{HashingStrategyAlways, "always"},
		{HashingStrategyOnConflict, "on_conflict"},
		{HashingStrategyDisabled, "disabled"},
	}

	for _, tt := range strategies {
		t.Run(string(tt.strategy), func(t *testing.T) {
			if string(tt.strategy) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.strategy)
			}
		})
	}
}

func TestFileCacheEntryIsUnchanged(t *testing.T) {
	now := time.Now()
	entry := FileCacheEntry{
		Path:    "/path/to/file.mkv",
		Size:    1000,
		ModTime: now,
		Hash:    "abc123",
	}

	tests := []struct {
		name     string
		modTime  time.Time
		size     int64
		expected bool
	}{
		{"unchanged", now, 1000, true},
		{"different size", now, 2000, false},
		{"different time", now.Add(time.Hour), 1000, false},
		{"both different", now.Add(time.Hour), 2000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := entry.IsUnchanged(tt.modTime, tt.size)
			if result != tt.expected {
				t.Errorf("IsUnchanged(%v, %d) = %v, want %v", tt.modTime, tt.size, result, tt.expected)
			}
		})
	}
}

func TestScanJobWithTargetPaths(t *testing.T) {
	job := ScanJob{
		ID:          1,
		LibraryID:   100,
		Status:      ScanStatusRunning,
		TargetPaths: []string{"/movies/new_movie", "/movies/another_movie"},
	}

	if len(job.TargetPaths) != 2 {
		t.Errorf("Expected 2 target paths, got %d", len(job.TargetPaths))
	}
	if job.TargetPaths[0] != "/movies/new_movie" {
		t.Errorf("Expected first target path, got %s", job.TargetPaths[0])
	}
}

func TestAudioTrackInfoCreation(t *testing.T) {
	track := AudioTrackInfo{
		StreamIndex:   0,
		Codec:         "aac",
		CodecProfile:  "LC",
		Channels:      6,
		ChannelLayout: "5.1",
		SampleRate:    48000,
		BitRate:       384000,
		Language:      "eng",
		Title:         "English 5.1",
		IsDefault:     true,
		IsCommentary:  false,
		IsDescriptive: false,
	}

	if track.Channels != 6 {
		t.Errorf("Expected Channels=6, got %d", track.Channels)
	}
	if !track.IsDefault {
		t.Error("Expected IsDefault=true")
	}
}

func TestSubtitleTrackInfoCreation(t *testing.T) {
	track := SubtitleTrackInfo{
		StreamIndex:  0,
		Codec:        "srt",
		Language:     "eng",
		Title:        "English",
		IsDefault:    false,
		IsForced:     false,
		IsSDH:        true,
		IsCommentary: false,
		IsBitmap:     false,
	}

	if !track.IsSDH {
		t.Error("Expected IsSDH=true")
	}
	if track.IsBitmap {
		t.Error("Expected IsBitmap=false")
	}
}
