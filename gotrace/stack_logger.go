package devtrace

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// StackLoggerOptions configures the enhanced stack logger
type StackLoggerOptions struct {
	Prefix      string // Prefix for log messages
	Skip        int    // Number of stack frames to skip
	Limit       int    // Maximum number of frames to show
	ShowSnippet int    // Lines of code context to show
	OnlyApp     bool   // Show only application code (not stdlib)
	PreferApp   bool   // Prefer application code over stdlib
	AppPattern  string // Pattern to identify application code
	ShowMeta    bool   // Show diagnostic information
	Ascending   bool   // Show stack root -> call-site (vs call-site -> root)
}

// DefaultStackLoggerOptions provides sensible defaults
var DefaultStackLoggerOptions = StackLoggerOptions{
	Prefix:      "📞 CALL STACK",
	Skip:        2,
	Limit:       5,
	ShowSnippet: 2,
	OnlyApp:     false,
	PreferApp:   true,
	AppPattern:  "/",
	ShowMeta:    false,
	Ascending:   true,
}

// EnhancedLogger wraps the standard logging with stack trace information
type EnhancedLogger struct {
	options StackLoggerOptions
	logger  Logger
}

// NewEnhancedLogger creates a new enhanced logger with the given options
func NewEnhancedLogger(opts *StackLoggerOptions) *EnhancedLogger {
	if opts == nil {
		opts = &DefaultStackLoggerOptions
	}
	
	return &EnhancedLogger{
		options: *opts,
		logger:  GlobalLogger,
	}
}

// SetLogger sets a custom logger for the enhanced logger
func (el *EnhancedLogger) SetLogger(logger Logger) {
	el.logger = logger
}

// getCodeSnippet retrieves code snippet around the given file and line
func getCodeSnippet(filename string, line int, contextLines int) (string, error) {
	if contextLines <= 0 {
		return "", nil
	}
	
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lines := make([]string, 0)
	currentLine := 0
	
	for scanner.Scan() {
		currentLine++
		lines = append(lines, scanner.Text())
	}
	
	if err := scanner.Err(); err != nil {
		return "", err
	}
	
	if line <= 0 || line > len(lines) {
		return "", fmt.Errorf("line %d out of range", line)
	}
	
	start := max(0, line-contextLines-1)
	end := min(len(lines), line+contextLines)
	
	snippet := strings.Builder{}
	for i := start; i < end; i++ {
		lineNum := i + 1
		marker := " "
		if lineNum == line {
			marker = ">"
		}
		snippet.WriteString(fmt.Sprintf("      %s %d %s\n", marker, lineNum, lines[i]))
	}
	
	return strings.TrimRight(snippet.String(), "\n"), nil
}

// formatFrame formats a single stack frame with optional code snippet
func (el *EnhancedLogger) formatFrame(frame *Frame, index int) string {
	name := frame.Function
	if name == "" {
		name = "<anonymous>"
	}
	
	fileName := filepath.Base(frame.File)
	header := fmt.Sprintf("  %d. %s:%d → %s", index+1, fileName, frame.Line, name)
	
	var parts []string
	parts = append(parts, header)
	
	// Add code snippet if requested
	if el.options.ShowSnippet > 0 && frame.File != "" {
		snippet, err := getCodeSnippet(frame.File, frame.Line, el.options.ShowSnippet)
		if err == nil && snippet != "" {
			parts = append(parts, snippet)
		}
	}
	
	// Add variable information if available
	if frame.Args != nil && len(frame.Args) > 0 {
		vars := NewDebugVars(frame.Args)
		parts = append(parts, fmt.Sprintf("     Vars: %s", vars.String()))
	}
	
	// Add timing information if available
	if frame.Duration > 0 && el.options.ShowMeta {
		parts = append(parts, fmt.Sprintf("     Time: %v", frame.Duration))
	}
	
	return strings.Join(parts, "\n")
}

// getStackFrames retrieves the current stack frames
func (el *EnhancedLogger) getStackFrames(ctx context.Context) []*Frame {
	// First, try to get frames from context
	traceCtx := FromContext(ctx)
	frames := traceCtx.Stack()
	
	if len(frames) > 0 {
		return frames
	}
	
	// Fallback to runtime stack trace
	pc := make([]uintptr, 50)
	n := runtime.Callers(el.options.Skip, pc)
	pc = pc[:n]
	
	frames = make([]*Frame, 0, n)
	runtimeFrames := runtime.CallersFrames(pc)
	
	for {
		rFrame, more := runtimeFrames.Next()
		
		frame := &Frame{
			Function: rFrame.Function,
			File:     rFrame.File,
			Line:     rFrame.Line,
			Args:     nil, // No args available from runtime
		}
		
		frames = append(frames, frame)
		
		if !more {
			break
		}
	}
	
	return frames
}

