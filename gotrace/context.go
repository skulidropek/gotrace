package devtrace

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// contextKey is used as a key for storing trace context in Go context
type contextKey string

const traceContextKey contextKey = "devtrace_context"

// Global trace context for non-context aware usage
var (
	globalContext *TraceContext
	globalMutex   sync.RWMutex
)

// InitGlobalContext initializes the global trace context
func InitGlobalContext() {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	
	if globalContext == nil {
		globalContext = &TraceContext{
			Frames:  make([]*Frame, 0),
			Depth:   0,
			StartAt: time.Now(),
		}
	}
}

// GetGlobalContext returns the global trace context
func GetGlobalContext() *TraceContext {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	
	if globalContext == nil {
		return &TraceContext{
			Frames:  make([]*Frame, 0),
			Depth:   0,
			StartAt: time.Now(),
		}
	}
	
	return globalContext
}

// WithTraceContext attaches a trace context to the given context
func WithTraceContext(ctx context.Context, traceCtx *TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey, traceCtx)
}

// FromContext extracts the trace context from the given context
func FromContext(ctx context.Context) *TraceContext {
	if ctx == nil {
		return GetGlobalContext()
	}
	
	if traceCtx, ok := ctx.Value(traceContextKey).(*TraceContext); ok {
		return traceCtx
	}
	
	return GetGlobalContext()
}

// NewTraceContext creates a new trace context
func NewTraceContext() *TraceContext {
	return &TraceContext{
		Frames:  make([]*Frame, 0),
		Depth:   0,
		StartAt: time.Now(),
	}
}

// Enter adds a new frame to the trace context
func (tc *TraceContext) Enter(frame *Frame) {
	if tc == nil {
		return
	}
	
	tc.Frames = append(tc.Frames, frame)
	tc.Depth++
}

// Leave removes the most recent frame from the trace context
func (tc *TraceContext) Leave() *Frame {
	if tc == nil || len(tc.Frames) == 0 {
		return nil
	}
	
	frame := tc.Frames[len(tc.Frames)-1]
	tc.Frames = tc.Frames[:len(tc.Frames)-1]
	tc.Depth--
	
	// Update frame end time and duration
	frame.EndTime = time.Now()
	if !frame.StartTime.IsZero() {
		frame.Duration = frame.EndTime.Sub(frame.StartTime)
	}
	
	return frame
}

// Stack returns a copy of the current stack frames
func (tc *TraceContext) Stack() []*Frame {
	if tc == nil {
		return []*Frame{}
	}
	
	// Create a copy to avoid race conditions
	stack := make([]*Frame, len(tc.Frames))
	copy(stack, tc.Frames)
	return stack
}

// Depth returns the current stack depth
func (tc *TraceContext) GetDepth() int {
	if tc == nil {
		return 0
	}
	return tc.Depth
}

// GetCurrentFrame returns the most recent frame without removing it
func (tc *TraceContext) GetCurrentFrame() *Frame {
	if tc == nil || len(tc.Frames) == 0 {
		return nil
	}
	return tc.Frames[len(tc.Frames)-1]
}

// CreateFrame creates a new frame with the given parameters
func CreateFrame(functionName, file string, line int, args map[string]interface{}) *Frame {
	frame := &Frame{
		Function:  functionName,
		File:      file,
		Line:      line,
		Args:      args,
		StartTime: time.Now(),
	}
	
	// Capture caller information
	if pc, callerFile, callerLine, ok := runtime.Caller(2); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			frame.CallerInfo = &runtime.Frame{
				PC:       pc,
				Func:     fn,
				Function: fn.Name(),
				File:     callerFile,
				Line:     callerLine,
			}
		}
	}
	
	return frame
}

// GlobalEnter adds a frame to the global trace context
func GlobalEnter(frame *Frame) {
	InitGlobalContext()
	
	globalMutex.Lock()
	defer globalMutex.Unlock()
	
	globalContext.Enter(frame)
}

// GlobalLeave removes a frame from the global trace context
func GlobalLeave() *Frame {
	if globalContext == nil {
		return nil
	}
	
	globalMutex.Lock()
	defer globalMutex.Unlock()
	
	return globalContext.Leave()
}

// GlobalStack returns the current global stack
func GlobalStack() []*Frame {
	return GetGlobalContext().Stack()
}
