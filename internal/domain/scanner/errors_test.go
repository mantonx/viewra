package scanner

import (
	"errors"
	"os"
	"testing"
)

func TestCategorizeError_NilError(t *testing.T) {
	result := CategorizeError(nil)
	if result != "" {
		t.Errorf("CategorizeError(nil) = %s, want empty string", result)
	}
}

func TestCategorizeError_PathError(t *testing.T) {
	pathErr := &os.PathError{
		Op:   "open",
		Path: "/test/path",
		Err:  errors.New("permission denied"),
	}

	result := CategorizeError(pathErr)
	if result != ErrorCategoryFilesystem {
		t.Errorf("CategorizeError(PathError) = %s, want %s", result, ErrorCategoryFilesystem)
	}
}

func TestCategorizeError_LinkError(t *testing.T) {
	linkErr := &os.LinkError{
		Op:  "link",
		Old: "/old",
		New: "/new",
		Err: errors.New("operation not permitted"),
	}

	result := CategorizeError(linkErr)
	if result != ErrorCategoryFilesystem {
		t.Errorf("CategorizeError(LinkError) = %s, want %s", result, ErrorCategoryFilesystem)
	}
}

func TestCategorizeError_SyscallError(t *testing.T) {
	syscallErr := &os.SyscallError{
		Syscall: "read",
		Err:     errors.New("input/output error"),
	}

	result := CategorizeError(syscallErr)
	if result != ErrorCategoryFilesystem {
		t.Errorf("CategorizeError(SyscallError) = %s, want %s", result, ErrorCategoryFilesystem)
	}
}

func TestCategorizeError_DatabasePatterns(t *testing.T) {
	tests := []string{
		"database is locked",
		"sqlite_busy error occurred",
		"pq: connection refused",
		"constraint violation on insert",
		"duplicate key value",
		"foreign key constraint violated",
		"deadlock detected",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := CategorizeError(err)
			if result != ErrorCategoryDatabase {
				t.Errorf("CategorizeError(%q) = %s, want %s", errMsg, result, ErrorCategoryDatabase)
			}
		})
	}
}

func TestCategorizeError_FFmpegPatterns(t *testing.T) {
	tests := []string{
		"ffmpeg: error while encoding",
		"ffprobe: unable to find stream info",
		"avcodec: unsupported codec",
		"moov atom not found in file",
		"invalid data found when processing input",
		"decoder not found for codec",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := CategorizeError(err)
			if result != ErrorCategoryFFmpeg {
				t.Errorf("CategorizeError(%q) = %s, want %s", errMsg, result, ErrorCategoryFFmpeg)
			}
		})
	}
}

func TestCategorizeError_FilesystemPatterns(t *testing.T) {
	tests := []string{
		"permission denied",
		"no such file or directory",
		"read-only file system",
		"disk quota exceeded",
		"no space left on device",
		"too many open files",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := CategorizeError(err)
			if result != ErrorCategoryFilesystem {
				t.Errorf("CategorizeError(%q) = %s, want %s", errMsg, result, ErrorCategoryFilesystem)
			}
		})
	}
}

func TestCategorizeError_MetadataPatterns(t *testing.T) {
	tests := []string{
		"nfo parse error",
		"id3 tag extraction failed",
		"invalid xml in nfo",
		"json unmarshal error",
		"malformed metadata",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := CategorizeError(err)
			if result != ErrorCategoryMetadata {
				t.Errorf("CategorizeError(%q) = %s, want %s", errMsg, result, ErrorCategoryMetadata)
			}
		})
	}
}

func TestCategorizeError_ParsingPatterns(t *testing.T) {
	tests := []string{
		"could not parse filename",
		"invalid filename format",
		"season number not found",
		"episode number invalid",
		"unable to extract title",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := CategorizeError(err)
			if result != ErrorCategoryParsing {
				t.Errorf("CategorizeError(%q) = %s, want %s", errMsg, result, ErrorCategoryParsing)
			}
		})
	}
}

func TestCategorizeError_UnknownDefaultsToParsing(t *testing.T) {
	err := errors.New("some random unknown error")
	result := CategorizeError(err)
	if result != ErrorCategoryParsing {
		t.Errorf("CategorizeError(unknown) = %s, want %s (default)", result, ErrorCategoryParsing)
	}
}

func TestIsTransientError_NilError(t *testing.T) {
	result := IsTransientError(nil)
	if result {
		t.Error("IsTransientError(nil) = true, want false")
	}
}

func TestIsTransientError_DeadlineExceeded(t *testing.T) {
	result := IsTransientError(os.ErrDeadlineExceeded)
	if !result {
		t.Error("IsTransientError(ErrDeadlineExceeded) = false, want true")
	}
}

func TestIsTransientError_TransientPatterns(t *testing.T) {
	tests := []string{
		"database is locked",
		"sqlite_busy",
		"connection refused",
		"connection reset by peer",
		"timeout exceeded",
		"deadline exceeded",
		"stale file handle",
		"resource temporarily unavailable",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := IsTransientError(err)
			if !result {
				t.Errorf("IsTransientError(%q) = false, want true", errMsg)
			}
		})
	}
}

func TestIsTransientError_NonTransientError(t *testing.T) {
	tests := []string{
		"permission denied",
		"invalid codec",
		"file not found",
		"could not parse filename",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := IsTransientError(err)
			if result {
				t.Errorf("IsTransientError(%q) = true, want false", errMsg)
			}
		})
	}
}

func TestIsScanJobDeleted_NilError(t *testing.T) {
	result := IsScanJobDeleted(nil)
	if result {
		t.Error("IsScanJobDeleted(nil) = true, want false")
	}
}

func TestIsScanJobDeleted_DeletedPatterns(t *testing.T) {
	tests := []string{
		"foreign key constraint violated",
		"violates foreign key",
		"no rows in result set",
		"record not found",
		"scan job not found",
		"library not found",
		"table does not exist",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := IsScanJobDeleted(err)
			if !result {
				t.Errorf("IsScanJobDeleted(%q) = false, want true", errMsg)
			}
		})
	}
}

func TestIsScanJobDeleted_NonDeletionError(t *testing.T) {
	tests := []string{
		"permission denied",
		"connection refused",
		"invalid codec",
		"timeout",
	}

	for _, errMsg := range tests {
		t.Run(errMsg, func(t *testing.T) {
			err := errors.New(errMsg)
			result := IsScanJobDeleted(err)
			if result {
				t.Errorf("IsScanJobDeleted(%q) = true, want false", errMsg)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s          string
		substrings []string
		expected   bool
	}{
		{"hello world", []string{"hello"}, true},
		{"hello world", []string{"foo", "bar"}, false},
		{"hello world", []string{"foo", "world"}, true},
		{"", []string{"foo"}, false},
		{"foo", []string{}, false},
		{"database is locked error", []string{"locked"}, true},
	}

	for _, tt := range tests {
		result := containsAny(tt.s, tt.substrings)
		if result != tt.expected {
			t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrings, result, tt.expected)
		}
	}
}

func TestDomainErrors(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{ErrNotFound, "scan job not found"},
		{ErrInvalidPath, "invalid library path"},
		{ErrPathNotExist, "library path does not exist"},
		{ErrPathNotDirectory, "library path is not a directory"},
		{ErrAlreadyRunning, "scan already running for this library"},
		{ErrNotRunning, "scan is not running"},
		{ErrInvalidStatus, "invalid scan status"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("error message = %q, want %q", tt.err.Error(), tt.expected)
			}
		})
	}
}
