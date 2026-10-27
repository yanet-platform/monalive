// Package workerpool manages workers whose cancellation belongs to their owner.
package workerpool

import (
	"context"
	"sync"
)

type Worker interface {
	Run(context.Context)
}

// Pool stages workers before Run and joins admitted workers on Close.
type Pool struct {
	mu      sync.Mutex
	staged  []Worker
	started bool
	closing bool
	ctx     context.Context
	wg      sync.WaitGroup
	done    chan struct{}
	close   sync.Once
}

func New() *Pool {
	return &Pool{done: make(chan struct{})}
}

// Run starts staged workers once and waits for Close to join all workers.
// The owner must cancel workers before calling Close.
func (m *Pool) Run(ctx context.Context) {
	m.mu.Lock()
	if !m.started && !m.closing {
		m.started = true
		m.ctx = ctx
		for _, worker := range m.staged {
			m.startLocked(worker)
		}

		m.staged = nil
	}
	m.mu.Unlock()

	<-m.done
}

// Add stages a worker before Run, starts it after Run, or ignores it after Close.
func (m *Pool) Add(w Worker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closing {
		return
	}

	if !m.started {
		m.staged = append(m.staged, w)
		return
	}

	m.startLocked(w)
}

func (m *Pool) startLocked(w Worker) {
	// Admission closure uses the same mutex. Wait starts only after closure.
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		w.Run(m.ctx)
	}()
}

// Close closes admission and joins workers without holding their admission lock.
// It does not cancel workers; that is the owner's responsibility.
func (m *Pool) Close() {
	m.close.Do(func() {
		m.mu.Lock()
		m.closing = true
		m.staged = nil
		m.mu.Unlock()

		m.wg.Wait()
		close(m.done)
	})

	<-m.done
}
