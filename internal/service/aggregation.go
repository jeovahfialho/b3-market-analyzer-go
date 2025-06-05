package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jeovahfialho/b3-analyzer/internal/domain"
	"github.com/shopspring/decimal"
)

type AggregationService struct {
	pool        *pgxpool.Pool
	redisClient *redis.Client
	cacheTTL    time.Duration
}

func NewAggregationService(pool *pgxpool.Pool, redisClient *redis.Client, cacheTTL time.Duration) *AggregationService {
	return &AggregationService{
		pool:        pool,
		redisClient: redisClient,
		cacheTTL:    cacheTTL,
	}
}

func (s *AggregationService) GetTickerAggregation(ctx context.Context, ticker string, startDate *time.Time) (*domain.Aggregation, error) {
	cacheKey := s.generateCacheKey(ticker, startDate)

	cached, err := s.getFromCache(ctx, cacheKey)
	if err == nil && cached != nil {
		return cached, nil
	}

	aggregation, err := s.queryAggregation(ctx, ticker, startDate)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar agregação: %w", err)
	}

	if err := s.saveToCache(ctx, cacheKey, aggregation); err != nil {
		// Log do erro de cache, mas não falha a operação
	}

	return aggregation, nil
}

func (s *AggregationService) queryAggregation(ctx context.Context, ticker string, startDate *time.Time) (*domain.Aggregation, error) {
	query := `
		WITH ticker_data AS (
			SELECT
				max_price,
				total_volume
			FROM daily_aggregations
			WHERE codigo_instrumento = $1
			%s
		)
		SELECT
			COALESCE(MAX(max_price), 0) as max_range_value,
			COALESCE(MAX(total_volume), 0) as max_daily_volume
		FROM ticker_data
	`

	args := []interface{}{ticker}
	dateFilter := ""

	if startDate != nil {
		dateFilter = "AND data_negocio >= $2"
		args = append(args, *startDate)
	}

	query = fmt.Sprintf(query, dateFilter)

	var maxRangeValue decimal.Decimal
	var maxDailyVolume int64

	err := s.pool.QueryRow(ctx, query, args...).Scan(&maxRangeValue, &maxDailyVolume)
	if err != nil {
		return nil, err
	}

	return &domain.Aggregation{
		Ticker:         ticker,
		MaxRangeValue:  maxRangeValue,
		MaxDailyVolume: maxDailyVolume,
	}, nil
}

func (s *AggregationService) generateCacheKey(ticker string, startDate *time.Time) string {
	if startDate == nil {
		return fmt.Sprintf("agg:%s:all", ticker)
	}
	return fmt.Sprintf("agg:%s:%s", ticker, startDate.Format("2006-01-02"))
}

func (s *AggregationService) getFromCache(ctx context.Context, key string) (*domain.Aggregation, error) {
	if s.redisClient == nil {
		return nil, fmt.Errorf("redis not available")
	}

	val, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var aggregation domain.Aggregation
	if err := json.Unmarshal([]byte(val), &aggregation); err != nil {
		return nil, err
	}

	return &aggregation, nil
}

func (s *AggregationService) saveToCache(ctx context.Context, key string, aggregation *domain.Aggregation) error {
	if s.redisClient == nil {
		return nil
	}

	data, err := json.Marshal(aggregation)
	if err != nil {
		return err
	}

	return s.redisClient.Set(ctx, key, data, s.cacheTTL).Err()
}

func (s *AggregationService) RefreshMaterializedViews(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY daily_aggregations")
	return err
}
