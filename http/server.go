// Package http provides HTTP server configuration for the internal server.
package http

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultReadHeaderTimeout = 2 * time.Second
	defaultShutdownTimeout   = 5 * time.Second
)

// Server wraps the standard HTTP server with sensible defaults.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
	mutex      sync.RWMutex
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
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	s.listener = listener
	s.httpServer.Addr = listener.Addr().String()
	s.mutex.Unlock()

	return s.httpServer.Serve(listener)
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown() error {
	shutdownContext, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	return s.httpServer.Shutdown(shutdownContext)
}

// Address returns the server's configured address (may be ":0" if dynamic port).
func (s *Server) Address() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.httpServer.Addr
}

// ActualAddress returns the actual listening address after the server has started.
// This is useful when using ":0" to get the OS-assigned port.
// Returns empty string if the server hasn't started yet.
func (s *Server) ActualAddress() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}
