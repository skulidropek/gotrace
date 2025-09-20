package devtrace

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// TracedFunc represents a traced function wrapper
type TracedFunc struct {
	Name       string
	Signature  string
	Original   reflect.Value
	Options    TraceOptions
	SourceFile string
	SourceLine int
	ParamNames []string
}

// TraceResult contains the result of a traced function call
type TraceResult struct {
	Duration  time.Duration
	Args      []interface{}
	Results   []interface{}
	Error     error
	StartTime time.Time
	EndTime   time.Time
}

// NewTracedFunc creates a new traced function wrapper
func NewTracedFunc(fn interface{}, options *TraceOptions) *TracedFunc {
	if options == nil {
		opts := DefaultTraceOptions
		options = &opts
	}

	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		panic("NewTracedFunc: argument must be a function")
	}

	// Try to get function name
	name := options.Label
	if name == "" {
		if pc := fnValue.Pointer(); pc != 0 {
			if fn := runtime.FuncForPC(pc); fn != nil {
				name = fn.Name()
			}
		}
		if name == "" {
			name = "<anonymous>"
		}
	}

	signature := buildReflectSignature(name, fnValue.Type())
	sourceFile := ""
	sourceLine := 0
	var paramNames []string

	if fn := runtime.FuncForPC(fnValue.Pointer()); fn != nil {
		sourceFile, sourceLine = fn.FileLine(fnValue.Pointer())
		if fnSig := getSignatureForLocation(sourceFile, sourceLine, name); fnSig != nil {
			signature = fnSig.signature
			paramNames = append(paramNames, fnSig.params...)
		}
	}

	return &TracedFunc{
		Name:       name,
		Signature:  signature,
		Original:   fnValue,
		Options:    *options,
		SourceFile: sourceFile,
		SourceLine: sourceLine,
		ParamNames: paramNames,
	}
}

func buildReflectSignature(fullName string, fnType reflect.Type) string {
	if fnType == nil {
		return fullName
	}

	simpleName := simplifyFunctionName(fullName)
	if simpleName == "" {
		simpleName = "<anonymous>"
	}

	var builder strings.Builder
	builder.WriteString(simpleName)
	builder.WriteString("(")

	for i := 0; i < fnType.NumIn(); i++ {
		if i > 0 {
			builder.WriteString(", ")
		}

		typeStr := fnType.In(i).String()
		if fnType.IsVariadic() && i == fnType.NumIn()-1 {
			typeStr = "..." + fnType.In(i).Elem().String()
		}

		builder.WriteString(fmt.Sprintf("arg%d %s", i, typeStr))
	}

	builder.WriteString(")")

	switch fnType.NumOut() {
	case 0:
		// no return values
	case 1:
		builder.WriteString(" ")
		builder.WriteString(fnType.Out(0).String())
	default:
		builder.WriteString(" (")
		for i := 0; i < fnType.NumOut(); i++ {
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(fnType.Out(i).String())
		}
		builder.WriteString(")")
	}

	return builder.String()
}

func simplifyFunctionName(name string) string {
	if name == "" {
		return ""
	}

	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}

	return name
}

// Call executes the traced function with the given arguments
func (tf *TracedFunc) Call(ctx context.Context, args ...interface{}) *TraceResult {
	startTime := time.Now()

	fnType := tf.Original.Type()
	numIn := fnType.NumIn()

	createValue := func(arg interface{}, typ reflect.Type) reflect.Value {
		if arg == nil {
			return reflect.Zero(typ)
		}
		value := reflect.ValueOf(arg)
		if !value.Type().AssignableTo(typ) {
			if value.Type().ConvertibleTo(typ) {
				return value.Convert(typ)
			}
			return reflect.Zero(typ)
		}
		return value
	}

	buildArgs := func() []reflect.Value {
		if fnType.IsVariadic() {
			vals := make([]reflect.Value, 0, numIn)
			for i := 0; i < numIn-1; i++ {
				if i < len(args) {
					vals = append(vals, createValue(args[i], fnType.In(i)))
				} else {
					vals = append(vals, reflect.Zero(fnType.In(i)))
				}
			}

			variadicType := fnType.In(numIn - 1)
			if len(args) >= numIn-1 {
				variadicCount := len(args) - (numIn - 1)
				slice := reflect.MakeSlice(variadicType, variadicCount, variadicCount)
				for idx := 0; idx < variadicCount; idx++ {
					slice.Index(idx).Set(createValue(args[numIn-1+idx], variadicType.Elem()))
				}
				vals = append(vals, slice)
			} else {
				vals = append(vals, reflect.MakeSlice(variadicType, 0, 0))
			}

			return vals
		}

		vals := make([]reflect.Value, numIn)
		for i := 0; i < numIn; i++ {
			if i < len(args) {
				vals[i] = createValue(args[i], fnType.In(i))
			} else {
				vals[i] = reflect.Zero(fnType.In(i))
			}
		}
		return vals
	}

	reflectArgs := buildArgs()

	// Create frame for tracing
	var frame *Frame
	if IsEnabled() {
		// Get caller information
		_, file, line, _ := runtime.Caller(tf.Options.SkipFrames)

		// Prepare args map
		argsMap := make(map[string]interface{})
		for i, arg := range args {
			argsMap[fmt.Sprintf("arg%d", i)] = arg
		}

		frame = CreateFrame(tf.Name, tf.Signature, file, line, argsMap)
		normalizeFrameArgs(frame, tf.ParamNames)

		// Add frame to context
		traceCtx := FromContext(ctx)
		traceCtx.Enter(frame)

		if Config.ShowTiming && GlobalLogger != nil {
			GlobalLogger.Debug("▶ trace enter: %s", tf.Name)
		}
	}

	// Execute the function
	var results []reflect.Value
	var err error
	var resultValues []interface{}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}

		// Leave the trace context
		if IsEnabled() && frame != nil {
			traceCtx := FromContext(ctx)
			traceCtx.Leave()
		}
	}()

	// Call the original function
	if fnType.IsVariadic() {
		results = tf.Original.CallSlice(reflectArgs)
	} else {
		results = tf.Original.Call(reflectArgs)
	}

	// Convert results back to interface{}
	resultValues = make([]interface{}, len(results))
	for i, result := range results {
		resultValues[i] = result.Interface()
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Log trace information
	if IsEnabled() && Config.ShowTiming && GlobalLogger != nil {
		GlobalLogger.Debug("▶ trace exit: %s (duration: %v)", tf.Name, duration)
	}

	return &TraceResult{
		Duration:  duration,
		Args:      args,
		Results:   resultValues,
		Error:     err,
		StartTime: startTime,
		EndTime:   endTime,
	}
}

