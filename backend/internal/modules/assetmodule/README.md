# Media Asset Module

A comprehensive entity-based media asset management system for Viewra that supports multiple entity types with hash-based file organization to prevent conflicts. **All images are automatically converted to WebP format** for optimal storage and performance.

## Hash-Based File Organization

The system uses SHA256 hashes to organize files and prevent conflicts:

### Directory Structure

```
assets/
├── artists/
│   ├── aa/
│   │   ├── logo_fanart.tv_7d4a2b8f6c3e1a9d.webp
│   │   └── photo_local_3f8e7c4d2a1b9e6f.webp
│   ├── ab/
│   └── ...
├── albums/
│   ├── 12/
│   │   ├── cover_musicbrainz_8a5c3e7d1f9b2a6e.webp
│   │   └── disc_local_6f2d8a3e9c7b1a4d.webp
│   └── ...
├── tracks/
├── movies/
├── tv_shows/
├── episodes/
├── directors/
├── actors/
├── studios/
├── labels/
├── networks/
├── genres/
├── collections/
└── misc/
```

### Path Generation Algorithm

1. **Entity Hash**: `SHA256(entity_type:entity_id)` → `aa12b34c...`
2. **Entity Directory**: First 2 chars of entity hash → `aa/`
3. **Content Hash**: `SHA256(file_data)` → `7d4a2b8f...`
4. **Filename**: `{asset_type}_{source}_{content_hash[:16]}.webp`

Example: `artists/aa/logo_fanart.tv_7d4a2b8f6c3e1a9d.webp`

### Benefits

- **No Conflicts**: Content hashes ensure identical content doesn't conflict
- **Deduplication**: Same content from different sources gets unique names
- **Scalability**: Directory sharding prevents too many files in one directory
- **Deterministic**: Same content always generates the same path
- **Organized**: Entity types provide logical separation
- **Optimized Storage**: All images converted to WebP for better compression

## WebP Conversion & Quality Control

### Automatic WebP Conversion

- **All input images** are automatically converted to WebP format
- Supports input formats: JPEG, PNG, GIF, BMP, TIFF, SVG, WebP
- **Default quality**: 95% for storage (high quality)
- Preserves image dimensions and metadata

### Quality Parameter Support

The system supports dynamic quality adjustment for serving images:

- **Frontend default**: 90% quality for optimal balance
- **Quality range**: 1-100 (higher = better quality, larger file)
- **Original quality**: Use `quality=0` or omit parameter
- **URL format**: `/api/v1/assets/{id}/data?quality=75`

### Quality Presets

- **Low**: 50% quality - Thumbnails, previews
- **Medium**: 75% quality - General use
- **High**: 90% quality - **Default for frontend**
- **Original**: Stored quality (95%)

## Supported Entity Types

- **artist**: Musicians, bands, composers
- **album**: Music albums, compilations
- **track**: Individual songs
- **movie**: Films, documentaries
- **tv_show**: TV series
- **episode**: Individual TV episodes
- **director**: Film/TV directors
- **actor**: Actors, performers
- **studio**: Production studios
- **label**: Record labels
- **network**: TV networks
- **genre**: Music/media genres
- **collection**: Custom collections

## Asset Types by Entity

### Artist Assets

- `logo`, `photo`, `background`, `banner`, `thumb`, `clearart`, `fanart`

### Album Assets

- `cover`, `thumb`, `disc`, `background`, `booklet`

### Track Assets

- `waveform`, `spectrogram`, `cover`

### Movie Assets

- `poster`, `logo`, `banner`, `background`, `thumb`, `fanart`

### TV Show Assets

- `poster`, `logo`, `banner`, `background`, `network_logo`, `thumb`, `fanart`

### Episode Assets

- `screenshot`, `thumb`, `poster`

### Actor/Director Assets

- `headshot`, `photo`, `thumb`, `signature`, `portrait`, `logo`

### Studio/Label Assets

- `logo`, `hq_photo`, `banner`

### Network Assets

- `logo`, `banner`

### Genre Assets

- `icon`, `background`, `banner`

