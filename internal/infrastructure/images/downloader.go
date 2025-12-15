package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/time/rate"
)

// Downloader handles downloading remote images to local cache.
type Downloader struct {
	cacheDir    string
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	logger      *slog.Logger
}

// DownloaderConfig configures the image downloader.
type DownloaderConfig struct {
	// CacheDir is the directory to store downloaded images.
	CacheDir string

	// Timeout for HTTP requests.
	Timeout time.Duration

	// RateLimit is requests per second (0 = unlimited).
	RateLimit float64

	// Logger for structured logging.
	Logger *slog.Logger
}

// NewDownloader creates a new image downloader.
func NewDownloader(cfg DownloaderConfig) *Downloader {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	var limiter *rate.Limiter
	if cfg.RateLimit > 0 {
		limiter = rate.NewLimiter(rate.Limit(cfg.RateLimit), 1)
	}

	return &Downloader{
		cacheDir: cfg.CacheDir,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		rateLimiter: limiter,
		logger:      cfg.Logger,
	}
}

// Download fetches a remote image and stores it locally.
// Returns the local file path.
func (d *Downloader) Download(ctx context.Context, url string) (string, error) {
	// 1. Hash URL for cache path
	urlHash := hashURL(url)
	ext := inferExtension(url)

	// 2. Create sharded path: remote/{first2}/{next2}/{hash}{ext}
	localPath := filepath.Join(d.cacheDir, "remote", urlHash[:2], urlHash[2:4], urlHash+ext)

	// 3. Check if already downloaded
	if _, err := os.Stat(localPath); err == nil {
		d.logger.Debug("image already cached", slog.String("url", url), slog.String("path", localPath))
		return localPath, nil
	}

	// 4. Apply rate limiting if configured
	if d.rateLimiter != nil {
		if err := d.rateLimiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limiter: %w", err)
		}
	}

	// 5. Download with timeout
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "ViewRA Media Server/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// 6. Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	// 7. Write to disk atomically (write to temp, then rename)
	tmpPath := localPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("write file: %w", err)
	}

	// Rename to final path
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename temp file: %w", err)
	}

	d.logger.Debug("downloaded image",
		slog.String("url", url),
		slog.String("path", localPath),
		slog.Int64("size", resp.ContentLength))

	return localPath, nil
}

// hashURL generates a SHA256 hash of the URL for cache key.
func hashURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:])
}

// inferExtension attempts to infer the file extension from URL.
func inferExtension(url string) string {
	ext := filepath.Ext(url)

	// Clean up query strings from extension
	for i, c := range ext {
		if c == '?' || c == '#' {
			ext = ext[:i]
			break
		}
	}

	// Validate it's a known image extension
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return ext
	default:
		return ".jpg" // Default to jpg
	}
}
