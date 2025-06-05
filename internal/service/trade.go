package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jeovahfialho/b3-analyzer/internal/domain"
	"github.com/jeovahfialho/b3-analyzer/pkg/logger"
	"github.com/jeovahfialho/b3-analyzer/pkg/metrics"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type TradeService struct {
	pool *pgxpool.Pool
}

func NewTradeService(pool *pgxpool.Pool) *TradeService {
	return &TradeService{pool: pool}
}

func (s *TradeService) GetTickerHistory(ctx context.Context, ticker string, startDate, endDate *time.Time) ([]domain.DailyAggregation, error) {
	timer := metrics.NewTimer()
	defer timer.ObserveDuration(metrics.DatabaseQueryDuration.WithLabelValues("ticker_history"))

	query := `
        SELECT 
            codigo_instrumento,
            data_negocio,
            max_price,
            min_price,
            avg_price,
            total_volume,
            trade_count,
            price_stddev
        FROM daily_aggregations
        WHERE codigo_instrumento = $1
    `

	args := []interface{}{ticker}
	argCount := 1

	if startDate != nil {
		argCount++
		query += fmt.Sprintf(" AND data_negocio >= $%d", argCount)
		args = append(args, *startDate)
	}

	if endDate != nil {
		argCount++
		query += fmt.Sprintf(" AND data_negocio <= $%d", argCount)
		args = append(args, *endDate)
	}

	query += " ORDER BY data_negocio DESC"

	logger.Debug("executando query de histórico",
		zap.String("ticker", ticker),
		zap.Any("start_date", startDate),
		zap.Any("end_date", endDate))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		metrics.DatabaseQueries.WithLabelValues("ticker_history", "error").Inc()
		return nil, fmt.Errorf("erro ao buscar histórico: %w", err)
	}
	defer rows.Close()

	var history []domain.DailyAggregation
	for rows.Next() {
		var agg domain.DailyAggregation
		var priceStdDev *decimal.Decimal

		err := rows.Scan(
			&agg.CodigoInstrumento,
			&agg.DataNegocio,
			&agg.MaxPrice,
			&agg.MinPrice,
			&agg.AvgPrice,
			&agg.TotalVolume,
			&agg.TradeCount,
			&priceStdDev,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear linha: %w", err)
		}

		if priceStdDev != nil {
			agg.PriceStdDev = *priceStdDev
		}

		history = append(history, agg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar resultados: %w", err)
	}

	metrics.DatabaseQueries.WithLabelValues("ticker_history", "success").Inc()
	logger.Info("histórico recuperado",
		zap.String("ticker", ticker),
		zap.Int("records", len(history)))

	return history, nil
}

func (s *TradeService) GetRecentTrades(ctx context.Context, ticker string, limit int) ([]domain.Trade, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	timer := metrics.NewTimer()
	defer timer.ObserveDuration(metrics.DatabaseQueryDuration.WithLabelValues("recent_trades"))

	query := `
        SELECT 
            id,
            hora_fechamento,
            data_negocio,
            codigo_instrumento,
            preco_negocio,
            quantidade_negociada,
            created_at
        FROM trades
        WHERE codigo_instrumento = $1
        ORDER BY data_negocio DESC, hora_fechamento DESC
        LIMIT $2
    `

	rows, err := s.pool.Query(ctx, query, ticker, limit)
	if err != nil {
		metrics.DatabaseQueries.WithLabelValues("recent_trades", "error").Inc()
		return nil, fmt.Errorf("erro ao buscar trades recentes: %w", err)
	}
	defer rows.Close()

	trades := make([]domain.Trade, 0, limit)
	for rows.Next() {
		var trade domain.Trade
		err := rows.Scan(
			&trade.ID,
			&trade.HoraFechamento,
			&trade.DataNegocio,
			&trade.CodigoInstrumento,
			&trade.PrecoNegocio,
			&trade.QuantidadeNegociada,
			&trade.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear trade: %w", err)
		}
		trades = append(trades, trade)
	}

	metrics.DatabaseQueries.WithLabelValues("recent_trades", "success").Inc()
	return trades, nil
}

