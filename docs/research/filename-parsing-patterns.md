# Filename Parsing Patterns

Research document for implementing filename parsers for movies, TV shows, and music.

## Movie Filename Patterns

### Common Patterns

```
Inception (2010).mkv
The Matrix (1999) 1080p BluRay.mp4
Avatar.2009.1080p.BluRay.x264.mkv
The.Dark.Knight.2008.PROPER.720p.BluRay.x264-SPARKS.mkv
Interstellar (2014) [1080p] [YTS.AG].mp4
Avengers Endgame (2019) 2160p UHD BluRay.mkv
The Shawshank Redemption (1994).avi
Movie Name 2020 BluRay 1080p x264.mkv
```

### Regex Patterns

**Pattern 1: Title (Year)**
```regex
^(.+?)\s*\((\d{4})\)
```
Examples:
- `Inception (2010).mkv` → Title: "Inception", Year: 2010
- `The Matrix (1999) 1080p.mp4` → Title: "The Matrix", Year: 1999

**Pattern 2: Title.Year.Quality**
```regex
^(.+?)[.\s](\d{4})[.\s]
```
Examples:
- `Avatar.2009.1080p.mkv` → Title: "Avatar", Year: 2009
- `The.Dark.Knight.2008.720p.mkv` → Title: "The Dark Knight", Year: 2008

**Pattern 3: Title Year (no parens)**
```regex
^(.+?)\s+(\d{4})\s+
```
Examples:
- `Movie Name 2020 BluRay.mkv` → Title: "Movie Name", Year: 2020

### Metadata to Extract

- **Title** (required): Movie name
- **Year** (optional): Release year (1900-2099)
- **Resolution** (optional): 720p, 1080p, 2160p, 4K
- **Quality** (optional): BluRay, WEB-DL, HDTV, DVDRip
- **Codec** (optional): x264, x265, h264, h265, HEVC

### Cleaning Rules

1. Remove file extension
2. Replace dots/underscores with spaces
3. Remove quality tags (1080p, BluRay, etc.)
4. Remove codec tags (x264, etc.)
5. Remove group tags ([YTS], -SPARKS, etc.)
6. Trim whitespace

## TV Show Filename Patterns

### Common Patterns

```
Breaking Bad - S01E01 - Pilot.mkv
Game.of.Thrones.S08E06.1080p.WEB.H264.mkv
The.Office.US.S05E14.720p.BluRay.x264.mkv
Friends S01E01.mp4
Breaking.Bad.1x01.Pilot.mkv
Mr.Robot.S01E01E02.1080p.WEB-DL.mkv
The Mandalorian S02E08 Chapter 16.mkv
```

### Regex Patterns

**Pattern 1: SxxExx (Standard)**
```regex
[Ss](\d{1,2})[Ee](\d{1,2})(?:[Ee](\d{1,2}))?
```
Examples:
- `S01E01` → Season: 1, Episode: 1
- `s05e14` → Season: 5, Episode: 14
- `S01E01E02` → Season: 1, Episodes: 1-2

**Pattern 2: xYY (Alternate)**
```regex
(\d{1,2})x(\d{1,2})
```
Examples:
- `1x01` → Season: 1, Episode: 1
- `5x14` → Season: 5, Episode: 14

**Pattern 3: Season X Episode Y**
```regex
[Ss]eason\s*(\d{1,2})\s*[Ee]pisode\s*(\d{1,2})
```
Examples:
- `Season 1 Episode 5` → Season: 1, Episode: 5

### Metadata to Extract

- **Show Name** (required): TV show title
- **Season** (required): Season number (1-99)
- **Episode** (required): Episode number (1-999)
- **Episode Title** (optional): Episode name
- **Year** (optional): Release year
- **Resolution** (optional): Same as movies
- **Quality** (optional): Same as movies

### Cleaning Rules

1. Extract show name before season/episode marker
2. Replace dots/underscores with spaces
3. Extract episode title after episode number (if present)
4. Remove quality/codec tags
5. Handle country codes (e.g., "US", "UK")
6. Trim whitespace