// filterFrames applies filtering logic to stack frames
func (el *EnhancedLogger) filterFrames(frames []*Frame) []*Frame {
	if len(frames) == 0 {
		return frames
	}
	
	filtered := make([]*Frame, 0, len(frames))
	
	for _, frame := range frames {
		// Skip internal devtrace frames
		if strings.Contains(frame.Function, "devtrace.") ||
		   strings.Contains(frame.File, "devtrace") ||
		   strings.Contains(frame.Function, "runtime.") {
			continue
		}
		
		filtered = append(filtered, frame)
	}
	
	// Apply app-specific filtering
	if el.options.OnlyApp || el.options.PreferApp {
		appFrames := make([]*Frame, 0)
		
		for _, frame := range filtered {
			if strings.Contains(frame.File, el.options.AppPattern) {
				appFrames = append(appFrames, frame)
			}
		}
		
		if len(appFrames) > 0 && el.options.OnlyApp {
			filtered = appFrames
		} else if len(appFrames) > 0 && el.options.PreferApp {
			// Mix app frames with some stdlib frames
			result := make([]*Frame, 0)
			appIndex, otherIndex := 0, 0
			
			for len(result) < el.options.Limit && (appIndex < len(appFrames) || otherIndex < len(filtered)) {
				// Prefer app frames
				if appIndex < len(appFrames) {
					result = append(result, appFrames[appIndex])
					appIndex++
				} else if otherIndex < len(filtered) && !strings.Contains(filtered[otherIndex].File, el.options.AppPattern) {
					result = append(result, filtered[otherIndex])
					otherIndex++
				}
				
				// Skip frames we already added
				for otherIndex < len(filtered) && appIndex > 0 {
					if filtered[otherIndex] == appFrames[appIndex-1] {
						otherIndex++
						break
					}
					otherIndex++
				}
			}
			
			filtered = result
		}
	}
	
	// Apply limit
	if el.options.Limit > 0 && len(filtered) > el.options.Limit {
		if el.options.Ascending {
			filtered = filtered[len(filtered)-el.options.Limit:]
		} else {
			filtered = filtered[:el.options.Limit]
		}
	}
	
	// Apply ordering
	if !el.options.Ascending {
		// Reverse the slice for descending order (call-site -> root)
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}
	
	return filtered
}

// LogWithStack logs a message with enhanced stack trace information
func (el *EnhancedLogger) LogWithStack(ctx context.Context, level, message string, args ...interface{}) {
	if !IsEnabled() {
		// Fallback to regular logging when devtrace is disabled
		el.logger.Log(level, message, args...)
		return
	}
	
	// Get and filter stack frames
	frames := el.getStackFrames(ctx)
	filtered := el.filterFrames(frames)
	
	// Format the stack trace
	parts := make([]string, 0, len(filtered)+3)
	parts = append(parts, el.options.Prefix)
	
	for i, frame := range filtered {
		parts = append(parts, el.formatFrame(frame, i))
	}
	
	// Add meta information if requested
	if el.options.ShowMeta {
		meta := fmt.Sprintf("\nConfig{ limit=%d, skip=%d, snippet=%d, onlyApp=%t, preferApp=%t, ascending=%t }",
			el.options.Limit, el.options.Skip, el.options.ShowSnippet,
			el.options.OnlyApp, el.options.PreferApp, el.options.Ascending)
		parts = append(parts, meta)
		
		frameMeta := fmt.Sprintf("Frames{ total=%d, shown=%d }", len(frames), len(filtered))
		parts = append(parts, frameMeta)
	}
	
	// Add the actual log message
	if len(args) > 0 {
		logMessage := fmt.Sprintf("\nMessage Log: "+message, args...)
		parts = append(parts, logMessage)
	} else {
		logMessage := "\nMessage Log: " + message
		parts = append(parts, logMessage)
	}
	
	// Check for debug variables in args
	debugVars := make([]*DebugVars, 0)
	for _, arg := range args {
		if dv, ok := arg.(*DebugVars); ok {
			debugVars = append(debugVars, dv)
		}
	}
	
	if len(debugVars) > 0 {
		parts = append(parts, "\nVars:")
		for _, dv := range debugVars {
			parts = append(parts, dv.String())
		}
	}
	
	// Log the complete message
	completeMessage := strings.Join(parts, "\n")
	el.logger.Log(level, completeMessage)
}

// Debug logs a debug message with stack trace
func (el *EnhancedLogger) Debug(ctx context.Context, message string, args ...interface{}) {
	el.LogWithStack(ctx, "DEBUG", message, args...)
}

// Info logs an info message with stack trace
func (el *EnhancedLogger) Info(ctx context.Context, message string, args ...interface{}) {
	el.LogWithStack(ctx, "INFO", message, args...)
}

// Warn logs a warning message with stack trace
func (el *EnhancedLogger) Warn(ctx context.Context, message string, args ...interface{}) {
	el.LogWithStack(ctx, "WARN", message, args...)
}

// Error logs an error message with stack trace
func (el *EnhancedLogger) Error(ctx context.Context, message string, args ...interface{}) {
	el.LogWithStack(ctx, "ERROR", message, args...)
}

// Global enhanced logger instance
var GlobalEnhancedLogger = NewEnhancedLogger(nil)

// InstallStackLogger installs the enhanced stack logger globally
func InstallStackLogger(opts *StackLoggerOptions) {
	if opts == nil {
		opts = &DefaultStackLoggerOptions
	}
	GlobalEnhancedLogger = NewEnhancedLogger(opts)
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
