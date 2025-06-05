package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/jeovahfialho/b3-analyzer/internal/config"
	"github.com/jeovahfialho/b3-analyzer/internal/ingestion"
	"github.com/jeovahfialho/b3-analyzer/internal/service"
	"github.com/jeovahfialho/b3-analyzer/internal/storage/cache"
	"github.com/jeovahfialho/b3-analyzer/internal/storage/postgres"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "b3-analyzer",
		Short: "B3 Market Data Analyzer CLI",
		Long: `CLI para análise de dados do mercado B3.
Permite baixar, carregar e consultar dados de negociação.`,
	}

	var downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Baixa arquivos de dados da B3",
		Long: `Baixa os arquivos de negociação da B3 dos últimos N dias úteis.
Os arquivos são baixados em formato ZIP e automaticamente extraídos.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			days, _ := cmd.Flags().GetInt("days")
			outputDir, _ := cmd.Flags().GetString("output")
			extract, _ := cmd.Flags().GetBool("extract")
			startDate, _ := cmd.Flags().GetString("start-date")
			return downloadB3Files(days, outputDir, extract, startDate)
		},
	}

	downloadCmd.Flags().IntP("days", "d", 7, "Número de dias úteis para baixar")
	downloadCmd.Flags().StringP("output", "o", "./data", "Diretório de saída")
	downloadCmd.Flags().BoolP("extract", "e", true, "Extrair arquivos ZIP automaticamente")
	downloadCmd.Flags().StringP("start-date", "s", "", "Data inicial (YYYY-MM-DD)")

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "Lista arquivos disponíveis para carregar",
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, _ := cmd.Flags().GetString("dir")
			return listFiles(dataDir)
		},
	}

	listCmd.Flags().StringP("dir", "d", "./data", "Diretório dos dados")

	var loadCmd = &cobra.Command{
		Use:   "load [files...]",
		Short: "Carrega arquivos CSV",
		Long: `Carrega arquivos CSV no banco de dados.
Aceita múltiplos arquivos e suporta wildcards.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return loadFiles(args)
		},
	}

	var queryCmd = &cobra.Command{
		Use:   "query [ticker]",
		Short: "Consulta agregações de um ticker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			startDate, _ := cmd.Flags().GetString("start-date")
			return queryTicker(args[0], startDate)
		},
	}

	queryCmd.Flags().StringP("start-date", "s", "", "Data inicial (YYYY-MM-DD)")

	var refreshCmd = &cobra.Command{
		Use:   "refresh",
		Short: "Atualiza materialized views",
		RunE: func(cmd *cobra.Command, args []string) error {
			return refreshViews()
		},
	}

	var healthCmd = &cobra.Command{
		Use:   "health",
		Short: "Verifica saúde do sistema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkHealth()
		},
	}

	rootCmd.AddCommand(downloadCmd, listCmd, loadCmd, queryCmd, refreshCmd, healthCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func downloadB3Files(days int, outputDir string, extract bool, startDateStr string) error {
	fmt.Printf("🚀 Baixando negócios à vista da B3 dos últimos %d dias úteis...\n", days)
	fmt.Println("📍 URL Base: https://arquivos.b3.com.br/rapinegocios/tickercsv")

	var startDate time.Time
	if startDateStr != "" {
		var err error
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return fmt.Errorf("data inválida: %w", err)
		}
		fmt.Printf("📅 Iniciando a partir de: %s\n", startDate.Format("02/01/2006"))
	} else {
		startDate = time.Now().AddDate(0, 0, -7)
		fmt.Printf("📅 Usando data padrão: %s (1 semana atrás)\n", startDate.Format("02/01/2006"))
	}

	ctx := context.Background()

	downloader := NewCustomDownloader("", 4, startDate)

	fmt.Println("\n📥 Iniciando downloads...")
	zipFiles, err := downloader.DownloadLastDays(ctx, days, outputDir)
	if err != nil {
		fmt.Printf("⚠️  Aviso: %v\n", err)
	}

	if len(zipFiles) == 0 {
		return fmt.Errorf("nenhum arquivo foi baixado")
	}

	fmt.Printf("\n✅ %d arquivos ZIP baixados\n", len(zipFiles))

	if extract {
		fmt.Println("\n📦 Extraindo arquivos...")
		var csvFiles []string

		for _, zipFile := range zipFiles {
			extracted, err := unzipFile(zipFile, outputDir)
			if err != nil {
				fmt.Printf("❌ Erro ao extrair %s: %v\n", filepath.Base(zipFile), err)
				continue
			}

			csvFiles = append(csvFiles, extracted...)
			fmt.Printf("✅ Extraído: %s (%d arquivos TXT)\n", filepath.Base(zipFile), len(extracted))
		}

		fmt.Printf("\n🎉 Total: %d arquivos TXT extraídos\n", len(csvFiles))

		if len(csvFiles) > 0 {
			fmt.Println("\n📋 Arquivos disponíveis para carregar:")
			for i, file := range csvFiles {
				if i >= 5 {
					fmt.Printf("   ... e mais %d arquivos\n", len(csvFiles)-5)
					break
				}
				fmt.Printf("   - %s\n", filepath.Base(file))
			}
		}
	}

	fmt.Println("\n✅ Download concluído!")
	fmt.Println("\n💡 Próximo passo: use 'load data/*.txt' para carregar os dados no banco")

	return nil
}

