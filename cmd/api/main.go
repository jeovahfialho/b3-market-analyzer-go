package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/jeovahfialho/b3-analyzer/internal/api"
	"github.com/jeovahfialho/b3-analyzer/internal/config"
	"github.com/jeovahfialho/b3-analyzer/internal/ingestion"
	"github.com/jeovahfialho/b3-analyzer/internal/service"
	"github.com/jeovahfialho/b3-analyzer/internal/storage/cache"
	"github.com/jeovahfialho/b3-analyzer/internal/storage/postgres"
	pkglogger "github.com/jeovahfialho/b3-analyzer/pkg/logger"
)

// @title B3 Market Data Analyzer API
// @version 1.0
// @description API para análise de dados do mercado B3
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8000
// @BasePath /api/v1
// @schemes http https
func main() {
	cfg := config.Load()

	if err := pkglogger.Init(cfg.LogLevel, cfg.Environment == "development"); err != nil {
		log.Fatal("Erro ao inicializar logger:", err)
	}
	defer pkglogger.Close()

	db, err := connectPostgres(cfg)
	if err != nil {
		log.Fatal("Erro ao conectar PostgreSQL:", err)
	}
	defer db.Close()

	cacheService := connectRedis(cfg)
	if cacheService != nil {
		defer cacheService.Close()
	}

	// Services
	aggregationService := service.NewAggregationService(db.Pool(), nil, cfg.CacheTTL)
	tradeService := service.NewTradeService(db.Pool())
	analysisService := service.NewAnalysisService(db.Pool())

	// Ingestion
	parser := ingestion.NewParser(cfg.BatchSize, cfg.Workers)
	loader := ingestion.NewBulkLoader(db.Pool(), cfg.BatchSize)
	ingestionService := service.NewIngestionService(parser, loader)

	// Handler
	handler := api.NewHandler(
		db,
		cacheService,
		aggregationService,
		tradeService,
		analysisService,
		ingestionService,
	)

	// Fiber app
	app := fiber.New(fiber.Config{
		Prefork:                 false,
		ServerHeader:            "B3-Analyzer",
		DisableStartupMessage:   false,
		AppName:                 "B3 Market Data Analyzer v1.0.0",
		ReadTimeout:             cfg.APIReadTimeout,
		WriteTimeout:            cfg.APIWriteTimeout,
		IdleTimeout:             120 * time.Second,
		ReadBufferSize:          8192,
		WriteBufferSize:         8192,
		CompressedFileSuffix:    ".gz",
		ProxyHeader:             "X-Forwarded-For",
		EnableTrustedProxyCheck: true,
		BodyLimit:               10 * 1024 * 1024, // 10MB
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Setup routes
	api.SetupRoutes(app, handler)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.APIHost, cfg.APIPort)
	log.Printf("Starting server on %s", addr)

	if err := app.Listen(addr); err != nil {
		log.Fatal("Server error:", err)
	}
}

func connectPostgres(cfg *config.Config) (*postgres.DB, error) {
	db, err := postgres.NewDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar conexão: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.HealthCheck(ctx); err != nil {
		return nil, fmt.Errorf("erro ao testar conexão: %w", err)
	}

	log.Println("✅ Conectado ao PostgreSQL")
	return db, nil
}

func connectRedis(cfg *config.Config) *cache.RedisCache {
	redisCache, err := cache.NewRedisCache(cfg)
	if err != nil {
		log.Printf("⚠️ Redis não disponível: %v (continuando sem cache)", err)
		return nil
	}

	log.Println("✅ Conectado ao Redis")
	return redisCache
}
