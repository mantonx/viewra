package filesystem

import (
	"os"
	"path/filepath"
)

// CalculateDirSize calculates the total size of all files in a directory recursively.
// Returns the size in bytes, or an error if the directory cannot be read.
func CalculateDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files/directories that can't be accessed
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
