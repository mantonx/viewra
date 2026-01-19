package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"
)

// Downloader handles downloading and installing plugins from GitHub releases.
type Downloader struct {
	httpClient *http.Client
	pluginDir  string // Final installation directory
	tempDir    string // Temporary download directory
	logger     *slog.Logger
}

// DownloadResult contains the result of a successful download.
type DownloadResult struct {
	PluginDir string             // Final installation directory
	Manifest  *manifest.Manifest // Parsed plugin manifest
}

// ProgressCallback is called during download with progress percentage (0-100).
type ProgressCallback func(percent int)

// NewDownloader creates a new plugin downloader.
func NewDownloader(pluginDir, tempDir string, logger *slog.Logger) *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 0, // No timeout for large downloads; use context
		},
		pluginDir: pluginDir,
		tempDir:   tempDir,
		logger:    logger,
	}
}

// Download downloads, verifies, and installs a plugin.
func (d *Downloader) Download(ctx context.Context, asset *Asset, checksum string, progress ProgressCallback) (*DownloadResult, error) {
	// Create temp directory for this download
	downloadDir, err := os.MkdirTemp(d.tempDir, "plugin-download-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(downloadDir) // Clean up on any exit

	tarPath := filepath.Join(downloadDir, asset.Name)
	stagingDir := filepath.Join(downloadDir, "staging")

	// Download the tarball
	if err := d.downloadFile(ctx, asset.BrowserDownloadURL, tarPath, asset.Size, progress); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	// Verify checksum if provided
	if checksum != "" {
		if err := d.verifyChecksum(tarPath, checksum); err != nil {
			return nil, err
		}
	}

	// Extract to staging
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}

	if err := d.extractTarGz(tarPath, stagingDir); err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	// Load and validate manifest
	mf, err := manifest.Load(stagingDir)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin manifest: %w", err)
	}

	// Atomic install: move staging to final directory
	finalDir := filepath.Join(d.pluginDir, mf.ID)
	if err := d.atomicInstall(stagingDir, finalDir); err != nil {
		return nil, fmt.Errorf("install: %w", err)
	}

	return &DownloadResult{
		PluginDir: finalDir,
		Manifest:  mf,
	}, nil
}

// downloadFile downloads a file with progress reporting.
func (d *Downloader) downloadFile(ctx context.Context, url, destPath string, expectedSize int64, progress ProgressCallback) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Use Content-Length if expectedSize not provided
	totalSize := expectedSize
	if totalSize == 0 && resp.ContentLength > 0 {
		totalSize = resp.ContentLength
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy with progress tracking
	var written int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			nw, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			written += int64(nw)

			// Report progress
			if progress != nil && totalSize > 0 {
				percent := int(float64(written) / float64(totalSize) * 100)
				progress(percent)
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of a file.
func (d *Downloader) verifyChecksum(filePath, expectedChecksum string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualChecksum, expectedChecksum) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	d.logger.Debug("checksum verified", "file", filepath.Base(filePath), "checksum", actualChecksum[:16]+"...")
	return nil
}

// extractTarGz extracts a .tar.gz archive to a directory.
func (d *Downloader) extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Skip root directory entries (. or ./)
		cleanName := filepath.Clean(header.Name)
		if cleanName == "." {
			continue
		}

		// Security: prevent path traversal
		target := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			// Preserve executable bit
			mode := os.FileMode(header.Mode)
			if mode&0111 != 0 {
				mode = 0755
			} else {
				mode = 0644
			}

			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

// atomicInstall moves a staging directory to the final location atomically.
func (d *Downloader) atomicInstall(stagingDir, finalDir string) error {
	// If final directory exists, create a backup
	backupDir := finalDir + ".backup"
	if _, err := os.Stat(finalDir); err == nil {
		// Remove any existing backup
		os.RemoveAll(backupDir)

		// Move current to backup
		if err := os.Rename(finalDir, backupDir); err != nil {
			return fmt.Errorf("backup existing: %w", err)
		}
	}

	// Move staging to final
	if err := os.Rename(stagingDir, finalDir); err != nil {
		// Restore backup on failure
		if _, statErr := os.Stat(backupDir); statErr == nil {
			os.Rename(backupDir, finalDir)
		}
		return fmt.Errorf("move to final: %w", err)
	}

	// Clean up backup on success
	os.RemoveAll(backupDir)

	d.logger.Debug("plugin installed", "dir", finalDir)
	return nil
}

// Remove removes a plugin directory.
func (d *Downloader) Remove(pluginID string) error {
	dir := filepath.Join(d.pluginDir, pluginID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Already gone
	}
	return os.RemoveAll(dir)
}

// GetPluginPath returns the path to a plugin's binary.
func (d *Downloader) GetPluginPath(pluginID string) string {
	return filepath.Join(d.pluginDir, pluginID, pluginID)
}
