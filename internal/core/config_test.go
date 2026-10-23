package core_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/core"
	"github.com/yanet-platform/monalive/pkg/keepalived"
)

func Test_KeepalivedConfigLoader_CheckerCount(t *testing.T) {
	for _, tc := range []struct {
		name     string
		checkers []string
		invalid  bool
	}{
		{name: "NoChecker"},
		{name: "OneTCP", checkers: []string{"TCP_CHECK"}},
		{name: "OneHTTP", checkers: []string{"HTTP_GET"}},
		{name: "OneHTTPS", checkers: []string{"SSL_GET"}},
		{name: "OneGRPC", checkers: []string{"GRPC_CHECK"}},
		{name: "TwoTCP", checkers: []string{"TCP_CHECK", "TCP_CHECK"}, invalid: true},
		{name: "TwoHTTP", checkers: []string{"HTTP_GET", "HTTP_GET"}, invalid: true},
		{name: "TwoHTTPS", checkers: []string{"SSL_GET", "SSL_GET"}, invalid: true},
		{name: "TwoGRPC", checkers: []string{"GRPC_CHECK", "GRPC_CHECK"}, invalid: true},
		{name: "MixedTypes", checkers: []string{"TCP_CHECK", "HTTP_GET"}, invalid: true},
		{name: "AllTypes", checkers: []string{"TCP_CHECK", "HTTP_GET", "SSL_GET", "GRPC_CHECK"}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var blocks strings.Builder
			for _, checker := range tc.checkers {
				fmt.Fprintf(&blocks, "%s {\n}\n", checker)
			}
			text := fmt.Sprintf("virtual_server 2001:db8::1 80 {\nprotocol TCP\nreal_server 2001:db8::2 8080 {\n%s}\n}\n", blocks.String())
			path := filepath.Join(t.TempDir(), "services.conf")
			require.NoError(t, os.WriteFile(path, []byte(text), 0600))

			// Parsing accepts all checker lists; the restriction belongs to Monalive.
			parsed, err := keepalived.LoadConfig(path)
			require.NoError(t, err)
			require.Len(t, parsed.Services[0].Reals[0].Checkers(), len(tc.checkers))
			cfg, err := core.KeepalivedConfigLoader(path)
			if tc.invalid {
				require.Error(t, err)
				require.Nil(t, cfg)
				require.ErrorContains(t, err, "2001:db8::2")
				require.ErrorContains(t, err, "8080")
				require.ErrorContains(t, err, fmt.Sprintf("got %d", len(tc.checkers)))
			} else {
				require.NoError(t, err)
				require.Len(t, cfg.Services[0].Reals[0].Checkers(), len(tc.checkers))
			}
		})
	}
}
