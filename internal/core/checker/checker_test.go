package checker_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yanet-platform/monalive/internal/core/checker"
	"github.com/yanet-platform/monalive/internal/core/checker/check"
	"github.com/yanet-platform/monalive/internal/types/xevent"
	"github.com/yanet-platform/monalive/internal/utils/xnet"
	"github.com/yanet-platform/monalive/pkg/keepalived"
	"go.uber.org/zap"
)

func wait(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("checker did not complete")
	}
}

func newChecker(handler xevent.Handler) *checker.Checker {
	cfg := checker.Config(*keepalived.DefaultCheckerConfig())
	cfg.Type = keepalived.TCPChecker

	return checker.New(&cfg, handler, 1, xnet.ForwardingData{}, zap.NewNop())
}

func Test_Checker_StopBeforeRunClosesCallbackAdmission(t *testing.T) {
	var callbacks atomic.Int32
	c := newChecker(func(*xevent.Event) { callbacks.Add(1) })
	c.Stop()
	c.Run(context.Background())
	c.ProcessCheck(check.Metadata{Alive: true}, nil)
	c.Stop()

	if callbacks.Load() != 0 {
		t.Fatal("stopped checker emitted a callback")
	}
}

func Test_Checker_StopJoinsFinalCallback(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var callbacks atomic.Int32
	c := newChecker(func(event *xevent.Event) {
		callbacks.Add(1)
		if event.Type == xevent.Shutdown {
			close(entered)
			<-release
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runDone := make(chan struct{})
	go func() { c.Run(ctx); close(runDone) }()
	wait(t, entered)

	stopDone := make(chan struct{})
	go func() { c.Stop(); close(stopDone) }()
	select {
	case <-stopDone:
		t.Fatal("Stop returned before the final callback")
	case <-runDone:
		t.Fatal("Run returned before the final callback")
	default:
	}

	close(release)
	wait(t, stopDone)
	wait(t, runDone)
	c.ProcessCheck(check.Metadata{Alive: true}, nil)
	if callbacks.Load() != 1 {
		t.Fatal("final callback lost or callback emitted after join")
	}
}

func Test_Checker_StopJoinsDirectProcessCheck(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	c := newChecker(func(*xevent.Event) { close(entered); <-release })
	processed := make(chan struct{})
	go func() { c.ProcessCheck(check.Metadata{Alive: true}, nil); close(processed) }()
	wait(t, entered)
	stopped := make(chan struct{})
	go func() { c.Stop(); close(stopped) }()

	select {
	case <-stopped:
		t.Fatal("Stop did not join an admitted ProcessCheck")
	default:
	}

	close(release)
	wait(t, stopped)
	wait(t, processed)
}
