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

	// Comando download
	var downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Baixa arquivos de dados da B3",
		Long: `Baixa os arquivos de negociação da B3 dos últimos N dias úteis.
Os arquivos são baixados em formato ZIP e automaticamente extraídos.
URL base: https://arquivos.b3.com.br/apinegocios/tickercsv`,
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
	downloadCmd.Flags().StringP("start-date", "s", "", "Data inicial (YYYY-MM-DD) - opcional, padrão: alguns dias atrás")

	// Comando list
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "Lista arquivos disponíveis para carregar",
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, _ := cmd.Flags().GetString("dir")
			return listFiles(dataDir)
		},
	}

	listCmd.Flags().StringP("dir", "d", "./data", "Diretório dos dados")

	// Comando load
	var loadCmd = &cobra.Command{
		Use:   "load [files...]",
		Short: "Carrega arquivos CSV",
		Long: `Carrega arquivos CSV no banco de dados.
Aceita múltiplos arquivos e suporta wildcards (ex: data/*.csv)`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return loadFiles(args)
		},
	}

	// Comando query
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

	// Comando refresh
	var refreshCmd = &cobra.Command{
		Use:   "refresh",
		Short: "Atualiza materialized views",
		RunE: func(cmd *cobra.Command, args []string) error {
			return refreshViews()
		},
	}

	// Comando health
	var healthCmd = &cobra.Command{
		Use:   "health",
		Short: "Verifica saúde do sistema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkHealth()
		},
	}

	// Adiciona todos os comandos
	rootCmd.AddCommand(downloadCmd, listCmd, loadCmd, queryCmd, refreshCmd, healthCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// downloadB3Files baixa arquivos reais da B3
func downloadB3Files(days int, outputDir string, extract bool, startDateStr string) error {
	fmt.Printf("🚀 Baixando cotações históricas da B3 dos últimos %d dias úteis...\n", days)
	fmt.Println("📍 URL Base: https://bvmf.bmfbovespa.com.br/InstDados/SerHist/")

	// Se uma data foi especificada, use ela como base
	var startDate time.Time
	if startDateStr != "" {
		var err error
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return fmt.Errorf("data inválida: %w", err)
		}
		fmt.Printf("📅 Iniciando a partir de: %s\n", startDate.Format("02/01/2006"))
	} else {
		// Use data padrão: algumas semanas atrás para ter certeza
		startDate = time.Now().AddDate(0, 0, -21) // 3 semanas atrás
		fmt.Printf("📅 Usando data padrão: %s (3 semanas atrás)\n", startDate.Format("02/01/2006"))
	}

	ctx := context.Background()

	// Cria downloader com data customizada
	downloader := NewCustomDownloader("", 4, startDate)

	// Baixa arquivos
	fmt.Println("\n📥 Iniciando downloads...")
	zipFiles, err := downloader.DownloadLastDays(ctx, days, outputDir)
	if err != nil {
		fmt.Printf("⚠️  Aviso: %v\n", err)
	}

	if len(zipFiles) == 0 {
		return fmt.Errorf("nenhum arquivo foi baixado - tente uma data mais antiga ou verifique a conectividade")
	}

	fmt.Printf("\n✅ %d arquivos ZIP baixados\n", len(zipFiles))

	// Extrai arquivos se solicitado
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
			fmt.Printf("✅ Extraído: %s (%d arquivos CSV)\n", filepath.Base(zipFile), len(extracted))
		}

		fmt.Printf("\n🎉 Total: %d arquivos TXT extraídos\n", len(csvFiles))

		// Lista alguns arquivos
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

