export interface ErrorPageProps {
  /**
   * The error object to display
   */
  error: unknown

  /**
   * Context for the error message (e.g., "libraries", "media")
   * Used to create message like "Error loading libraries"
   */
  context: string
}
