package streaming

import "strings"

// DetectContentType determines the MIME type based on file extension
func DetectContentType(fileName string) string {
	ext := getFileExtension(fileName)

	// Video formats
	if contentType, ok := videoTypes[ext]; ok {
		return contentType
	}

	// Audio formats
	if contentType, ok := audioTypes[ext]; ok {
		return contentType
	}

	// Default to octet-stream for unknown types
	return "application/octet-stream"
}

// getFileExtension extracts and normalizes the file extension
func getFileExtension(fileName string) string {
	lastDot := strings.LastIndex(fileName, ".")
	if lastDot == -1 {
		return ""
	}
	return strings.ToLower(fileName[lastDot+1:])
}

// videoTypes maps file extensions to MIME types for video files
var videoTypes = map[string]string{
	"mp4":  "video/mp4",
	"mkv":  "video/x-matroska",
	"webm": "video/webm",
	"avi":  "video/x-msvideo",
	"mov":  "video/quicktime",
	"wmv":  "video/x-ms-wmv",
	"flv":  "video/x-flv",
	"m4v":  "video/x-m4v",
	"mpg":  "video/mpeg",
	"mpeg": "video/mpeg",
	"ts":   "video/mp2t",
	"3gp":  "video/3gpp",
	"ogv":  "video/ogg",
}

// audioTypes maps file extensions to MIME types for audio files
var audioTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"wav":  "audio/wav",
	"m4a":  "audio/mp4",
	"aac":  "audio/aac",
	"ogg":  "audio/ogg",
	"opus": "audio/opus",
	"wma":  "audio/x-ms-wma",
	"oga":  "audio/ogg",
	"webm": "audio/webm",
}