// unzipFile descompacta um arquivo ZIP
func unzipFile(zipPath, destDir string) ([]string, error) {
	var extractedFiles []string

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		// Constrói caminho do arquivo
		path := filepath.Join(destDir, file.Name)

		// Verifica se é diretório
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		// Cria diretório pai se necessário
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}

		// Abre arquivo do ZIP
		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer fileReader.Close()

		// Cria arquivo de destino
		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return nil, err
		}
		defer targetFile.Close()

		// Copia conteúdo
		_, err = io.Copy(targetFile, fileReader)
		if err != nil {
			return nil, err
		}

		// Adiciona à lista se for TXT (formato da B3)
		if strings.HasSuffix(strings.ToLower(file.Name), ".txt") {
			extractedFiles = append(extractedFiles, path)
		}
	}

	return extractedFiles, nil
}

// listFiles lista arquivos disponíveis
func listFiles(dataDir string) error {
	fmt.Printf("📂 Listando arquivos em %s\n\n", dataDir)

	// Lista arquivos TXT (formato B3)
	txtFiles, err := filepath.Glob(filepath.Join(dataDir, "*.txt"))
	if err != nil {
		return err
	}

	// Lista arquivos CSV (formato alternativo)
	csvFiles, err := filepath.Glob(filepath.Join(dataDir, "*.csv"))
	if err != nil {
		return err
	}

	// Lista arquivos ZIP
	zipFiles, err := filepath.Glob(filepath.Join(dataDir, "*.zip"))
	if err != nil {
		return err
	}

	if len(txtFiles) == 0 && len(csvFiles) == 0 && len(zipFiles) == 0 {
		fmt.Println("❌ Nenhum arquivo encontrado")
		fmt.Println("💡 Use 'download' para baixar dados da B3")
		return nil
	}

	// Mostra arquivos ZIP
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

	// Mostra arquivos TXT (formato oficial da B3)
	if len(txtFiles) > 0 {
		fmt.Printf("📊 %d arquivos TXT (B3):\n", len(txtFiles))
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

	// Mostra arquivos CSV (formato alternativo)
	if len(csvFiles) > 0 {
		fmt.Printf("📊 %d arquivos CSV:\n", len(csvFiles))
		totalSize := int64(0)
		for _, file := range csvFiles {
			info, _ := os.Stat(file)
			size := info.Size()
			totalSize += size

			fmt.Printf("  - %-30s %10s\n",
				filepath.Base(file),
				formatBytes(size))
		}
		fmt.Printf("\n💾 Tamanho total CSV: %s\n", formatBytes(totalSize))
	}

	return nil
}

// formatBytes formata tamanho em bytes
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

// connectDB conecta ao PostgreSQL
func connectDB(cfg *config.Config) (*pgxpool.Pool, error) {
	db, err := postgres.NewDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}
	return db.Pool(), nil
}

// connectRedis conecta ao Redis
func connectRedis(cfg *config.Config) *cache.RedisCache {
	redisCache, err := cache.NewRedisCache(cfg)
	if err != nil {
		fmt.Printf("Aviso: Redis não disponível, continuando sem cache: %v\n", err)
		return nil
	}
	return redisCache
}

