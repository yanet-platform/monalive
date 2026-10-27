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

func (f workerFunc) Run(ctx context.Context) { f(ctx) }

func await(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle operation did not complete")
	}
}

func Test_Pool_AddBeforeRunAndBlockingWorkerJoin(t *testing.T) {
	pool := workerpool.New()
	entered, release, workerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	pool.Add(workerFunc(func(context.Context) {
		close(entered)
		<-release
		close(workerDone)
	}))

	select {
	case <-entered:
		t.Fatal("staged worker started before Run")
	default:
	}

	runDone := make(chan struct{})
	go func() { pool.Run(context.Background()); close(runDone) }()
	await(t, entered)

	secondEntered := make(chan struct{})
	pool.Add(workerFunc(func(context.Context) {
		close(secondEntered)
		<-release
	}))
	await(t, secondEntered)

	closeDone := make(chan struct{})
	go func() { pool.Close(); close(closeDone) }()

	select {
	case <-closeDone:
		t.Fatal("Close returned before the held worker")
	default:
	}

	close(release)
	await(t, closeDone)
	await(t, workerDone)
	await(t, runDone)

	var late atomic.Bool
	pool.Add(workerFunc(func(context.Context) { late.Store(true) }))
	pool.Close()
	pool.Run(context.Background())
	if late.Load() {
		t.Fatal("late Add executed after Close")
	}
}

func Test_Pool_CloseBeforeRunDiscardsStagedAndLateWorkers(t *testing.T) {
	pool := workerpool.New()
	var calls atomic.Int32
	worker := workerFunc(func(context.Context) { calls.Add(1) })
	pool.Add(worker)
	pool.Close()
	pool.Add(worker)
	pool.Run(context.Background())
	pool.Close()

	if calls.Load() != 0 {
		t.Fatal("closed pool started a worker")
	}
}

func Test_Pool_AddAndRunRaceClose(t *testing.T) {
	for i := 0; i < 100; i++ {
		pool := workerpool.New()
		start := make(chan struct{})
		var workers, calls atomic.Int32
		var operations sync.WaitGroup
		operations.Add(3)

		go func() {
			defer operations.Done()
			<-start
			pool.Add(workerFunc(func(context.Context) {
				workers.Add(1)
				// A worker must be able to reenter admission while Close waits.
				pool.Add(workerFunc(func(context.Context) { calls.Add(1) }))
				workers.Add(-1)
			}))
		}()
		go func() { defer operations.Done(); <-start; pool.Run(context.Background()) }()
		go func() { defer operations.Done(); <-start; pool.Close() }()

		close(start)
		done := make(chan struct{})
		go func() { operations.Wait(); close(done) }()
		await(t, done)

		if workers.Load() != 0 {
			t.Fatal("Close did not join admitted workers")
		}

		before := calls.Load()
		pool.Add(workerFunc(func(context.Context) { calls.Add(1) }))
		pool.Close()
		if calls.Load() != before {
			t.Fatal("late worker admitted")
		}
	}
}
