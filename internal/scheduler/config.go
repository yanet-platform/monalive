package scheduler

import "time"

const (
	defaultDelayLoop  = 60 * time.Second // default delay between tasks
	defaultRetries    = 1                // default number of retry attempts
	defaultRetryDelay = 3 * time.Second  // default delay before retrying
)

// Config holds the configuration for scheduling tasks.
type Config struct {
	// Delay between task execution in seconds.
	DelayLoop *float64 `keepalive:"delay_loop" json:"delay_loop"`
	// Number of retry attempts.
	Retries *int `keepalive:"retry,nb_get_retry" json:"retries"`
	// Delay before retrying in seconds.
	RetryDelay *float64 `keepalive:"delay_before_retry" json:"retry_delay"`
}

// Default sets the configuration to default values.
func (m *Config) Default() {
	delayLoop := defaultDelayLoop.Seconds()
	retries := defaultRetries
	retryDelay := defaultRetryDelay.Seconds()

	m.DelayLoop = &delayLoop
	m.Retries = &retries
	m.RetryDelay = &retryDelay
}

// GetDelayLoop returns the delay loop duration.
// If the delay loop is not set, it returns the default delay loop value.
func (m Config) GetDelayLoop() time.Duration {
	if m.DelayLoop == nil {
		return defaultDelayLoop
	}
	return time.Duration(*m.DelayLoop) * time.Second
}

// GetRetries returns the number of retry attempts.
// If retries are not set, it returns the default number of retries.
func (m Config) GetRetries() int {
	if m.Retries == nil {
		return defaultRetries
	}
	return *m.Retries
}

// GetRetryDelay returns the retry delay duration.
// If retry delay is not set, it returns the default retry delay value.
func (m Config) GetRetryDelay() time.Duration {
	if m.RetryDelay == nil {
		return defaultRetryDelay
	}
	return time.Duration(*m.RetryDelay) * time.Second
}
