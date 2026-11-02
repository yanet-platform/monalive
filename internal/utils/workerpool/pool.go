// Package workerpool starts workers with Run's context and joins them on Close.
// Worker cancellation belongs to the caller, not the pool.
package workerpool

import (
	"context"
	"sync"

	"github.com/yanet-platform/monalive/internal/utils/shutdown"
)

// Worker runs until its work finishes or its owner's context is canceled.
type Worker interface {
	Run(context.Context)
}

type poolState uint8

const (
	poolIdle poolState = iota
	poolRunning
	poolStopped
)

// Pool manages concurrent workers with blocking admission and waits for their
// completion on Close. The owner must start Run before synchronously adding
// workers, and cancel workers before waiting for Close.
type Pool struct {
	workers  chan Worker        // unbuffered worker handoff to Run
	shutdown *shutdown.Shutdown // signals admission and receive-loop shutdown, not completion
	done     chan struct{}      // signals completion after worker join or Close before Run
	mu       sync.Mutex         // protects lifecycle state and completion ownership
	state    poolState          // lifecycle state; stopped does not imply worker completion
}

// New creates a pool with blocking admission and no running workers.
func New() *Pool {
	return &Pool{
		workers:  make(chan Worker),
		shutdown: shutdown.New(),
		done:     make(chan struct{}),
	}
}

// Run starts accepted workers with ctx and blocks until Close is called and all
// accepted workers finish. Only one call starts workers; concurrent and later
// calls wait for the same completion without replacing its context. Calls after
// Close do not start workers.
func (m *Pool) Run(ctx context.Context) {
	m.mu.Lock()
	if m.state != poolIdle {
		m.mu.Unlock()
		<-m.done
		return
	}

	m.state = poolRunning
	m.mu.Unlock()

	// This Run owns all registrations and the terminal join. Wait cannot
	// overlap a later Add to wg: both happen in this goroutine, in order.
	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		close(m.done)
	}()

	for {
		// Do not keep selecting pending sends once closure has been observed.
		select {
		case <-m.shutdown.Done():
			return
		default:
		}

		select {
		case <-m.shutdown.Done():
			return
		case worker := <-m.workers:
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker.Run(ctx)
			}()
		}
	}
}

// Add blocks until w is accepted for execution or Close stops admission,
// including when Run has not started. Each accepted Add starts w exactly once;
// Close waits for it to finish. An Add concurrent with Close may be accepted or
// ignored; calls begun after admission closes are ignored.
func (m *Pool) Add(w Worker) {
	select {
	case <-m.shutdown.Done():
		return
	default:
	}

	// The worker channel is never closed, so a send racing closure cannot panic.
	select {
	case <-m.shutdown.Done():
	case m.workers <- w:
	}
}

// Close stops admission, releases blocked Add calls, and waits for all accepted
// workers to finish. If Run has not started, Close returns without requiring it
// to start. Repeated or concurrent calls wait for the same completion. Close
// never cancels workers; their owner must cancel them before waiting for Close.
func (m *Pool) Close() {
	m.mu.Lock()
	m.shutdown.Do()
	if m.state == poolIdle {
		// No Run claimed completion; this first Close owns it instead.
		close(m.done)
	}

	m.state = poolStopped
	m.mu.Unlock()

	<-m.done
}
