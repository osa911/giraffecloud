package logging

import (
	"errors"
	"fmt"
	"giraffecloud/internal/config"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	*log.Logger
	writer *lumberjack.Logger
}

func NewLogger(config *config.LoggingConfig) (*Logger, error) {
	// Expand home directory in log file path
	logFile := config.File
	if strings.HasPrefix(logFile, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		logFile = filepath.Join(homeDir, logFile[2:])
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Set up log rotation
	writer := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    config.MaxSize,    // MB
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,     // days
		Compress:   true,
	}

	// Create logger with timestamp and file:line prefix
	logger := log.New(writer, "", log.LstdFlags|log.Lshortfile)

	return &Logger{
		Logger: logger,
		writer: writer,
	}, nil
}

func (l *Logger) Close() error {
	return l.writer.Close()
}

// Log levels
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Log methods
func (l *Logger) Debug(format string, v ...interface{}) {
	l.Printf("[DEBUG] "+format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.Printf("[INFO] "+format, v...)
}

func (l *Logger) Warn(format string, v ...interface{}) {
	l.Printf("[WARN] "+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.Printf("[ERROR] "+format, v...)
}

// Error handling utilities
type ErrorWithContext struct {
	Err     error
	Context string
}

func (e *ErrorWithContext) Error() string {
	return fmt.Sprintf("%s: %v", e.Context, e.Err)
}

func (e *ErrorWithContext) Unwrap() error {
	return e.Err
}

func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return &ErrorWithContext{
		Err:     err,
		Context: context,
	}
}

// Common errors
var (
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrConnection    = errors.New("connection error")
	ErrProtocol      = errors.New("protocol error")
	ErrSecurity      = errors.New("security error")
	ErrService       = errors.New("service error")
)