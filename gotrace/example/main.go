package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/hackathon/gotrace"
)

// User represents a simple user struct for demonstration
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserService provides user-related operations
type UserService struct {
	users []User
}

// NewUserService creates a new UserService instance
func NewUserService() *UserService {
	return &UserService{
		users: []User{
			{ID: 1, Name: "Alice", Email: "alice@example.com"},
			{ID: 2, Name: "Bob", Email: "bob@example.com"},
			{ID: 3, Name: "Charlie", Email: "charlie@example.com"},
		},
	}
}

// GetUser retrieves a user by ID (will be auto-instrumented)
func (s *UserService) GetUser(id int) (*User, error) {
	log.Printf("Looking for user with ID: %d", id)
	
	for _, user := range s.users {
		if user.ID == id {
			log.Printf("Found user: %s", user.Name)
			return &user, nil
		}
	}
	
	return nil, fmt.Errorf("user with ID %d not found", id)
}

// CreateUser adds a new user (will be auto-instrumented)
func (s *UserService) CreateUser(name, email string) *User {
	newUser := User{
		ID:    len(s.users) + 1,
		Name:  name,
		Email: email,
	}
	
	s.users = append(s.users, newUser)
	log.Printf("Created new user: %+v", newUser)
	
	return &newUser
}

// CalculateFibonacci calculates fibonacci number (for performance testing)
func CalculateFibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return CalculateFibonacci(n-1) + CalculateFibonacci(n-2)
}

// ProcessData simulates some data processing work
func ProcessData(data []int) []int {
	log.Printf("Processing %d items", len(data))
	
	result := make([]int, len(data))
	for i, val := range data {
		// Simulate some work
		time.Sleep(time.Millisecond * 10)
		result[i] = int(math.Pow(float64(val), 2))
	}
	
	log.Printf("Processing complete, result length: %d", len(result))
	return result
}

// SlowFunction simulates a slow function call
func SlowFunction(duration time.Duration) {
	log.Printf("Starting slow operation for %v", duration)
	time.Sleep(duration)
	log.Printf("Slow operation completed")
}

