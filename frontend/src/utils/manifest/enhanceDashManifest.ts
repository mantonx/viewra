/**
 * Enhance DASH manifest for better compatibility with dash.js
 * 
 * Some DASH players have issues with relative URLs in manifests.
 * This utility adds BaseURL elements to help with URL resolution.
 */

export async function enhanceDashManifest(manifestUrl: string): Promise<string> {
  try {
    // Fetch the original manifest
    const response = await fetch(manifestUrl);
    if (!response.ok) {
      throw new Error(`Failed to fetch manifest: ${response.status}`);
    }
    
    const manifestText = await response.text();
    
    // Extract base URL from manifest URL
    const baseUrl = manifestUrl.substring(0, manifestUrl.lastIndexOf('/') + 1);
    
    // Parse the manifest
    const parser = new DOMParser();
    const doc = parser.parseFromString(manifestText, 'application/xml');
    
    // Check if BaseURL already exists at MPD level
    const mpd = doc.querySelector('MPD');
    const existingBaseUrl = mpd?.querySelector(':scope > BaseURL');
    
    if (existingBaseUrl) {
      console.log('Manifest already has BaseURL');
      return manifestText;
    }
    
    // Add BaseURL as the first child of MPD element
    if (mpd) {
      const baseUrlElement = doc.createElement('BaseURL');
      baseUrlElement.textContent = baseUrl;
      mpd.insertBefore(baseUrlElement, mpd.firstChild);
      
      console.log('Added BaseURL to manifest:', baseUrl);
    }
    
    // Serialize back to string
    const serializer = new XMLSerializer();
    return serializer.serializeToString(doc);
  } catch (error) {
    console.error('Failed to enhance DASH manifest:', error);
    // Return original URL as fallback
    return manifestUrl;
  }
}

/**
 * Create a data URL from manifest content
 */
export function createManifestDataUrl(manifestContent: string): string {
  const blob = new Blob([manifestContent], { type: 'application/dash+xml' });
  return URL.createObjectURL(blob);
}