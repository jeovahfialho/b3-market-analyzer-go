package api

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jeovahfialho/b3-analyzer/internal/service"
	"github.com/jeovahfialho/b3-analyzer/internal/storage/cache"
	"github.com/jeovahfialho/b3-analyzer/internal/storage/postgres"
	"github.com/jeovahfialho/b3-analyzer/pkg/logger"
	"github.com/jeovahfialho/b3-analyzer/pkg/metrics"
	"go.uber.org/zap"
)

type Handler struct {
	db                 *postgres.DB
	cacheService       *cache.RedisCache
	aggregationService *service.AggregationService
	tradeService       *service.TradeService
	analysisService    *service.AnalysisService
	ingestionService   *service.IngestionService
}

func NewHandler(
	db *postgres.DB,
	cacheService *cache.RedisCache,
	aggregationService *service.AggregationService,
	tradeService *service.TradeService,
	analysisService *service.AnalysisService,
	ingestionService *service.IngestionService,
) *Handler {
	return &Handler{
		db:                 db,
		cacheService:       cacheService,
		aggregationService: aggregationService,
		tradeService:       tradeService,
		analysisService:    analysisService,
		ingestionService:   ingestionService,
	}
}

func (h *Handler) GetTickerAggregation(c *fiber.Ctx) error {
	start := time.Now()

	ticker := c.Params("ticker")
	if ticker == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error:     "ticker é obrigatório",
			Code:      fiber.StatusBadRequest,
			RequestID: c.Locals("requestID").(string),
			Timestamp: time.Now(),
		})
	}

	var startDate *time.Time
	if dateStr := c.Query("start_date"); dateStr != "" {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error:     "formato de data inválido (use YYYY-MM-DD)",
				Code:      fiber.StatusBadRequest,
				RequestID: c.Locals("requestID").(string),
				Timestamp: time.Now(),
			})
		}
		startDate = &parsed
	}

	logger.Info("buscando agregação",
		zap.String("ticker", ticker),
		zap.Any("start_date", startDate),
		zap.String("request_id", c.Locals("requestID").(string)))

	aggregation, err := h.aggregationService.GetTickerAggregation(
		c.Context(),
		ticker,
		startDate,
	)

	if err != nil {
		logger.Error("erro ao buscar agregação",
			zap.String("ticker", ticker),
			zap.Error(err))

		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:     "erro ao buscar agregação",
			Code:      fiber.StatusInternalServerError,
			RequestID: c.Locals("requestID").(string),
			Timestamp: time.Now(),
		})
	}

	if aggregation.MaxRangeValue.IsZero() && aggregation.MaxDailyVolume == 0 {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{
			Error:     fmt.Sprintf("nenhum dado encontrado para o ticker %s", ticker),
			Code:      fiber.StatusNotFound,
			RequestID: c.Locals("requestID").(string),
			Timestamp: time.Now(),
		})
	}

	processingTime := time.Since(start)

	response := TickerAggregationResponse{
		Ticker:         aggregation.Ticker,
		MaxRangeValue:  aggregation.MaxRangeValue,
		MaxDailyVolume: aggregation.MaxDailyVolume,
		ProcessingTime: processingTime.String(),
	}

	metrics.RecordAggregationRequest(ticker, false)

	return c.JSON(response)
}

func (h *Handler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	})
}

func (h *Handler) ReadinessCheck(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	services := make(map[string]ServiceHealth)

	dbStart := time.Now()
	if err := h.db.HealthCheck(ctx); err != nil {
		services["database"] = ServiceHealth{
			Status: "unhealthy",
			Error:  err.Error(),
		}
	} else {
		services["database"] = ServiceHealth{
			Status:  "healthy",
			Latency: time.Since(dbStart).String(),
		}
	}

	redisStart := time.Now()
	if err := h.cacheService.HealthCheck(ctx); err != nil {
		services["redis"] = ServiceHealth{
			Status: "unhealthy",
			Error:  err.Error(),
		}
	} else {
		services["redis"] = ServiceHealth{
			Status:  "healthy",
			Latency: time.Since(redisStart).String(),
		}
	}

	status := "ready"
	for _, service := range services {
		if service.Status != "healthy" {
			status = "not_ready"
			break
		}
	}

	response := HealthResponse{
		Status:    status,
		Version:   "1.0.0",
		Timestamp: time.Now(),
		Services:  services,
	}

	if status != "ready" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(response)
	}

	return c.JSON(response)
}

func (h *Handler) GetTickerHistory(c *fiber.Ctx) error {
	ticker := c.Params("ticker")

	var startDate, endDate *time.Time

	if dateStr := c.Query("start_date"); dateStr != "" {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "formato de data inicial inválido",
				Code:  fiber.StatusBadRequest,
			})
		}
		startDate = &parsed
	}

	if dateStr := c.Query("end_date"); dateStr != "" {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "formato de data final inválido",
				Code:  fiber.StatusBadRequest,
			})
		}
		endDate = &parsed
	}

	history, err := h.tradeService.GetTickerHistory(c.Context(), ticker, startDate, endDate)
	if err != nil {
		logger.Error("erro ao buscar histórico",
			zap.String("ticker", ticker),
			zap.Error(err))

		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "erro ao buscar histórico",
			Code:  fiber.StatusInternalServerError,
		})
	}

	return c.JSON(fiber.Map{
		"ticker":  ticker,
		"history": history,
		"count":   len(history),
	})
}

