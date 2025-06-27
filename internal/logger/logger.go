package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Field represents a structured logging field
type Field struct {
	Key   string
	Value interface{}
}

// Enhanced logging with structured fields

// Info logs informational messages (supports both old format and new structured format)
func Info(format string, args ...interface{}) {
	if len(args) > 0 {
		// Check if last arg is a Field slice (structured logging)
		if fields, ok := args[len(args)-1].([]Field); ok {
			InfoStructured(format, fields...)
			return
		}
	}
	log.Printf("INFO: "+format, args...)
}

// Warn logs warning messages
func Warn(format string, args ...interface{}) {
	if len(args) > 0 {
		if fields, ok := args[len(args)-1].([]Field); ok {
			WarnStructured(format, fields...)
			return
		}
	}
	log.Printf("WARN: "+format, args...)
}

// Error logs error messages
func Error(format string, args ...interface{}) {
	if len(args) > 0 {
		if fields, ok := args[len(args)-1].([]Field); ok {
			ErrorStructured(format, fields...)
			return
		}
	}
	log.Printf("ERROR: "+format, args...)
}

// Debug logs debug messages
func Debug(format string, args ...interface{}) {
	if len(args) > 0 {
		if fields, ok := args[len(args)-1].([]Field); ok {
			DebugStructured(format, fields...)
			return
		}
	}
	log.Printf("DEBUG: "+format, args...)
}

// Structured logging functions
func InfoStructured(msg string, fields ...Field) {
	logStructured("INFO", msg, fields...)
}

func WarnStructured(msg string, fields ...Field) {
	logStructured("WARN", msg, fields...)
}

func ErrorStructured(msg string, fields ...Field) {
	logStructured("ERROR", msg, fields...)
}

func DebugStructured(msg string, fields ...Field) {
	if os.Getenv("LOG_LEVEL") == "debug" {
		logStructured("DEBUG", msg, fields...)
	}
}

func logStructured(level, msg string, fields ...Field) {
	if os.Getenv("LOG_FORMAT") == "json" {
		// JSON structured logging
		logEntry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"level":     level,
			"message":   msg,
		}

		for _, field := range fields {
			logEntry[field.Key] = field.Value
		}

		jsonData, _ := json.Marshal(logEntry)
		log.Println(string(jsonData))
	} else {
		// Human-readable structured logging
		fieldStr := ""
		if len(fields) > 0 {
			fieldStr = " "
			for i, field := range fields {
				if i > 0 {
					fieldStr += " "
				}
				fieldStr += fmt.Sprintf("%s=%v", field.Key, field.Value)
			}
		}
		log.Printf("%s: %s%s", level, msg, fieldStr)
	}
}

// Helper functions for common field types
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Err(key string, err error) Field {
	if err == nil {
		return Field{Key: key, Value: nil}
	}
	return Field{Key: key, Value: err.Error()}
}
