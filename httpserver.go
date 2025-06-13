package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// HTTP methods.
type Method int

// HTTP methods enumerations
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
// Make sure you have called ModifyConfig() before using this function,
// or any changes made after this function will be ignored.
func NewServer() *Server {
	// Initialize logger.
	_InitLogger()

	// Default configs.
	return &Server{
		port:   ":" + strconv.FormatUint(uint64(gConfig.port), 10),
		router: _NewRouter(),
	}
}

// Make the server to start listening.
func (s *Server) Serve() (res error) {
	gLogger.Info("Starting server...")

	// Turn res into error if there is something panicked.
	res = nil
	defer func() {
		if r := recover(); r != nil {
			gLogger.Error(fmt.Sprintf("Server terminated with panic: %v", r))
			res = fmt.Errorf("panic: %v", r)
			return
		}
		gLogger.Info("Server terminated.")
	}()

	s.server = http.Server{
		Addr:    s.port,
		Handler: s.router,
	}

	// Start serving.
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			gLogger.Error(fmt.Sprintf("Error running server: %v", err))
		}
	}()
	gLogger.Info(fmt.Sprintf("Server now listening on port %s", s.port))

	exePath, _ := os.Executable()
	gLogger.Info("Current executable directory: " + exePath)

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
	gLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		gLogger.Error(fmt.Sprintf("Forced shutdown: %v", err))
		return
	}

	gLogger.Info("Server shut down gracefully.")
}

// Get global route group.
func (s *Server) GetGlobalRouteGroup() *RouteGroup {
	return &s.router.RouteGroup
}

// Add new HTTP method.
func (s *Server) AddMethod(method int, methodName string) {
	s.router.AddMethod(method, methodName)
}

// Get the listening address.
func (s *Server) GetAddr() string {
	return "*" + s.port
}