// Trace wraps a function with tracing capabilities
func Trace(fn interface{}, options *TraceOptions) interface{} {
	tracedFunc := NewTracedFunc(fn, options)
	fnType := reflect.TypeOf(fn)

	// Create a new function with the same signature as the original
	return reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		// Convert reflect values to interface{}
		interfaceArgs := make([]interface{}, len(args))
		for i, arg := range args {
			interfaceArgs[i] = arg.Interface()
		}

		// Use context.Background() as default context and detect if first arg is a context
		ctx := context.Background()
		if len(interfaceArgs) > 0 {
			if maybeCtx, ok := interfaceArgs[0].(context.Context); ok {
				ctx = maybeCtx
			}
		}

		result := tracedFunc.Call(ctx, interfaceArgs...)

		// Convert results back to reflect values
		resultValues := make([]reflect.Value, len(result.Results))
		for i, res := range result.Results {
			resultValues[i] = reflect.ValueOf(res)
		}

		// Add error as last return value if the function returns error
		if fnType.NumOut() > 0 && fnType.Out(fnType.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if result.Error != nil {
				resultValues[len(resultValues)-1] = reflect.ValueOf(result.Error)
			} else {
				resultValues[len(resultValues)-1] = reflect.Zero(fnType.Out(fnType.NumOut() - 1))
			}
		}

		return resultValues
	}).Interface()
}

// TraceFunc is a convenience function that traces a function and returns the traced version
func TraceFunc(fn interface{}, label ...string) interface{} {
	options := DefaultTraceOptions
	if len(label) > 0 {
		options.Label = label[0]
	}
	return Trace(fn, &options)
}

// TraceWithOptions traces a function with custom options
func TraceWithOptions(fn interface{}, options TraceOptions) interface{} {
	return Trace(fn, &options)
}

// TimeFunc measures the execution time of a function call
func TimeFunc(fn func()) time.Duration {
	if !IsEnabled() {
		fn()
		return 0
	}

	start := time.Now()
	fn()
	duration := time.Since(start)

	if Config.ShowTiming && GlobalLogger != nil {
		GlobalLogger.Debug("⏱ function executed in %v", duration)
	}

	return duration
}

// TimeFuncWithResult measures execution time and captures the result
func TimeFuncWithResult[T any](fn func() T) (T, time.Duration) {
	if !IsEnabled() {
		return fn(), 0
	}

	start := time.Now()
	result := fn()
	duration := time.Since(start)

	if Config.ShowTiming && GlobalLogger != nil {
		GlobalLogger.Debug("⏱ function executed in %v with result: %+v", duration, result)
	}

	return result, duration
}

// Benchmark runs a function multiple times and returns statistics
type BenchmarkResult struct {
	Iterations  int
	TotalTime   time.Duration
	AverageTime time.Duration
	MinTime     time.Duration
	MaxTime     time.Duration
}

// BenchmarkFunc runs a function multiple times and returns performance statistics
func BenchmarkFunc(fn func(), iterations int) *BenchmarkResult {
	if !IsEnabled() || iterations <= 0 {
		return &BenchmarkResult{}
	}

	times := make([]time.Duration, iterations)
	totalTime := time.Duration(0)
	minTime := time.Duration(^uint64(0) >> 1) // Max duration
	maxTime := time.Duration(0)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		fn()
		duration := time.Since(start)

		times[i] = duration
		totalTime += duration

		if duration < minTime {
			minTime = duration
		}
		if duration > maxTime {
			maxTime = duration
		}
	}

	avgTime := totalTime / time.Duration(iterations)

	result := &BenchmarkResult{
		Iterations:  iterations,
		TotalTime:   totalTime,
		AverageTime: avgTime,
		MinTime:     minTime,
		MaxTime:     maxTime,
	}

	if GlobalLogger != nil {
		GlobalLogger.Info("📊 Benchmark: %d iterations, avg: %v, min: %v, max: %v, total: %v",
			iterations, avgTime, minTime, maxTime, totalTime)
	}

	return result
}
