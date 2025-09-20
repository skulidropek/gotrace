# Go DevTrace

Go DevTrace is a comprehensive development tracing and debugging toolkit for Go applications, inspired by the JavaScript/TypeScript devtrace library. It provides enhanced stack traces, function timing, variable debugging, and automatic code instrumentation capabilities.

## Features

- 📞 **Enhanced Stack Traces**: Detailed call stacks with source code snippets and variable context
- ⏱️ **Function Timing**: Automatic execution time measurement for traced functions
- 🔍 **Variable Debugging**: Capture and display local variables at log points
- 🔧 **Code Instrumentation**: Automatic code transformation for seamless tracing
- 📊 **Performance Benchmarking**: Built-in benchmarking utilities
- 🎯 **Context-Aware**: Support for Go's context package for request tracing
- 🎨 **Flexible Configuration**: Customizable output format and behavior

## Example Output

```log
📞 CALL STACK
  1. main.go:125 → performComplexOperation
        124         // Simulate some nested operations
      > 125         validateInput(ctx, taskName, value)
        126         processInput(ctx, value)
     Vars: {"taskName": "example-task", "value": 42}
  2. main.go:138 → validateInput
        137         log.Printf("Validating input for task: %s", taskName)
      > 138         
        139         if value <= 0 {
     Vars: {"taskName": "example-task", "value": 42}

Message Log: Input validation successful
```

## Quick Start

### 1. Installation

```bash
go get github.com/hackathon/gotrace
```

### 2. Basic Usage

Add to your `main.go`:

```go
package main

import (
    "context"
    "log"
    "github.com/hackathon/gotrace"
)

func main() {
    // Enable devtrace (usually only in development)
    devtrace.SetConfig(devtrace.DevTraceConfig{
        Enabled:     true,
        StackLimit:  5,
        ShowArgs:    true,
        ShowTiming:  true,
        ShowSnippet: 2,
        AppPattern:  "your-app",
        DebugLevel:  1,
    })
    
    // Install enhanced stack logger
    devtrace.InstallStackLogger(nil) // Use default options
    
    // Your application code here
    result := someFunction(42, "test")
    log.Printf("Result: %v", result)
}

func someFunction(value int, name string) string {
    // This log will automatically show enhanced stack trace
    log.Printf("Processing %s with value %d", name, value)
    return fmt.Sprintf("processed-%s-%d", name, value)
}
```

### 3. Manual Function Tracing

```go
// Trace individual functions
tracedFunc := devtrace.TraceFunc(myFunction, "my-function").(func(int) string)
result := tracedFunc(42)

// Or with custom options
tracedFunc2 := devtrace.TraceWithOptions(myFunction, devtrace.TraceOptions{
    ShowTiming: true,
    ShowArgs:   true,
    Label:     "custom-label",
})
```

### 4. Performance Monitoring

```go
// Time function execution
duration := devtrace.TimeFunc(func() {
    // Your code here
})

// Time with result capture
result, duration := devtrace.TimeFuncWithResult(func() string {
    return expensiveOperation()
})

// Benchmark function
benchResult := devtrace.BenchmarkFunc(func() {
    expensiveFunction()
}, 10) // Run 10 times
```

## Code Instrumentation

For automatic instrumentation of your entire codebase:

### 1. Install the instrumentation tool

```bash
cd gotrace/cmd/gotrace-instrument
go build -o gotrace-instrument
```

### 2. Instrument your code

```bash
# Instrument current directory
./gotrace-instrument -src ./myapp -verbose

# Dry run to see what would be changed
./gotrace-instrument -src ./myapp -dry-run -verbose

# Instrument specific patterns
./gotrace-instrument -src ./myapp -pattern "*.go" -exclude "_test.go,vendor/"
```

The instrumentation tool will:
- Add function entry/exit tracing to all functions
- Convert `log.*` calls to enhanced devtrace logging
- Add proper imports automatically
- Skip test files and vendor code

## Configuration Options

### DevTraceConfig

