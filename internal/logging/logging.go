package logging

import (
	"log"
	"strings"
)

// LogLevel defines the level of logging.
type LogLevel int

const (
	// LogLevelError logs only errors.
	LogLevelError LogLevel = iota
	// LogLevelInfo logs info and errors.
	LogLevelInfo
	// LogLevelDebug logs debug, info, and errors.
	LogLevelDebug
)

var currentLogLevel = LogLevelInfo // Default level

// SetLevel sets the global log level based on a string identifier.
func SetLevel(levelStr string) {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		currentLogLevel = LogLevelDebug
	case "INFO":
		currentLogLevel = LogLevelInfo
	case "ERROR":
		currentLogLevel = LogLevelError
	default:
		// Log warning only if levelStr is not empty
		if levelStr != "" {
			log.Printf("Warning: Invalid LOG_LEVEL '%s'. Defaulting to INFO.", levelStr)
		}
		currentLogLevel = LogLevelInfo
	}
	// Print log level setting only if not default due to empty input
	if levelStr != "" || currentLogLevel != LogLevelInfo {
		log.Printf("Log level set to: %s", LevelToString(currentLogLevel))
	}
}

// LevelToString converts a LogLevel to its string representation.
func LevelToString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Debug logs a message at the DEBUG level.
func Debug(format string, v ...interface{}) {
	if currentLogLevel >= LogLevelDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs a message at the INFO level.
func Info(format string, v ...interface{}) {
	if currentLogLevel >= LogLevelInfo {
		log.Printf("[INFO] "+format, v...)
	}
}

// Error logs a message at the ERROR level.
func Error(format string, v ...interface{}) {
	if currentLogLevel >= LogLevelError {
		log.Printf("[ERROR] "+format, v...)
	}
}
