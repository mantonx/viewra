/**
 * Path utilities for working with file paths in the browser.
 */

/**
 * Extract the filename from a file path.
 * Works with both Unix and Windows paths.
 *
 * @example
 * basename('/home/user/movies/movie.mkv') // 'movie.mkv'
 * basename('C:\\Users\\movies\\movie.mkv') // 'movie.mkv'
 * basename('movie.mkv') // 'movie.mkv'
 */
export const basename = (path: string): string => {
  if (!path) return ''

  // Handle both Unix and Windows path separators
  const lastSlash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))

  if (lastSlash === -1) {
    return path
  }

  return path.slice(lastSlash + 1)
}

/**
 * Extract the directory from a file path.
 * Works with both Unix and Windows paths.
 *
 * @example
 * dirname('/home/user/movies/movie.mkv') // '/home/user/movies'
 * dirname('C:\\Users\\movies\\movie.mkv') // 'C:\\Users\\movies'
 * dirname('movie.mkv') // ''
 */
export const dirname = (path: string): string => {
  if (!path) return ''

  // Handle both Unix and Windows path separators
  const lastSlash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))

  if (lastSlash === -1) {
    return ''
  }

  return path.slice(0, lastSlash)
}