func unzipFile(zipPath, destDir string) ([]string, error) {
	var extractedFiles []string

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}

		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return nil, err
		}
		defer targetFile.Close()

		_, err = io.Copy(targetFile, fileReader)
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(strings.ToLower(file.Name), ".txt") {
			extractedFiles = append(extractedFiles, path)
		}
	}

	return extractedFiles, nil
}

func listFiles(dataDir string) error {
	fmt.Printf("📂 Listando arquivos em %s\n\n", dataDir)

	txtFiles, err := filepath.Glob(filepath.Join(dataDir, "*.txt"))
	if err != nil {
		return err
	}

	zipFiles, err := filepath.Glob(filepath.Join(dataDir, "*.zip"))
	if err != nil {
		return err
	}

	if len(txtFiles) == 0 && len(zipFiles) == 0 {
		fmt.Println("❌ Nenhum arquivo encontrado")
		fmt.Println("💡 Use 'download' para baixar dados da B3")
		return nil
	}

	if len(zipFiles) > 0 {
		fmt.Printf("📦 %d arquivos ZIP:\n", len(zipFiles))
		for _, file := range zipFiles {
			info, _ := os.Stat(file)
			fmt.Printf("  - %-30s %10s\n",
				filepath.Base(file),
				formatBytes(info.Size()))
		}
		fmt.Println()
	}

	if len(txtFiles) > 0 {
		fmt.Printf("📊 %d arquivos TXT:\n", len(txtFiles))
		totalSize := int64(0)
		for _, file := range txtFiles {
			info, _ := os.Stat(file)
			size := info.Size()
			totalSize += size

			fmt.Printf("  - %-30s %10s\n",
				filepath.Base(file),
				formatBytes(size))
		}
		fmt.Printf("\n💾 Tamanho total TXT: %s\n", formatBytes(totalSize))
	}

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func connectDB(cfg *config.Config) (*pgxpool.Pool, error) {
	db, err := postgres.NewDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}
	return db.Pool(), nil
}

func connectRedis(cfg *config.Config) *cache.RedisCache {
	redisCache, err := cache.NewRedisCache(cfg)
	if err != nil {
		fmt.Printf("Aviso: Redis não disponível, continuando sem cache: %v\n", err)
		return nil
	}
	return redisCache
}

