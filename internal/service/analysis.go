package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type AnalysisService struct {
	pool *pgxpool.Pool
}

func NewAnalysisService(pool *pgxpool.Pool) *AnalysisService {
	return &AnalysisService{pool: pool}
}

type TopVolumeTicker struct {
	Ticker      string          `json:"ticker"`
	TotalVolume int64           `json:"total_volume"`
	AvgPrice    decimal.Decimal `json:"avg_price"`
	TradeCount  int             `json:"trade_count"`
}

func (s *AnalysisService) GetTopVolumeTickets(ctx context.Context, limit int) ([]TopVolumeTicker, error) {

	return []TopVolumeTicker{}, nil
}

type PriceRangeResult struct {
	Ticker       string          `json:"ticker"`
	MinPrice     decimal.Decimal `json:"min_price"`
	MaxPrice     decimal.Decimal `json:"max_price"`
	Range        decimal.Decimal `json:"range"`
	RangePercent float64         `json:"range_percent"`
}

func (s *AnalysisService) GetPriceRange(ctx context.Context, ticker string, days int) (*PriceRangeResult, error) {

	return &PriceRangeResult{
		Ticker:   ticker,
		MinPrice: decimal.NewFromFloat(10.0),
		MaxPrice: decimal.NewFromFloat(20.0),
		Range:    decimal.NewFromFloat(10.0),
	}, nil
}

type VolatilityResult struct {
	Ticker       string  `json:"ticker"`
	Volatility   float64 `json:"volatility"`
	StdDev       float64 `json:"std_dev"`
	DaysAnalyzed int     `json:"days_analyzed"`
}

func (s *AnalysisService) GetVolatility(ctx context.Context, ticker string, days int) (*VolatilityResult, error) {

	return &VolatilityResult{
		Ticker:       ticker,
		Volatility:   15.5,
		StdDev:       2.3,
		DaysAnalyzed: days,
	}, nil
}
