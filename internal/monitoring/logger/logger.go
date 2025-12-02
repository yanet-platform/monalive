package logger

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a new logger instance with the given configuration.
func New(ctx context.Context, config *Config) (*zap.Logger, error) {
	// Construct zap configuration.
	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(config.Level),
		Encoding:          config.Encoding,
		DisableStacktrace: true,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "name",
			CallerKey:      "caller",
			MessageKey:     "msg",
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := zapConfig.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}

	// Add hostname to the logger.
	hostname, err := os.Hostname()
	if err == nil {
		logger = logger.With(zap.String("host", hostname))
	} else {
		logger.Error("Could not detect hostname", zap.Error(err))
	}

	// If OTEL exporter is configured, add exporter to the logger.
	if config.OTEL != nil {
		otelCore, err := setupOTELExporter(ctx, config.OTEL)
		if err != nil {
			return nil, fmt.Errorf("failed to setup OTEL exporter: %w", err)
		}

		logger = logger.WithOptions(
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return zapcore.NewTee(core, otelCore)
			}),
		)
	}

	return logger, nil
}
