//go:build wireinject
// +build wireinject

package doakeswire

import (
	"github.com/domesama/doakes/server"
	"github.com/google/wire"
)

// InitializeTelemetryServer creates a fully configured internal telemetry server using Wire.
// The server is created but NOT started. You must call Start() yourself.
// To get a meter scoped to your service name, call GetMeter() after initialization.
func InitializeTelemetryServer() (*server.TelemetryServer, error) {
	wire.Build(TelemetrySet)
	return nil, nil
}

// InitializeTelemetryServerWithAutoStart creates and starts a internal telemetry server using Wire.
// Returns the server, a cleanup function, and an error.
// The server is started but health checks are NOT enabled - call EnableHealthCheck() after setup.
// To get a meter scoped to your service name, call GetMeter() after initialization.
//
// Usage:
//
//	srv, cleanup, err := wire.InitializeTelemetryServerWithAutoStart()
//	if err != nil {
//	    return err
//	}
//	defer cleanup()
//
//	srv.RegisterHealthCheck("database", checkDB)
//	srv.EnableHealthCheck()
//
//	// Get meter for creating metrics
//	meter := doakeswire.GetMeter()
//	counter, _ := meter.Int64Counter("requests_total")
func InitializeTelemetryServerWithAutoStart() (*server.TelemetryServer, func(), error) {
	wire.Build(TelemetrySetWithAutoStart)
	return nil, nil, nil
}
