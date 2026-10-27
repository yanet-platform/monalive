package core_test

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/core"
	"github.com/yanet-platform/monalive/internal/core/service"
	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
	"github.com/yanet-platform/monalive/pkg/keepalived"
	"go.uber.org/zap"
)

func Test_CoreReload_RejectsMultipleCheckersBeforeApplyingConfig(t *testing.T) {
	cfg := &core.Config{Services: []*service.Config{{Reals: []*keepalived.Real{{
		RSKey:       keepalived.RSKey{IP: netip.MustParseAddr("192.0.2.2"), Port: 8080},
		TCPCheckers: []*keepalived.Checker{{}, {}},
	}}}}}
	// An invalid in-memory configuration must be rejected before accessing any
	// runtime component, including the announcer or worker pools.
	m := &core.Core{}
	require.ErrorContains(t, m.Reload(cfg), "got 2")
}

func lifecycleCore() *core.Core {
	logger := zap.NewNop()
	scoped := metrics.NewScopedMetrics(logger)
	a := announcer.New(&announcer.Config{}, nil, scoped.Scope(metrics.Global), logger)
	b := balancer.New(&balancer.Config{}, nil, a, logger)

	return core.New(a, b, scoped, logger)
}

func waitCore(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("core did not complete")
	}
}

func lifecycleConfig() *core.Config {
	return &core.Config{Services: []*service.Config{{
		VSKey:  keepalived.VSKey{VIP: netip.MustParseAddr("192.0.2.1"), Protocol: "TCP"},
		Quorum: 1,
		Reals:  []*keepalived.Real{{RSKey: keepalived.RSKey{IP: netip.MustParseAddr("192.0.2.2")}}},
	}}}
}

func Test_Core_StagedReloadThenStopBeforeRun(t *testing.T) {
	c := lifecycleCore()
	if err := c.Reload(lifecycleConfig()); err != nil {
		t.Fatal(err)
	}

	c.Stop()
	c.Run(context.Background())
	if err := c.Reload(&core.Config{}); err != nil {
		t.Fatal(err)
	}

	if len(c.Status()) != 1 || len(c.Status()[0].Reals) != 0 {
		t.Fatal("Stop-before-Run allowed late installation or reload")
	}
}

func Test_Core_RunReloadAndStopJoin(t *testing.T) {
	for i := 0; i < 50; i++ {
		c := lifecycleCore()
		if err := c.Reload(lifecycleConfig()); err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); <-start; c.Run(context.Background()) }()
		go func() { defer wg.Done(); <-start; _ = c.Reload(&core.Config{}) }()
		go func() { defer wg.Done(); <-start; c.Stop() }()
		close(start)
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		waitCore(t, done)

		before := c.Status()
		if err := c.Reload(lifecycleConfig()); err != nil {
			t.Fatal(err)
		}

		if len(c.Status()) != len(before) {
			t.Fatal("reload after producer join installed a service")
		}
	}
}
