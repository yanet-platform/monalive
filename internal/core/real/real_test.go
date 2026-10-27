package real_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yanet-platform/monalive/internal/core/real"
	"github.com/yanet-platform/monalive/internal/types/xevent"
	"github.com/yanet-platform/monalive/pkg/keepalived"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func wait(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("real did not complete")
	}
}

func Test_Real_StopBeforeRunDoesNotActivate(t *testing.T) {
	var calls atomic.Int32
	r := real.New(&real.Config{}, func(*xevent.Event) { calls.Add(1) }, zap.NewNop(), real.WithActivationFunc(func(context.Context) bool {
		calls.Add(1)
		return true
	}))
	r.Stop()
	r.Run(context.Background())
	r.Reload(&real.Config{Weight: 2})
	r.HandleEvent(&xevent.Event{Type: xevent.Enable})

	if calls.Load() != 0 {
		t.Fatal("stopped real activated or emitted a callback")
	}
}

func Test_Real_CanceledActivationIsJoined(t *testing.T) {
	for _, stop := range []bool{false, true} {
		name := "ParentCancellation"
		if stop {
			name = "Stop"
		}

		t.Run(name, func(t *testing.T) {
			entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			r := real.New(&real.Config{}, func(*xevent.Event) { t.Error("inactive real emitted a callback") }, zap.NewNop(), real.WithActivationFunc(func(ctx context.Context) bool {
				close(entered)
				<-ctx.Done()
				close(canceled)
				<-release
				return false
			}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runDone := make(chan struct{})
			go func() { r.Run(ctx); close(runDone) }()
			wait(t, entered)

			stopDone := make(chan struct{})
			if stop {
				go func() { r.Stop(); close(stopDone) }()
			} else {
				cancel()
			}
			wait(t, canceled)

			select {
			case <-runDone:
				t.Fatal("Run left activation unjoined")
			case <-stopDone:
				t.Fatal("Stop left activation unjoined")
			default:
			}

			close(release)
			wait(t, runDone)
			r.Stop()
			if stop {
				wait(t, stopDone)
			}
		})
	}
}

func Test_Real_StopJoinsCallbackAndRejectsLaterEvents(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	r := real.New(&real.Config{Weight: 1}, func(*xevent.Event) {
		calls.Add(1)
		close(entered)
		<-release
	}, zap.NewNop())
	processed := make(chan struct{})
	go func() { r.HandleEvent(&xevent.Event{Type: xevent.Enable}); close(processed) }()
	wait(t, entered)
	stopped := make(chan struct{})
	go func() { r.Stop(); close(stopped) }()

	select {
	case <-stopped:
		t.Fatal("Stop did not join callback")
	default:
	}

	close(release)
	wait(t, stopped)
	wait(t, processed)
	r.HandleEvent(&xevent.Event{Type: xevent.Shutdown})
	if calls.Load() != 1 {
		t.Fatal("callback after join")
	}
}

func Test_Real_ReloadRaceStopWhileActivating(t *testing.T) {
	entered := make(chan struct{})
	r := real.New(&real.Config{}, func(*xevent.Event) {}, zap.NewNop(), real.WithActivationFunc(func(ctx context.Context) bool {
		close(entered)
		<-ctx.Done()
		return false
	}))
	runDone := make(chan struct{})
	go func() { r.Run(context.Background()); close(runDone) }()
	wait(t, entered)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; r.Reload(&real.Config{Weight: 7}) }()
	go func() { defer wg.Done(); <-start; r.Stop() }()
	close(start)
	completed := make(chan struct{})
	go func() { wg.Wait(); close(completed) }()
	wait(t, completed)
	wait(t, runDone)
}

func Test_Real_StopPreservesAndJoinsCheckerShutdownEvent(t *testing.T) {
	checkerEntered, checkerRelease := make(chan struct{}), make(chan struct{})
	callbackEntered, callbackRelease := make(chan struct{}), make(chan struct{})
	logger := zap.NewExample(zap.Hooks(func(entry zapcore.Entry) error {
		if entry.Message == "running checker" {
			close(checkerEntered)
			<-checkerRelease
		}

		return nil
	}))
	cfg := &real.Config{Weight: 1, InhibitOnFailure: true, TCPCheckers: []*keepalived.Checker{keepalived.DefaultCheckerConfig()}}
	cfg.TCPCheckers[0].Type = keepalived.TCPChecker
	var shutdowns atomic.Int32
	r := real.New(cfg, func(event *xevent.Event) {
		if event.Type != xevent.Shutdown {
			t.Error("expected checker final shutdown, not a probe result")
		}

		shutdowns.Add(1)
		close(callbackEntered)
		<-callbackRelease
	}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan struct{})
	go func() { r.Run(ctx); close(runDone) }()
	wait(t, checkerEntered)

	// The logger barrier precedes scheduler.Run. Canceling now prevents any
	// actual network probe and exercises the real final-event path.
	cancel()
	close(checkerRelease)
	wait(t, callbackEntered)
	stopDone := make(chan struct{})
	go func() { r.Stop(); close(stopDone) }()
	select {
	case <-runDone:
		t.Fatal("Real.Run returned before its child's final event")
	case <-stopDone:
		t.Fatal("Real.Stop returned before its child's final event")
	default:
	}

	close(callbackRelease)
	wait(t, stopDone)
	wait(t, runDone)
	if shutdowns.Load() != 1 || r.State().Inhibited {
		t.Fatal("final shutdown lost, duplicated, or left the real inhibited")
	}
}
