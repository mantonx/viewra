/**
 * Frontend error handling utilities for proper error propagation and display.
 * 
 * This module provides a comprehensive error handling system that mirrors the backend's
 * error structure, enabling proper error propagation from plugins through the backend
 * to the frontend. It includes error parsing, retry logic, user-friendly messages,
 * and integration hooks for error monitoring services.
 */

/**
 * Represents an error returned by the Viewra API.
 * This structure matches the backend's error format for consistency.
 */
export interface ApiError {
  code: string;
  message: string;
  details?: string;
  user_message?: string;
  retryable: boolean;
  retry_after?: number; // seconds
  context?: Record<string, any>;
  request_id?: string;
}

export interface ErrorResponse {
  error: ApiError;
  success: false;
}

/**
 * Error codes matching backend error types.
 * These codes provide a standardized way to identify and handle
 * specific error conditions throughout the application.
 */
export enum ErrorCode {
  // General errors
  UNKNOWN_ERROR = 'UNKNOWN_ERROR',
  INTERNAL_ERROR = 'INTERNAL_ERROR',
  VALIDATION_ERROR = 'VALIDATION_ERROR',
  NOT_FOUND = 'NOT_FOUND',
  CONFLICT = 'CONFLICT',
  RATE_LIMIT = 'RATE_LIMIT',
  TIMEOUT = 'TIMEOUT',
  CANCELLED = 'CANCELLED',
  
  // Media errors
  MEDIA_NOT_FOUND = 'MEDIA_NOT_FOUND',
  MEDIA_CORRUPTED = 'MEDIA_CORRUPTED',
  MEDIA_UNSUPPORTED = 'MEDIA_UNSUPPORTED',
  MEDIA_ACCESS_DENIED = 'MEDIA_ACCESS_DENIED',
  MEDIA_IN_PROCESSING = 'MEDIA_IN_PROCESSING',
  
  // Transcoding errors
  TRANSCODING_FAILED = 'TRANSCODING_FAILED',
  TRANSCODING_UNAVAILABLE = 'TRANSCODING_UNAVAILABLE',
  TRANSCODING_IN_PROGRESS = 'TRANSCODING_IN_PROGRESS',
  TRANSCODING_CANCELLED = 'TRANSCODING_CANCELLED',
  TRANSCODING_TIMEOUT = 'TRANSCODING_TIMEOUT',
  
  // Plugin errors
  PLUGIN_NOT_FOUND = 'PLUGIN_NOT_FOUND',
  PLUGIN_FAILED = 'PLUGIN_FAILED',
  PLUGIN_TIMEOUT = 'PLUGIN_TIMEOUT',
  PLUGIN_UNAVAILABLE = 'PLUGIN_UNAVAILABLE',
  PLUGIN_CONFIG = 'PLUGIN_CONFIG_ERROR',
  
  // FFmpeg specific errors
  FFMPEG_NOT_FOUND = 'FFMPEG_NOT_FOUND',
  FFMPEG_FAILED = 'FFMPEG_FAILED',
  FFMPEG_KILLED = 'FFMPEG_KILLED',
  FFMPEG_INVALID_ARGS = 'FFMPEG_INVALID_ARGS',
  FFMPEG_UNSUPPORTED = 'FFMPEG_UNSUPPORTED',
  
  // Resource errors
  RESOURCE_EXHAUSTED = 'RESOURCE_EXHAUSTED',
  DISK_FULL = 'DISK_FULL',
  MEMORY_EXHAUSTED = 'MEMORY_EXHAUSTED',
  CPU_OVERLOADED = 'CPU_OVERLOADED',
  GPU_UNAVAILABLE = 'GPU_UNAVAILABLE',
  
  // Session errors
  SESSION_NOT_FOUND = 'SESSION_NOT_FOUND',
  SESSION_EXPIRED = 'SESSION_EXPIRED',
  SESSION_INVALID = 'SESSION_INVALID',
  SESSION_LIMIT_REACHED = 'SESSION_LIMIT_REACHED',
}

/**
 * Enhanced error class for Viewra application errors.
 * Extends the native Error class with additional metadata for better
 * error handling, display, and recovery options.
 */
export class ViewraError extends Error {
  code: string;
  details?: string;
  userMessage?: string;
  retryable: boolean;
  retryAfter?: number;
  context?: Record<string, any>;
  requestId?: string;

  constructor(error: ApiError | Error | string) {
    if (typeof error === 'string') {
      super(error);
      this.code = ErrorCode.UNKNOWN_ERROR;
      this.retryable = false;
    } else if (error instanceof Error) {
      super(error.message);
      this.code = ErrorCode.UNKNOWN_ERROR;
      this.retryable = false;
    } else {
      super(error.message);
      this.code = error.code;
      this.details = error.details;
      this.userMessage = error.user_message;
      this.retryable = error.retryable;
      this.retryAfter = error.retry_after;
      this.context = error.context;
      this.requestId = error.request_id;
    }

    this.name = 'ViewraError';
  }

