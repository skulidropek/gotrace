package devtrace

import (
	"context"
	"log"
	"strings"
)

type stackLogWriter struct{}

func (w *stackLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSuffix(string(p), "\n")
	GlobalEnhancedLogger.Info(context.Background(), msg)
	return len(p), nil
}

// RedirectStandardLogger routes the default log package through the enhanced stack logger.
func RedirectStandardLogger() {
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(&stackLogWriter{})
}
