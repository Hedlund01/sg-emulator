package scalegraph

import (
	"log/slog"
)

// testLogger creates a logger configured for testing
func testLogger() *slog.Logger {
	// Create a no-op logger for tests
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}
