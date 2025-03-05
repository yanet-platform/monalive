// Package workerpool provides a worker pool implementation that allows tasks to
// be processed concurrently.
package workerpool

import (
	"context"
	"sync"
)

// Worker is an interface that represents a task to be executed by a worker in
// the pool.
type Worker interface {
	Run(ctx context.Context)
}

// Pool represents a pool of workers that can execute tasks concurrently.
type Pool struct {
	worker   chan Worker    // channel through which workers are sent to be processed
	workerMu sync.RWMutex   // used to protect the worker channel
	wg       sync.WaitGroup // used to wait for all workers to complete their tasks
}

// New creates a new instance of Pool.
func New() *Pool {
	return &Pool{
		worker: make(chan Worker),
	}
}

// Run starts the pool, allowing it to process incoming workers with the
// provided context.
func (m *Pool) Run(ctx context.Context) {
	// Loop over the worker channel to receive and process each worker.
	// The iteration over the worker channel will end when the channel
	// is closed, which is handled by the Close method.
	for w := range m.worker {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			w.Run(ctx)
		}()
	}
}

// Add adds a new worker to the pool for execution.
func (m *Pool) Add(w Worker) {
	// Acquire a read lock to safely check if the worker channel is open.
	m.workerMu.RLock()
	defer m.workerMu.RUnlock()

	// If the worker channel has not been closed and nilled out, send the worker
	// to the channel.
	if m.worker != nil {
		m.worker <- w
	}
}

// Close safely closes the pool, ensuring all workers have completed their
// tasks. It waits for all workers to finish their execution.
func (m *Pool) Close() {
	// Acquire a write lock to safely close the channel.
	m.workerMu.Lock()
	defer m.workerMu.Unlock()

	// Ensure the worker channel is closed only once.
	if m.worker != nil {
		// Close the worker channel to interrupt worker runner execution.
		close(m.worker)
		// Set it to nil to indicate that the channel is closed.
		m.worker = nil
	}

	// Wait for all workers to finish their execution.
	m.wg.Wait()
}
