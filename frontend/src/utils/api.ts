// API utility functions

/**
 * Builds an image URL with quality parameter for optimal frontend serving
 * @param baseUrl - The base image URL
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns URL with quality parameter
 */
export const buildImageUrl = (baseUrl: string, quality: number = 90): string => {
  if (!baseUrl) return baseUrl;

  // Check if URL already has query parameters
  const separator = baseUrl.includes('?') ? '&' : '?';
  return `${baseUrl}${separator}quality=${quality}`;
};

/**
 * Generates a deterministic album UUID based on media file ID
 * This matches the server-side logic for creating album placeholders
 * Uses UUID v5 with OID namespace to match Go's uuid.NewSHA1(uuid.NameSpaceOID, ...)
 * @param mediaFileId - The media file ID
 * @returns Album UUID string
 */
export const generateAlbumUUID = (mediaFileId: number): string => {
  // OID namespace UUID from RFC 4122: 6ba7b812-9dad-11d1-80b4-00c04fd430c8
  const oidNamespace = '6ba7b8129dad11d180b400c04fd430c8';

  // Create the same string that the server uses
  const albumString = `album-placeholder-${mediaFileId}`;

  // Simple SHA-1 based UUID v5 implementation
  // This is a simplified version - in production you'd use a proper UUID library
  let hash = 0;
  for (let i = 0; i < albumString.length; i++) {
    const char = albumString.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash; // Convert to 32-bit integer
  }

  // Combine with namespace for better distribution
  const combinedHash = hash + parseInt(oidNamespace.slice(0, 8), 16);
  const hex = Math.abs(combinedHash).toString(16).padStart(12, '0');

  // Format as UUID v5 (version 5 uses SHA-1)
  return `${hex.slice(0, 8)}-${hex.slice(0, 4)}-5${hex.slice(1, 4)}-${(8 + (Math.abs(combinedHash) % 4)).toString(16)}${hex.slice(1, 4)}-${hex.slice(0, 12)}`;
};

/**
 * Builds an artwork URL for a media file using the new asset system
 * This uses a simpler approach with the backend's album-artwork endpoint
 * @param mediaFileId - The media file ID
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized artwork URL for the new asset system
 */
export const buildArtworkUrl = (mediaFileId: number | string, quality: number = 90): string => {
  const fileId = typeof mediaFileId === 'string' ? parseInt(mediaFileId) : mediaFileId;

  // Use the backend's album-artwork endpoint which properly redirects to the asset system
  return buildImageUrl(`/api/media/files/${fileId}/album-artwork`, quality);
};

/**
 * Builds an artwork URL using the new entity-based asset system
 * @param albumUUID - The album UUID
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized artwork URL for the new asset system
 */
export const buildAlbumArtworkUrl = (albumUUID: string, quality: number = 90): string => {
  return buildImageUrl(`/api/v1/assets/entity/album/${albumUUID}/preferred/cover/data`, quality);
};

/**
 * Builds an artwork URL for a media file by generating the album UUID
 * This is a bridge function for the new asset system
 * @param mediaFileId - The media file ID
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized artwork URL for the new asset system
 */
export const buildArtworkUrlNew = (mediaFileId: number | string, quality: number = 90): string => {
  const fileId = typeof mediaFileId === 'string' ? parseInt(mediaFileId) : mediaFileId;
  const albumUUID = generateAlbumUUID(fileId);
  return buildAlbumArtworkUrl(albumUUID, quality);
};

/**
 * Builds an asset data URL with quality optimization
 * @param assetId - The asset ID (UUID)
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized asset data URL
 */
export const buildAssetUrl = (assetId: string, quality: number = 90): string => {
  return buildImageUrl(`/api/v1/assets/${assetId}/data`, quality);
};

/**
 * Gets the preferred asset for an entity
 * @param entityType - The entity type (e.g., 'album')
 * @param entityId - The entity UUID
 * @param assetType - The asset type (e.g., 'cover')
 * @returns Promise with asset data
 */
export const getPreferredAsset = async (
  entityType: string,
  entityId: string,
  assetType: string
) => {
  const response = await fetch(
    `/api/v1/assets/entity/${entityType}/${entityId}/preferred/${assetType}`
  );
  if (!response.ok) {
    throw new Error(`Failed to get preferred asset: ${response.statusText}`);
  }
  return response.json();
};

/**
 * Gets all assets for an entity
 * @param entityType - The entity type (e.g., 'album')
 * @param entityId - The entity UUID
 * @param filter - Optional filter parameters
 * @returns Promise with assets data
 */
export const getEntityAssets = async (
  entityType: string,
  entityId: string,
  filter?: Record<string, string | number | boolean>
) => {
  const params = new URLSearchParams();
  if (filter) {
    Object.entries(filter).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        params.append(key, String(value));
      }
    });
  }

  const url = `/api/v1/assets/entity/${entityType}/${entityId}${params.toString() ? `?${params.toString()}` : ''}`;
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to get entity assets: ${response.statusText}`);
  }
  return response.json();
};

/**
 * Gets the album ID for a media file from the backend
 * @param mediaFileId - The media file ID
 * @returns Promise with album information
 */
export const getMediaFileAlbumId = async (
  mediaFileId: number | string
): Promise<{ media_file_id: number; album_id: string; asset_url: string }> => {
  const fileId = typeof mediaFileId === 'string' ? parseInt(mediaFileId) : mediaFileId;
  const response = await fetch(`/api/media/files/${fileId}/album-id`);
  if (!response.ok) {
    throw new Error(`Failed to get album ID: ${response.statusText}`);
  }
  return response.json();
};
