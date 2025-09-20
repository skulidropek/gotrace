package devtrace

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type captureLogger struct {
	messages []string
}

func (c *captureLogger) Log(level string, msg string, args ...interface{}) {
	formatted := msg
	if len(args) > 0 {
		formatted = fmt.Sprintf(msg, args...)
	}
	c.messages = append(c.messages, formatted)
}

func (c *captureLogger) Debug(msg string, args ...interface{}) { c.Log("DEBUG", msg, args...) }
func (c *captureLogger) Info(msg string, args ...interface{})  { c.Log("INFO", msg, args...) }
func (c *captureLogger) Warn(msg string, args ...interface{})  { c.Log("WARN", msg, args...) }
func (c *captureLogger) Error(msg string, args ...interface{}) { c.Log("ERROR", msg, args...) }

type testRequest struct {
	ID    int
	Name  string
	Meta  map[string]string
	Items []string
}

var invokeWorker = func(ctx context.Context, payload testRequest, limit int) {
	worker(ctx, payload, limit)
}

func worker(ctx context.Context, payload testRequest, limit int) {
	GlobalEnhancedLogger.Info(ctx, "worker processing", NewDebugVars(map[string]interface{}{
		"id":    payload.ID,
		"limit": limit,
	}))
}

func coordinator(ctx context.Context, payload testRequest) {
	invokeWorker(ctx, payload, 3)
}

func TestStackLoggerCapturesFunctionArgs(t *testing.T) {
	originalConfig := Config
	originalLogger := GlobalLogger
	originalEnhanced := GlobalEnhancedLogger

	signatureCacheMu.Lock()
	originalCache := signatureCache
	signatureCache = make(map[string]*fileSignature)
	signatureCacheMu.Unlock()

	t.Cleanup(func() {
		SetConfig(originalConfig)
		GlobalLogger = originalLogger
		GlobalEnhancedLogger = originalEnhanced
		InstallStackLogger(nil)
		signatureCacheMu.Lock()
		signatureCache = originalCache
		signatureCacheMu.Unlock()
	})

	SetConfig(DevTraceConfig{
		Enabled:     true,
		StackLimit:  5,
		ShowArgs:    true,
		ShowTiming:  false,
		ShowSnippet: 0,
		AppPattern:  "stack_logger_test.go",
		DebugLevel:  2,
	})

	logger := &captureLogger{}
	GlobalLogger = logger
	InstallStackLogger(&StackLoggerOptions{
		Prefix:      "📞 CALL STACK",
		Skip:        2,
		Limit:       5,
		ShowSnippet: 0,
		OnlyApp:     false,
		PreferApp:   false,
		AppPattern:  "stack_logger_test.go",
		Ascending:   true,
	})

	ctx := WithTraceContext(context.Background(), NewTraceContext())
	payload := testRequest{
		ID:    42,
		Name:  "alpha",
		Meta:  map[string]string{"env": "qa"},
		Items: []string{"one", "two"},
	}

	originalInvoker := invokeWorker
	defer func() { invokeWorker = originalInvoker }()

	workerOpts := TraceOptions{
		SkipFrames:  2,
		MaxDepth:    5,
		ShowArgs:    true,
		ShowTiming:  false,
		ShowSnippet: 0,
		Label:       "worker",
	}
	tracedWorker := TraceWithOptions(worker, workerOpts).(func(context.Context, testRequest, int))
	invokeWorker = tracedWorker

	coordinatorOpts := TraceOptions{
		SkipFrames:  2,
		MaxDepth:    5,
		ShowArgs:    true,
		ShowTiming:  false,
		ShowSnippet: 0,
		Label:       "coordinator",
	}
	tracedCoordinator := TraceWithOptions(coordinator, coordinatorOpts).(func(context.Context, testRequest))

	tracedCoordinator(ctx, payload)

	if len(logger.messages) == 0 {
		t.Fatalf("expected captured log entry")
	}
	entry := logger.messages[len(logger.messages)-1]

	if !strings.Contains(entry, "Route: coordinator → worker") {
		t.Fatalf("stack route missing: %s", entry)
	}

	if strings.Contains(entry, "\"arg0\"") {
		t.Fatalf("argument names were not normalized: %s", entry)
	}

	if !strings.Contains(entry, "worker(ctx context.Context, payload testRequest, limit int)") {
		t.Fatalf("signature missing from stack frame: %s", entry)
	}

	if !strings.Contains(entry, "\"ctx\": context.Background") {
		t.Fatalf("context argument missing: %s", entry)
	}

	if !strings.Contains(entry, "\"payload\": {ID:42") {
		t.Fatalf("payload argument not captured: %s", entry)
	}

	if !strings.Contains(entry, "\"limit\": 3") {
		t.Fatalf("limit argument missing: %s", entry)
	}

	if !strings.Contains(entry, "Message Log: worker processing") {
		t.Fatalf("log message missing: %s", entry)
	}
}