## Music Filename Patterns

### ID3 Tag Approach

For music files, we should prioritize **ID3 tags** over filename parsing, as they provide more accurate metadata.

**Priority:**
1. ID3v2 tags (most common)
2. ID3v1 tags (fallback)
3. Filename parsing (last resort)

### Common Filename Patterns (Fallback)

```
01 - Track Name.mp3
Track Name - Artist Name.mp3
Artist - Album - 01 - Track.mp3
01. Track Name.flac
Artist Name/Album Name/01 Track Name.mp3
```

### Metadata to Extract

From ID3 tags:
- **Title** (TIT2): Track title
- **Artist** (TPE1): Artist name
- **Album** (TALB): Album name
- **Track Number** (TRCK): Track number
- **Year** (TYER/TDRC): Release year
- **Genre** (TCON): Music genre
- **Duration**: Track length
- **Album Artist** (TPE2): Album artist

From filename (fallback):
```regex
^(?:(\d+)[\s.-]+)?(.+?)(?:\s*-\s*(.+?))?\.(\w+)$
```
Examples:
- `01 - Track Name.mp3` → Track: 1, Title: "Track Name"
- `Track Name - Artist.mp3` → Title: "Track Name", Artist: "Artist"

## Implementation Strategy

### Phase 1: Domain Types

Create types in `internal/domain/scanner/`:

```go
type MovieInfo struct {
    Title      string
    Year       int
    Resolution string
    Quality    string
}

type TVEpisodeInfo struct {
    ShowName     string
    Season       int
    Episode      int
    EpisodeTitle string
    Year         int
}

type MusicInfo struct {
    Title       string
    Artist      string
    Album       string
    TrackNumber int
    Year        int
    Genre       string
}
```

### Phase 2: Parser Interface

```go
type FilenameParser interface {
    ParseMovie(filename string) (*MovieInfo, error)
    ParseTVEpisode(filename string) (*TVEpisodeInfo, error)
    ParseMusic(path string) (*MusicInfo, error) // Uses ID3 tags + filename
}
```

### Phase 3: Implementation Order

1. **Movie Parser** - Start with simplest patterns, add more as needed
2. **TV Show Parser** - Handle SxxExx and xYY formats
3. **Music Parser** - ID3 tag reading with filename fallback

### Phase 4: Testing Strategy

Table-driven tests with real-world examples:
- 50+ movie filename patterns
- 50+ TV show filename patterns
- Mix of ID3-tagged and non-tagged music files

## Libraries to Consider

### Go ID3 Tag Libraries

1. **`github.com/bogem/id3v2`** - Popular, well-maintained
   - Supports ID3v2.3 and ID3v2.4
   - Easy to use API
   - Good documentation

2. **`github.com/dhowden/tag`** - Multi-format support
   - Supports ID3v1, ID3v2, MP4, FLAC, OGG
   - Unified interface
   - More comprehensive

**Recommendation**: Use `github.com/dhowden/tag` for broader format support.

## References

- TMDb Naming Conventions: https://www.themoviedb.org/
- Plex Naming Conventions: https://support.plex.tv/articles/naming-and-organizing-your-movie-media-files/
- Plex TV Naming: https://support.plex.tv/articles/naming-and-organizing-your-tv-show-files/
- Kodi Naming: https://kodi.wiki/view/Naming_video_files
- ID3 Specification: https://id3.org/

## Edge Cases to Handle

### Movies
- Movies without years
- Movies with multiple years (remakes)
- Foreign characters in titles
- Special editions (Director's Cut, Extended Edition)
- Part 1/Part 2 movies

### TV Shows
- Specials (S00E01)
- Multi-episode files (S01E01E02)
- Anime episode numbering
- Miniseries vs full series

### Music
- Files without ID3 tags
- Multiple artists (featuring)
- Compilation albums
- Classical music with composer info
- Audiobooks vs music
