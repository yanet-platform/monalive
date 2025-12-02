package check

import (
	"fmt"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

const ErrorLabel = "error"

var ErrorLabelUnknown = metrics.Labels{ErrorLabel: "unknown"}

type LabeledError interface {
	Label() metrics.Labels
}

type Error struct {
	error
	labelValue string
}

func (m Error) Extend(err error) Error {
	m.error = fmt.Errorf("%w: %w", m.error, err)
	return m
}

func (m Error) Label() metrics.Labels {
	return metrics.Labels{ErrorLabel: m.labelValue}
}

func NewLabledError(err Error, labelValue string) Error {
	return Error{
		error:      err,
		labelValue: labelValue,
	}
}
