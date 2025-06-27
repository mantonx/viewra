/**
 * API utility functions for Viewra frontend.
 * 
 * This module provides utility functions for building API URLs, particularly for
 * media assets like artwork and streaming endpoints. It handles URL construction
 * with proper query parameters and integrates with the backend's asset system.
 */

/**
 * Builds an image URL with quality parameter for optimal frontend serving.
 * 
 * This function adds a quality parameter to image URLs to enable server-side
 * image optimization. The backend can use this parameter to resize and compress
 * images appropriately for different use cases (thumbnails vs full images).
 * 
 * @param baseUrl - The base image URL
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns URL with quality parameter appended
 */
export const buildImageUrl = (baseUrl: string, quality: number = 90): string => {
  if (!baseUrl) return baseUrl;

  // Check if URL already has query parameters
  const separator = baseUrl.includes('?') ? '&' : '?';
  return `${baseUrl}${separator}quality=${quality}`;
};

/**
 * Builds an artwork URL for a media file using the new asset system.
 * 
 * This uses the backend's album-artwork endpoint which properly resolves the real Album.ID
 * from the media file's metadata. The endpoint handles the lookup internally and returns
 * the appropriate album artwork or falls back to a placeholder if none exists.
 * 
 * @param mediaFileId - The media file ID (UUID string or legacy numeric ID)
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized artwork URL for the new asset system
 */
export const buildArtworkUrl = (mediaFileId: number | string, quality: number = 90): string => {
  // Handle both string UUIDs and legacy numeric IDs
  const fileId = String(mediaFileId);

  // For now, use the legacy endpoint that gets the real album ID and handles the asset lookup
  // This endpoint will redirect to the proper asset URL internally
  const artworkUrl = buildImageUrl(`/api/media/files/${fileId}/album-artwork`, quality);

  // Return the URL - the frontend will handle fallback to placeholder in the img tag's onError
  return artworkUrl;
};

/**
 * Builds an artwork URL using the new entity-based asset system.
 * 
 * This function directly accesses album artwork using the album's UUID,
 * bypassing the need to look up through a media file. Use this when you
 * already have the album UUID available.
 * 
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
 * Gets the preferred asset for an entity.
 * 
 * Retrieves metadata about the preferred asset of a specific type for an entity.
 * The backend determines which asset is "preferred" based on quality, source,
 * and user preferences.
 * 
 * @param entityType - The entity type (e.g., 'album', 'artist', 'movie')
 * @param entityId - The entity UUID
 * @param assetType - The asset type (e.g., 'cover', 'backdrop', 'logo')
 * @returns Promise resolving to asset metadata
 * @throws Error if the request fails
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
 * Gets all assets for an entity.
 * 
 * Retrieves a list of all assets associated with an entity, optionally filtered
 * by type, quality, or other parameters. Useful for displaying multiple artwork
 * options or building asset galleries.
 * 
 * @param entityType - The entity type (e.g., 'album', 'artist', 'movie')
 * @param entityId - The entity UUID
 * @param filter - Optional filter parameters (type, quality, source, etc.)
 * @returns Promise resolving to array of asset metadata
 * @throws Error if the request fails
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
 * Gets the real album ID for a media file from the backend.
 * 
 * This replaces the old placeholder UUID generation with proper database lookup.
 * The backend examines the media file's metadata to determine its associated album
 * and returns the actual Album entity UUID from the database.
 * 
 * @param mediaFileId - The media file ID (UUID string or legacy numeric ID)
 * @returns Promise with album information including real Album.ID and asset URL
 * @throws Error if the album lookup fails
 */
export const getMediaFileAlbumId = async (
  mediaFileId: number | string
): Promise<{ media_file_id: string; album_id: string; asset_url: string }> => {
  const fileId = String(mediaFileId);
  const response = await fetch(`/api/media/files/${fileId}/album-id`);
  if (!response.ok) {
    throw new Error(`Failed to get album ID: ${response.statusText}`);
  }
  return response.json();
};

/**
 * Builds an artwork URL for a media file by fetching the real Album.ID from the backend.
 * 
 * This is an async alternative to buildArtworkUrl for when you need the actual Album.ID.
 * It performs a two-step process: first fetching the album ID, then constructing the
 * direct asset URL. Falls back to the simpler endpoint if the album lookup fails.
 * 
 * @param mediaFileId - The media file ID (UUID string or legacy numeric ID)
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Promise resolving to optimized artwork URL using real Album.ID
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

/**
 * Builds a streaming URL for a media file.
 * 
 * Constructs the URL for streaming media content. The backend will handle
 * transcoding, range requests, and format negotiation based on the client's
 * capabilities.
 * 
 * @param mediaFileId - The media file ID (UUID string or legacy numeric ID)
 * @returns Streaming URL for the media file
 */
export const buildStreamUrl = (mediaFileId: number | string): string => {
  const fileId = String(mediaFileId);
  return `/api/media/files/${fileId}/stream`;
};
