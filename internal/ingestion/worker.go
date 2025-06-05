package ingestion

import (
	"context"
	"fmt"
	"os"
	"sync"
)

type WorkerPool struct {
	workers  int
	parser   *Parser
	loader   *BulkLoader
	jobQueue chan Job
	wg       sync.WaitGroup
}

type Job struct {
	FilePath string
	Result   chan<- JobResult
}

type JobResult struct {
	FilePath     string
	RecordsCount int64
	Error        error
}

func NewWorkerPool(workers int, parser *Parser, loader *BulkLoader) *WorkerPool {
	return &WorkerPool{
		workers:  workers,
		parser:   parser,
		loader:   loader,
		jobQueue: make(chan Job, workers*2),
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}
}

func (wp *WorkerPool) Stop() {
	close(wp.jobQueue)
	wp.wg.Wait()
}

func (wp *WorkerPool) Submit(job Job) {
	wp.jobQueue <- job
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return

		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}

			result := wp.processFile(ctx, job.FilePath)
			job.Result <- result
		}
	}
}

func (wp *WorkerPool) processFile(ctx context.Context, filePath string) JobResult {

	file, err := os.Open(filePath)
	if err != nil {
		return JobResult{
			FilePath: filePath,
			Error:    fmt.Errorf("erro ao abrir arquivo: %w", err),
		}
	}
	defer file.Close()

	parseResult, err := wp.parser.ParseFile(ctx, file)
	if err != nil {
		return JobResult{
			FilePath: filePath,
			Error:    fmt.Errorf("erro no parse: %w", err),
		}
	}

	count, err := wp.loader.LoadTradesConcurrent(ctx, parseResult.Trades)
	if err != nil {
		return JobResult{
			FilePath:     filePath,
			RecordsCount: count,
			Error:        fmt.Errorf("erro ao carregar: %w", err),
		}
	}

	return JobResult{
		FilePath:     filePath,
		RecordsCount: count,
		Error:        nil,
	}
}
