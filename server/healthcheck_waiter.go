package server

import (
	"log/slog"
	"sync"
	"time"
)

// healthCheckWaiter monitors whether EnableHealthCheck() is called within a timeout.
//
// Why this exists:
// In Kubernetes, services start receiving traffic as soon as health checks pass.
// If health checks are enabled by default, traffic would route to services still
// initializing (connecting to DB, warming caches, etc).
//
// This forces developers to explicitly call EnableHealthCheck() after initialization,
// ensuring the service is truly ready. If they forget, we panic after timeout to
// fail fast rather than silently accepting traffic too early.
type healthCheckWaiter struct {
	server       *TelemetryServer
	timeout      time.Duration
	pollInterval time.Duration

	mutex    sync.Mutex
	stopChan chan struct{}
	stopped  bool
}

func newHealthCheckWaiter(server *TelemetryServer, timeout time.Duration,
	pollInterval time.Duration) *healthCheckWaiter {
	return &healthCheckWaiter{
		server:       server,
		timeout:      timeout,
		pollInterval: pollInterval,
		stopChan:     make(chan struct{}),
	}
}

func (w *healthCheckWaiter) start() {
	go w.waitForHealthCheckEnabled()
}

func (w *healthCheckWaiter) stop() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.stopped {
		return
	}

	w.stopped = true
	close(w.stopChan)
}

func (w *healthCheckWaiter) waitForHealthCheckEnabled() {
	deadline := time.Now().Add(w.timeout)
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			slog.Debug("Health check watcher stopped")
			return

		case <-ticker.C:
			if !w.server.IsRunning() {
				return
			}

			if w.server.IsHealthCheckEnabled() {
				slog.Info("Health check enabled successfully")
				return
			}

			if time.Now().After(deadline) {
				msg := "Health check not enabled within timeout - please call EnableHealthCheck()"
				slog.Error(msg, "timeout", w.timeout)
				panic(msg)
			}

			remainingTime := time.Until(deadline)
			slog.Warn("Health check still not enabled - waiting", "remaining", remainingTime)
		}
	}
}
