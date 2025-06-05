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

type Downloader struct {
	baseURL    string
	httpClient *http.Client
	workers    int
}

func NewDownloader(baseURL string, workers int) *Downloader {
	if baseURL == "" {
		baseURL = "https://arquivos.b3.com.br/rapinegocios/tickercsv"
	}

	return &Downloader{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		workers: workers,
	}
}

func (d *Downloader) DownloadFile(ctx context.Context, date time.Time, outputDir string) (string, error) {
	filename := fmt.Sprintf("%s_NEGOCIOSAVISTA.zip", date.Format("02-01-2006"))
	url := fmt.Sprintf("%s/%s", d.baseURL, date.Format("2006-01-02"))

	outputPath := filepath.Join(outputDir, filename)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("erro ao criar diretório: %w", err)
	}

	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("⏭️  Arquivo já existe: %s\n", filename)
		return outputPath, nil
	}

	fmt.Printf("⬇️  Baixando: %s\n", filename)

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

	tempFile := outputPath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("erro ao criar arquivo: %w", err)
	}

	written, err := io.Copy(file, resp.Body)
	file.Close()

	if err != nil {
		os.Remove(tempFile)
		return "", fmt.Errorf("erro ao salvar arquivo: %w", err)
	}

	if err := os.Rename(tempFile, outputPath); err != nil {
		os.Remove(tempFile)
		return "", fmt.Errorf("erro ao renomear arquivo: %w", err)
	}

	fmt.Printf("✅ Baixado: %s (%.2f MB)\n", filename, float64(written)/(1024*1024))

	return outputPath, nil
}

func (d *Downloader) DownloadLastDays(ctx context.Context, days int, outputDir string) ([]string, error) {
	dates := getLastBusinessDays(days)

	var wg sync.WaitGroup
	results := make(chan string, len(dates))
	errors := make(chan error, len(dates))

	sem := make(chan struct{}, d.workers)

	for _, date := range dates {
		wg.Add(1)
		go func(dt time.Time) {
			defer wg.Done()

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

	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

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

func getLastBusinessDays(days int) []time.Time {
	var businessDays []time.Time
	date := time.Now()

	date = date.AddDate(0, 0, -1)

	for len(businessDays) < days {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			date = date.AddDate(0, 0, -1)
			continue
		}

		if isHoliday(date) {
			date = date.AddDate(0, 0, -1)
			continue
		}

		businessDays = append(businessDays, date)
		date = date.AddDate(0, 0, -1)
	}

	return businessDays
}

func isHoliday(date time.Time) bool {
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
