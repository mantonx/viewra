package playbackmodule

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	httputil "github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/http"
)

// SessionHandler handles session-based content serving
type SessionHandler struct {
	sessionStore SessionStoreInterface
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sessionStore SessionStoreInterface) *SessionHandler {
	return &SessionHandler{
		sessionStore: sessionStore,
	}
}

// ServeSessionContent serves files from session directories
// GET /api/v1/sessions/:sessionId/*file
func (h *SessionHandler) ServeSessionContent(c *gin.Context) {
	sessionID := c.Param("sessionId")
	requestedFile := c.Param("file")

	// Remove leading slash if present
	if strings.HasPrefix(requestedFile, "/") {
		requestedFile = requestedFile[1:]
	}

	// Extract just the filename for simpler cases
	fileName := filepath.Base(requestedFile)

	logger.Info("ServeSessionContent called", "sessionID", sessionID, "file", fileName)

	// Get session from session store
	if h.sessionStore == nil {
		logger.Error("Session store not available")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Session store not available",
		})
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		logger.Warn("Session not found", "sessionID", sessionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found",
		})
		return
	}

	// Get session directory
	sessionDirName := session.DirectoryPath
	if sessionDirName == "" {
		logger.Error("Session has no directory path", "sessionID", sessionID)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session directory not found",
		})
		return
	}

	// Construct full path - session directories are stored in the transcoding data directory
	// The DirectoryPath in database is just the directory name, not the full path
	transcodingBaseDir := "/app/viewra-data/transcoding"
	sessionDir := filepath.Join(transcodingBaseDir, sessionDirName)

	logger.Info("Serving session content", "sessionID", sessionID, "dirName", sessionDirName, "fullPath", sessionDir)

	// Determine subdirectory based on file type
	var subDir string
	switch {
	case fileName == "manifest.mpd" || strings.HasSuffix(fileName, ".m3u8"):
		subDir = "packaged"
	case fileName == "intermediate.mp4" || strings.HasSuffix(fileName, ".mp4"):
		subDir = "encoded"
	case strings.HasSuffix(fileName, ".m4s") || strings.HasSuffix(fileName, ".ts"):
		subDir = "packaged"
	default:
		// Try to find the file in any subdirectory
		subDir = ""
	}

	// Construct file path
	var filePath string
	if subDir != "" {
		filePath = filepath.Join(sessionDir, subDir, fileName)
	} else {
		// Try multiple locations
		possiblePaths := []string{
			filepath.Join(sessionDir, fileName),
			filepath.Join(sessionDir, "packaged", fileName),
			filepath.Join(sessionDir, "encoded", fileName),
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				filePath = path
				break
			}
		}

		if filePath == "" {
			logger.Warn("File not found in any subdirectory", "sessionID", sessionID, "file", fileName)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "File not found",
			})
			return
		}
	}

	// Security check: ensure file is within session directory
	if !strings.HasPrefix(filePath, sessionDir) {
		logger.Warn("Path traversal attempt", "sessionID", sessionID, "file", fileName)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		logger.Warn("File not found", "sessionID", sessionID, "file", fileName, "path", filePath)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found",
		})
		return
	}

	// Set appropriate headers based on file type
	httputil.SetContentHeaders(c, fileName)

	// Session content is temporary, so don't cache it
	httputil.SetCacheHeaders(c, false, sessionID)

	// Serve the file
	c.File(filePath)
}
