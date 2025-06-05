package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Aggregation struct {
	Ticker         string          `json:"ticker"`
	MaxRangeValue  decimal.Decimal `json:"max_range_value"`
	MaxDailyVolume int64           `json:"max_daily_volume"`
}

type DailyAggregation struct {
	CodigoInstrumento string          `db:"codigo_instrumento" json:"codigo_instrumento"`
	DataNegocio       time.Time       `db:"data_negocio" json:"data_negocio"`
	MaxPrice          decimal.Decimal `db:"max_price" json:"max_price"`
	MinPrice          decimal.Decimal `db:"min_price" json:"min_price"`
	AvgPrice          decimal.Decimal `db:"avg_price" json:"avg_price"`
	TotalVolume       int64           `db:"total_volume" json:"total_volume"`
	TradeCount        int             `db:"trade_count" json:"trade_count"`
	PriceStdDev       decimal.Decimal `db:"price_stddev" json:"price_stddev,omitempty"`
}

type TickerStats struct {
	Ticker         string             `json:"ticker"`
	Period         string             `json:"period"`
	TotalVolume    int64              `json:"total_volume"`
	TotalTrades    int                `json:"total_trades"`
	AvgDailyVolume int64              `json:"avg_daily_volume"`
	AvgPrice       decimal.Decimal    `json:"avg_price"`
	MinPrice       decimal.Decimal    `json:"min_price"`
	MaxPrice       decimal.Decimal    `json:"max_price"`
	PriceRange     decimal.Decimal    `json:"price_range"`
	Volatility     float64            `json:"volatility"`
	DaysTraded     int                `json:"days_traded"`
	LastUpdate     time.Time          `json:"last_update"`
	DailyStats     []DailyAggregation `json:"daily_stats,omitempty"`
}

type AggregationFilter struct {
	Tickers   []string   `json:"tickers,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	MinVolume *int64     `json:"min_volume,omitempty"`
	MaxVolume *int64     `json:"max_volume,omitempty"`
}

type AggregationResult struct {
	Data       []Aggregation `json:"data"`
	TotalCount int           `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	HasMore    bool          `json:"has_more"`
}

type VolumeRanking struct {
	Position            int     `json:"position"`
	Ticker              string  `json:"ticker"`
	TotalVolume         int64   `json:"total_volume"`
	AvgDailyVolume      int64   `json:"avg_daily_volume"`
	MaxDailyVolume      int64   `json:"max_daily_volume"`
	VolumeChangePercent float64 `json:"volume_change_percent"`
}

type PriceMovement struct {
	Ticker        string          `json:"ticker"`
	CurrentPrice  decimal.Decimal `json:"current_price"`
	PreviousPrice decimal.Decimal `json:"previous_price"`
	Change        decimal.Decimal `json:"change"`
	ChangePercent float64         `json:"change_percent"`
	DayHigh       decimal.Decimal `json:"day_high"`
	DayLow        decimal.Decimal `json:"day_low"`
	WeekHigh      decimal.Decimal `json:"week_high"`
	WeekLow       decimal.Decimal `json:"week_low"`
}

type MarketOverview struct {
	Date          time.Time       `json:"date"`
	TotalVolume   int64           `json:"total_volume"`
	TotalTrades   int             `json:"total_trades"`
	ActiveTickers int             `json:"active_tickers"`
	TopGainers    []PriceMovement `json:"top_gainers"`
	TopLosers     []PriceMovement `json:"top_losers"`
	MostTraded    []VolumeRanking `json:"most_traded"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
