package httpserver

import (
	"log/slog"
	"os"
)

var gLogger *slog.Logger

// Initialize logger.
func _InitLogger() {
	// For development, set log level to DEBUG.
	handlerOpts := &slog.HandlerOptions{
		Level: gConfig.logLevel,
	}
	textHandler := slog.NewTextHandler(os.Stdout, handlerOpts)

	gLogger = slog.New(textHandler)
}
