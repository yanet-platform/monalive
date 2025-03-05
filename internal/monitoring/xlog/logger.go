package xlog

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Level slog.Level `yaml:"level"`
}

func New(config *Config) (*slog.Logger, error) {
	logger := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{
				AddSource:   true,
				Level:       config.Level,
				ReplaceAttr: replaceAttr,
			},
		),
	)

	hostname, err := os.Hostname()
	if err == nil {
		logger = logger.With(slog.String("host", hostname))
	} else {
		logger.Error("Could not detect hostname", slog.Any("error", err))
	}

	return logger, nil
}

func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey {
		source := a.Value.Any().(*slog.Source)

		a.Key = "caller"
		idx := strings.LastIndexByte(source.File, '/')
		if idx == -1 {
			a.Value = slog.StringValue(source.File + ":" + strconv.Itoa(source.Line))
			return a
		}
		// Find the penultimate separator.
		idx = strings.LastIndexByte(source.File[:idx], '/')
		if idx == -1 {
			a.Value = slog.StringValue(source.File + ":" + strconv.Itoa(source.Line))
			return a
		}

		// Keep everything after the penultimate separator.
		a.Value = slog.StringValue(source.File[idx+1:] + ":" + strconv.Itoa(source.Line))
		return a
	}
	return a
}
