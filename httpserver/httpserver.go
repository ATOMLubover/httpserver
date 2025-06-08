package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// http methods
type Method int

// http methods enumerations
const (
	GET Method = iota
	POST
	PUT
	DELETE
)

var methodToString = map[Method]string{
	GET:    "GET",
	POST:   "POST",
	PUT:    "PUT",
	DELETE: "DELETE",
}

var stringToMethod = map[string]Method{
	"":       GET, // empty mean GET
	"GET":    GET,
	"POST":   POST,
	"PUT":    PUT,
	"DELETE": DELETE,
}

func (m Method) String() string {
	return methodToString[m]
}

// alias of functions
type (
	// handler function
	HandlerFunc func(ctx *Context)

	// middleware function
	MiddlewareFunc func(next HandlerFunc) HandlerFunc
)

// Server instance.
type Server struct {
	server http.Server // the base server run on
	port   string      // the port to listen

	router *_Router // core router to find and use handler
}

// Create a new server.
func NewServer(port uint16) *Server {
	// For development, set log level to DEBUG.
	handlerOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	// At current stage, output to stdout.
	textHandler := slog.NewTextHandler(os.Stdout, handlerOpts)
	logger := slog.New(textHandler)
	slog.SetDefault(logger)

	if port == 0 {
		port = 8080
	}

	return &Server{
		port:   ":" + strconv.FormatUint(uint64(port), 10),
		router: _NewRouter(),
	}
}

// Make the server to start listening.
func (s *Server) Serve() (res error) {
	slog.Info("Starting server...")

	// Turn res into error if there is something panicked.
	res = nil
	defer func() {
		if r := recover(); r != nil {
			slog.Error(fmt.Sprintf("Server terminated with panic: %v", r))
			res = fmt.Errorf("panic: %v", r)
			return
		}
		slog.Info("Server terminated.")
	}()

	// Default listening port is 8080.
	if s.port == "" {
		s.port = ":8080"
	}

	s.server = http.Server{
		Addr:    s.port,
		Handler: s.router,
	}

	// Start serving.
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error(fmt.Sprintf("Error running server: %v", err))
		}
	}()
	slog.Info(fmt.Sprintf("Server now listening on port %s", s.port))

	quitChan := make(chan os.Signal, 1)
	signal.Notify(quitChan, syscall.SIGTERM, syscall.SIGINT)
	// Wait for shutdown signal.
	<-quitChan

	s.Shutdown()

	return res
}

// Try to shutdown server gracefully.
// Will wait for the running handler to run another some seconds to be finished.
func (s *Server) Shutdown() {
	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		slog.Error(fmt.Sprintf("Forced shutdown: %v", err))
		return
	}

	slog.Info("Server shut down gracefully.")
}

// Get global route group.
func (s *Server) GetGlobalRouteGroup() *RouteGroup {
	return &s.router.RouteGroup
}

// Get the listening address.
func (s *Server) GetAddr() string {
	return "*" + s.port
}