func (h *Handler) GetTickerStats(c *fiber.Ctx) error {
	ticker := c.Params("ticker")
	days := c.QueryInt("days", 30)

	stats, err := h.tradeService.GetTickerStats(c.Context(), ticker, days)
	if err != nil {
		logger.Error("erro ao buscar estatísticas",
			zap.String("ticker", ticker),
			zap.Error(err))

		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "erro ao buscar estatísticas",
			Code:  fiber.StatusInternalServerError,
		})
	}

	return c.JSON(stats)
}

func (h *Handler) RefreshViews(c *fiber.Ctx) error {
	ctx := c.Context()

	start := time.Now()
	if err := h.aggregationService.RefreshMaterializedViews(ctx); err != nil {
		logger.Error("erro ao atualizar views", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:     "erro ao atualizar views",
			Code:      fiber.StatusInternalServerError,
			RequestID: getRequestID(c),
			Timestamp: time.Now(),
		})
	}

	duration := time.Since(start)

	return c.JSON(fiber.Map{
		"status":   "success",
		"message":  "views atualizadas com sucesso",
		"duration": duration.String(),
	})
}

func (h *Handler) InvalidateCache(c *fiber.Ctx) error {
	pattern := c.Params("pattern", "*")

	if err := h.cacheService.DeletePattern(c.Context(), pattern); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error:     "erro ao invalidar cache",
			Code:      fiber.StatusInternalServerError,
			RequestID: getRequestID(c),
			Timestamp: time.Now(),
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("cache invalidado para padrão: %s", pattern),
	})
}

func (h *Handler) GetSystemStats(c *fiber.Ctx) error {

	dbStats := h.db.Stats()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	response := SystemStatsResponse{
		Database: DatabaseStats{
			ActiveConnections: dbStats.AcquiredConns(),
			IdleConnections:   dbStats.IdleConns(),
			TotalConnections:  dbStats.TotalConns(),
			WaitCount:         dbStats.EmptyAcquireCount(),
			WaitDuration:      dbStats.AcquireDuration().String(),
		},
		Cache: CacheStats{
			MemoryUsed: fmt.Sprintf("%d MB", m.Alloc/1024/1024),
		},
		API: APIStats{
			ActiveGoroutines: runtime.NumGoroutine(),
		},
	}

	return c.JSON(response)
}

func (h *Handler) LoadDataFromFile(c *fiber.Ctx) error {
	var req LoadDataRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "corpo da requisição inválido",
			Code:  fiber.StatusBadRequest,
		})
	}

	if req.Async {

		jobID := generateJobID()

		go func() {
			ctx := context.Background()
			result, err := h.ingestionService.ProcessFile(ctx, req.FilePath)

			if err != nil {
				logger.Error("erro ao processar arquivo",
					zap.String("file", req.FilePath),
					zap.String("job_id", jobID),
					zap.Error(err))
			} else {
				logger.Info("arquivo processado com sucesso",
					zap.String("file", req.FilePath),
					zap.String("job_id", jobID),
					zap.Int64("records", result.RecordsCount))
			}
		}()

		return c.JSON(LoadDataResponse{
			JobID:   jobID,
			Status:  "processing",
			Message: "processamento iniciado",
		})
	}

	result, err := h.ingestionService.ProcessFile(c.Context(), req.FilePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "erro ao processar arquivo",
			Code:  fiber.StatusInternalServerError,
		})
	}

	return c.JSON(LoadDataResponse{
		RecordsCount: result.RecordsCount,
		Status:       "completed",
		Message:      "arquivo processado com sucesso",
	})
}

func (h *Handler) GetTopVolume(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)

	result, err := h.analysisService.GetTopVolumeTickets(c.Context(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "erro ao buscar top volume",
			Code:  fiber.StatusInternalServerError,
		})
	}

	return c.JSON(fiber.Map{
		"data":  result,
		"count": len(result),
	})
}

func (h *Handler) GetPriceRange(c *fiber.Ctx) error {
	ticker := c.Query("ticker", "")
	days := c.QueryInt("days", 30)

	if ticker == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "ticker é obrigatório",
			Code:  fiber.StatusBadRequest,
		})
	}

	result, err := h.analysisService.GetPriceRange(c.Context(), ticker, days)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "erro ao buscar range de preços",
			Code:  fiber.StatusInternalServerError,
		})
	}

	return c.JSON(result)
}

func (h *Handler) GetVolatility(c *fiber.Ctx) error {
	ticker := c.Query("ticker", "")
	days := c.QueryInt("days", 30)

	if ticker == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "ticker é obrigatório",
			Code:  fiber.StatusBadRequest,
		})
	}

	result, err := h.analysisService.GetVolatility(c.Context(), ticker, days)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "erro ao calcular volatilidade",
			Code:  fiber.StatusInternalServerError,
		})
	}

	return c.JSON(result)
}

func generateJobID() string {
	return fmt.Sprintf("job_%d_%s", time.Now().Unix(), randomString(8))
}

func getRequestID(c *fiber.Ctx) string {
	if id := c.Locals("requestID"); id != nil {
		return id.(string)
	}
	return ""
}
