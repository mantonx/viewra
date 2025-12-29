package pathbrowser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/domain/library"
)

// PathValidator validates filesystem paths with security checks
type PathValidator struct {
	allowedBasePaths []string
	blockedPaths     []string
}

// NewPathValidator creates a new path validator
func NewPathValidator(allowedPaths []string) *PathValidator {
	// System directories to always block
	blocked := []string{
		"/etc", "/sys", "/proc", "/dev",
		"/boot", "/root", "/usr/bin", "/usr/sbin",
		"/bin", "/sbin", "/lib", "/lib64",
		"/var/log", "/var/run", "/tmp",
	}

	return &PathValidator{
		allowedBasePaths: allowedPaths,
		blockedPaths:     blocked,
	}
}

// Validate performs multi-layer validation on a path
func (v *PathValidator) Validate(path string) error {
	if path == "" {
		return fmt.Errorf("%w: empty path", library.ErrInvalidPath)
	}

	// 1. Clean path first (resolves . and .. in the path string)
	cleanPath := filepath.Clean(path)

	// 2. Check for traversal attempts before resolving symlinks
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("%w: path contains parent directory references", library.ErrPathTraversal)
	}

	// 3. Resolve symlinks to get actual path
	realPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		// If the path doesn't exist yet, that's okay for some use cases
		// But we still need to validate the clean path
		if os.IsNotExist(err) {
			realPath = cleanPath
		} else {
			return errors.Join(library.ErrInvalidPath, err)
		}
	}

	// 4. Ensure absolute path
	if !filepath.IsAbs(realPath) {
		return fmt.Errorf("%w: path must be absolute", library.ErrInvalidPath)
	}

	// 5. Check against blocklist (system directories)
	for _, blocked := range v.blockedPaths {
		if realPath == blocked || strings.HasPrefix(realPath, blocked+string(filepath.Separator)) {
			return fmt.Errorf("%w: %s", library.ErrSystemDirectory, realPath)
		}
	}

	// 6. Check against whitelist (allowed base paths)
	allowed := false
	for _, base := range v.allowedBasePaths {
		if realPath == base || strings.HasPrefix(realPath, base+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: %s", library.ErrOutsideAllowed, realPath)
	}

	// 7. Check if path exists and is readable
	info, err := os.Stat(realPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", library.ErrDirectoryNotFound, realPath)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("%w: %s", library.ErrPermissionDenied, realPath)
		}
		return errors.Join(library.ErrInvalidPath, err)
	}

	// 8. Ensure it's a directory
	if !info.IsDir() {
		return fmt.Errorf("%w: path is not a directory", library.ErrInvalidPath)
	}

	// 9. Check if directory is readable
	if !isReadable(realPath) {
		return fmt.Errorf("%w: cannot read directory", library.ErrPermissionDenied)
	}

	return nil
}

// isReadable checks if a directory is readable
func isReadable(path string) bool {
	_, err := os.ReadDir(path)
	return err == nil
}