  /**
   * Get user-friendly message suitable for display to end users.
   * Provides context-appropriate messages that avoid technical jargon.
   */
  getUserMessage(): string {
    if (this.userMessage) {
      return this.userMessage;
    }

    // Provide default user messages based on error code
    switch (this.code) {
      case ErrorCode.MEDIA_NOT_FOUND:
        return 'The requested media file could not be found.';
      case ErrorCode.MEDIA_UNSUPPORTED:
        return 'This media format is not supported.';
      case ErrorCode.TRANSCODING_UNAVAILABLE:
        return 'Video processing is temporarily unavailable. Please try again later.';
      case ErrorCode.TRANSCODING_FAILED:
        return 'Failed to process the video. Please try again.';
      case ErrorCode.SESSION_LIMIT_REACHED:
        return 'Too many active video streams. Please try again in a few minutes.';
      case ErrorCode.RESOURCE_EXHAUSTED:
      case ErrorCode.DISK_FULL:
      case ErrorCode.MEMORY_EXHAUSTED:
        return 'The server is experiencing high load. Please try again later.';
      case ErrorCode.TIMEOUT:
      case ErrorCode.TRANSCODING_TIMEOUT:
        return 'The request took too long. Please try again.';
      case ErrorCode.VALIDATION_ERROR:
        return 'Invalid request. Please check your input and try again.';
      case ErrorCode.NOT_FOUND:
      case ErrorCode.SESSION_NOT_FOUND:
        return 'The requested resource was not found.';
      default:
        return 'An unexpected error occurred. Please try again.';
    }
  }

  /**
   * Check if error is retryable.
   * Retryable errors are typically temporary conditions that may
   * succeed if attempted again.
   */
  isRetryable(): boolean {
    return this.retryable;
  }

  /**
   * Get retry delay in milliseconds.
   * Returns server-suggested delay or intelligent defaults based
   * on the error type.
   */
  getRetryDelay(): number {
    if (this.retryAfter) {
      return this.retryAfter * 1000;
    }
    
    // Default retry delays based on error type
    switch (this.code) {
      case ErrorCode.RATE_LIMIT:
        return 60000; // 1 minute
      case ErrorCode.RESOURCE_EXHAUSTED:
      case ErrorCode.SESSION_LIMIT_REACHED:
        return 30000; // 30 seconds
      case ErrorCode.TIMEOUT:
      case ErrorCode.TRANSCODING_TIMEOUT:
        return 10000; // 10 seconds
      default:
        return 5000; // 5 seconds
    }
  }

  /**
   * Check if error is due to temporary condition.
   * Temporary errors are expected to resolve themselves over time
   * without user intervention.
   */
  isTemporary(): boolean {
    return [
      ErrorCode.TRANSCODING_UNAVAILABLE,
      ErrorCode.PLUGIN_UNAVAILABLE,
      ErrorCode.RESOURCE_EXHAUSTED,
      ErrorCode.DISK_FULL,
      ErrorCode.MEMORY_EXHAUSTED,
      ErrorCode.CPU_OVERLOADED,
      ErrorCode.GPU_UNAVAILABLE,
      ErrorCode.RATE_LIMIT,
      ErrorCode.TIMEOUT,
      ErrorCode.TRANSCODING_TIMEOUT,
      ErrorCode.SESSION_LIMIT_REACHED,
    ].includes(this.code as ErrorCode);
  }

  /**
   * Check if error is critical (should be reported).
   * Critical errors indicate system-level issues that require
   * investigation or administrative action.
   */
  isCritical(): boolean {
    return [
      ErrorCode.INTERNAL_ERROR,
      ErrorCode.FFMPEG_NOT_FOUND,
      ErrorCode.PLUGIN_CONFIG,
    ].includes(this.code as ErrorCode);
  }
}

/**
 * Parse error response from API.
 * Converts various error response formats into a consistent ViewraError
 * instance with appropriate error codes and metadata.
 */