func (s *TradeService) GetTradesByDate(ctx context.Context, ticker string, date time.Time) ([]domain.Trade, error) {
	query := `
        SELECT 
            id,
            hora_fechamento,
            data_negocio,
            codigo_instrumento,
            preco_negocio,
            quantidade_negociada,
            created_at
        FROM trades
        WHERE codigo_instrumento = $1 AND data_negocio = $2
        ORDER BY hora_fechamento ASC
    `

	rows, err := s.pool.Query(ctx, query, ticker, date)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar trades por data: %w", err)
	}
	defer rows.Close()

	var trades []domain.Trade
	for rows.Next() {
		var trade domain.Trade
		err := rows.Scan(
			&trade.ID,
			&trade.HoraFechamento,
			&trade.DataNegocio,
			&trade.CodigoInstrumento,
			&trade.PrecoNegocio,
			&trade.QuantidadeNegociada,
			&trade.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("erro ao escanear trade: %w", err)
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

func (s *TradeService) GetTickerStats(ctx context.Context, ticker string, days int) (*domain.TickerStats, error) {
	timer := metrics.NewTimer()
	defer timer.ObserveDuration(metrics.DatabaseQueryDuration.WithLabelValues("ticker_stats"))

	query := `
        WITH stats AS (
            SELECT 
                COUNT(DISTINCT data_negocio) as days_traded,
                SUM(total_volume) as total_volume,
                SUM(trade_count) as total_trades,
                AVG(total_volume) as avg_daily_volume,
                AVG(avg_price) as avg_price,
                MIN(min_price) as min_price,
                MAX(max_price) as max_price,
                STDDEV(avg_price) as price_stddev
            FROM daily_aggregations
            WHERE codigo_instrumento = $1
            AND data_negocio >= CURRENT_DATE - INTERVAL '%d days'
        )
        SELECT * FROM stats
    `

	query = fmt.Sprintf(query, days)

	var stats domain.TickerStats
	var priceStdDev float64

	err := s.pool.QueryRow(ctx, query, ticker).Scan(
		&stats.DaysTraded,
		&stats.TotalVolume,
		&stats.TotalTrades,
		&stats.AvgDailyVolume,
		&stats.AvgPrice,
		&stats.MinPrice,
		&stats.MaxPrice,
		&priceStdDev,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("nenhum dado encontrado para ticker %s", ticker)
	}
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar estatísticas: %w", err)
	}

	stats.Ticker = ticker
	stats.Period = fmt.Sprintf("%d days", days)
	stats.PriceRange = stats.MaxPrice.Sub(stats.MinPrice)
	stats.Volatility = priceStdDev * 15.87
	stats.LastUpdate = time.Now()

	metrics.DatabaseQueries.WithLabelValues("ticker_stats", "success").Inc()
	return &stats, nil
}

func (s *TradeService) GetMarketOverview(ctx context.Context, date time.Time) (*domain.MarketOverview, error) {

	statsQuery := `
        SELECT 
            COUNT(DISTINCT codigo_instrumento) as active_tickers,
            SUM(total_volume) as total_volume,
            SUM(trade_count) as total_trades
        FROM daily_aggregations
        WHERE data_negocio = $1
    `

	var overview domain.MarketOverview
	overview.Date = date

	err := s.pool.QueryRow(ctx, statsQuery, date).Scan(
		&overview.ActiveTickers,
		&overview.TotalVolume,
		&overview.TotalTrades,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar estatísticas gerais: %w", err)
	}

	moversQuery := `
        WITH price_changes AS (
            SELECT 
                t1.codigo_instrumento,
                t1.avg_price as current_price,
                t2.avg_price as previous_price,
                t1.max_price as day_high,
                t1.min_price as day_low,
                ((t1.avg_price - t2.avg_price) / t2.avg_price * 100) as change_percent
            FROM daily_aggregations t1
            JOIN daily_aggregations t2 
                ON t1.codigo_instrumento = t2.codigo_instrumento
                AND t2.data_negocio = t1.data_negocio - INTERVAL '1 day'
            WHERE t1.data_negocio = $1
        )
        SELECT * FROM (
            SELECT * FROM price_changes 
            ORDER BY change_percent DESC 
            LIMIT 5
        ) gainers
        UNION ALL
        SELECT * FROM (
            SELECT * FROM price_changes 
            ORDER BY change_percent ASC 
            LIMIT 5
        ) losers
    `

	rows, err := s.pool.Query(ctx, moversQuery, date)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar movers: %w", err)
	}
	defer rows.Close()

	var gainers, losers []domain.PriceMovement
	count := 0

	for rows.Next() {
		var movement domain.PriceMovement
		var changePercent float64

		err := rows.Scan(
			&movement.Ticker,
			&movement.CurrentPrice,
			&movement.PreviousPrice,
			&movement.DayHigh,
			&movement.DayLow,
			&changePercent,
		)
		if err != nil {
			continue
		}

		movement.Change = movement.CurrentPrice.Sub(movement.PreviousPrice)
		movement.ChangePercent = changePercent

		if count < 5 {
			gainers = append(gainers, movement)
		} else {
			losers = append(losers, movement)
		}
		count++
	}

	overview.TopGainers = gainers
	overview.TopLosers = losers
	overview.UpdatedAt = time.Now()

	return &overview, nil
}
