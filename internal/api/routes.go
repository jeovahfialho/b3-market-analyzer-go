package api

import (
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRoutes(app *fiber.App, handler *Handler) {
	// Global middlewares
	app.Use(RequestID())
	app.Use(ErrorHandler())

	// Health checks (sem rate limiting)
	app.Get("/health", handler.HealthCheck)
	app.Get("/ready", handler.ReadinessCheck)

	// Metrics endpoint para Prometheus (sem rate limiting)
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// Swagger documentation (sem rate limiting)
	app.Get("/swagger/*", swagger.HandlerDefault)

	// API v1 - com middlewares de rate limiting e métricas
	v1 := app.Group("/api/v1")
	v1.Use(RateLimiter())
	v1.Use(PrometheusMiddleware())

	// Ticker routes
	ticker := v1.Group("/ticker")
	ticker.Get("/:ticker/aggregation", handler.GetTickerAggregation)
	ticker.Get("/:ticker/history", handler.GetTickerHistory)
	ticker.Get("/:ticker/stats", handler.GetTickerStats)

	// Admin routes
	admin := v1.Group("/admin")
	admin.Use(BasicAuth())
	admin.Post("/refresh-views", handler.RefreshViews)
	admin.Delete("/cache/:pattern", handler.InvalidateCache)
	admin.Get("/stats", handler.GetSystemStats)
	admin.Post("/load", handler.LoadDataFromFile)

	// Analysis routes
	analysis := v1.Group("/analysis")
	analysis.Get("/top-volume", handler.GetTopVolume)
	analysis.Get("/price-range", handler.GetPriceRange)
	analysis.Get("/volatility", handler.GetVolatility)
}

func BasicAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth != "Basic YWRtaW46c2VjcmV0" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}
		return c.Next()
	}
}
