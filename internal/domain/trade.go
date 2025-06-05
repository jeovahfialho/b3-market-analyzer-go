package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Trade struct {
	ID                  int64           `db:"id"`
	HoraFechamento      time.Time       `db:"hora_fechamento"`
	DataNegocio         time.Time       `db:"data_negocio"`
	CodigoInstrumento   string          `db:"codigo_instrumento"`
	PrecoNegocio        decimal.Decimal `db:"preco_negocio"`
	QuantidadeNegociada int64           `db:"quantidade_negociada"`
	CreatedAt           time.Time       `db:"created_at"`
}

type TradeFilter struct {
	Ticker    string
	StartDate *time.Time
	EndDate   *time.Time
}
