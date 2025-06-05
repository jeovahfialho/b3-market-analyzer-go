package ingestion

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParser(b *testing.B) {

	csvData := generateTestCSV(100000)

	benchmarks := []struct {
		name      string
		batchSize int
		workers   int
	}{
		{"SingleWorker", 1000, 1},
		{"FourWorkers", 1000, 4},
		{"EightWorkers", 1000, 8},
		{"LargeBatch", 10000, 4},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			parser := NewParser(bm.batchSize, bm.workers)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := bytes.NewReader([]byte(csvData))
				ctx := context.Background()

				_, err := parser.ParseFile(ctx, reader)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBulkLoader(b *testing.B) {

	pool := setupTestDB(b)
	defer pool.Close()

	trades := generateTestTrades(10000)

	benchmarks := []struct {
		name      string
		batchSize int
	}{
		{"SmallBatch", 100},
		{"MediumBatch", 1000},
		{"LargeBatch", 10000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			loader := NewBulkLoader(pool, bm.batchSize)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := loader.LoadTrades(ctx, trades)
				if err != nil {
					b.Fatal(err)
				}

				pool.Exec(ctx, "TRUNCATE trades")
			}
		})
	}
}

func generateTestCSV(lines int) string {
	var sb strings.Builder
	sb.WriteString("HoraFechamento;DataNegocio;CodigoInstrumento;PrecoNegocio;QuantidadeNegociada\n")

	tickers := []string{"PETR4", "VALE3", "ITUB4", "BBDC4"}

	for i := 0; i < lines; i++ {
		ticker := tickers[i%len(tickers)]
		price := fmt.Sprintf("%.2f", float64(20+i%30))
		quantity := fmt.Sprintf("%d", 100+i%1000)

		sb.WriteString(fmt.Sprintf(
			"15:30:00;2024-01-15;%s;%s;%s\n",
			ticker, price, quantity,
		))
	}

	return sb.String()
}
