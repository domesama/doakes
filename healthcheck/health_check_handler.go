// Package healthcheck provides a thread-safe health check handler with explicit enablement.
package healthcheck

import (
	"log/slog"
	"net/http"
	"sync"
)

// CheckFunction is a function that performs a health check.
// Return nil if healthy, or an error if unhealthy.
type CheckFunction func() error

// Handler manages registered health checks and serves HTTP health check requests.
//
// Health checks must be explicitly enabled via Enable() to prevent services
// from passing health checks during initialization.
type Handler struct {
	serviceName string
	checks      map[string]CheckFunction
	checksMutex sync.RWMutex

	enabledMutex sync.RWMutex
	enabled      bool
}

// NewHandler creates a new health check handler for the given service.
func NewHandler(serviceName string) *Handler {
	return &Handler{
		serviceName: serviceName,
		checks:      make(map[string]CheckFunction),
	}
}

// RegisterCheck registers a health check function with the given name.
// This is thread-safe and can be called concurrently during server initialization.
func (h *Handler) RegisterCheck(name string, checkFn CheckFunction) {
	h.checksMutex.Lock()
	defer h.checksMutex.Unlock()

	h.checks[name] = checkFn
	slog.Info("Registered health check", "name", name)
}

// Enable activates health checks.
// Until this is called, health check requests will return 503 Service Unavailable.
func (h *Handler) Enable() {
	h.enabledMutex.Lock()
	defer h.enabledMutex.Unlock()

	h.enabled = true
	slog.Info("Health check enabled")
}

// IsEnabled returns true if health checks are enabled.
func (h *Handler) IsEnabled() bool {
	h.enabledMutex.RLock()
	defer h.enabledMutex.RUnlock()

	return h.enabled
}

// ServeHTTP handles HTTP health check requests.
// Returns 200 OK if all checks pass, 503 Service Unavailable otherwise.
func (h *Handler) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	if !h.IsEnabled() {
		h.writeResponse(writer, http.StatusServiceUnavailable, "not enabled")
		return
	}

	if err := h.runAllChecks(); err != nil {
		h.writeResponse(writer, http.StatusServiceUnavailable, "unhealthy")
		return
	}

	h.writeResponse(writer, http.StatusOK, "ok")
}

func (h *Handler) runAllChecks() error {
	h.checksMutex.RLock()
	defer h.checksMutex.RUnlock()

	for checkName, checkFn := range h.checks {
		if err := checkFn(); err != nil {
			slog.Error(
				"Health check failed",
				"service_name", h.serviceName,
				"check_name", checkName,
				"error", err,
			)
			return err
		}
	}

	return nil
}

func (h *Handler) writeResponse(writer http.ResponseWriter, statusCode int, message string) {
	writer.WriteHeader(statusCode)
	_, _ = writer.Write([]byte(message))
}
