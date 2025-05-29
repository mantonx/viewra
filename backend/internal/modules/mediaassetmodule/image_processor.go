package mediaassetmodule

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

// ImageProcessor handles image conversion and quality adjustments
type ImageProcessor struct{}

// NewImageProcessor creates a new image processor instance
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{}
}

// ConvertToWebP converts an image to WebP format at full quality
func (ip *ImageProcessor) ConvertToWebP(data []byte, originalMimeType string) ([]byte, error) {
	// Decode the original image
	img, err := ip.decodeImage(data, originalMimeType)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Encode as WebP at 100% quality (lossless)
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Lossless: true}); err != nil {
		return nil, fmt.Errorf("failed to encode as WebP: %w", err)
	}

	return buf.Bytes(), nil
}

// ProcessImageWithQuality processes an image and adjusts quality if needed
func (ip *ImageProcessor) ProcessImageWithQuality(data []byte, originalMimeType string, quality int) ([]byte, string, error) {
	// Validate quality parameter (1-100)
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	// If quality is 100, return the original data for WebP, or convert to WebP for other formats
	if quality == 100 {
		if originalMimeType == "image/webp" {
			return data, originalMimeType, nil
		}
		// Convert to WebP at full quality
		webpData, err := ip.ConvertToWebP(data, originalMimeType)
		if err != nil {
			// Fallback to original if conversion fails
			return data, originalMimeType, nil
		}
		return webpData, "image/webp", nil
	}

	// For quality < 100, decode and re-encode with quality adjustment
	img, err := ip.decodeImage(data, originalMimeType)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Re-encode with quality adjustment
	return ip.encodeImageWithQuality(img, quality)
}

// decodeImage decodes an image from bytes based on MIME type
func (ip *ImageProcessor) decodeImage(data []byte, mimeType string) (image.Image, error) {
	reader := bytes.NewReader(data)

	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg":
		return jpeg.Decode(reader)
	case "image/png":
		return png.Decode(reader)
	case "image/webp":
		return webp.Decode(reader)
	case "image/gif":
		return imaging.Decode(reader)
	case "image/bmp":
		return imaging.Decode(reader)
	case "image/tiff":
		return imaging.Decode(reader)
	default:
		// Try generic decode
		return imaging.Decode(reader)
	}
}

// encodeImageWithQuality encodes an image with the specified quality
func (ip *ImageProcessor) encodeImageWithQuality(img image.Image, quality int) ([]byte, string, error) {
	var buf bytes.Buffer

	// Use WebP for quality-adjusted images as it provides better compression
	// Convert quality percentage to WebP quality factor (0-100)
	webpQuality := float32(quality)
	options := &webp.Options{
		Lossless: false,
		Quality:  webpQuality,
	}
	
	if err := webp.Encode(&buf, img, options); err != nil {
		// Fallback to JPEG if WebP encoding fails
		buf.Reset()
		jpegOptions := &jpeg.Options{Quality: quality}
		if err := jpeg.Encode(&buf, img, jpegOptions); err != nil {
			return nil, "", fmt.Errorf("failed to encode image with quality %d: %w", quality, err)
		}
		return buf.Bytes(), "image/jpeg", nil
	}

	return buf.Bytes(), "image/webp", nil
}

// GetImageDimensions extracts width and height from image data
func (ip *ImageProcessor) GetImageDimensions(data []byte, mimeType string) (int, int, error) {
	img, err := ip.decodeImage(data, mimeType)
	if err != nil {
		return 0, 0, err
	}

	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}

// IsImageMimeType checks if a MIME type represents an image
func (ip *ImageProcessor) IsImageMimeType(mimeType string) bool {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", 
		 "image/webp", "image/bmp", "image/tiff", "image/svg+xml":
		return true
	default:
		return strings.HasPrefix(strings.ToLower(mimeType), "image/")
	}
}

// ProcessAssetData processes asset data for saving - converts images to WebP at full quality
func (ip *ImageProcessor) ProcessAssetData(data []byte, mimeType string) ([]byte, string, int, int, error) {
	// Only process images
	if !ip.IsImageMimeType(mimeType) {
		return data, mimeType, 0, 0, nil
	}

	// Get dimensions first
	width, height, err := ip.GetImageDimensions(data, mimeType)
	if err != nil {
		// If we can't get dimensions, return original data but with zero dimensions
		return data, mimeType, 0, 0, nil
	}

	// If it's already WebP, just return it
	if strings.ToLower(mimeType) == "image/webp" {
		return data, mimeType, width, height, nil
	}

	// Convert to WebP at full quality for storage
	webpData, err := ip.ConvertToWebP(data, mimeType)
	if err != nil {
		// If conversion fails, return original data
		return data, mimeType, width, height, nil
	}

	return webpData, "image/webp", width, height, nil
} 