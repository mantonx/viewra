// Media Asset Types for comprehensive entity-based asset management

export type EntityType =
  | 'artist'
  | 'album'
  | 'track'
  | 'movie'
  | 'tv_show'
  | 'episode'
  | 'director'
  | 'actor'
  | 'studio'
  | 'label'
  | 'network'
  | 'genre'
  | 'collection';

export type AssetType =
  // Universal types
  | 'logo'
  | 'photo'
  | 'background'
  | 'banner'
  | 'thumb'
  | 'fanart'
  // Artist specific
  | 'clearart'
  // Album/Collection specific
  | 'cover'
  | 'disc'
  | 'booklet'
  // Track specific
  | 'waveform'
  | 'spectrogram'
  // Movie/TV specific
  | 'poster'
  // TV Show specific
  | 'network_logo'
  // Episode specific
  | 'screenshot'
  // Actor/Director specific
  | 'headshot'
  | 'portrait'
  | 'signature'
  // Studio/Label specific
  | 'hq_photo'
  // Genre specific
  | 'icon';

export type AssetSource =
  | 'local'
  | 'user'
  | 'plugin' // Generic source for all external plugins
  | 'embedded';

export interface MediaAsset {
  id: string; // UUID
  entity_type: EntityType;
  entity_id: string; // UUID
  type: AssetType;
  source: AssetSource;
  path: string;
  width: number;
  height: number;
  format: string; // Always 'image/webp' for new assets
  preferred: boolean;
  language?: string;
  created_at: string;
  updated_at: string;
}

export interface AssetRequest {
  entity_type: EntityType;
  entity_id: string; // UUID
  type: AssetType;
  source: AssetSource;
  data: ArrayBuffer | Uint8Array;
  width?: number;
  height?: number;
  format: string; // Input format - will be converted to WebP
  preferred?: boolean;
  language?: string;
}

export interface AssetFilter {
  entity_type?: EntityType;
  entity_id?: string; // UUID
  type?: AssetType;
  source?: AssetSource;
  preferred?: boolean;
  language?: string;
  limit?: number;
  offset?: number;
}

export interface AssetStats {
  total_assets: number;
  total_size: number;
  assets_by_entity: Record<EntityType, number>;
  assets_by_type: Record<AssetType, number>;
  assets_by_source: Record<AssetSource, number>;
  average_size: number;
  largest_asset_size: number;
  preferred_assets: number;
  supported_formats: string[];
}

// Quality levels for WebP compression
export type ImageQuality = 'low' | 'medium' | 'high' | 'original';

export const QUALITY_VALUES: Record<ImageQuality, number> = {
  low: 50,
  medium: 75,
  high: 90,
  original: 0, // 0 means use original quality
};

// Default quality for frontend
export const DEFAULT_QUALITY = 90;

// Asset URL generation with quality support
export interface AssetUrlOptions {
  quality?: ImageQuality | number;
  cache?: boolean;
}

// Utility type to get valid asset types for each entity type
export const ENTITY_ASSET_TYPES: Record<EntityType, AssetType[]> = {
  artist: ['logo', 'photo', 'background', 'banner', 'thumb', 'clearart', 'fanart'],
  album: ['cover', 'thumb', 'disc', 'background', 'booklet'],
  track: ['waveform', 'spectrogram', 'cover'],
  movie: ['poster', 'logo', 'banner', 'background', 'thumb', 'fanart'],
  tv_show: ['poster', 'logo', 'banner', 'background', 'network_logo', 'thumb', 'fanart'],
  episode: ['screenshot', 'thumb', 'poster'],
  actor: ['headshot', 'photo', 'thumb', 'signature'],
  director: ['portrait', 'signature', 'logo'],
  studio: ['logo', 'hq_photo', 'banner'],
  label: ['logo', 'hq_photo', 'banner'],
  network: ['logo', 'banner'],
  genre: ['icon', 'background', 'banner'],
  collection: ['cover', 'background', 'logo'],
};

// Human-readable labels for entity types
export const ENTITY_TYPE_LABELS: Record<EntityType, string> = {
  artist: 'Artist',
  album: 'Album',
  track: 'Track',
  movie: 'Movie',
  tv_show: 'TV Show',
  episode: 'Episode',
  director: 'Director',
  actor: 'Actor',
  studio: 'Studio',
  label: 'Label',
  network: 'Network',
  genre: 'Genre',
  collection: 'Collection',
};

// Human-readable labels for asset types
export const ASSET_TYPE_LABELS: Record<AssetType, string> = {
  logo: 'Logo',
  photo: 'Photo',
  background: 'Background',
  banner: 'Banner',
  thumb: 'Thumbnail',
  fanart: 'Fan Art',
  clearart: 'Clear Art',
  cover: 'Cover',
  disc: 'Disc Art',
  booklet: 'Booklet',
  waveform: 'Waveform',
  spectrogram: 'Spectrogram',
  poster: 'Poster',
  network_logo: 'Network Logo',
  screenshot: 'Screenshot',
  headshot: 'Headshot',
  portrait: 'Portrait',
  signature: 'Signature',
  hq_photo: 'HQ Photo',
  icon: 'Icon',
};

