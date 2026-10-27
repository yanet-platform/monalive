package service_test

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/core/service"
	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
	"github.com/yanet-platform/monalive/internal/types/xevent"
	"github.com/yanet-platform/monalive/pkg/keepalived"
	"go.uber.org/zap"
)

// heldGauge holds an existing metric call, placing a barrier inside installation
// or event handling without exposing production test hooks.
type heldGauge struct {
	metrics.NopGauge
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *heldGauge) Add(float64) {
	g.once.Do(func() { close(g.entered); <-g.release })
}

func wait(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("service did not complete")
	}
}

func newService(cfg *service.Config) *service.Service {
	b := balancer.New(&balancer.Config{}, nil, nil, zap.NewNop())

	return service.New(cfg, nil, b, zap.NewNop())
}

func Test_Service_StopDuringInitialInstallation(t *testing.T) {
	cfg := &service.Config{Quorum: 1, Reals: []*keepalived.Real{{RSKey: keepalived.RSKey{IP: netip.MustParseAddr("192.0.2.2")}}}}
	s := newService(cfg)
	gauge := &heldGauge{entered: make(chan struct{}), release: make(chan struct{})}
	s.SetMetrics(service.SetRealsMetric(gauge))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan struct{})
	go func() { s.Run(ctx); close(runDone) }()
	wait(t, gauge.entered)

	cancel()
	stopDone, reloadDone := make(chan struct{}), make(chan struct{})
	go func() { s.Stop(); close(stopDone) }()
	go func() { s.Reload(&service.Config{Quorum: 1}); close(reloadDone) }()
	select {
	case <-runDone:
		t.Fatal("Run returned while initial installation was still active")
	case <-stopDone:
		t.Fatal("Stop returned while initial installation was still active")
	default:
	}

	close(gauge.release)
	wait(t, runDone)
	wait(t, stopDone)
	wait(t, reloadDone)

	before := s.Status()
	s.Reload(cfg)
	s.Run(context.Background())
	if len(s.Status().Reals) != len(before.Reals) {
		t.Fatal("stopped service accepted a late reload")
	}
}

func Test_Service_StopJoinsHandlerAndRejectsLateEvents(t *testing.T) {
	s := newService(&service.Config{Quorum: 1})
	gauge := &heldGauge{entered: make(chan struct{}), release: make(chan struct{})}
	s.SetMetrics(service.SetServiceEnabledMetric(gauge))
	processed := make(chan struct{})
	go func() {
		s.HandleEvent(&xevent.Event{Type: xevent.Enable, New: xevent.Status{Weight: 1}})
		close(processed)
	}()
	wait(t, gauge.entered)
	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()

	select {
	case <-stopped:
		t.Fatal("Stop returned while a handler was active")
	default:
	}

	close(gauge.release)
	wait(t, processed)
	wait(t, stopped)
	before := s.State()
	s.HandleEvent(&xevent.Event{Type: xevent.Shutdown, Init: xevent.Status{Weight: 1}})
	if s.State() != before {
		t.Fatal("stopped service accepted a late callback")
	}
}

func Test_Service_StopBeforeRunDoesNotInstallChildren(t *testing.T) {
	cfg := &service.Config{Reals: []*keepalived.Real{{}}}
	s := newService(cfg)
	s.Stop()
	s.Run(context.Background())
	s.Reload(cfg)

	if len(s.Status().Reals) != 0 {
		t.Fatal("stopped service installed children")
	}
}
