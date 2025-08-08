package logging

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ANSI color codes for terminal output
const (
	colorRed     = "\033[97;41m" // White text on red background
	colorGreen   = "\033[97;42m" // White text on green background
	colorYellow  = "\033[90;43m" // Black text on yellow background
	colorBlue    = "\033[97;44m" // White text on blue background
	colorCyan    = "\033[97;46m" // White text on cyan background
	colorMagenta = "\033[97;45m" // White text on magenta background
	colorReset   = "\033[0m"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "debug"
	case LogLevelInfo:
		return "info"
	case LogLevelWarn:
		return "warn"
	case LogLevelError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLogLevel parses a string into a LogLevel
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug, nil
	case "info":
		return LogLevelInfo, nil
	case "warn":
		return LogLevelWarn, nil
	case "error":
		return LogLevelError, nil
	default:
		return LogLevelInfo, fmt.Errorf("invalid log level: %s", level)
	}
}

// LogConfig holds configuration for the logger
type LogConfig struct {
	File       string // Log file path
	MaxSize    int    // Maximum size in megabytes before log rotation
	MaxBackups int    // Maximum number of old log files to retain
	MaxAge     int    // Maximum number of days to retain old log files
	Level      string // Log level (debug, info, warn, error)
}

// colorStripper is a custom writer that strips ANSI color codes
type colorStripper struct {
	writer io.Writer
	re     *regexp.Regexp
}

func newColorStripper(w io.Writer) *colorStripper {
	return &colorStripper{
		writer: w,
		re:     regexp.MustCompile("\033\\[[0-9;]*m"),
	}
}

func (cs *colorStripper) Write(p []byte) (n int, err error) {
	clean := cs.re.ReplaceAll(p, []byte{})
	_, err = cs.writer.Write(clean)
	return len(p), err // Return original length to satisfy io.Writer
}

type Logger struct {
	*log.Logger
	fileWriter   *lumberjack.Logger
	stdoutWriter io.Writer
	multiWriter  io.Writer
	useColors    bool
	level        LogLevel
}

// Singleton pattern variables
var (
	globalLogger *Logger
	initOnce     sync.Once
	loggerMutex  sync.RWMutex
)

// InitLogger sets up the global logger instance
func InitLogger(config *LogConfig) error {
	var err error
	initOnce.Do(func() {
		var logger *Logger
		logger, err = newLogger(config)
		if err == nil {
			globalLogger = logger
		}
	})
	return err
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	return globalLogger
}

// newLogger creates a new logger instance (internal function)
func newLogger(config *LogConfig) (*Logger, error) {
	// Parse log level
	level := LogLevelInfo // default
	if config.Level != "" {
		var err error
		level, err = ParseLogLevel(config.Level)
		if err != nil {
			return nil, fmt.Errorf("invalid log level: %w", err)
		}
	}

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
	fileWriter := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    config.MaxSize, // MB
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge, // days
		Compress:   true,
	}

	// Create a color-stripped writer for the file
	strippedFileWriter := newColorStripper(fileWriter)

	// Always use stdout for terminal output
	stdoutWriter := os.Stdout

	// Create a multi-writer that writes to both file and stdout
	multiWriter := io.MultiWriter(strippedFileWriter, stdoutWriter)

	// Create logger with timestamp and microseconds
	logger := log.New(multiWriter, "", log.LstdFlags|log.Lmicroseconds)

	return &Logger{
		Logger:       logger,
		fileWriter:   fileWriter,
		stdoutWriter: stdoutWriter,
		multiWriter:  multiWriter,
		useColors:    true, // Always enable colors since we strip them for file output
		level:        level,
	}, nil
}

func (l *Logger) Close() error {
	return l.fileWriter.Close()
}

// GetWriter returns the logger's multiWriter
func (l *Logger) GetWriter() io.Writer {
	return l.multiWriter
}

// shouldLog checks if a message should be logged based on the configured level
func (l *Logger) shouldLog(msgLevel LogLevel) bool {
	return msgLevel >= l.level
}

// SetLevel updates the log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// Log methods with optional colors and level filtering
func (l *Logger) Debug(format string, v ...interface{}) {
	if !l.shouldLog(LogLevelDebug) {
		return
	}
	prefix := "[DEBUG]"
	if l.useColors {
		prefix = colorBlue + prefix + colorReset
	}
	l.Printf(prefix+" "+format, v...)
}

func (l *Logger) Info(format string, v ...interface{}) {
	if !l.shouldLog(LogLevelInfo) {
		return
	}
	prefix := "[INFO]"
	if l.useColors {
		prefix = colorGreen + prefix + colorReset
	}
	l.Printf(prefix+" "+format, v...)
}

func (l *Logger) Warn(format string, v ...interface{}) {
	if !l.shouldLog(LogLevelWarn) {
		return
	}
	prefix := "[WARN]"
	if l.useColors {
		prefix = colorYellow + prefix + colorReset
	}
	l.Printf(prefix+" "+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	if !l.shouldLog(LogLevelError) {
		return
	}
	prefix := "[ERROR]"
	if l.useColors {
		prefix = colorRed + prefix + colorReset
	}
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

// FormatLatency formats latency with appropriate unit
func FormatLatency(duration time.Duration) string {
	switch {
	case duration.Nanoseconds() < 1000:
		return fmt.Sprintf("%dns", duration.Nanoseconds())
	case duration.Nanoseconds() < 1000000:
		return fmt.Sprintf("%.3fµs", float64(duration.Nanoseconds())/1000)
	case duration.Nanoseconds() < 1000000000:
		return fmt.Sprintf("%.3fms", float64(duration.Nanoseconds())/1000000)
	default:
		return fmt.Sprintf("%.3fs", duration.Seconds())
	}
}

// GinLoggerConfig returns a Gin logger middleware with our custom format
func (l *Logger) GinLoggerConfig() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Output: l.multiWriter,
		Formatter: func(param gin.LogFormatterParams) string {
			// Get status color (using background colors)
			var statusColor string
			switch {
			case param.StatusCode >= 200 && param.StatusCode < 300:
				statusColor = colorGreen // Green background
			case param.StatusCode >= 300 && param.StatusCode < 400:
				statusColor = colorYellow // Yellow background
			case param.StatusCode >= 400 && param.StatusCode < 500:
				statusColor = colorRed // Red background
			default:
				statusColor = colorMagenta // Magenta background
			}

			// Format timestamp with milliseconds
			timestamp := param.TimeStamp.Format("2006/01/02-15:04:05.000")

			// Return single line log format
			return fmt.Sprintf("[%s] %s | %s %s | %s%d%s | %s | %s\n",
				timestamp,
				param.ClientIP,
				param.Method,
				param.Path,
				statusColor,
				param.StatusCode,
				colorReset,
				FormatLatency(param.Latency),
				param.ErrorMessage,
			)
		},
	})
}
