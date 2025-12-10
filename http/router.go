package http

import (
	"net/http"

	"github.com/domesama/doakes/healthcheck"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

// RouterConfig contains handlers for the internal server routes.
type RouterConfig struct {
	HealthCheckHandler http.Handler
	MetricsHandler     http.Handler
	IndexHandler       gin.HandlerFunc
}

// NewRouter creates a new Gin router with all internal server routes registered.
func NewRouter(config RouterConfig) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	registerAllRoutes(router, config)

	return router
}

func registerAllRoutes(router *gin.Engine, config RouterConfig) {
	registerIndexRoute(router, config.IndexHandler)
	registerHealthCheckRoute(router, config.HealthCheckHandler)
	registerMetricsRoute(router, config.MetricsHandler)
	registerProfilingRoutes(router)
}

func registerIndexRoute(router *gin.Engine, handler gin.HandlerFunc) {
	router.GET("/", handler)
}

func registerHealthCheckRoute(router *gin.Engine, handler http.Handler) {
	router.GET("/_hc", gin.WrapH(handler))
}

func registerMetricsRoute(router *gin.Engine, handler http.Handler) {
	router.GET("/metrics", gin.WrapH(handler))
}

func registerProfilingRoutes(router *gin.Engine) {
	profilingGroup := router.Group("/debug/pprof/")
	pprof.RouteRegister(profilingGroup, "")
}

// CreateIndexHandler creates a handler that returns basic service information.
func CreateIndexHandler(serviceName string, serviceVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(
			http.StatusOK, gin.H{
				"service": serviceName,
				"version": serviceVersion,
				"status":  "running",
			},
		)
	}
}

// NewHealthCheckHandler creates a new health check handler for the given service.
func NewHealthCheckHandler(serviceName string) *healthcheck.Handler {
	return healthcheck.NewHandler(serviceName)
}
