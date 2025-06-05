package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log   *zap.Logger
	Sugar *zap.SugaredLogger
)

func Init(level string, development bool) error {
	var config zap.Config

	if development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
	}

	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	var err error
	Log, err = config.Build()
	if err != nil {
		return err
	}

	Sugar = Log.Sugar()

	return nil
}

func WithContext(ctx context.Context) *zap.Logger {
	if requestID, ok := ctx.Value("requestID").(string); ok {
		return Log.With(zap.String("request_id", requestID))
	}
	return Log
}

func Close() {
	if Log != nil {
		_ = Log.Sync()
	}
}

func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
	os.Exit(1)
}

func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}
