export interface BadgePreferences {
  essential: {
    new: true // Always shown (not configurable)
    watched: true // Always shown (not configurable)
    progress: true // Always shown (not configurable)
  }
  optional: {
    resolution: boolean // Default: false
    contentRating: boolean // Default: false
    codec: boolean // Default: false
    extra: boolean // Default: false
    mediaType: boolean // Default: false
  }
}

export const DEFAULT_BADGE_PREFERENCES: BadgePreferences = {
  essential: {
    new: true,
    watched: true,
    progress: true,
  },
  optional: {
    resolution: false,
    contentRating: false,
    codec: false,
    extra: false,
    mediaType: false,
  },
}
