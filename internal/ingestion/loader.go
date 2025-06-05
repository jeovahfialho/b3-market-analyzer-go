package ingestion

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jeovahfialho/b3-analyzer/internal/domain"
)

type BulkLoader struct {
	pool      *pgxpool.Pool
	batchSize int
}

func NewBulkLoader(pool *pgxpool.Pool, batchSize int) *BulkLoader {
	return &BulkLoader{
		pool:      pool,
		batchSize: batchSize,
	}
}

func (l *BulkLoader) LoadTrades(ctx context.Context, trades []domain.Trade) (int64, error) {
	if len(trades) == 0 {
		return 0, nil
	}

	columns := []string{
		"hora_fechamento",
		"data_negocio",
		"codigo_instrumento",
		"preco_negocio",
		"quantidade_negociada",
	}

	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx)

	copyCount, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"trades"},
		columns,
		&tradeSource{trades: trades},
	)

	if err != nil {
		return 0, fmt.Errorf("erro no COPY: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("erro no commit: %w", err)
	}

	return copyCount, nil
}

type tradeSource struct {
	trades []domain.Trade
	index  int
}

func (ts *tradeSource) Next() bool {
	ts.index++
	return ts.index <= len(ts.trades)
}

func (ts *tradeSource) Values() ([]interface{}, error) {
	if ts.index > len(ts.trades) {
		return nil, nil
	}

	trade := ts.trades[ts.index-1]
	return []interface{}{
		trade.HoraFechamento,
		trade.DataNegocio,
		trade.CodigoInstrumento,
		trade.PrecoNegocio,
		trade.QuantidadeNegociada,
	}, nil
}

func (ts *tradeSource) Err() error {
	return nil
}

func (l *BulkLoader) LoadTradesConcurrent(ctx context.Context, trades []domain.Trade) (int64, error) {

	chunks := l.splitIntoChunks(trades)

	results := make(chan int64, len(chunks))
	errors := make(chan error, len(chunks))

	for _, chunk := range chunks {
		go func(chunk []domain.Trade) {
			count, err := l.LoadTrades(ctx, chunk)
			if err != nil {
				errors <- err
				return
			}
			results <- count
		}(chunk)
	}

	var totalCount int64
	for i := 0; i < len(chunks); i++ {
		select {
		case count := <-results:
			totalCount += count
		case err := <-errors:
			return totalCount, err
		case <-ctx.Done():
			return totalCount, ctx.Err()
		}
	}

	return totalCount, nil
}

func (l *BulkLoader) splitIntoChunks(trades []domain.Trade) [][]domain.Trade {
	var chunks [][]domain.Trade

	for i := 0; i < len(trades); i += l.batchSize {
		end := i + l.batchSize
		if end > len(trades) {
			end = len(trades)
		}
		chunks = append(chunks, trades[i:end])
	}

	return chunks
}
