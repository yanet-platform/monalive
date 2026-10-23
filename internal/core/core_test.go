package core_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/core"
	"github.com/yanet-platform/monalive/internal/core/service"
	"github.com/yanet-platform/monalive/pkg/keepalived"
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
