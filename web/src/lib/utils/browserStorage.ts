/**
 * Generic browser storage utilities
 * Handles localStorage operations with graceful fallbacks
 */

/**
 * Get a value from localStorage with type safety
 * @param key - Storage key
 * @param defaultValue - Default value if key doesn't exist or parsing fails
 * @returns Parsed value or default
 */
export const getFromStorage = <T>(key: string, defaultValue: T): T => {
  try {
    const stored = localStorage.getItem(key)
    return stored ? JSON.parse(stored) : defaultValue
  } catch {
    return defaultValue
  }
}

/**
 * Save a value to localStorage
 * @param key - Storage key
 * @param value - Value to store (will be JSON stringified)
 */
export const saveToStorage = <T>(key: string, value: T): void => {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Silently fail if localStorage is unavailable
  }
}

/**
 * Remove a value from localStorage
 * @param key - Storage key to remove
 */
export const removeFromStorage = (key: string): void => {
  try {
    localStorage.removeItem(key)
  } catch {
    // Silently fail if localStorage is unavailable
  }
}

/**
 * Manage a list of recent items with automatic deduplication and size limits
 * @param key - Storage key for the list
 * @param item - Item to add to the list
 * @param maxItems - Maximum number of items to keep (default: 10)
 */
export const addToRecentList = <T>(key: string, item: T, maxItems = 10): void => {
  try {
    const recent = getFromStorage<T[]>(key, [])
    // Remove duplicates (using JSON comparison for objects)
    const filtered = recent.filter((i) => JSON.stringify(i) !== JSON.stringify(item))
    const updated = [item, ...filtered].slice(0, maxItems)
    saveToStorage(key, updated)
  } catch {
    // Silently fail if localStorage is unavailable
  }
}

/**
 * Get a list of recent items
 * @param key - Storage key for the list
 * @param defaultValue - Default value if list doesn't exist
 * @returns Array of recent items
 */
export const getRecentList = <T>(key: string, defaultValue: T[] = []): T[] => {
  return getFromStorage<T[]>(key, defaultValue)
}

/**
 * Clear a recent items list
 * @param key - Storage key for the list
 */
export const clearRecentList = (key: string): void => {
  removeFromStorage(key)
}
