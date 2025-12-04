package logger

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap/zapcore"
)

// OTELConfig represents the OTEL exporter configuration.
type OTELConfig struct {
	// Endpoint through which the OTEL exporter will send logs.
	Endpoint string `yaml:"grpc_addr"`
}

func setupOTELExporter(ctx context.Context, config *OTELConfig) (zapcore.Core, error) {
	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(config.Endpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel grpc exporter: %w", err)
	}

	provider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(exporter)),
		log.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("monalive"),
			),
		),
	)

	otelCore := otelzap.NewCore("", otelzap.WithLoggerProvider(provider))

	return otelCore, nil
}
