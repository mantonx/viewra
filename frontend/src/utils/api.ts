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
 * Builds an artwork URL for a media file using the new asset system
 * This uses the backend's album-artwork endpoint which properly resolves the real Album.ID
 * @param mediaFileId - The media file ID
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized artwork URL for the new asset system
 */
export const buildArtworkUrl = (mediaFileId: number | string, quality: number = 90): string => {
  const fileId = typeof mediaFileId === 'string' ? parseInt(mediaFileId) : mediaFileId;

  // Use the backend's album-artwork endpoint which properly resolves to real Album.ID
  return buildImageUrl(`/api/media/files/${fileId}/album-artwork`, quality);
};

/**
 * Builds an artwork URL using the new entity-based asset system
 * @param albumUUID - The album UUID (real Album.ID from database)
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized artwork URL for the new asset system
 */
export const buildAlbumArtworkUrl = (albumUUID: string, quality: number = 90): string => {
  return buildImageUrl(`/api/v1/assets/entity/album/${albumUUID}/preferred/cover/data`, quality);
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
 * Gets the real album ID for a media file from the backend
 * This replaces the old placeholder UUID generation with proper database lookup
 * @param mediaFileId - The media file ID
 * @returns Promise with album information including real Album.ID
 */
export const getMediaFileAlbumId = async (
  mediaFileId: number | string
): Promise<{ media_file_id: string; album_id: string; asset_url: string }> => {
  const fileId = typeof mediaFileId === 'string' ? mediaFileId : mediaFileId.toString();
  const response = await fetch(`/api/media/files/${fileId}/album-id`);
  if (!response.ok) {
    throw new Error(`Failed to get album ID: ${response.statusText}`);
  }
  return response.json();
};

/**
 * Builds an artwork URL for a media file by fetching the real Album.ID from the backend
 * This is an async alternative to buildArtworkUrl for when you need the actual Album.ID
 * @param mediaFileId - The media file ID
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Promise with optimized artwork URL using real Album.ID
 */
export const buildArtworkUrlAsync = async (
  mediaFileId: number | string,
  quality: number = 90
): Promise<string> => {
  try {
    const albumInfo = await getMediaFileAlbumId(mediaFileId);
    return buildAlbumArtworkUrl(albumInfo.album_id, quality);
  } catch (error) {
    // Fallback to the direct endpoint if Album.ID lookup fails
    console.warn(
      `Failed to get album ID for media file ${mediaFileId}, falling back to direct artwork endpoint:`,
      error
    );
    return buildArtworkUrl(mediaFileId, quality);
  }
};
