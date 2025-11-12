/**
 * Types for the custom fetch instance
 */

export interface ErrorResponse {
  error: string
  message?: string
}

export interface CustomInstanceConfig extends RequestInit {
  url: string
  params?: Record<string, unknown>
}