export function parseApiError(response: Response, body?: any): ViewraError {
  // Check if body has error structure
  if (body?.error) {
    return new ViewraError(body.error);
  }

  // Fallback to simple error message
  if (body?.message || body?.error) {
    return new ViewraError({
      code: ErrorCode.UNKNOWN_ERROR,
      message: body.message || body.error || 'Unknown error',
      retryable: false,
    });
  }

  // Parse based on HTTP status
  switch (response.status) {
    case 400:
      return new ViewraError({
        code: ErrorCode.VALIDATION_ERROR,
        message: 'Bad request',
        retryable: false,
      });
    case 404:
      return new ViewraError({
        code: ErrorCode.NOT_FOUND,
        message: 'Resource not found',
        retryable: false,
      });
    case 429:
      return new ViewraError({
        code: ErrorCode.RATE_LIMIT,
        message: 'Rate limit exceeded',
        retryable: true,
        retry_after: parseInt(response.headers.get('Retry-After') || '60'),
      });
    case 503:
      return new ViewraError({
        code: ErrorCode.TRANSCODING_UNAVAILABLE,
        message: 'Service unavailable',
        retryable: true,
        retry_after: parseInt(response.headers.get('Retry-After') || '30'),
      });
    case 504:
      return new ViewraError({
        code: ErrorCode.TIMEOUT,
        message: 'Request timeout',
        retryable: true,
      });
    default:
      return new ViewraError({
        code: ErrorCode.INTERNAL_ERROR,
        message: `HTTP ${response.status}: ${response.statusText}`,
        retryable: response.status >= 500,
      });
  }
}

/**
 * Enhanced fetch with error handling.
 * Wraps the native fetch API to automatically parse errors and convert
 * them to ViewraError instances for consistent error handling.
 */
export async function fetchWithErrorHandling(
  url: string,
  options?: RequestInit
): Promise<Response> {
  try {
    const response = await fetch(url, options);
    
    if (!response.ok) {
      let body;
      try {
        body = await response.json();
      } catch {
        // Response is not JSON
      }
      
      throw parseApiError(response, body);
    }
    
    return response;
  } catch (error) {
    if (error instanceof ViewraError) {
      throw error;
    }
    
    // Network errors
    if (error instanceof TypeError && error.message === 'Failed to fetch') {
      throw new ViewraError({
        code: ErrorCode.INTERNAL_ERROR,
        message: 'Network error - please check your connection',
        retryable: true,
      });
    }
    
    // Other errors
    throw new ViewraError(error as Error);
  }
}

/**
 * Retry logic for retryable errors with exponential backoff.
 * Automatically retries failed operations that are marked as retryable,
 * with increasing delays between attempts.
 */
export async function retryWithBackoff<T>(
  fn: () => Promise<T>,
  maxRetries: number = 3,
  onRetry?: (error: ViewraError, attempt: number) => void
): Promise<T> {
  let lastError: ViewraError;
  
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await fn();
    } catch (error) {
      if (!(error instanceof ViewraError)) {
        throw error;
      }
      
      lastError = error;
      
      if (!error.isRetryable() || attempt === maxRetries) {
        throw error;
      }
      
      const delay = error.getRetryDelay() * attempt;
      
      if (onRetry) {
        onRetry(error, attempt);
      }
      
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
  
  throw lastError!;
}

/**
 * Error display helper for UI components.
 * Formats errors into user-friendly display objects with appropriate
 * titles, messages, and action buttons.
 */
export function formatErrorForDisplay(error: ViewraError | Error): {
  title: string;
  message: string;
  actions?: Array<{ label: string; action: () => void }>;
} {
  if (error instanceof ViewraError) {
    const message = error.getUserMessage();
    
    const result: any = {
      title: 'Error',
      message,
    };
    
    // Add retry action if applicable
    if (error.isRetryable()) {
      result.actions = [
        {
          label: 'Retry',
          action: () => {
            // This should be handled by the component
            window.location.reload();
          },
        },
      ];
    }
    
    // Add specific actions based on error type
    switch (error.code) {
      case ErrorCode.SESSION_LIMIT_REACHED:
        result.title = 'Too Many Streams';
        break;
      case ErrorCode.MEDIA_UNSUPPORTED:
        result.title = 'Unsupported Format';
        break;
      case ErrorCode.TRANSCODING_UNAVAILABLE:
        result.title = 'Service Unavailable';
        break;
    }
    
    return result;
  }
  
  // Generic error
  return {
    title: 'Error',
    message: error.message || 'An unexpected error occurred',
  };
}

/**
 * Log error with context for debugging and monitoring.
 * Captures error details along with environmental context for
 * troubleshooting. In production, this can send to error tracking services.
 */
export function logError(error: ViewraError | Error, context?: Record<string, any>): void {
  const errorData = {
    message: error.message,
    stack: error.stack,
    ...(error instanceof ViewraError ? {
      code: error.code,
      details: error.details,
      context: error.context,
      requestId: error.requestId,
      retryable: error.retryable,
    } : {}),
    userContext: context,
    timestamp: new Date().toISOString(),
    url: window.location.href,
    userAgent: navigator.userAgent,
  };
  
  // Log to console in development
  if (process.env.NODE_ENV === 'development') {
    console.error('ViewraError:', errorData);
  }
  
  // In production, this could send to error tracking service
  // Example: Sentry, LogRocket, etc.
  if (error instanceof ViewraError && error.isCritical()) {
    // Send to error tracking service
    console.error('Critical error:', errorData);
  }
}