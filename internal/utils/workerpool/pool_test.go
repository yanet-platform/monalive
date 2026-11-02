package workerpool_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yanet-platform/monalive/internal/utils/workerpool"
)

type workerFunc func(context.Context)

func (m workerFunc) Run(ctx context.Context) { m(ctx) }

// await bounds waits for observable completion; elapsed time is not evidence of ordering.
func await(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle operation did not complete")
	}
}

// Test_Pool_RunContext verifies that accepted workers receive the context of the
// original Run even when another Run is invoked.
func Test_Pool_RunContext(t *testing.T) {
	pool := workerpool.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone, laterRunDone := make(chan struct{}), make(chan struct{})
	go func() { pool.Run(ctx); close(runDone) }()
	t.Cleanup(func() {
		pool.Close()
		await(t, runDone)
		await(t, laterRunDone)
	})

	// The first accepted Add establishes which Run owns the session. The
	// buffered observations do not hold workers while the test examines them.
	contexts := make(chan context.Context, 2)
	worker := workerFunc(func(ctx context.Context) { contexts <- ctx })
	pool.Add(worker)
	go func() { pool.Run(context.Background()); close(laterRunDone) }()
	pool.Add(worker)

	for range 2 {
		select {
		case actual := <-contexts:
			if actual != ctx {
				t.Error("worker received a different Run context")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not report its Run context")
		}
	}
}

// Test_Pool_CloseBeforeRunReleasesAdd verifies that Close allows a pending Add
// to return without requiring Run to start.
func Test_Pool_CloseBeforeRunReleasesAdd(t *testing.T) {
	pool := workerpool.New()
	adding, added := make(chan struct{}), make(chan struct{})
	go func() {
		close(adding)
		pool.Add(workerFunc(func(context.Context) {}))
		close(added)
	}()
	t.Cleanup(func() {
		pool.Close()
		await(t, added)
	})
	await(t, adding)

	// The marker does not prove Add is already parked. There is no Run in
	// this scenario, so either an entered or a later-entering Add must return.
	pool.Close()
	await(t, added)
}

// Test_Pool_LateAddIsIgnored verifies that work submitted after Close finishes
// is not executed.
func Test_Pool_LateAddIsIgnored(t *testing.T) {
	pool := workerpool.New()
	runDone := make(chan struct{})
	go func() { pool.Run(context.Background()); close(runDone) }()
	t.Cleanup(func() {
		pool.Close()
		await(t, runDone)
	})
	pool.Add(workerFunc(func(context.Context) {}))
	pool.Close()

	var called atomic.Bool
	added := make(chan struct{})
	go func() {
		pool.Add(workerFunc(func(context.Context) { called.Store(true) }))
		close(added)
	}()
	await(t, added)
	if called.Load() {
		t.Fatal("closed pool executed late work")
	}
}

// Test_Pool_AddBeforeRunIsReceivedByRun verifies that work submitted before
// starting Run can be accepted and executed once Run starts.
func Test_Pool_AddBeforeRunIsReceivedByRun(t *testing.T) {
	pool := workerpool.New()
	adding, added, worked := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		close(adding)
		pool.Add(workerFunc(func(context.Context) { close(worked) }))
		close(added)
	}()
	// This establishes sender initiation, not that Add has already blocked.
	await(t, adding)

	runDone := make(chan struct{})
	go func() { pool.Run(context.Background()); close(runDone) }()
	t.Cleanup(func() {
		pool.Close()
		await(t, added)
		await(t, runDone)
	})
	await(t, worked)
}

// Test_Pool_CloseAccountsForReceivedWorker verifies that Close returns only
// after accepted work has finished.
func Test_Pool_CloseAccountsForReceivedWorker(t *testing.T) {
	// Close immediately after a successful Add, retaining the accounting race
	// regression without forcing an internal interleaving. Read the marker
	// before any further join could hide an early Close return.
	for range 1000 {
		pool := workerpool.New()
		runDone := make(chan struct{})
		var finished atomic.Bool
		go func() { pool.Run(context.Background()); close(runDone) }()
		pool.Add(workerFunc(func(context.Context) { finished.Store(true) }))
		pool.Close()
		finishedAtClose := finished.Load()
		await(t, runDone)
		if !finishedAtClose {
			t.Error("Close returned before accepted work finished")
		}
	}
}

// Test_Pool_ConcurrentRunStartsWorkerOnce verifies that concurrent Run calls
// execute each accepted submission exactly once.
func Test_Pool_ConcurrentRunStartsWorkerOnce(t *testing.T) {
	pool := workerpool.New()
	var calls atomic.Int32
	var runners sync.WaitGroup
	runners.Add(2)
	for range 2 {
		go func() {
			defer runners.Done()
			pool.Run(context.Background())
		}()
	}
	runDone := make(chan struct{})
	go func() { runners.Wait(); close(runDone) }()
	t.Cleanup(func() {
		pool.Close()
		await(t, runDone)
	})
	pool.Add(workerFunc(func(context.Context) { calls.Add(1) }))
	pool.Close()
	await(t, runDone)
	if calls.Load() != 1 {
		t.Fatalf("accepted work executed %d times, want 1", calls.Load())
	}
}

// Test_Pool_RepeatedCallsWaitForWorkers verifies that repeated Run and Close
// calls report completion only after previously accepted work finishes.
func Test_Pool_RepeatedCallsWaitForWorkers(t *testing.T) {
	// All return observations are captured by the callers themselves. Releasing
	// work after launching them offers schedules, not proof that they entered.
	for range 100 {
		pool := workerpool.New()
		release, runDone := make(chan struct{}), make(chan struct{})
		var finished atomic.Bool
		go func() { pool.Run(context.Background()); close(runDone) }()
		pool.Add(workerFunc(func(context.Context) {
			<-release
			finished.Store(true)
		}))
		results := make(chan bool, 4)
		for range 2 {
			go func() {
				pool.Run(context.Background())
				results <- finished.Load()
			}()
			go func() {
				pool.Close()
				results <- finished.Load()
			}()
		}
		close(release)
		for range 4 {
			select {
			case completed := <-results:
				if !completed {
					t.Error("repeated call returned before accepted work finished")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("repeated lifecycle call did not return")
			}
		}
		await(t, runDone)
	}
}

// Test_Pool_AddAndRunRaceClose verifies that concurrent admission and lifecycle
// calls can all return after shutdown.
func Test_Pool_AddAndRunRaceClose(t *testing.T) {
	// The barrier supplies competing schedules, not an asserted execution order.
	for range 100 {
		pool := workerpool.New()
		start := make(chan struct{})
		var operations sync.WaitGroup
		operations.Add(5)

		go func() {
			defer operations.Done()
			<-start
			pool.Add(workerFunc(func(context.Context) {}))
		}()
		go func() { defer operations.Done(); <-start; pool.Run(context.Background()) }()
		go func() { defer operations.Done(); <-start; pool.Run(context.Background()) }()
		go func() { defer operations.Done(); <-start; pool.Close() }()
		go func() { defer operations.Done(); <-start; pool.Close() }()

		close(start)
		done := make(chan struct{})
		go func() { operations.Wait(); close(done) }()
		await(t, done)
	}
}
