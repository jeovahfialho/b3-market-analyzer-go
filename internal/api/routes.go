package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
)

func SetupRoutes(app *fiber.App, handler *Handler) {

	app.Get("/swagger/*", swagger.HandlerDefault)

	app.Get("/health", handler.HealthCheck)
	app.Get("/ready", handler.ReadinessCheck)

	v1 := app.Group("/api/v1")

	v1.Use(RateLimiter())
	v1.Use(PrometheusMiddleware())

	ticker := v1.Group("/ticker")
	ticker.Get("/:ticker", handler.GetTickerAggregation)
	ticker.Get("/:ticker/history", handler.GetTickerHistory)
	ticker.Get("/:ticker/stats", handler.GetTickerStats)

	admin := v1.Group("/admin")
	admin.Use(BasicAuth())

	admin.Post("/refresh-views", handler.RefreshViews)
	admin.Delete("/cache/:pattern", handler.InvalidateCache)
	admin.Get("/stats", handler.GetSystemStats)
	admin.Post("/load", handler.LoadDataFromFile)

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
