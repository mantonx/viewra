/**
 * Analytics and Session Tracking
 * 
 * Provides comprehensive playback session tracking for debugging,
 * performance monitoring, and future dashboard features.
 */

export type {
  PlaybackSessionInfo,
  SessionEvent,
  SessionAnalytics,
} from './analytics.types';

export {
  PlaybackSessionTracker,
} from './sessionTracker';