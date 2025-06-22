/**
 * Video-related utility functions
 */

/**
 * Gets the buffered ranges from a video element
 * @param video - HTMLVideoElement
 * @returns Array of buffered time ranges
 */
export const getBufferedRanges = (video: HTMLVideoElement): Array<{start: number, end: number}> => {
  const ranges: Array<{start: number, end: number}> = [];
  
  if (video.buffered) {
    for (let i = 0; i < video.buffered.length; i++) {
      ranges.push({
        start: video.buffered.start(i),
        end: video.buffered.end(i)
      });
    }
  }
  
  return ranges;
};

/**
 * Gets the seekable ranges from a video element
 * @param video - HTMLVideoElement
 * @returns Array of seekable time ranges
 */
export const getSeekableRanges = (video: HTMLVideoElement): Array<{start: number, end: number}> => {
  const ranges: Array<{start: number, end: number}> = [];
  
  if (video.seekable) {
    for (let i = 0; i < video.seekable.length; i++) {
      ranges.push({
        start: video.seekable.start(i),
        end: video.seekable.end(i)
      });
    }
  }
  
  return ranges;
};

/**
 * Gets the total buffered duration
 * @param video - HTMLVideoElement
 * @returns Total buffered time in seconds
 */
export const getTotalBuffered = (video: HTMLVideoElement): number => {
  const ranges = getBufferedRanges(video);
  return ranges.reduce((total, range) => total + (range.end - range.start), 0);
};

/**
 * Gets the buffered amount at current position
 * @param video - HTMLVideoElement
 * @param currentTime - Current playback time (optional, uses video.currentTime)
 * @returns Buffered time ahead of current position
 */
export const getBufferedAhead = (video: HTMLVideoElement, currentTime?: number): number => {
  const time = currentTime ?? video.currentTime;
  const ranges = getBufferedRanges(video);
  
  for (const range of ranges) {
    if (time >= range.start && time <= range.end) {
      return range.end - time;
    }
  }
  
  return 0;
};

/**
 * Checks if a time position is buffered
 * @param video - HTMLVideoElement
 * @param time - Time to check
 * @returns Whether the time is buffered
 */
export const isTimeBuffered = (video: HTMLVideoElement, time: number): boolean => {
  const ranges = getBufferedRanges(video);
  
  return ranges.some(range => time >= range.start && time <= range.end);
};

/**
 * Gets the maximum buffered end time
 * @param video - HTMLVideoElement
 * @returns Latest buffered time
 */
export const getMaxBufferedTime = (video: HTMLVideoElement): number => {
  const ranges = getBufferedRanges(video);
  
  if (ranges.length === 0) return 0;
  
  return Math.max(...ranges.map(range => range.end));
};

/**
 * Gets the maximum seekable end time
 * @param video - HTMLVideoElement
 * @returns Latest seekable time
 */
export const getMaxSeekableTime = (video: HTMLVideoElement): number => {
  const ranges = getSeekableRanges(video);
  
  if (ranges.length === 0) return 0;
  
  return Math.max(...ranges.map(range => range.end));
};

/**
 * Tries to detect duration from multiple sources
 * @param video - HTMLVideoElement
 * @returns Detected duration in seconds
 */
export const detectDuration = (video: HTMLVideoElement): number => {
  // Try video.duration first
  if (isFinite(video.duration) && video.duration > 0) {
    return video.duration;
  }
  
  // Try seekable range
  const maxSeekable = getMaxSeekableTime(video);
  if (maxSeekable > 0) {
    return maxSeekable;
  }
  
  // Try buffered range as last resort
  const maxBuffered = getMaxBufferedTime(video);
  if (maxBuffered > 0) {
    return maxBuffered;
  }
  
  return 0;
};

/**
 * Checks if video can seek to a specific time
 * @param video - HTMLVideoElement
 * @param time - Time to check
 * @returns Whether seeking to this time is possible
 */
export const canSeekTo = (video: HTMLVideoElement, time: number): boolean => {
  const ranges = getSeekableRanges(video);
  
  return ranges.some(range => time >= range.start && time <= range.end);
};

/**
 * Gets video readiness state description
 * @param readyState - Video readyState value
 * @returns Human-readable description
 */
export const getReadyStateDescription = (readyState: number): string => {
  switch (readyState) {
    case 0: return 'HAVE_NOTHING';
    case 1: return 'HAVE_METADATA';
    case 2: return 'HAVE_CURRENT_DATA';
    case 3: return 'HAVE_FUTURE_DATA';
    case 4: return 'HAVE_ENOUGH_DATA';
    default: return 'UNKNOWN';
  }
};

/**
 * Checks if video has enough data to play
 * @param video - HTMLVideoElement
 * @returns Whether video can play
 */
export const hasEnoughData = (video: HTMLVideoElement): boolean => {
  return video.readyState >= 3; // HAVE_FUTURE_DATA or higher
};

/**
 * Gets video network state description
 * @param networkState - Video networkState value
 * @returns Human-readable description
 */
export const getNetworkStateDescription = (networkState: number): string => {
  switch (networkState) {
    case 0: return 'NETWORK_EMPTY';
    case 1: return 'NETWORK_IDLE';
    case 2: return 'NETWORK_LOADING';
    case 3: return 'NETWORK_NO_SOURCE';
    default: return 'UNKNOWN';
  }
};

/**
 * Calculates video aspect ratio
 * @param video - HTMLVideoElement
 * @returns Aspect ratio (width/height)
 */
export const getAspectRatio = (video: HTMLVideoElement): number => {
  if (video.videoWidth && video.videoHeight) {
    return video.videoWidth / video.videoHeight;
  }
  return 16 / 9; // Default aspect ratio
};

/**
 * Checks if video is playing
 * @param video - HTMLVideoElement
 * @returns Whether video is currently playing
 */
export const isPlaying = (video: HTMLVideoElement): boolean => {
  return !video.paused && !video.ended && video.readyState > 2;
};

/**
 * Gets video quality information
 * @param video - HTMLVideoElement
 * @returns Video quality info
 */
export const getVideoQuality = (video: HTMLVideoElement): {
  width: number;
  height: number;
  resolution: string;
} => {
  return {
    width: video.videoWidth,
    height: video.videoHeight,
    resolution: `${video.videoWidth}x${video.videoHeight}`
  };
};