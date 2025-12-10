//go:build wireinject
// +build wireinject

package doakeswire

import (
	"github.com/domesama/doakes/server"
	"github.com/google/wire"
)

// InitializeTelemetryServer creates a fully configured internal telemetry server using Wire.
// The server is created but NOT started. You must call Start() yourself.
func InitializeTelemetryServer() (*server.TelemetryServer, error) {
	wire.Build(TelemetrySet)
	return nil, nil
}

// InitializeTelemetryServerWithAutoStart creates and starts a internal telemetry server using Wire.
// Returns the server, a cleanup function, and an error.
// The server is started but health checks are NOT enabled - call EnableHealthCheck() after setup.
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
func InitializeTelemetryServerWithAutoStart() (*server.TelemetryServer, func(), error) {
	wire.Build(TelemetrySetWithAutoStart)
	return nil, nil, nil
}
