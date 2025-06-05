package ingestion

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeovahfialho/b3-analyzer/internal/domain"
	"github.com/shopspring/decimal"
)

type Parser struct {
	batchSize int
	workers   int
}

func NewParser(batchSize, workers int) *Parser {
	return &Parser{
		batchSize: batchSize,
		workers:   workers,
	}
}

type ParseResult struct {
	Trades []domain.Trade
	Errors []error
}

func (p *Parser) ParseFile(ctx context.Context, reader io.Reader) (*ParseResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = ';'
	csvReader.LazyQuotes = true

	jobs := make(chan []string, p.workers*2)
	results := make(chan *ParseResult, p.workers)

	var wg sync.WaitGroup

	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go p.worker(ctx, jobs, results, &wg)
	}

	go func() {
		defer close(jobs)

		if _, err := csvReader.Read(); err != nil {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				record, err := csvReader.Read()
				if err == io.EOF {
					return
				}
				if err != nil {
					continue
				}
				jobs <- record
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	finalResult := &ParseResult{
		Trades: make([]domain.Trade, 0, p.batchSize),
		Errors: make([]error, 0),
	}

	for result := range results {
		finalResult.Trades = append(finalResult.Trades, result.Trades...)
		finalResult.Errors = append(finalResult.Errors, result.Errors...)
	}

	return finalResult, nil
}

func (p *Parser) worker(ctx context.Context, jobs <-chan []string,
	results chan<- *ParseResult, wg *sync.WaitGroup) {

	defer wg.Done()

	batch := &ParseResult{
		Trades: make([]domain.Trade, 0, p.batchSize),
	}

	for {
		select {
		case <-ctx.Done():
			if len(batch.Trades) > 0 {
				results <- batch
			}
			return

		case record, ok := <-jobs:
			if !ok {
				if len(batch.Trades) > 0 {
					results <- batch
				}
				return
			}

			trade, err := p.parseRecord(record)
			if err != nil {
				batch.Errors = append(batch.Errors, err)
				continue
			}

			batch.Trades = append(batch.Trades, *trade)

			if len(batch.Trades) >= p.batchSize {
				results <- batch
				batch = &ParseResult{
					Trades: make([]domain.Trade, 0, p.batchSize),
				}
			}
		}
	}
}

func (p *Parser) parseRecord(record []string) (*domain.Trade, error) {
	if len(record) < 5 {
		return nil, fmt.Errorf("registro inválido: %v", record)
	}

	dataNegocio, err := time.Parse("2006-01-02", record[1])
	if err != nil {
		return nil, fmt.Errorf("data inválida: %w", err)
	}

	horaFechamento, err := time.Parse("15:04:05", record[0])
	if err != nil {
		return nil, fmt.Errorf("hora inválida: %w", err)
	}

	precoStr := strings.Replace(record[3], ",", ".", -1)
	preco, err := decimal.NewFromString(precoStr)
	if err != nil {
		return nil, fmt.Errorf("preço inválido: %w", err)
	}

	quantidade, err := strconv.ParseInt(record[4], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("quantidade inválida: %w", err)
	}

	return &domain.Trade{
		HoraFechamento:      horaFechamento,
		DataNegocio:         dataNegocio,
		CodigoInstrumento:   strings.TrimSpace(record[2]),
		PrecoNegocio:        preco,
		QuantidadeNegociada: quantidade,
	}, nil
}
