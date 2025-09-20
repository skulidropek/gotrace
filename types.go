package devtrace

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// Frame represents a single stack frame with enhanced debugging information
type Frame struct {
	Function   string                 `json:"function"`
	Signature  string                 `json:"signature,omitempty"`
	File       string                 `json:"file"`
	Line       int                    `json:"line"`
	Args       map[string]interface{} `json:"args,omitempty"`
	StartTime  time.Time              `json:"start_time,omitempty"`
	EndTime    time.Time              `json:"end_time,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
	CallerInfo *runtime.Frame         `json:"caller_info,omitempty"`
}

// TracedFunction represents a function that can be traced
type TracedFunction struct {
	Name     string
	File     string
	Line     int
	Original reflect.Value
}

// TraceOptions provides options for individual trace calls
type TraceOptions struct {
	SkipFrames  int
	MaxDepth    int
	ShowArgs    bool
	ShowTiming  bool
	ShowSnippet int
	Label       string
}

// DefaultTraceOptions provides default options for tracing
var DefaultTraceOptions = TraceOptions{
	SkipFrames:  2, // Skip the trace wrapper and the calling function
	MaxDepth:    Config.StackLimit,
	ShowArgs:    Config.ShowArgs,
	ShowTiming:  Config.ShowTiming,
	ShowSnippet: Config.ShowSnippet,
	Label:       "",
}

// DebugVars represents variables to be logged for debugging
type DebugVars struct {
	Vars map[string]interface{} `json:"vars"`
}

// NewDebugVars creates a new DebugVars instance
func NewDebugVars(vars map[string]interface{}) *DebugVars {
	return &DebugVars{Vars: vars}
}

// TraceContext represents the current tracing context
type TraceContext struct {
	Frames  []*Frame
	Depth   int
	StartAt time.Time
}

// String returns a string representation of debug variables
func (dv *DebugVars) String() string {
	if dv == nil || len(dv.Vars) == 0 {
		return "{}"
	}

	parts := make([]string, 0, len(dv.Vars))
	for k, v := range dv.Vars {
		parts = append(parts, fmt.Sprintf("%q: %+v", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
