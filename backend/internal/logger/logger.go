package logger

import (
	"log"
)

// Info logs informational messages
func Info(format string, args ...interface{}) {
	log.Printf("INFO: "+format, args...)
}

// Warn logs warning messages
func Warn(format string, args ...interface{}) {
	log.Printf("WARN: "+format, args...)
}

// Error logs error messages
func Error(format string, args ...interface{}) {
	log.Printf("ERROR: "+format, args...)
}

// Debug logs debug messages
func Debug(format string, args ...interface{}) {
	log.Printf("DEBUG: "+format, args...)
}
