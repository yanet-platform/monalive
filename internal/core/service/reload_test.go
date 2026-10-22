package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/types/key"
)

func TestReloadPublishesStateWhenAnnounceGroupAppears(t *testing.T) {
	for _, alive := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[alive], func(t *testing.T) {
			service := defaultService()
			service.state.Alive = alive
			if alive {
				service.state.Weight = 1
			}
			config := *service.config
			config.AnnounceGroup = "test"
			service.Reload(&config)
			require.Equal(t, map[key.Service]announcer.ServiceStatus{
				service.Key(): announcer.ServiceStatus(alive),
			}, service.announcer.FlushServiceEvents())
			require.Zero(t, service.state.Transitions)

			// Repeated reloads must not publish duplicate state updates.
			service.Reload(&config)
			require.Empty(t, service.announcer.FlushServiceEvents())
		})
	}
}

func TestReloadWithoutAnnounceGroupDoesNotPublishState(t *testing.T) {
	service := defaultService()
	service.state.Weight = 1
	service.Reload(service.config)
	require.True(t, service.State().Alive)
	require.Empty(t, service.announcer.FlushServiceEvents())
}
