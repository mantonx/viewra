/**
 * Time formatting utilities for media player.
 * 
 * This module provides a comprehensive set of time-related utilities for media playback,
 * including formatting durations, calculating progress, parsing time strings, and
 * handling edge cases like invalid or infinite values.
 */

/**
 * Formats time in seconds to readable string (HH:MM:SS or MM:SS)
 * @param time - Time in seconds
 * @returns Formatted time string
 */
export const formatTime = (time: number): string => {
  // Handle invalid time values
  if (!isFinite(time) || isNaN(time) || time < 0) {
    return '0:00';
  }
  
  const hours = Math.floor(time / 3600);
  const minutes = Math.floor((time % 3600) / 60);
  const seconds = Math.floor(time % 60);
  
  if (hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
  }
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
};

/**
 * Formats time as percentage of duration
 * @param currentTime - Current playback time in seconds
 * @param duration - Total duration in seconds
 * @returns Progress as percentage (0-100)
 */
export const formatProgress = (currentTime: number, duration: number): number => {
  if (!duration || duration <= 0 || !isFinite(currentTime) || !isFinite(duration)) {
    return 0;
  }
  
  return Math.max(0, Math.min(100, (currentTime / duration) * 100));
};

/**
 * Parses time string (MM:SS or HH:MM:SS) to seconds
 * @param timeString - Time string to parse
 * @returns Time in seconds
 */
export const parseTimeString = (timeString: string): number => {
  if (!timeString) return 0;
  
  const parts = timeString.split(':').map(part => parseInt(part, 10));
  
  if (parts.length === 2) {
    // MM:SS
    const [minutes, seconds] = parts;
    return minutes * 60 + seconds;
  } else if (parts.length === 3) {
    // HH:MM:SS
    const [hours, minutes, seconds] = parts;
    return hours * 3600 + minutes * 60 + seconds;
  }
  
  return 0;
};

/**
 * Clamps time value between 0 and duration
 * @param time - Time to clamp
 * @param duration - Maximum duration
 * @returns Clamped time value
 */
export const clampTime = (time: number, duration: number): number => {
  if (!isFinite(time) || !isFinite(duration)) {
    return 0;
  }
  
  return Math.max(0, Math.min(time, duration));
};

/**
 * Calculates remaining time
 * @param currentTime - Current playback time
 * @param duration - Total duration
 * @returns Remaining time in seconds
 */
export const getRemainingTime = (currentTime: number, duration: number): number => {
  if (!isFinite(currentTime) || !isFinite(duration) || duration <= 0) {
    return 0;
  }
  
  return Math.max(0, duration - currentTime);
};

/**
 * Formats remaining time with minus sign
 * @param currentTime - Current playback time
 * @param duration - Total duration
 * @returns Formatted remaining time string (e.g., "-1:23")
 */
export const formatRemainingTime = (currentTime: number, duration: number): string => {
  // Handle invalid inputs
  if (!isFinite(currentTime) || !isFinite(duration) || duration <= 0) {
    return '--:--';
  }
  
  // Ensure currentTime doesn't exceed duration to prevent negative display
  const clampedTime = Math.min(currentTime, duration);
  const remaining = Math.max(0, duration - clampedTime);
  
  return `-${formatTime(remaining)}`;
};

/**
 * Validates if time value is valid for video playback
 * @param time - Time to validate
 * @returns Whether time is valid
 */
export const isValidTime = (time: number): boolean => {
  return isFinite(time) && !isNaN(time) && time >= 0;
};

/**
 * Rounds time to nearest second
 * @param time - Time to round
 * @returns Rounded time
 */
export const roundTime = (time: number): number => {
  return Math.round(time);
};

/**
 * Formats time for display in progress tooltip
 * @param time - Time in seconds
 * @param showHours - Whether to show hours even for short durations
 * @returns Formatted time string
 */
export const formatTooltipTime = (time: number, showHours: boolean = false): string => {
  if (!isValidTime(time)) {
    return '0:00';
  }
  
  const hours = Math.floor(time / 3600);
  const minutes = Math.floor((time % 3600) / 60);
  const seconds = Math.floor(time % 60);
  
  if (showHours || hours > 0) {
    return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
  }
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
};