// refreshViews atualiza as materialized views
func refreshViews() error {
	ctx := context.Background()
	cfg := config.Load()

	// Conecta ao banco
	pool, err := connectDB(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	fmt.Println("🔄 Atualizando materialized views...")

	// Executa refresh
	_, err = pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY daily_aggregations")
	if err != nil {
		return fmt.Errorf("erro ao atualizar views: %w", err)
	}

	fmt.Println("✅ Views atualizadas com sucesso!")
	return nil
}

// checkHealth verifica a saúde do sistema
func checkHealth() error {
	ctx := context.Background()
	cfg := config.Load()

	fmt.Println("🏥 Verificando saúde do sistema...\n")

	// Testa PostgreSQL
	fmt.Print("PostgreSQL: ")
	pool, err := connectDB(cfg)
	if err != nil {
		fmt.Printf("❌ Erro: %v\n", err)
	} else {
		defer pool.Close()

		// Testa query simples
		var result int
		err = pool.QueryRow(ctx, "SELECT 1").Scan(&result)
		if err != nil {
			fmt.Printf("❌ Erro na query: %v\n", err)
		} else {
			fmt.Println("✅ OK")
		}
	}

	// Testa Redis
	fmt.Print("Redis: ")
	redisClient := connectRedis(cfg)
	if redisClient == nil {
		fmt.Println("❌ Não disponível")
	} else {
		defer redisClient.Close()

		// Testa ping
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

	// Conecta ao banco
	pool, err := connectDB(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Cria parser e loader
	parser := ingestion.NewParser(cfg.BatchSize, cfg.Workers)
	loader := ingestion.NewBulkLoader(pool, cfg.BatchSize)

	// Cria worker pool
	workerPool := ingestion.NewWorkerPool(cfg.Workers, parser, loader)
	workerPool.Start(ctx)
	defer workerPool.Stop()

	// Processa arquivos
	results := make(chan ingestion.JobResult, len(files))

	fmt.Printf("📥 Carregando %d arquivo(s)...\n\n", len(files))

	for _, file := range files {
		job := ingestion.Job{
			FilePath: file,
			Result:   results,
		}
		workerPool.Submit(job)
	}

	// Coleta resultados
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

	// Atualiza views automaticamente após carga
	fmt.Println("\n🔄 Atualizando agregações...")
	pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY daily_aggregations")

	return nil
}

func queryTicker(ticker string, startDateStr string) error {
	ctx := context.Background()
	cfg := config.Load()

	// Conecta ao banco
	pool, err := connectDB(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Cria serviço sem cache (passa nil)
	aggregationService := service.NewAggregationService(pool, nil, cfg.CacheTTL)

	// Parse data
	var startDate *time.Time
	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return fmt.Errorf("data inválida: %w", err)
		}
		startDate = &parsed
	}

	// Busca agregação
	fmt.Printf("🔍 Buscando dados para %s", ticker)
	if startDate != nil {
		fmt.Printf(" desde %s", startDate.Format("02/01/2006"))
	}
	fmt.Println("...")

	result, err := aggregationService.GetTickerAggregation(ctx, ticker, startDate)
	if err != nil {
		return fmt.Errorf("erro ao buscar agregação: %w", err)
	}

	// Mostra resultado
	fmt.Printf("\n📊 Resultados para %s:\n", ticker)
	fmt.Printf("├─ Maior Preço: R$ %s\n", result.MaxRangeValue.String())
	fmt.Printf("└─ Maior Volume Diário: %s\n", formatNumber(result.MaxDailyVolume))

	return nil
}

// formatNumber formata número com separadores de milhares
func formatNumber(n int64) string {
	if n == 0 {
		return "0"
	}

	// Converte para string
	str := fmt.Sprintf("%d", n)

	// Adiciona pontos como separadores
	result := ""
	for i, char := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += "."
		}
		result += string(char)
	}

	return result
}

// connectRedisSimple versão alternativa
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

// CustomDownloader com data base customizada
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
		// Pula finais de semana
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			date = date.AddDate(0, 0, -1)
			continue
		}

		// Pula feriados brasileiros
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
	// Feriados fixos brasileiros 2024-2025
	holidays := map[string]bool{
		"01-01": true, // Ano Novo
		"04-21": true, // Tiradentes
		"05-01": true, // Dia do Trabalho
		"09-07": true, // Independência
		"10-12": true, // Nossa Senhora Aparecida
		"11-02": true, // Finados
		"11-15": true, // Proclamação da República
		"12-25": true, // Natal

		// Feriados móveis 2025
		"04-18": true, // Sexta-feira Santa
		"06-19": true, // Corpus Christi

		// Feriados móveis 2024 (para dados históricos)
		"03-29": true, // Sexta-feira Santa 2024
		"05-30": true, // Corpus Christi 2024
	}

	dateStr := date.Format("01-02")
	return holidays[dateStr]
}
