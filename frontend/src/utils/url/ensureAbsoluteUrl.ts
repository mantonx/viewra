/**
 * Ensure a URL is absolute
 */
export function ensureAbsoluteUrl(url: string): string {
  if (!url) return '';
  
  // Already absolute
  if (url.startsWith('http://') || url.startsWith('https://')) {
    return url;
  }
  
  // Protocol-relative
  if (url.startsWith('//')) {
    return window.location.protocol + url;
  }
  
  // Root-relative
  if (url.startsWith('/')) {
    return window.location.origin + url;
  }
  
  // Relative - append to current path
  const base = window.location.href.substring(0, window.location.href.lastIndexOf('/') + 1);
  return base + url;
}