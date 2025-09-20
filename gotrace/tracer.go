package devtrace

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"time"
)

// TracedFunc represents a traced function wrapper
type TracedFunc struct {
	Name     string
	Original reflect.Value
	Options  TraceOptions
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
	
	return &TracedFunc{
		Name:     name,
		Original: fnValue,
		Options:  *options,
	}
}

// Call executes the traced function with the given arguments
func (tf *TracedFunc) Call(ctx context.Context, args ...interface{}) *TraceResult {
	startTime := time.Now()
	
	// Convert args to reflect values
	fnType := tf.Original.Type()
	numIn := fnType.NumIn()
	
	// Handle variadic functions
	var reflectArgs []reflect.Value
	if fnType.IsVariadic() {
		reflectArgs = make([]reflect.Value, numIn)
		for i := 0; i < numIn-1; i++ {
			if i < len(args) {
				reflectArgs[i] = reflect.ValueOf(args[i])
			} else {
				reflectArgs[i] = reflect.Zero(fnType.In(i))
			}
		}
		
		// Handle variadic arguments
		if len(args) >= numIn-1 {
			variadicArgs := args[numIn-1:]
			variadicSlice := reflect.MakeSlice(fnType.In(numIn-1), len(variadicArgs), len(variadicArgs))
			for i, arg := range variadicArgs {
				variadicSlice.Index(i).Set(reflect.ValueOf(arg))
			}
			reflectArgs[numIn-1] = variadicSlice
		} else {
			reflectArgs[numIn-1] = reflect.Zero(fnType.In(numIn-1))
		}
	} else {
		reflectArgs = make([]reflect.Value, len(args))
		for i, arg := range args {
			if i < numIn {
				reflectArgs[i] = reflect.ValueOf(arg)
			}
		}
		
		// Fill missing args with zero values
		for i := len(args); i < numIn; i++ {
			reflectArgs = append(reflectArgs, reflect.Zero(fnType.In(i)))
		}
	}
	
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
		
		frame = CreateFrame(tf.Name, file, line, argsMap)
		
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
		
		// Use context.Background() as default context
		ctx := context.Background()
		
		// If the first argument is a context, use it
		if len(args) > 0 {
			if ctx, ok := interfaceArgs[0].(context.Context); ok {
				result := tracedFunc.Call(ctx, interfaceArgs[1:]...)
				
				// Convert results back to reflect values
				resultValues := make([]reflect.Value, len(result.Results))
				for i, res := range result.Results {
					resultValues[i] = reflect.ValueOf(res)
				}
				
				// Add error as last return value if the function returns error
				if fnType.NumOut() > 0 && fnType.Out(fnType.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
					if result.Error != nil {
						resultValues[len(resultValues)-1] = reflect.ValueOf(result.Error)
					}
				}
				
				return resultValues
			}
		}
		
		// Call with all arguments
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
				resultValues[len(resultValues)-1] = reflect.Zero(fnType.Out(fnType.NumOut()-1))
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
	Iterations   int
	TotalTime    time.Duration
	AverageTime  time.Duration
	MinTime      time.Duration
	MaxTime      time.Duration
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
