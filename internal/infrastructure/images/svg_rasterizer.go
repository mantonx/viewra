package images

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"os"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
)

// SVGRasterizer converts SVG files to raster images.
type SVGRasterizer struct {
	// DefaultDPI is the resolution used for rasterization.
	// Higher values produce larger, more detailed images.
	DefaultDPI float64
}

// NewSVGRasterizer creates a new SVG rasterizer with default settings.
func NewSVGRasterizer() *SVGRasterizer {
	return &SVGRasterizer{
		DefaultDPI: 96.0, // Standard screen DPI
	}
}

// IsSVG checks if a file path or URL points to an SVG file.
func IsSVG(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".svg")
}

// RasterizeFile converts an SVG file to an RGBA image.
func (r *SVGRasterizer) RasterizeFile(svgPath string) (*image.RGBA, error) {
	f, err := os.Open(svgPath)
	if err != nil {
		return nil, fmt.Errorf("open svg file: %w", err)
	}
	defer f.Close()

	return r.Rasterize(f)
}

// RasterizeBytes converts SVG bytes to an RGBA image.
func (r *SVGRasterizer) RasterizeBytes(data []byte) (*image.RGBA, error) {
	return r.Rasterize(bytes.NewReader(data))
}

// Rasterize converts SVG data from a reader to an RGBA image.
func (r *SVGRasterizer) Rasterize(reader io.Reader) (*image.RGBA, error) {
	// Parse SVG
	c, err := canvas.ParseSVG(reader)
	if err != nil {
		return nil, fmt.Errorf("parse svg: %w", err)
	}

	// Convert DPI to canvas resolution (dots per millimeter)
	// 1 inch = 25.4 mm, so DPMM = DPI / 25.4
	resolution := canvas.DPMM(r.DefaultDPI / 25.4)

	// Rasterize to RGBA image with transparency support
	img := rasterizer.Draw(c, resolution, canvas.DefaultColorSpace)

	return img, nil
}

// RasterizeWithSize converts SVG data to an RGBA image with a target size.
// The image is scaled to fit within maxWidth x maxHeight while preserving aspect ratio.
func (r *SVGRasterizer) RasterizeWithSize(reader io.Reader, maxWidth, maxHeight int) (*image.RGBA, error) {
	// Parse SVG
	c, err := canvas.ParseSVG(reader)
	if err != nil {
		return nil, fmt.Errorf("parse svg: %w", err)
	}

	// Calculate resolution to fit target size
	// Canvas dimensions are in millimeters
	svgWidth := c.W
	svgHeight := c.H

	if svgWidth <= 0 || svgHeight <= 0 {
		// Fallback to default resolution if SVG has no dimensions
		resolution := canvas.DPMM(r.DefaultDPI / 25.4)
		return rasterizer.Draw(c, resolution, canvas.DefaultColorSpace), nil
	}

	// Calculate scale to fit within max dimensions
	scaleX := float64(maxWidth) / svgWidth
	scaleY := float64(maxHeight) / svgHeight
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	// Convert to resolution (pixels per millimeter)
	resolution := canvas.DPMM(scale)

	return rasterizer.Draw(c, resolution, canvas.DefaultColorSpace), nil
}
