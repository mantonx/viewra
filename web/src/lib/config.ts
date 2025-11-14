/**
 * Application configuration constants.
 * Centralized location for all environment-based configuration.
 */

/**
 * API Base URL for backend requests.
 * Defaults to localhost:8080 if VITE_API_BASE_URL is not set.
 */
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

/**
 * Environment mode (development, production, etc.)
 */
export const isDevelopment = import.meta.env.DEV
export const isProduction = import.meta.env.PROD
