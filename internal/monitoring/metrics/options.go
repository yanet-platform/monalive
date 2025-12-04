package metrics

type MetricOption func(opts *MetricOpts)

type MetricOpts struct {
	ConstLabels Labels
	Description string
}

func WithDescription(description string) MetricOption {
	return func(opts *MetricOpts) {
		opts.Description = description
	}
}

func WithConstLabels(labels Labels) MetricOption {
	return func(opts *MetricOpts) {
		opts.ConstLabels = labels
	}
}
