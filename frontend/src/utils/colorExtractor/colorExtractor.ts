import type { ColorPalette } from './colorExtractor.types';

/**
 * Extracts a color palette from an image using canvas-based pixel sampling
 * 
 * This function loads an image, renders it to a small canvas, and analyzes
 * the pixel data to extract dominant colors. It uses color clustering to
 * group similar colors and returns the most frequent ones as a palette.
 * 
 * @param imageUrl - URL of the image to analyze (must be CORS-enabled)
 * @returns Promise resolving to a ColorPalette with primary, secondary, and accent colors
 * 
 * @example
 * ```typescript
 * const palette = await extractColorsFromImage('/poster.jpg');
 * console.log(palette.primary); // 'rgb(139, 69, 19)'
 * ```
 * 
 * @remarks
 * - Uses a 50x50 canvas for performance optimization
 * - Skips transparent, very dark, or very light pixels
 * - Groups similar colors using a threshold of 30 RGB units
 * - Returns fallback colors if extraction fails
 * - Requires images to be CORS-enabled (crossOrigin = 'anonymous')
 */
export const extractColorsFromImage = async (imageUrl: string): Promise<ColorPalette> => {
  return new Promise((resolve) => {
    const img = new Image();
    img.crossOrigin = 'anonymous';

    img.onload = () => {
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');

      if (!ctx) {
        // Fallback colors if canvas context is not available
        resolve({
          primary: 'rgb(139, 69, 19)', // Brown
          secondary: 'rgb(75, 85, 99)', // Slate
          accent: 'rgb(147, 51, 234)', // Purple
        });
        return;
      }

      // Set canvas size to a small size for faster processing
      canvas.width = 50;
      canvas.height = 50;

      // Draw the image onto the canvas
      ctx.drawImage(img, 0, 0, 50, 50);

      try {
        const imageData = ctx.getImageData(0, 0, 50, 50);
        const pixels = imageData.data;

        // Simple color extraction - get average colors from different regions
        const colors: Array<{ r: number; g: number; b: number; count: number }> = [];

        // Sample pixels (every 4th pixel to reduce processing)
        for (let i = 0; i < pixels.length; i += 16) {
          // 16 = 4 pixels * 4 channels (RGBA)
          const r = pixels[i];
          const g = pixels[i + 1];
          const b = pixels[i + 2];
          const a = pixels[i + 3];

          // Skip transparent or very dark/light pixels
          if (a < 128 || r + g + b < 30 || r + g + b > 700) continue;

          // Find similar color or add new one
          const existingColor = colors.find(
            (color) =>
              Math.abs(color.r - r) < 30 && Math.abs(color.g - g) < 30 && Math.abs(color.b - b) < 30
          );

          if (existingColor) {
            existingColor.count++;
            // Update average
            existingColor.r = Math.round((existingColor.r + r) / 2);
            existingColor.g = Math.round((existingColor.g + g) / 2);
            existingColor.b = Math.round((existingColor.b + b) / 2);
          } else {
            colors.push({ r, g, b, count: 1 });
          }
        }

        // Sort by frequency and get top 3 colors
        colors.sort((a, b) => b.count - a.count);

        const getColorString = (color: { r: number; g: number; b: number }) =>
          `rgb(${color.r}, ${color.g}, ${color.b})`;

        const palette: ColorPalette = {
          primary: colors[0] ? getColorString(colors[0]) : 'rgb(139, 69, 19)',
          secondary: colors[1] ? getColorString(colors[1]) : 'rgb(75, 85, 99)',
          accent: colors[2] ? getColorString(colors[2]) : 'rgb(147, 51, 234)',
        };

        resolve(palette);
      } catch {
        // Fallback colors if color extraction fails
        resolve({
          primary: 'rgb(139, 69, 19)',
          secondary: 'rgb(75, 85, 99)',
          accent: 'rgb(147, 51, 234)',
        });
      }
    };

    img.onerror = () => {
      // Fallback colors if image fails to load
      resolve({
        primary: 'rgb(139, 69, 19)',
        secondary: 'rgb(75, 85, 99)',
        accent: 'rgb(147, 51, 234)',
      });
    };

    img.src = imageUrl;
  });
};
