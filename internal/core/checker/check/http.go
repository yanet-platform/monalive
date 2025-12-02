package check

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/netip"

	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/utils/exp"
	"github.com/yanet-platform/monalive/internal/utils/xnet"
	"github.com/yanet-platform/monalive/internal/utils/xtls"
)

const UserAgentRequestHeader = "monalive"

// HTTPCheck performs HTTP or HTTPS checks based on the provided configuration.
type HTTPCheck struct {
	config    Config      // configuration for the HTTP check
	uri       string      // URI for the HTTP request
	tlsConfig *tls.Config // TLS configuration for secure connections
	client    http.Client // HTTP client used to make requests
}

// HTTPCheckOption is a function that configures an HTTPCheck.
type HTTPCheckOption func(*HTTPCheck)

// HTTPWithTLS returns an HTTPCheckOption that enables TLS for the HTTP check.
func HTTPWithTLS() HTTPCheckOption {
	return func(check *HTTPCheck) {
		if check.config.Virtualhost != nil && exp.TLSSNIEnabled() {
			// Use the virtualhost as ServerName for SNI.
			check.tlsConfig = xtls.TLSConfigWithServerName(*check.config.Virtualhost)
			return
		}
		check.tlsConfig = xtls.TLSConfig()
	}
}

// NewHTTPCheck creates a new instance of HTTPCheck.
func NewHTTPCheck(config Config, forwardingData xnet.ForwardingData, opts ...HTTPCheckOption) *HTTPCheck {
	check := &HTTPCheck{
		config: config,
	}

	// Apply optional configurations.
	for _, opt := range opts {
		opt(check)
	}

	check.uri = check.URI()
	dialer := xnet.NewDialer(config.BindIP, config.GetConnectTimeout(), forwardingData)
	check.client = http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     check.tlsConfig,    // set TLS configuration if available
			MaxIdleConnsPerHost: -1,                 // disable connection pooling
			DisableKeepAlives:   true,               // disable keep-alives
			DialContext:         dialer.DialContext, // use custom dialer
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // do not follow redirects
		},
		Timeout: config.Net.GetCheckTimeout(), // set request timeout
	}

	return check
}

// Do performs the HTTP check by sending a request to the configured URI. It
// updates the Metadata based on the response or marks it inactive if an error
// has occurred.
func (m *HTTPCheck) Do(ctx context.Context, md *Metadata) (err error) {
	defer func() {
		if err != nil {
			// Mark the metadata inactive if an error has occurred.
			md.SetInactive()
		}
	}()

	request, err := m.newRequest(ctx, md)
	if err != nil {
		return fmt.Errorf("failed to create new request: %w", err)
	}

	response, err := m.client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to process request: %w", err)
	}
	defer response.Body.Close()

	// Handle the response and update metadata.
	return m.handle(md, response)
}

// URI returns the URI for the HTTP request based on the configuration. It
// formats the IP address and port from the configuration into a string suitable
// for use with the HTTP client.
func (m *HTTPCheck) URI() string {
	if m.uri != "" {
		// Return the precomputed URI if available.
		return m.uri
	}

	schema := "http"
	if m.tlsConfig != nil {
		// Use HTTPS if TLS configuration is set.
		schema = "https"
	}
	host := netip.AddrPortFrom(m.config.ConnectIP, m.config.ConnectPort.Value()).String()

	path := m.config.Path

	return fmt.Sprintf("%s://%s%s", schema, host, path)
}

// newRequest creates a new HTTP request with the provided context and metadata.
// It sets the appropriate headers and host based on the configuration and
// metadata.
func (m *HTTPCheck) newRequest(ctx context.Context, md *Metadata) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.uri, nil)
	if err != nil {
		return nil, err
	}

	if m.config.Virtualhost != nil {
		// Set virtual host if configured.
		request.Host = *m.config.Virtualhost
	}

	// Set Monalive User-Agent header.
	request.Header.Set("User-Agent", UserAgentRequestHeader)
	if m.config.DynamicWeight {
		// Add weight header if dynamic weight is enabled.
		request.Header.Add("X-RS-Weight", md.Weight.String())

		// Set alive status header.
		aliveStatus := "0"
		if md.Alive {
			aliveStatus = "1"
		}
		request.Header.Add("X-RS-Alive", aliveStatus)
	}

	// Ensure the connection is closed after the request.
	request.Close = true

	return request, nil
}

// handle processes the HTTP response. It validates the status code, checks the
// response body against the configured digest, and updates the Metadata with
// the response details.
func (m *HTTPCheck) handle(md *Metadata, response *http.Response) error {
	if !m.matchStatusCode(response.StatusCode) {
		// Return error if status code mismatch.
		return fmt.Errorf("status code does not match: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		// Return error if reading body fails.
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if !m.matchDigest(body) {
		// Return error if digest mismatch.
		return fmt.Errorf("digest does not match")
	}

	// Update metadata to indicate the connection is alive.
	md.Alive = true
	// Update metadata with the weight from response.
	md.Weight = m.getWeightFrom(response, body)

	return nil
}

// matchStatusCode checks if the response status code matches the expected
// status code configured.
func (m *HTTPCheck) matchStatusCode(statusCode int) bool {
	// Skip status_code check if it doesn't defined.
	return m.config.StatusCode == 0 || m.config.StatusCode == statusCode
}

// matchDigest checks if the MD5 digest of the response body matches the
// expected digest configured.
func (m *HTTPCheck) matchDigest(body []byte) bool {
	digest := m.config.Digest
	if digest == "" {
		// No digest configured, so any body is acceptable.
		return true
	}

	md5Sum := md5.Sum(body)
	hash := hex.EncodeToString(md5Sum[:])

	return digest == hash
}

// getWeightFrom extracts the weight from the response based on the configured
// dynamic weight settings.
func (m *HTTPCheck) getWeightFrom(response *http.Response, body []byte) weight.Weight {
	if !m.config.DynamicWeight {
		// Return omitted if dynamic weight is not enabled.
		return weight.Omitted
	}

	var weightBuf []byte
	if m.config.DynamicWeightHeader {
		weightBuf = []byte(response.Header.Get("RS-Weight"))
		if len(weightBuf) == 0 {
			// Return omitted if weight header is missing.
			return weight.Omitted
		}
	} else {
		// Get only the first line of the body.
		firstBodyLine, _, _ := bytes.Cut(body, []byte("\n"))

		// Cut the weight prefix from the line if so.
		var found bool
		if weightBuf, found = bytes.CutPrefix(firstBodyLine, []byte("rs_weight=")); !found {
			// Return omitted if weight prefix is not found in body.
			return weight.Omitted
		}
		// Additionally trim spaces.
		weightBuf = bytes.TrimSpace(weightBuf)
	}

	var weight weight.Weight
	// Attempt to unmarshal weight from buffer.
	_ = weight.UnmarshalText(weightBuf)

	return weight
}