### Collection Assets

- `cover`, `background`, `logo`

## Asset Sources

- `local`: Locally stored assets
- `fanart.tv`: Fanart.tv service
- `theaudiodb`: TheAudioDB service
- `tmdb`: The Movie Database
- `tvdb`: TheTVDB
- `musicbrainz`: MusicBrainz
- `lastfm`: Last.fm
- `spotify`: Spotify
- `user`: User uploaded
- `plugin`: Plugin generated
- `embedded`: Embedded in media files

## API Endpoints

### Asset Management

- `POST /api/v1/assets/` - Create new asset
- `GET /api/v1/assets/:id` - Get asset metadata
- `PUT /api/v1/assets/:id/preferred` - Set as preferred
- `DELETE /api/v1/assets/:id` - Remove asset

### Entity-Based Access

- `GET /api/v1/assets/entity/:type/:id` - Get all assets for entity
- `GET /api/v1/assets/entity/:type/:id/preferred/:asset_type` - Get preferred asset
- `DELETE /api/v1/assets/entity/:type/:id` - Remove all entity assets

### Asset Data (with Quality Support)

- `GET /api/v1/assets/:id/data` - Get asset binary data (default quality)
- `GET /api/v1/assets/:id/data?quality=90` - Get asset with specific quality
- `GET /api/v1/assets/:id/data?quality=0` - Get asset with original quality

### Utility

- `GET /api/v1/assets/stats` - Asset statistics
- `POST /api/v1/assets/cleanup` - Clean orphaned files
- `GET /api/v1/assets/types` - Get valid asset types
- `GET /api/v1/assets/sources` - Get valid sources
- `GET /api/v1/assets/entity-types` - Get valid entity types

## Database Schema

```sql
CREATE TABLE media_assets (
    id UUID PRIMARY KEY,
    entity_type VARCHAR NOT NULL,
    entity_id UUID NOT NULL,
    type VARCHAR NOT NULL,
    source VARCHAR NOT NULL,
    path VARCHAR NOT NULL,
    width INTEGER DEFAULT 0,
    height INTEGER DEFAULT 0,
    format VARCHAR NOT NULL DEFAULT 'image/webp',
    preferred BOOLEAN DEFAULT FALSE,
    language VARCHAR DEFAULT '',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- Indexes for performance
CREATE INDEX idx_media_assets_entity ON media_assets(entity_type, entity_id);
CREATE INDEX idx_media_assets_type ON media_assets(type);
CREATE INDEX idx_media_assets_source ON media_assets(source);
CREATE INDEX idx_media_assets_preferred ON media_assets(preferred);
CREATE INDEX idx_media_assets_entity_type_preferred ON media_assets(entity_type, entity_id, type, preferred);
```

## Events

The module publishes events for all asset operations:

- `asset.created` - New asset created
- `asset.updated` - Asset updated
- `asset.removed` - Asset removed
- `asset.preferred` - Asset set as preferred

## Supported Input Formats

The system accepts these input formats and converts them to WebP:

- `image/jpeg`, `image/jpg`
- `image/png`
- `image/webp`
- `image/gif`
- `image/bmp`
- `image/tiff`
- `image/svg+xml`

**Output Format**: All assets are stored as `image/webp` for optimal compression and performance.

## Frontend Integration

### TypeScript Usage

```typescript
import { getAssetUrl, ImageQuality } from './types/mediaAssets';

// Default quality (90%)
const assetUrl = getAssetUrl(assetId);

// Specific quality
const thumbnailUrl = getAssetUrl(assetId, { quality: 'low' });
const highQualityUrl = getAssetUrl(assetId, { quality: 'high' });
const customQualityUrl = getAssetUrl(assetId, { quality: 75 });

// Original quality
const originalUrl = getAssetUrl(assetId, { quality: 'original' });
```

### Quality Guidelines

- **Thumbnails/Lists**: Use `quality: 'low'` (50%)
- **General Display**: Use default quality (90%)
- **Full-Screen/Zoom**: Use `quality: 'original'` (95%)
- **Custom**: Specify numeric value 1-100
