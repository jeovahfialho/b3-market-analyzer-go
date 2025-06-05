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

func (p *Parser) worker(ctx context.Context, jobs <-chan []string, results chan<- *ParseResult, wg *sync.WaitGroup) {
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
	if len(record) < 11 {
		return nil, fmt.Errorf("registro inválido, esperado 11 campos, recebido %d", len(record))
	}

	dataNegocio, err := time.Parse("2006-01-02", strings.TrimSpace(record[8]))
	if err != nil {
		return nil, fmt.Errorf("data negócio inválida: %w", err)
	}

	horaFechamentoStr := strings.TrimSpace(record[5])

	var horaFechamento time.Time
	if horaFechamentoStr != "" {
		if len(horaFechamentoStr) == 9 {
			horaFechamento, err = time.Parse("150405000", horaFechamentoStr)
		} else if len(horaFechamentoStr) == 8 {
			horaFechamento, err = time.Parse("15040500", horaFechamentoStr)
		} else if len(horaFechamentoStr) == 6 {
			horaFechamento, err = time.Parse("150405", horaFechamentoStr)
		} else {
			err = fmt.Errorf("formato de hora não reconhecido: %s", horaFechamentoStr)
		}

		if err != nil {
			horaFechamento = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		}
	} else {
		horaFechamento = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	precoStr := strings.TrimSpace(record[3])
	precoStr = strings.ReplaceAll(precoStr, ",", ".")
	preco, err := decimal.NewFromString(precoStr)
	if err != nil {
		return nil, fmt.Errorf("preço inválido: %w", err)
	}

	quantidade, err := strconv.ParseInt(strings.TrimSpace(record[4]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("quantidade inválida: %w", err)
	}

	return &domain.Trade{
		HoraFechamento:      horaFechamento,
		DataNegocio:         dataNegocio,
		CodigoInstrumento:   strings.TrimSpace(record[1]),
		PrecoNegocio:        preco,
		QuantidadeNegociada: quantidade,
	}, nil
}
