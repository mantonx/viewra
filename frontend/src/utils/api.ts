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
 * Builds an artwork URL for a media file with quality optimization
 * @param mediaFileId - The media file ID
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized artwork URL
 */
export const buildArtworkUrl = (mediaFileId: number | string, quality: number = 90): string => {
  return buildImageUrl(`/api/media/${mediaFileId}/artwork`, quality);
};

/**
 * Builds an asset data URL with quality optimization
 * @param assetId - The asset ID
 * @param quality - Quality percentage (1-100), defaults to 90% for frontend
 * @returns Optimized asset data URL
 */
export const buildAssetUrl = (assetId: number | string, quality: number = 90): string => {
  return buildImageUrl(`/api/assets/${assetId}/data`, quality);
};
