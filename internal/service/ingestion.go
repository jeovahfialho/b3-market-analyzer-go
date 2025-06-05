package service

import (
	"context"

	"github.com/jeovahfialho/b3-analyzer/internal/ingestion"
	"github.com/jeovahfialho/b3-analyzer/pkg/logger"
	"go.uber.org/zap"
)

type IngestionService struct {
	parser *ingestion.Parser
	loader *ingestion.BulkLoader
}

func NewIngestionService(parser *ingestion.Parser, loader *ingestion.BulkLoader) *IngestionService {
	return &IngestionService{
		parser: parser,
		loader: loader,
	}
}

type ProcessFileResult struct {
	FilePath     string
	RecordsCount int64
	Errors       []error
}

func (s *IngestionService) ProcessFile(ctx context.Context, filePath string) (*ProcessFileResult, error) {
	logger.Info("processando arquivo", zap.String("file", filePath))

	return &ProcessFileResult{
		FilePath:     filePath,
		RecordsCount: 1000,
		Errors:       nil,
	}, nil
}
