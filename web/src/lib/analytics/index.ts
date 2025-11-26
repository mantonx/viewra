export {
  createAnalyticsState,
  startSession,
  endSession,
  recordQualitySwitch,
  recordStallEvent,
  recordPlayTime,
  shouldFlush,
  getEventsToFlush,
  requeueEvents,
  flushEvents,
  flushEventsBeacon,
  DEFAULT_ANALYTICS_CONFIG,
} from './PlaybackAnalytics'

export type {
  QualitySwitchEvent,
  PlaybackSession,
  PlaybackAnalyticsConfig,
  PlaybackAnalyticsState,
} from './PlaybackAnalytics'
