package check

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/utils/xnet"
)

func TestHTTPCheckTLSHandshakeTimeout(t *testing.T) {
	config := DefaultConfig()
	config.ConnectTimeout = 0.05
	config.CheckTimeout = 5
	check := NewHTTPCheck(config, xnet.ForwardingData{}, HTTPWithTLS())
	transport := check.client.Transport.(*http.Transport)
	require.Equal(t, 50*time.Millisecond, transport.TLSHandshakeTimeout)
	require.Equal(t, 5*time.Second, check.client.Timeout)

	// Accept the ClientHello but never send a TLS handshake response.
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(io.Discard, server)
	}()
	transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
		return client, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := check.Do(ctx, &Metadata{Alive: true})
	require.ErrorContains(t, err, "TLS handshake timeout")
	client.Close()
	server.Close()
	<-done
}
