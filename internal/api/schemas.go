package api

import (
	"time"

	"github.com/shopspring/decimal"
)

type TickerAggregationRequest struct {
	StartDate *time.Time `query:"start_date" format:"date"`
	EndDate   *time.Time `query:"end_date" format:"date"`
}

type TickerAggregationResponse struct {
	Ticker         string          `json:"ticker"`
	MaxRangeValue  decimal.Decimal `json:"max_range_value"`
	MaxDailyVolume int64           `json:"max_daily_volume"`
	CacheHit       bool            `json:"cache_hit,omitempty"`
	ProcessingTime string          `json:"processing_time,omitempty"`
}

type HealthResponse struct {
	Status    string                   `json:"status"`
	Version   string                   `json:"version"`
	Timestamp time.Time                `json:"timestamp"`
	Services  map[string]ServiceHealth `json:"services,omitempty"`
}

type ServiceHealth struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

type SystemStatsResponse struct {
	Database DatabaseStats `json:"database"`
	Cache    CacheStats    `json:"cache"`
	API      APIStats      `json:"api"`
}

type DatabaseStats struct {
	ActiveConnections int32  `json:"active_connections"`
	IdleConnections   int32  `json:"idle_connections"`
	TotalConnections  int32  `json:"total_connections"`
	WaitCount         int64  `json:"wait_count"`
	WaitDuration      string `json:"wait_duration"`
}

type CacheStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	HitRate    float64 `json:"hit_rate"`
	Keys       int64   `json:"keys"`
	MemoryUsed string  `json:"memory_used"`
}

type APIStats struct {
	RequestsTotal    int64   `json:"requests_total"`
	RequestsPerSec   float64 `json:"requests_per_sec"`
	AverageLatency   string  `json:"average_latency"`
	ErrorRate        float64 `json:"error_rate"`
	ActiveGoroutines int     `json:"active_goroutines"`
}

type ErrorResponse struct {
	Error     string    `json:"error"`
	Code      int       `json:"code"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type LoadDataRequest struct {
	FilePath string `json:"file_path" validate:"required"`
	Async    bool   `json:"async"`
}

type LoadDataResponse struct {
	JobID        string `json:"job_id,omitempty"`
	RecordsCount int64  `json:"records_count,omitempty"`
	Status       string `json:"status"`
	Message      string `json:"message"`
}

type TickerStatsRequest struct {
	Days int `query:"days" default:"30"`
}

type TickerStatsResponse struct {
	Ticker         string          `json:"ticker"`
	Period         string          `json:"period"`
	TotalVolume    int64           `json:"total_volume"`
	TotalTrades    int             `json:"total_trades"`
	AvgDailyVolume int64           `json:"avg_daily_volume"`
	AvgPrice       decimal.Decimal `json:"avg_price"`
	MinPrice       decimal.Decimal `json:"min_price"`
	MaxPrice       decimal.Decimal `json:"max_price"`
	PriceRange     decimal.Decimal `json:"price_range"`
	Volatility     float64         `json:"volatility"`
	DaysTraded     int             `json:"days_traded"`
	LastUpdate     time.Time       `json:"last_update"`
}

type MarketOverviewResponse struct {
	Date          string             `json:"date"`
	TotalVolume   int64              `json:"total_volume"`
	TotalTrades   int                `json:"total_trades"`
	ActiveTickers int                `json:"active_tickers"`
	TopGainers    []PriceMovementDTO `json:"top_gainers"`
	TopLosers     []PriceMovementDTO `json:"top_losers"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

type PriceMovementDTO struct {
	Ticker        string `json:"ticker"`
	CurrentPrice  string `json:"current_price"`
	PreviousPrice string `json:"previous_price"`
	Change        string `json:"change"`
	ChangePercent string `json:"change_percent"`
	DayHigh       string `json:"day_high"`
	DayLow        string `json:"day_low"`
}
