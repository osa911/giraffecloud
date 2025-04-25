package logging

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// ANSI color codes for terminal output
const (
	colorRed    = "\033[97;41m" // White text on red background
	colorGreen  = "\033[97;42m" // White text on green background
	colorYellow = "\033[90;43m" // Black text on yellow background
	colorBlue   = "\033[97;44m" // White text on blue background
	colorCyan   = "\033[97;46m" // White text on cyan background
	colorReset  = "\033[0m"
)

type Logger struct {
	*log.Logger
	writer *lumberjack.Logger
}

func NewLogger(config *Config) (*Logger, error) {
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

	// Create a multi-writer that writes to both file and stdout
	multiWriter := io.MultiWriter(writer, os.Stdout)

	// Create logger with timestamp and file:line prefix
	logger := log.New(multiWriter, "", log.LstdFlags)

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

// Log methods with colors (always enabled for better visibility)
func (l *Logger) Debug(format string, v ...interface{}) {
	prefix := colorBlue + "[DEBUG]" + colorReset
	l.Printf(prefix+" "+format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
	prefix := colorGreen + "[INFO]" + colorReset
	l.Printf(prefix+" "+format, v...)
}

func (l *Logger) Warn(format string, v ...interface{}) {
	prefix := colorYellow + "[WARN]" + colorReset
	l.Printf(prefix+" "+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	prefix := colorRed + "[ERROR]" + colorReset
	l.Printf(prefix+" "+format, v...)
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

// FormatHTTPMethod returns a colored string based on the HTTP method
func (l *Logger) FormatHTTPMethod(method string) string {
	var color string
	switch method {
	case http.MethodGet:
		color = colorBlue
	case http.MethodPost:
		color = colorCyan
	case http.MethodPut, http.MethodPatch:
		color = colorYellow
	case http.MethodDelete:
		color = colorRed
	default:
		color = colorBlue
	}
	return fmt.Sprintf("%s %s %s", color, method, colorReset)
}

// FormatHTTPStatus returns a colored string based on the status code
func (l *Logger) FormatHTTPStatus(status int) string {
	var color string
	switch {
	case status >= 500:
		color = colorRed
	case status >= 400:
		color = colorYellow
	case status >= 300:
		color = colorCyan
	case status >= 200:
		color = colorGreen
	default:
		color = colorBlue
	}
	return fmt.Sprintf("%s %d %s", color, status, colorReset)
}

// LogHTTPRequest logs an HTTP request with colored output
func (l *Logger) LogHTTPRequest(method, path, clientIP string, status, bytes int, latency string) {
	if os.Getenv("LOG_REQUESTS") != "true" {
		return
	}

	methodFormatted := l.FormatHTTPMethod(method)
	statusFormatted := l.FormatHTTPStatus(status)

	l.Printf("[HTTP] %s | %15s | %-17s | %s | %d bytes | %s",
		statusFormatted,
		clientIP,
		methodFormatted,
		path,
		bytes,
		latency,
	)
}

// LogHTTPError logs an HTTP error with colored output
func (l *Logger) LogHTTPError(method, path, clientIP string, status int, message string, err error) {
	if os.Getenv("LOG_REQUESTS") != "true" {
		return
	}

	methodFormatted := l.FormatHTTPMethod(method)
	statusFormatted := l.FormatHTTPStatus(status)

	l.Printf("[HTTP-ERROR] %s | %15s | %-17s | %s | %s: %v",
		statusFormatted,
		clientIP,
		methodFormatted,
		path,
		message,
		err,
	)
}