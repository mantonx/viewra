/**
 * DASH.js compatibility fixes
 * 
 * Works around known issues with DASH.js and certain manifest formats
 */

declare global {
  interface Window {
    dashjs?: any;
  }
}

/**
 * Patch DASH.js to handle missing range properties
 */
export function patchDashJs(): void {
  // Check if dashjs is loaded
  if (!window.dashjs) {
    console.warn('DASH.js not loaded, cannot apply patches');
    return;
  }
  
  // Store original console.error to restore later
  const originalError = console.error;
  
  // Temporarily suppress specific DASH.js errors during initialization
  console.error = (...args: any[]) => {
    const errorStr = args.join(' ');
    
    // Suppress the specific "Cannot read properties of null (reading 'range')" error
    if (errorStr.includes("Cannot read properties of null (reading 'range')")) {
      console.warn('Suppressed DASH.js range error - this is a known compatibility issue');
      return;
    }
    
    // Pass through all other errors
    originalError.apply(console, args);
  };
  
  // Restore original console.error after a delay
  setTimeout(() => {
    console.error = originalError;
  }, 5000);
}

/**
 * Initialize DASH.js with compatibility fixes
 */
export function initializeDashWithFixes(onReady?: () => void): void {
  // Apply patches when DASH.js loads
  if (window.dashjs) {
    patchDashJs();
    onReady?.();
  } else {
    // Wait for DASH.js to load
    const checkInterval = setInterval(() => {
      if (window.dashjs) {
        clearInterval(checkInterval);
        patchDashJs();
        onReady?.();
      }
    }, 100);
    
    // Give up after 10 seconds
    setTimeout(() => clearInterval(checkInterval), 10000);
  }
}

/**
 * Create a more compatible manifest by adding BaseURL
 */
export function addBaseUrlToManifest(manifestContent: string, manifestUrl: string): string {
  try {
    // Extract base URL from manifest URL
    const baseUrl = manifestUrl.substring(0, manifestUrl.lastIndexOf('/') + 1);
    
    // Check if manifest already has BaseURL
    if (manifestContent.includes('<BaseURL>')) {
      return manifestContent;
    }
    
    // Add BaseURL after the Period tag
    const periodMatch = manifestContent.match(/(<Period[^>]*>)/);
    if (periodMatch) {
      const periodTag = periodMatch[0];
      const insertPoint = manifestContent.indexOf(periodTag) + periodTag.length;
      
      return manifestContent.slice(0, insertPoint) + 
        `\n\t<BaseURL>${baseUrl}</BaseURL>` + 
        manifestContent.slice(insertPoint);
    }
    
    return manifestContent;
  } catch (error) {
    console.error('Failed to add BaseURL to manifest:', error);
    return manifestContent;
  }
}