/**
 * Media validation and helper utilities
 */

import type { MediaType, MediaItem, Episode, Movie } from '@/components/MediaPlayer/types';

/**
 * Validates UUID format
 * @param uuid - String to validate
 * @returns Whether string is a valid UUID
 */
export const isValidUUID = (uuid: string): boolean => {
  const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  return uuidRegex.test(uuid);
};

/**
 * Validates transcoding session ID format (UUID-based)
 * @param sessionId - Session ID to validate
 * @returns Whether session ID is valid
 */
export const isValidSessionId = (sessionId: string): boolean => {
  if (!sessionId) return false;
  // Accept both plain UUID and legacy ffmpeg_UUID format
  const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  const legacyPattern = /^ffmpeg_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  return uuidPattern.test(sessionId) || legacyPattern.test(sessionId);
};

/**
 * Type guards for media items
 */

/**
 * Checks if media item is an episode
 * @param media - Media item to check
 * @returns Type predicate for Episode
 */
export const isEpisode = (media: MediaItem): media is Episode => {
  return media.type === 'episode';
};

/**
 * Checks if media item is a movie
 * @param media - Media item to check
 * @returns Type predicate for Movie
 */
export const isMovie = (media: MediaItem): media is Movie => {
  return media.type === 'movie';
};

/**
 * Validates media type
 * @param type - String to validate
 * @returns Whether string is a valid MediaType
 */
export const isValidMediaType = (type: string): type is MediaType => {
  return type === 'episode' || type === 'movie';
};

/**
 * Validates episode structure
 * @param data - Data to validate
 * @returns Whether data is a valid Episode
 */
export const isValidEpisode = (data: any): data is Episode => {
  return (
    typeof data === 'object' &&
    data !== null &&
    data.type === 'episode' &&
    typeof data.id === 'string' &&
    typeof data.title === 'string' &&
    typeof data.episode_number === 'number' &&
    typeof data.season_number === 'number' &&
    typeof data.series === 'object' &&
    data.series !== null &&
    typeof data.series.id === 'string' &&
    typeof data.series.title === 'string'
  );
};

/**
 * Validates movie structure
 * @param data - Data to validate
 * @returns Whether data is a valid Movie
 */
export const isValidMovie = (data: any): data is Movie => {
  return (
    typeof data === 'object' &&
    data !== null &&
    data.type === 'movie' &&
    typeof data.id === 'string' &&
    typeof data.title === 'string'
  );
};

/**
 * Validates media item structure
 * @param data - Data to validate
 * @returns Whether data is a valid MediaItem
 */
export const isValidMediaItem = (data: any): data is MediaItem => {
  return isValidEpisode(data) || isValidMovie(data);
};

/**
 * Validates duration value
 * @param duration - Duration to validate
 * @returns Whether duration is valid
 */
export const isValidDuration = (duration: number): boolean => {
  return isFinite(duration) && !isNaN(duration) && duration > 0;
};

/**
 * Validates time value for video playback
 * @param time - Time to validate
 * @returns Whether time is valid
 */
export const isValidTime = (time: number): boolean => {
  return isFinite(time) && !isNaN(time) && time >= 0;
};

/**
 * Validates seek progress (0-1)
 * @param progress - Progress to validate
 * @returns Whether progress is valid
 */
export const isValidProgress = (progress: number): boolean => {
  return isFinite(progress) && !isNaN(progress) && progress >= 0 && progress <= 1;
};

/**
 * Validates volume level (0-1)
 * @param volume - Volume to validate
 * @returns Whether volume is valid
 */
export const isValidVolume = (volume: number): boolean => {
  return isFinite(volume) && !isNaN(volume) && volume >= 0 && volume <= 1;
};

/**
 * Validates URL format
 * @param url - URL to validate
 * @returns Whether URL is valid
 */
export const isValidUrl = (url: string): boolean => {
  try {
    new URL(url);
    return true;
  } catch {
    return false;
  }
};

/**
 * Validates manifest URL format (DASH/HLS)
 * @param url - URL to validate
 * @returns Whether URL is a valid manifest
 */
export const isValidManifestUrl = (url: string): boolean => {
  return isValidUrl(url) && (url.includes('.mpd') || url.includes('.m3u8'));
};

/**
 * Cleans and validates media title
 * @param title - Title to clean
 * @returns Cleaned title
 */
export const cleanMediaTitle = (title: string): string => {
  if (typeof title !== 'string') return 'Unknown Title';
  return title.trim() || 'Unknown Title';
};

/**
 * Generates display title for media item
 * @param media - Media item
 * @returns Display title
 */
export const getDisplayTitle = (media: MediaItem): string => {
  const baseTitle = cleanMediaTitle(media.title);
  
  if (isEpisode(media)) {
    return `${media.series.title} - S${media.season_number}E${media.episode_number}: ${baseTitle}`;
  }
  
  return baseTitle;
};

/**
 * Generates short title for media item
 * @param media - Media item
 * @returns Short title
 */
export const getShortTitle = (media: MediaItem): string => {
  if (isEpisode(media)) {
    return `S${media.season_number}E${media.episode_number}: ${cleanMediaTitle(media.title)}`;
  }
  
  return cleanMediaTitle(media.title);
};

/**
 * Gets media poster/backdrop image
 * @param media - Media item
 * @param preferBackdrop - Whether to prefer backdrop over poster
 * @returns Image URL or null
 */
export const getMediaImage = (media: MediaItem, preferBackdrop: boolean = false): string | null => {
  if (isEpisode(media)) {
    if (preferBackdrop) {
      return media.still_image || media.series.backdrop || media.series.poster || null;
    }
    return media.series.poster || media.still_image || media.series.backdrop || null;
  }
  
  if (preferBackdrop) {
    return media.backdrop || media.poster || null;
  }
  return media.poster || media.backdrop || null;
};

/**
 * Validates file extension for supported media types
 * @param filename - Filename to check
 * @returns Whether file is supported
 */
export const isSupportedMediaFile = (filename: string): boolean => {
  const supportedExtensions = [
    '.mp4', '.mkv', '.avi', '.mov', '.wmv', '.flv', '.webm',
    '.m4v', '.3gp', '.mpg', '.mpeg', '.ogv', '.ts', '.m2ts'
  ];
  
  const extension = filename.toLowerCase().split('.').pop();
  return supportedExtensions.includes(`.${extension}`);
};

/**
 * Validates codec string
 * @param codec - Codec to validate
 * @returns Whether codec is valid
 */
export const isValidCodec = (codec: string): boolean => {
  const validCodecs = [
    'h264', 'h265', 'hevc', 'vp8', 'vp9', 'av1',
    'aac', 'mp3', 'opus', 'vorbis', 'flac', 'dts'
  ];
  
  return validCodecs.includes(codec.toLowerCase());
};