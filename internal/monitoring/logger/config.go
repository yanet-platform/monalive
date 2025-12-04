package logger

import (
	"go.uber.org/zap/zapcore"
)

// Config represents the logger configuration.
type Config struct {
	// Encoding is the log encoding.
	// Possible values: json, console.
	Encoding string `yaml:"encoding"`
	// Level is the log level.
	Level zapcore.Level `yaml:"level"`
	// OTEL is the OTEL exporter configuration.
	OTEL *OTELConfig `yaml:"otel_exporter"`
}
