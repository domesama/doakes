package healthcheck_test

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/domesama/doakes/healthcheck"
	"github.com/stretchr/testify/assert"
)

func TestHandler_RegisterAndEnable(t *testing.T) {
	handler := healthcheck.NewHandler("test-service")

	called := false
	handler.RegisterCheck(
		"test", func() error {
			called = true
			return nil
		},
	)

	// Before enabling, should return 503
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, nil)
	assert.Equal(t, 503, recorder.Code)
	assert.Equal(t, "not enabled", recorder.Body.String())
	assert.False(t, called, "check should not be called when disabled")

	// After enabling, should return 200
	handler.Enable()
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, nil)
	assert.Equal(t, 200, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
	assert.True(t, called, "check should be called when enabled")
}

func TestHandler_EmptyChecks(t *testing.T) {
	handler := healthcheck.NewHandler("test-service")
	handler.Enable()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, nil)

	assert.Equal(t, 200, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
}

func TestHandler_SuccessfulChecks(t *testing.T) {
	handler := healthcheck.NewHandler("test-service")

	check1Called := false
	check2Called := false

	handler.RegisterCheck(
		"database", func() error {
			check1Called = true
			return nil
		},
	)

	handler.RegisterCheck(
		"cache", func() error {
			check2Called = true
			return nil
		},
	)

	handler.Enable()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, nil)

	assert.Equal(t, 200, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
	assert.True(t, check1Called, "database check should be called")
	assert.True(t, check2Called, "cache check should be called")
}

func TestHandler_FailedCheck(t *testing.T) {
	handler := healthcheck.NewHandler("test-service")

	handler.RegisterCheck(
		"database", func() error {
			return nil
		},
	)

	handler.RegisterCheck(
		"cache", func() error {
			return errors.New("cache connection failed")
		},
	)

	handler.Enable()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, nil)

	assert.Equal(t, 503, recorder.Code)
	assert.Equal(t, "unhealthy", recorder.Body.String())
}

func TestHandler_AllChecksFail(t *testing.T) {
	handler := healthcheck.NewHandler("test-service")

	handler.RegisterCheck(
		"database", func() error {
			return errors.New("database down")
		},
	)

	handler.RegisterCheck(
		"cache", func() error {
			return errors.New("cache down")
		},
	)

	handler.Enable()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, nil)

	// Should fail on first error
	assert.Equal(t, 503, recorder.Code)
	assert.Equal(t, "unhealthy", recorder.Body.String())
}

func TestHandler_IsEnabled(t *testing.T) {
	handler := healthcheck.NewHandler("test-service")

	assert.False(t, handler.IsEnabled(), "should not be enabled initially")

	handler.Enable()

	assert.True(t, handler.IsEnabled(), "should be enabled after Enable()")
}

func TestHandler_DuplicateRegistration(t *testing.T) {
	handler := healthcheck.NewHandler("test-service")

	firstCalled := false
	secondCalled := false

	handler.RegisterCheck(
		"duplicate", func() error {
			firstCalled = true
			return errors.New("should be overwritten")
		},
	)

	// Second registration should overwrite the first
	handler.RegisterCheck(
		"duplicate", func() error {
			secondCalled = true
			return nil
		},
	)

	handler.Enable()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, nil)

	assert.Equal(t, 200, recorder.Code)
	assert.False(t, firstCalled, "first check should be overwritten")
	assert.True(t, secondCalled, "second check should be called")
}

// NOTE: We simplified the handler to not use mutex during registration.
// This test documents WHY we don't test concurrent registration:
// - Registration happens during server initialization (single-threaded)
// - Checks don't change after Start() is called
// - Only the enabled flag needs thread-safety (for concurrent HTTP requests)
//
// If you need concurrent registration in the future, add mutex back to RegisterCheck().
func TestHandler_ConcurrentRequests(t *testing.T) {
	// This test verifies that multiple HTTP requests can check health concurrently
	// (the enabled flag is protected by mutex)

	handler := healthcheck.NewHandler("test-service")

	callCount := 0
	handler.RegisterCheck(
		"test", func() error {
			callCount++
			return nil
		},
	)

	handler.Enable()

	// Make 10 concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, nil)
			assert.Equal(t, 200, recorder.Code)
			done <- true
		}()
	}

	// Wait for all requests
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, callCount, "all checks should have been called")
}