```go
type DevTraceConfig struct {
    Enabled     bool   // Enable/disable tracing
    StackLimit  int    // Maximum stack frames to show
    ShowArgs    bool   // Show function arguments
    ShowTiming  bool   // Show execution timing
    ShowSnippet int    // Lines of code context (0 to disable)
    AppPattern  string // Pattern to identify app code vs libraries
    DebugLevel  int    // Debug verbosity (0=off, 1=info, 2=debug)
}
```

### StackLoggerOptions

```go
type StackLoggerOptions struct {
    Prefix      string // Log message prefix (default: "📞 CALL STACK")
    Skip        int    // Stack frames to skip
    Limit       int    // Maximum frames to show
    ShowSnippet int    // Code context lines
    OnlyApp     bool   // Show only application code
    PreferApp   bool   // Prefer app code over stdlib
    AppPattern  string // Pattern to identify app code
    ShowMeta    bool   // Show diagnostic info
    Ascending   bool   // Order: root → call-site (vs call-site → root)
}
```

## Environment Variables

- `DEVTRACE_ENABLED`: Enable/disable tracing (true/false)
- `GO_ENV`: When set to "development", auto-enables tracing

## Advanced Features

### Context-Aware Logging

```go
ctx := context.Background()
ctx = devtrace.WithTraceContext(ctx, devtrace.NewTraceContext())

devtrace.GlobalEnhancedLogger.Info(ctx, "Processing request", 
    devtrace.NewDebugVars(map[string]interface{}{
        "userID": userID,
        "action": "create_user",
    }))
```

### Debug Variables

```go
// Add debug variables to any log statement
debugVars := devtrace.NewDebugVars(map[string]interface{}{
    "userId":     123,
    "timestamp":  time.Now(),
    "requestId":  "req-456",
})

devtrace.GlobalEnhancedLogger.Error(ctx, "Database connection failed", debugVars)
```

### Custom Loggers

```go
// Implement the Logger interface
type MyLogger struct{}

func (l *MyLogger) Log(level, msg string, args ...interface{}) {
    // Your custom logging implementation
}

// Use your custom logger
devtrace.SetLogger(&MyLogger{})
```

## Example Project

See the complete example in `example/main.go`:

```bash
cd gotrace/example
go run main.go
```

This demonstrates:
- Manual function tracing
- Automatic instrumentation
- Performance monitoring
- Debug variable capture
- Context-aware logging
- Benchmark utilities

## Comparison with JavaScript DevTrace

| Feature | JavaScript DevTrace | Go DevTrace |
|---------|-------------------|-------------|
| Stack Traces | ✅ Via Error.stack | ✅ Via runtime.Caller |
| Source Maps | ✅ Via StackTrace.js | ✅ Direct source access |
| Function Tracing | ✅ Babel plugin | ✅ AST transformation |
| Variable Capture | ✅ Scope analysis | ✅ Parameter capture |
| Async Context | ✅ Zone.js | ✅ Go context |
| Performance Timing | ✅ performance.now() | ✅ time.Now() |
| Auto-instrumentation | ✅ Build-time | ✅ Build-time |

## Development

### Building

```bash
# Build the main library
go build

# Build instrumentation tool
cd cmd/gotrace-instrument && go build

# Run tests
go test ./...

# Run example
cd example && go run main.go
```

### Testing

```bash
# Run all tests
go test -v ./...

# Run with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

## Performance Considerations

- **Development Only**: DevTrace is designed for development and debugging. Disable in production by setting `Enabled: false` or `DEVTRACE_ENABLED=false`.
- **Overhead**: Function tracing adds minimal overhead (~1-5μs per call), but stack trace generation can be more expensive.
- **Memory**: Stack frames and debug variables are kept in memory. Use reasonable limits for long-running applications.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Run the test suite
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Related Projects

- [JavaScript DevTrace](https://github.com/ton-ai-core/devtrace) - The original inspiration for this Go version
- [Go runtime](https://pkg.go.dev/runtime) - Runtime introspection capabilities
- [Go AST](https://pkg.go.dev/go/ast) - Abstract syntax tree manipulation