func refreshViews() error {
	ctx := context.Background()
	cfg := config.Load()

	pool, err := connectDB(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	fmt.Println("🔄 Atualizando materialized views...")

	_, err = pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY daily_aggregations")
	if err != nil {
		return fmt.Errorf("erro ao atualizar views: %w", err)
	}

	fmt.Println("✅ Views atualizadas com sucesso!")
	return nil
}

func checkHealth() error {
	ctx := context.Background()
	cfg := config.Load()

	fmt.Println("🏥 Verificando saúde do sistema...\n")

	fmt.Print("PostgreSQL: ")
	pool, err := connectDB(cfg)
	if err != nil {
		fmt.Printf("❌ Erro: %v\n", err)
	} else {
		defer pool.Close()

		var result int
		err = pool.QueryRow(ctx, "SELECT 1").Scan(&result)
		if err != nil {
			fmt.Printf("❌ Erro na query: %v\n", err)
		} else {
			fmt.Println("✅ OK")
		}
	}

	fmt.Print("Redis: ")
	redisClient := connectRedis(cfg)
	if redisClient == nil {
		fmt.Println("❌ Não disponível")
	} else {
		defer redisClient.Close()

		err = redisClient.HealthCheck(ctx)
		if err != nil {
			fmt.Printf("❌ Erro: %v\n", err)
		} else {
			fmt.Println("✅ OK")
		}
	}

	fmt.Println("\n✅ Verificação concluída!")
	return nil
}

func loadFiles(files []string) error {
	ctx := context.Background()
	cfg := config.Load()

	pool, err := connectDB(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	parser := ingestion.NewParser(cfg.BatchSize, cfg.Workers)
	loader := ingestion.NewBulkLoader(pool, cfg.BatchSize)

	workerPool := ingestion.NewWorkerPool(cfg.Workers, parser, loader)
	workerPool.Start(ctx)
	defer workerPool.Stop()

	results := make(chan ingestion.JobResult, len(files))

	fmt.Printf("📥 Carregando %d arquivo(s)...\n\n", len(files))

	for _, file := range files {
		job := ingestion.Job{
			FilePath: file,
			Result:   results,
		}
		workerPool.Submit(job)
	}

	var totalRecords int64
	for i := 0; i < len(files); i++ {
		result := <-results
		if result.Error != nil {
			fmt.Printf("❌ Erro em %s: %v\n", result.FilePath, result.Error)
		} else {
			fmt.Printf("✅ Carregados %d registros de %s\n", result.RecordsCount, result.FilePath)
			totalRecords += result.RecordsCount
		}
	}

	fmt.Printf("\n📊 Total: %d registros carregados\n", totalRecords)

	fmt.Println("\n🔄 Atualizando agregações...")
	pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY daily_aggregations")

	return nil
}

func queryTicker(ticker string, startDateStr string) error {
	ctx := context.Background()
	cfg := config.Load()

	pool, err := connectDB(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	aggregationService := service.NewAggregationService(pool, nil, cfg.CacheTTL)

	var startDate *time.Time
	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return fmt.Errorf("data inválida: %w", err)
		}
		startDate = &parsed
	}

	fmt.Printf("🔍 Buscando dados para %s", ticker)
	if startDate != nil {
		fmt.Printf(" desde %s", startDate.Format("02/01/2006"))
	}
	fmt.Println("...")

	result, err := aggregationService.GetTickerAggregation(ctx, ticker, startDate)
	if err != nil {
		return fmt.Errorf("erro ao buscar agregação: %w", err)
	}

	fmt.Printf("\n📊 Resultados para %s:\n", ticker)
	fmt.Printf("├─ Maior Preço: R$ %s\n", result.MaxRangeValue.String())
	fmt.Printf("└─ Maior Volume Diário: %s\n", formatNumber(result.MaxDailyVolume))

	return nil
}

func formatNumber(n int64) string {
	if n == 0 {
		return "0"
	}

	str := fmt.Sprintf("%d", n)

	result := ""
	for i, char := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += "."
		}
		result += string(char)
	}

	return result
}

func connectRedisSimple(cfg *config.Config) *redis.Client {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil
	}

	return client
}

type CustomDownloader struct {
	*ingestion.Downloader
	baseDate time.Time
}

func NewCustomDownloader(baseURL string, workers int, baseDate time.Time) *CustomDownloader {
	return &CustomDownloader{
		Downloader: ingestion.NewDownloader(baseURL, workers),
		baseDate:   baseDate,
	}
}

func (cd *CustomDownloader) DownloadLastDays(ctx context.Context, days int, outputDir string) ([]string, error) {
	dates := getLastBusinessDaysFrom(days, cd.baseDate)

	var results []string
	var errors []error

	for _, date := range dates {
		path, err := cd.DownloadFile(ctx, date, outputDir)
		if err != nil {
			errors = append(errors, fmt.Errorf("erro ao baixar %s: %w", date.Format("2006-01-02"), err))
			continue
		}
		results = append(results, path)
	}

	if len(errors) > 0 {
		fmt.Printf("⚠️  Alguns downloads falharam:\n")
		for _, err := range errors {
			fmt.Printf("   - %v\n", err)
		}
	}

	return results, nil
}

func getLastBusinessDaysFrom(days int, startDate time.Time) []time.Time {
	var businessDays []time.Time
	date := startDate

	for len(businessDays) < days {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			date = date.AddDate(0, 0, -1)
			continue
		}

		if isHolidayBR(date) {
			date = date.AddDate(0, 0, -1)
			continue
		}

		businessDays = append(businessDays, date)
		date = date.AddDate(0, 0, -1)
	}

	return businessDays
}

func isHolidayBR(date time.Time) bool {
	holidays := map[string]bool{
		"01-01": true,
		"04-21": true,
		"05-01": true,
		"09-07": true,
		"10-12": true,
		"11-02": true,
		"11-15": true,
		"12-25": true,
		"04-18": true,
		"06-19": true,
		"03-29": true,
		"05-30": true,
	}

	dateStr := date.Format("01-02")
	return holidays[dateStr]
}
