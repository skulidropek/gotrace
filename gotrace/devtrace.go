// Package devtrace provides enhanced development tracing and debugging capabilities for Go applications
package devtrace

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// DevTraceConfig holds global configuration for devtrace
type DevTraceConfig struct {
	Enabled     bool
	StackLimit  int
	ShowArgs    bool
	ShowTiming  bool
	ShowSnippet int // lines of code context
	AppPattern  string
	DebugLevel  int
}

// DefaultConfig provides sensible defaults for devtrace
var DefaultConfig = DevTraceConfig{
	Enabled:     strings.ToLower(os.Getenv("DEVTRACE_ENABLED")) == "true" || strings.ToLower(os.Getenv("GO_ENV")) == "development",
	StackLimit:  5,
	ShowArgs:    true,
	ShowTiming:  true,
	ShowSnippet: 2,
	AppPattern:  "/",
	DebugLevel:  1,
}

// Config holds the current devtrace configuration
var Config = DefaultConfig

// Logger interface allows custom logging implementations
type Logger interface {
	Log(level string, msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// DefaultLogger implements the Logger interface using Go's standard log package
type DefaultLogger struct{}

func (l *DefaultLogger) Log(level string, msg string, args ...interface{}) {
	prefix := fmt.Sprintf("[DEVTRACE-%s] ", level)
	log.Printf(prefix+msg, args...)
}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	if Config.DebugLevel >= 2 {
		l.Log("DEBUG", msg, args...)
	}
}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	if Config.DebugLevel >= 1 {
		l.Log("INFO", msg, args...)
	}
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	l.Log("WARN", msg, args...)
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	l.Log("ERROR", msg, args...)
}

// GlobalLogger is the default logger instance
var GlobalLogger Logger = &DefaultLogger{}

// SetLogger sets a custom logger implementation
func SetLogger(logger Logger) {
	GlobalLogger = logger
}

// SetConfig updates the global configuration
func SetConfig(config DevTraceConfig) {
	Config = config
}

// IsEnabled returns whether devtrace is currently enabled
func IsEnabled() bool {
	return Config.Enabled
}
