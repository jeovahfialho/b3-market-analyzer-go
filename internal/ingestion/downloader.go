// internal/ingestion/downloader.go
package ingestion

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Downloader baixa arquivos da B3
type Downloader struct {
	baseURL    string
	httpClient *http.Client
	workers    int
}

// NewDownloader cria novo downloader
func NewDownloader(baseURL string, workers int) *Downloader {
	if baseURL == "" {
		// URL base correta para arquivos históricos da B3
		// Os arquivos ficam em: https://bvmf.bmfbovespa.com.br/InstDados/SerHist/
		baseURL = "https://bvmf.bmfbovespa.com.br/InstDados/SerHist"
	}

	return &Downloader{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Timeout maior para arquivos grandes
		},
		workers: workers,
	}
}

// DownloadFile baixa um arquivo específico
func (d *Downloader) DownloadFile(ctx context.Context, date time.Time, outputDir string) (string, error) {
	// Formato do nome do arquivo B3: COTAHIST_AAAAMMDD.ZIP
	filename := fmt.Sprintf("COTAHIST_%s.ZIP", date.Format("20060102"))
	url := fmt.Sprintf("%s/%s", d.baseURL, filename)

	outputPath := filepath.Join(outputDir, filename)

	// Cria diretório se não existir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("erro ao criar diretório: %w", err)
	}

	// Verifica se arquivo já existe
	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("⏭️  Arquivo já existe: %s\n", filename)
		return outputPath, nil
	}

	fmt.Printf("⬇️  Baixando: %s\n", filename)

	// Faz request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao fazer download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code: %d para URL: %s", resp.StatusCode, url)
	}

	// Cria arquivo temporário
	tempFile := outputPath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("erro ao criar arquivo: %w", err)
	}

	// Copia conteúdo com progresso
	written, err := io.Copy(file, resp.Body)
	file.Close()

	if err != nil {
		os.Remove(tempFile)
		return "", fmt.Errorf("erro ao salvar arquivo: %w", err)
	}

	// Renomeia arquivo temporário para final
	if err := os.Rename(tempFile, outputPath); err != nil {
		os.Remove(tempFile)
		return "", fmt.Errorf("erro ao renomear arquivo: %w", err)
	}

	fmt.Printf("✅ Baixado: %s (%.2f MB)\n", filename, float64(written)/(1024*1024))

	return outputPath, nil
}

// DownloadLastDays baixa arquivos dos últimos N dias úteis
func (d *Downloader) DownloadLastDays(ctx context.Context, days int, outputDir string) ([]string, error) {
	dates := getLastBusinessDays(days)

	var wg sync.WaitGroup
	results := make(chan string, len(dates))
	errors := make(chan error, len(dates))

	// Semáforo para limitar workers
	sem := make(chan struct{}, d.workers)

	for _, date := range dates {
		wg.Add(1)
		go func(dt time.Time) {
			defer wg.Done()

			// Adquire semáforo
			sem <- struct{}{}
			defer func() { <-sem }()

			path, err := d.DownloadFile(ctx, dt, outputDir)
			if err != nil {
				errors <- fmt.Errorf("erro ao baixar %s: %w", dt.Format("2006-01-02"), err)
				return
			}
			results <- path
		}(date)
	}

	// Aguarda conclusão
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Coleta resultados
	var paths []string
	var errs []error

	for {
		select {
		case path, ok := <-results:
			if !ok {
				results = nil
			} else {
				paths = append(paths, path)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				errs = append(errs, err)
			}
		}

		if results == nil && errors == nil {
			break
		}
	}

	if len(errs) > 0 {
		fmt.Printf("⚠️  Alguns downloads falharam:\n")
		for _, err := range errs {
			fmt.Printf("   - %v\n", err)
		}
	}

	return paths, nil
}

// getLastBusinessDays retorna últimos dias úteis
func getLastBusinessDays(days int) []time.Time {
	var businessDays []time.Time
	date := time.Now()

	// A B3 disponibiliza arquivos com delay de 2-3 dias, vamos usar margem de segurança
	date = date.AddDate(0, 0, -3)

	for len(businessDays) < days {
		// Pula finais de semana
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			date = date.AddDate(0, 0, -1)
			continue
		}

		// Pula feriados principais (você pode expandir esta lista)
		if isHoliday(date) {
			date = date.AddDate(0, 0, -1)
			continue
		}

		businessDays = append(businessDays, date)
		date = date.AddDate(0, 0, -1)
	}

	return businessDays
}

// isHoliday verifica se é feriado (implementação básica)
func isHoliday(date time.Time) bool {
	// Lista de feriados fixos brasileiros 2025
	holidays := map[string]bool{
		"01-01": true, // Ano Novo
		"04-21": true, // Tiradentes
		"05-01": true, // Dia do Trabalho
		"09-07": true, // Independência
		"10-12": true, // Nossa Senhora Aparecida
		"11-02": true, // Finados
		"11-15": true, // Proclamação da República
		"12-25": true, // Natal

		// Feriados móveis 2025 (você deve atualizar esses valores)
		"04-18": true, // Sexta-feira Santa
		"06-19": true, // Corpus Christi
	}

	dateStr := date.Format("01-02")
	return holidays[dateStr]
}