// Human-readable labels for asset sources
export const ASSET_SOURCE_LABELS: Record<AssetSource, string> = {
  local: 'Local',
  user: 'User Upload',
  plugin: 'Plugin',
  embedded: 'Embedded',
};

// Supported image formats (input formats - all converted to WebP)
export const SUPPORTED_IMAGE_FORMATS = [
  'image/jpeg',
  'image/jpg',
  'image/png',
  'image/webp',
  'image/gif',
  'image/bmp',
  'image/tiff',
  'image/svg+xml',
];

// Utility functions
export const getValidAssetTypes = (entityType: EntityType): AssetType[] => {
  return ENTITY_ASSET_TYPES[entityType] || [];
};

export const isValidAssetType = (entityType: EntityType, assetType: AssetType): boolean => {
  return getValidAssetTypes(entityType).includes(assetType);
};

export const getEntityTypeLabel = (entityType: EntityType): string => {
  return ENTITY_TYPE_LABELS[entityType] || entityType;
};

export const getAssetTypeLabel = (assetType: AssetType): string => {
  return ASSET_TYPE_LABELS[assetType] || assetType;
};

export const getAssetSourceLabel = (source: AssetSource): string => {
  return ASSET_SOURCE_LABELS[source] || source;
};

export const isSupportedImageFormat = (mimeType: string): boolean => {
  return SUPPORTED_IMAGE_FORMATS.includes(mimeType);
};

export const getFileExtensionForMimeType = (mimeType: string): string => {
  switch (mimeType) {
    case 'image/jpeg':
    case 'image/jpg':
      return '.jpg';
    case 'image/png':
      return '.png';
    case 'image/webp':
      return '.webp';
    case 'image/gif':
      return '.gif';
    case 'image/bmp':
      return '.bmp';
    case 'image/tiff':
      return '.tiff';
    case 'image/svg+xml':
      return '.svg';
    default:
      return '.jpg';
  }
};

// Generate asset URL with quality and caching options
export const getAssetUrl = (assetId: string, options: AssetUrlOptions = {}): string => {
  const baseUrl = `/api/v1/assets/${assetId}/data`;
  const params = new URLSearchParams();

  // Use default quality of 90 if not specified
  const quality = options.quality !== undefined ? options.quality : DEFAULT_QUALITY;

  if (quality !== undefined) {
    const qualityValue = typeof quality === 'string' ? QUALITY_VALUES[quality] : quality;

    if (qualityValue > 0) {
      params.append('quality', qualityValue.toString());
    }
  }

  if (options.cache === false) {
    params.append('t', Date.now().toString());
  }

  const queryString = params.toString();
  return queryString ? `${baseUrl}?${queryString}` : baseUrl;
};

// Get quality value from quality name or number
export const getQualityValue = (quality: ImageQuality | number): number => {
  if (typeof quality === 'number') {
    return Math.max(0, Math.min(100, quality));
  }
  return QUALITY_VALUES[quality] || 0;
};

// API Response types
export interface AssetApiResponse {
  asset: MediaAsset;
  success: boolean;
  message?: string;
}

export interface AssetListApiResponse {
  assets: MediaAsset[];
  total: number;
  success: boolean;
  message?: string;
}

export interface AssetStatsApiResponse {
  stats: AssetStats;
  success: boolean;
  message?: string;
}

// Asset management utility types
export interface AssetUploadProgress {
  entityType: EntityType;
  entityId: string;
  assetType: AssetType;
  progress: number;
  status: 'pending' | 'uploading' | 'processing' | 'completed' | 'error';
  error?: string;
}

export interface AssetValidationResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
}

// Hook for asset validation
export const validateAssetRequest = (request: Partial<AssetRequest>): AssetValidationResult => {
  const errors: string[] = [];
  const warnings: string[] = [];

  if (!request.entity_type) {
    errors.push('Entity type is required');
  }

  if (!request.entity_id) {
    errors.push('Entity ID is required');
  }

  if (!request.type) {
    errors.push('Asset type is required');
  }

  if (!request.source) {
    errors.push('Asset source is required');
  }

  if (!request.format) {
    errors.push('Asset format is required');
  }

  if (!request.data) {
    errors.push('Asset data is required');
  }

  if (request.entity_type && request.type) {
    if (!isValidAssetType(request.entity_type, request.type)) {
      errors.push(
        `Asset type '${request.type}' is not valid for entity type '${request.entity_type}'`
      );
    }
  }

  if (request.format && !isSupportedImageFormat(request.format)) {
    errors.push(`Unsupported image format: ${request.format}`);
  }

  if (request.width && request.width < 1) {
    warnings.push('Width should be greater than 0');
  }

  if (request.height && request.height < 1) {
    warnings.push('Height should be greater than 0');
  }

  // Note about WebP conversion
  if (request.format && request.format !== 'image/webp') {
    warnings.push('Image will be automatically converted to WebP format for optimal storage');
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings,
  };
};
