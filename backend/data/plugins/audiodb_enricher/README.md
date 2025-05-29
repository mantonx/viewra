# AudioDB Metadata Enricher Plugin

A Viewra plugin that enriches music metadata using [The AudioDB](https://www.theaudiodb.com/) database. This plugin automatically enhances your music library with detailed information including genre, mood, artwork, artist biographies, and much more.

## Architecture

This plugin follows Viewra's **modular plugin architecture** principles:

- ✅ **Service-Based Design**: Uses `MediaAssetService` interface for asset operations
- ✅ **Dependency Injection**: Services are injected rather than directly imported
- ✅ **Interface Isolation**: No direct calls to internal modules
- ✅ **Testable**: All dependencies can be mocked for testing
- ✅ **Self-Contained**: Plugin binary includes all dependencies

### Service Integration

The plugin implements and uses these service interfaces:

- `MetadataScraperService` - For metadata extraction capabilities
- `ScannerHookService` - For scan lifecycle integration
- `DatabaseService` - For database schema management
- `SearchService` - For search functionality
- `APIRegistrationService` - For REST API endpoints
- `MediaAssetService` - For downloading and storing images _(injected dependency)_

## Features

- 🎵 **Automatic Metadata Enrichment**: Enriches music files during library scans
- 🎨 **Artwork Download**: Downloads high-quality album artwork using proper asset service
- 🔍 **Advanced Search**: Search AudioDB database via API endpoints
- 📊 **Smart Matching**: Uses fuzzy string matching with configurable thresholds
- 💾 **Intelligent Caching**: Caches API responses to minimize external requests
- 🎯 **Rate Limiting**: Respects API rate limits with configurable delays
- 🏷️ **Rich Metadata**: Provides genre, mood, style, and detailed track information

## Supported Audio Formats

- MP3 (audio/mpeg)
- FLAC (audio/flac)
- OGG (audio/ogg)
- WAV (audio/wav)
- AAC/M4A (audio/aac, audio/m4a)
- WMA (audio/wma)
- Opus (audio/opus)
- APE (audio/ape)

## Configuration

The plugin can be configured through the `plugin.cue` file or via runtime configuration:

### Basic Settings

```cue
settings: {
    enabled: true
    api_key: ""  // Optional - for enhanced API access
    user_agent: "Viewra AudioDB Enricher/1.0.0"
}
```

### Enrichment Settings

```cue
settings: {
    enable_artwork: true
    artwork_max_size: 1200
    artwork_quality: "front"  // "front", "back", "all"
    download_album_art: true     // Download album artwork
    download_artist_images: false // Download artist images (logos, fanart, etc.)
    prefer_high_quality: true    // Prefer high quality images when available
    match_threshold: 0.75        // 0.0-1.0
    auto_enrich: true
    overwrite_existing: false
}
```

### Asset Download Controls

```cue
settings: {
    max_asset_size: 10485760      // Max file size in bytes (10MB)
    asset_timeout_sec: 30         // Download timeout in seconds
    skip_existing_assets: true    // Skip if asset already exists
    retry_failed_downloads: true  // Retry failed downloads
    max_retries: 3               // Maximum retry attempts
}
```

### Performance Settings

```cue
settings: {
    cache_duration_hours: 168  // 1 week
    request_delay_ms: 1000     // 1 second between requests
}
```

## API Endpoints

When loaded, the plugin registers several API endpoints under `/api/plugins/audiodb_enricher/`:

### Search Tracks

```
GET /api/plugins/audiodb_enricher/search?title=Song&artist=Artist&album=Album
```

### Get Configuration

```
GET /api/plugins/audiodb_enricher/config
```

### Manual Enrichment

```
POST /api/plugins/audiodb_enricher/enrich/{mediaFileId}
```

### Artist Information

```
GET /api/plugins/audiodb_enricher/artist/{artistName}
```

### Album Information

```
GET /api/plugins/audiodb_enricher/album/{artistName}/{albumName}
```

## Database Schema

The plugin creates two database tables:

### `audiodb_cache`

Caches API responses to reduce external requests.

| Column       | Type      | Description           |
| ------------ | --------- | --------------------- |
| id           | uint      | Primary key           |
| search_query | string    | Cache key             |
| api_response | longtext  | Cached JSON response  |
| cached_at    | timestamp | Cache creation time   |
| expires_at   | timestamp | Cache expiration time |

### `audiodb_enrichment`

Stores enriched metadata for media files.

| Column            | Type      | Description                |
| ----------------- | --------- | -------------------------- |
| id                | uint      | Primary key                |
| media_file_id     | uint      | Reference to media file    |
| audiodb_track_id  | string    | AudioDB track ID           |
| audiodb_artist_id | string    | AudioDB artist ID          |
| audiodb_album_id  | string    | AudioDB album ID           |
| enriched_title    | string    | Enhanced track title       |
| enriched_artist   | string    | Enhanced artist name       |
| enriched_album    | string    | Enhanced album name        |
| enriched_year     | int       | Release year               |
| enriched_genre    | string    | Music genre                |
| match_score       | float64   | Confidence score (0.0-1.0) |
| artwork_url       | string    | Artwork download URL       |
| artwork_path      | string    | Local artwork file path    |
| enriched_at       | timestamp | Enrichment timestamp       |

## How It Works

### Automatic Enrichment

1. **Scan Hook**: When a music file is scanned, the plugin receives metadata
2. **Search**: Searches AudioDB using artist name to find tracks
3. **Match**: Uses fuzzy string matching to find the best match
4. **Enrich**: Downloads additional metadata and artwork
5. **Store**: Saves enriched data to the database

### Search Algorithm

The plugin uses a sophisticated matching algorithm:

1. **Title Similarity**: 40% weight for track title matching
2. **Artist Similarity**: 40% weight for artist name matching
3. **Album Similarity**: 20% weight for album name matching (when available)

Only matches above the configured threshold (default: 0.75) are accepted.

### Caching Strategy

- **Search Results**: Cached for 1 week by default
- **Cache Key**: Based on normalized search terms
- **Expiration**: Automatic cleanup of expired cache entries
- **Rate Limiting**: Configurable delay between API requests

## AudioDB API

This plugin uses The AudioDB API v1:

- **Base URL**: `https://www.theaudiodb.com/api/v1/json/1/`
- **Free Tier**: Available without API key
- **Paid Tier**: Enhanced access with API key
- **Rate Limits**: Respected through configurable delays

### API Endpoints Used

- `search.php?s={artist}` - Search for artist
- `track.php?m={artistId}` - Get tracks by artist
- `album.php?m={albumId}` - Get album information

## Building the Plugin

```bash
cd backend/data/plugins/audiodb_enricher
go mod download
go build -o audiodb_enricher .
```

## Installation

1. Place the plugin in the `backend/data/plugins/audiodb_enricher/` directory
2. Ensure the binary is executable
3. Restart Viewra to load the plugin
4. The plugin will automatically start enriching music files during scans

## Troubleshooting

### Common Issues

**Plugin not loading:**

- Check that the binary is executable
- Verify plugin.cue configuration is valid
- Check Viewra logs for error messages

**No enrichment happening:**

- Ensure `auto_enrich` is set to `true`
- Check that music files have title and artist metadata
- Verify internet connectivity to theaudiodb.com

**Low match rates:**

- Lower the `match_threshold` setting
- Check that metadata is clean (no extra characters)
- Verify artist/track names match AudioDB database

### Logs

Plugin logs are available in the Viewra application logs:

```
AudioDB: Starting track enrichment media_file_id=123 title="Song" artist="Artist"
AudioDB: Track enriched successfully media_file_id=123 match_score=0.85
```

## Contributing

Contributions are welcome! Please see the main Viewra repository for contribution guidelines.

## License

This plugin is licensed under the MIT License. See the main Viewra repository for details.

## Credits

- **The AudioDB**: https://www.theaudiodb.com/ - Music database API
- **Viewra Team**: Plugin development and maintenance
