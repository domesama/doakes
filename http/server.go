// Package http provides HTTP server configuration for the internal server.
package http

import (
	"context"
	"net/http"
	"time"
)

const (
	defaultReadHeaderTimeout = 2 * time.Second
	defaultShutdownTimeout   = 5 * time.Second
)

// Server wraps the standard HTTP server with sensible defaults.
type Server struct {
	httpServer *http.Server
}

// NewServer creates a new HTTP server with the given router.
func NewServer(router http.Handler) *Server {
	httpServer := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	return &Server{
		httpServer: httpServer,
	}
}

// Start begins serving HTTP requests on the specified address.
func (s *Server) Start(address string) error {
	s.httpServer.Addr = address
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown() error {
	shutdownContext, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	return s.httpServer.Shutdown(shutdownContext)
}

// Address returns the server's listening address.
func (s *Server) Address() string {
	return s.httpServer.Addr
}
