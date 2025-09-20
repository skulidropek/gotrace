package devtrace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

var (
	signatureCacheMu sync.RWMutex
	signatureCache   = make(map[string]*fileSignature)
)

type fileSignature struct {
	functions []functionSignature
}

type functionSignature struct {
	name      string
	startLine int
	endLine   int
	signature string
	params    []string
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
	displayName := resolveFrameSignature(frame)
	if displayName == "" {
		displayName = "<anonymous>"
	}

	fileName := filepath.Base(frame.File)
	header := fmt.Sprintf("  %d. %s:%d → %s", index+1, fileName, frame.Line, displayName)

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

func resolveFrameSignature(frame *Frame) string {
	if frame == nil {
		return ""
	}

	if frame.Signature != "" {
		return frame.Signature
	}

	if fnSig := getSignatureForLocation(frame.File, frame.Line, frame.Function); fnSig != nil {
		frame.Signature = fnSig.signature
		normalizeFrameArgs(frame, fnSig.params)
		return fnSig.signature
	}

	return frame.Function
}

func getSignatureForLocation(file string, line int, functionName string) *functionSignature {
	if file == "" || line <= 0 {
		return nil
	}

	signatureCacheMu.RLock()
	entry, ok := signatureCache[file]
	signatureCacheMu.RUnlock()

	if !ok {
		entry = parseFileSignatures(file)
		signatureCacheMu.Lock()
		signatureCache[file] = entry
		signatureCacheMu.Unlock()
	}

	if entry == nil {
		return nil
	}

	for i := range entry.functions {
		fn := &entry.functions[i]
		if line < fn.startLine || line > fn.endLine {
			continue
		}
		if functionName != "" && fn.name != "" && !strings.HasSuffix(functionName, fn.name) {
			continue
		}
		return fn
	}

	return nil
}

func parseFileSignatures(file string) *fileSignature {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	if err != nil {
		return nil
	}

	info := &fileSignature{functions: make([]functionSignature, 0)}

	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		signature := formatFuncSignature(fn, fset)
		params := extractParamNames(fn)

		info.functions = append(info.functions, functionSignature{
			name:      fn.Name.Name,
			startLine: start,
			endLine:   end,
			signature: signature,
			params:    params,
		})
	}

	return info
}

func extractParamNames(fn *ast.FuncDecl) []string {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return nil
	}

	names := make([]string, 0)
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}

	return names
}

func formatFuncSignature(fn *ast.FuncDecl, fset *token.FileSet) string {
	if fn == nil {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fn.Name.Name)
	builder.WriteString("(")

	params := make([]string, 0)
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			typeStr := renderAstExpr(fset, field.Type)
			if len(field.Names) == 0 {
				params = append(params, typeStr)
				continue
			}
			for _, name := range field.Names {
				params = append(params, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		}
	}

	builder.WriteString(strings.Join(params, ", "))
	builder.WriteString(")")

	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := make([]string, 0)
		for _, field := range fn.Type.Results.List {
			typeStr := renderAstExpr(fset, field.Type)
			if len(field.Names) == 0 {
				results = append(results, typeStr)
				continue
			}
			for _, name := range field.Names {
				results = append(results, fmt.Sprintf("%s %s", name.Name, typeStr))
			}
		}

		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			builder.WriteString(" ")
			builder.WriteString(results[0])
		} else {
			builder.WriteString(" (")
			builder.WriteString(strings.Join(results, ", "))
			builder.WriteString(")")
		}
	}

	return builder.String()
}

func renderAstExpr(fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		return ""
	}

	return buf.String()
}

func normalizeFrameArgs(frame *Frame, paramNames []string) {
	if frame == nil || frame.Args == nil || len(frame.Args) == 0 || len(paramNames) == 0 {
		return
	}

	normalized := make(map[string]interface{}, len(frame.Args))

	for i, param := range paramNames {
		key := fmt.Sprintf("arg%d", i)
		val, ok := frame.Args[key]
		if !ok {
			continue
		}

		name := param
		if name == "" {
			name = key
		}

		normalized[name] = val
	}

	for k, v := range frame.Args {
		if strings.HasPrefix(k, "arg") {
			continue
		}
		normalized[k] = v
	}

	frame.Args = normalized
}

// buildRouteLine describes the flow from the outermost shown frame to the current one.
func (el *EnhancedLogger) buildRouteLine(frames []*Frame) string {
	if len(frames) == 0 {
		return ""
	}

	origin := shortFrameLabel(frames[0])
	current := shortFrameLabel(frames[len(frames)-1])

	if origin == "" && current == "" {
		return ""
	}

	if origin == "" {
		return fmt.Sprintf("Route: → %s", current)
	}

	if current == "" {
		return fmt.Sprintf("Route: %s →", origin)
	}

	if origin == current {
		return fmt.Sprintf("Route: %s", current)
	}

	return fmt.Sprintf("Route: %s → %s", origin, current)
}

// shortFrameLabel picks a concise label for a frame (used in the route line).
func shortFrameLabel(frame *Frame) string {
	if frame == nil {
		return ""
	}

	sig := resolveFrameSignature(frame)
	if sig != "" {
		if idx := strings.Index(sig, "("); idx != -1 {
			return sig[:idx]
		}
		return sig
	}

	return shortFunctionName(frame.Function)
}

// shortFunctionName trims package prefixes from function names for compact display.
func shortFunctionName(name string) string {
	if name == "" {
		return ""
	}

	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	return name
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

	// Apply limit (cap to the most recent frames, maximum five)
	configuredLimit := el.options.Limit
	if configuredLimit <= 0 {
		configuredLimit = 5
	}
	if configuredLimit > 5 {
		configuredLimit = 5
	}

	if len(filtered) > configuredLimit {
		filtered = filtered[len(filtered)-configuredLimit:]
	}

	// Apply ordering: by default show root -> current; when Ascending=false, flip
	if !el.options.Ascending {
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
	parts := make([]string, 0, len(filtered)+4)
	parts = append(parts, el.options.Prefix)

	if route := el.buildRouteLine(filtered); route != "" {
		parts = append(parts, "  "+route)
	}

	for i, frame := range filtered {
		parts = append(parts, el.formatFrame(frame, i))
	}

	// Remove ShowMeta output (deprecated).

	// Separate debug variables from message formatting args
	debugVars := make([]*DebugVars, 0)
	messageArgs := make([]interface{}, 0, len(args))
	for _, arg := range args {
		if dv, ok := arg.(*DebugVars); ok {
			debugVars = append(debugVars, dv)
			continue
		}
		messageArgs = append(messageArgs, arg)
	}

	if len(debugVars) > 0 {
		parts = append(parts, "\nVars:")
		for _, dv := range debugVars {
			parts = append(parts, dv.String())
		}
	}

	// Add the actual log message at the end
	if len(messageArgs) > 0 {
		parts = append(parts, fmt.Sprintf("\nMessage Log: "+message, messageArgs...))
	} else {
		parts = append(parts, "\nMessage Log: "+message)
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