func main() {
	fmt.Println("🚀 Go DevTrace Example Application")
	fmt.Println("===================================")
	
	// Initialize devtrace with development settings
	devtrace.SetConfig(devtrace.DevTraceConfig{
		Enabled:     true,
		StackLimit:  10,
		ShowArgs:    true,
		ShowTiming:  true,
		ShowSnippet: 3,
		AppPattern:  "gotrace/example",
		DebugLevel:  2,
	})
	
	// Install enhanced stack logger
	devtrace.InstallStackLogger(&devtrace.StackLoggerOptions{
		Prefix:      "📞 CALL STACK",
		Skip:        2,
		Limit:       8,
		ShowSnippet: 2,
		OnlyApp:     false,
		PreferApp:   true,
		AppPattern:  "gotrace/example",
		ShowMeta:    true,
		Ascending:   true,
	})
	
	fmt.Println("\n1. Testing Manual Function Tracing")
	fmt.Println("----------------------------------")
	
	// Example 1: Manual function tracing
	tracedFib := devtrace.TraceFunc(CalculateFibonacci, "fibonacci").(func(int) int)
	result := tracedFib(10)
	fmt.Printf("Fibonacci(10) = %d\n", result)
	
	fmt.Println("\n2. Testing User Service Operations")
	fmt.Println("----------------------------------")
	
	// Example 2: Test user service (these functions will be auto-instrumented)
	userService := NewUserService()
	
	// Test getting existing user
	user, err := userService.GetUser(1)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Retrieved user: %+v\n", user)
	}
	
	// Test getting non-existent user
	user, err = userService.GetUser(999)
	if err != nil {
		log.Printf("Expected error: %v", err)
	}
	
	// Test creating new user
	newUser := userService.CreateUser("David", "david@example.com")
	fmt.Printf("Created user: %+v\n", newUser)
	
	fmt.Println("\n3. Testing Performance Monitoring")
	fmt.Println("---------------------------------")
	
	// Example 3: Performance testing
	data := []int{1, 2, 3, 4, 5}
	
	processedData, duration := devtrace.TimeFuncWithResult(func() []int {
		return ProcessData(data)
	})
	
	fmt.Printf("Processed data: %v (took %v)\n", processedData, duration)
	
	fmt.Println("\n4. Testing Benchmark Function")
	fmt.Println("-----------------------------")
	
	// Example 4: Benchmark a function
	benchResult := devtrace.BenchmarkFunc(func() {
		_ = CalculateFibonacci(15)
	}, 5)
	
	fmt.Printf("Benchmark results: %d iterations, avg: %v, min: %v, max: %v\n",
		benchResult.Iterations, benchResult.AverageTime, benchResult.MinTime, benchResult.MaxTime)
	
	fmt.Println("\n5. Testing Enhanced Logging with Context")
	fmt.Println("---------------------------------------")
	
	// Example 5: Context-aware logging
	ctx := context.Background()
	ctx = devtrace.WithTraceContext(ctx, devtrace.NewTraceContext())
	
	// Simulate nested function calls with context
	performComplexOperation(ctx, "example-task", 42)
	
	fmt.Println("\n6. Testing Debug Variables")
	fmt.Println("-------------------------")
	
	// Example 6: Debug variables
	debugVars := map[string]interface{}{
		"userCount":    len(userService.users),
		"processedLen": len(processedData),
		"fibonacci10":  result,
		"timestamp":    time.Now(),
	}
	
	devtrace.GlobalEnhancedLogger.Info(ctx, "Application state summary", devtrace.NewDebugVars(debugVars))
	
	fmt.Println("\n7. Testing Slow Operation with Tracing")
	fmt.Println("-------------------------------------")
	
	// Example 7: Trace a slow operation
	slowOperation := devtrace.TraceFunc(SlowFunction, "slow-operation").(func(time.Duration))
	slowOperation(100 * time.Millisecond)
	
	fmt.Println("\n✅ All examples completed successfully!")
	fmt.Println("\nTry running this with DEVTRACE_ENABLED=false to see the difference:")
	fmt.Println("DEVTRACE_ENABLED=false go run main.go")
	
	// Add final summary log
	devtrace.GlobalEnhancedLogger.Info(ctx, "Application finished successfully")
}

// performComplexOperation demonstrates nested function calls with context
func performComplexOperation(ctx context.Context, taskName string, value int) {
	// This will be auto-instrumented to show in stack traces
	log.Printf("Starting complex operation: %s with value %d", taskName, value)
	
	// Simulate some nested operations
	validateInput(ctx, taskName, value)
	processInput(ctx, value)
	finalizeOperation(ctx, taskName)
	
	log.Printf("Complex operation completed: %s", taskName)
}

func validateInput(ctx context.Context, taskName string, value int) {
	log.Printf("Validating input for task: %s", taskName)
	
	if value <= 0 {
		devtrace.GlobalEnhancedLogger.Error(ctx, "Invalid input value", devtrace.NewDebugVars(map[string]interface{}{
			"taskName": taskName,
			"value":    value,
		}))
		return
	}
	
	devtrace.GlobalEnhancedLogger.Debug(ctx, "Input validation successful")
}

func processInput(ctx context.Context, value int) {
	log.Printf("Processing input value: %d", value)
	
	// Simulate some processing time
	time.Sleep(50 * time.Millisecond)
	
	result := value * 2
	devtrace.GlobalEnhancedLogger.Info(ctx, "Input processed", devtrace.NewDebugVars(map[string]interface{}{
		"originalValue": value,
		"processedValue": result,
	}))
}

func finalizeOperation(ctx context.Context, taskName string) {
	log.Printf("Finalizing operation: %s", taskName)
	
	// Simulate finalization work
	time.Sleep(25 * time.Millisecond)
	
	devtrace.GlobalEnhancedLogger.Info(ctx, "Operation finalized successfully")
}
