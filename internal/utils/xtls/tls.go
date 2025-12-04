// Package xtls provides utilities for configuring TLS settings.
//
// It includes functionality to set the minimum TLS version and retrieve
// the TLS configuration.
package xtls

import (
	"crypto/tls"
	"fmt"
)

var tlsMinVersion uint16 // holds the minimum TLS version required for connections

// tlsConfig is the default TLS configuration used for connections.
var tlsConfig = tls.Config{
	InsecureSkipVerify: true, // Enforce verification of server certificates
	MinVersion:         tlsMinVersion,
	// Override default CurvePreferences to disable X25519Kyber768Draft00, which
	// is enabled by default in Go 1.23 for post-quantum key exchange (see
	// https://tip.golang.org/doc/go1.23#cryptotlspkgcryptotls).
	//
	// This is necessary because our CheckTun process reads packets via nfqueue
	// and is limited to 1500 bytes per packet by default. The Kyber-based
	// handshake generates larger TLS records that exceed this limit and are
	// dropped, causing the TLS handshake to fail. By specifying classic curves
	// only, we ensure that handshake messages remain within acceptable size
	// limits.
	CurvePreferences: []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
		tls.CurveP384,
		tls.CurveP521,
	},
}

func init() {
	// Set the default minimum TLS version to 1.2.
	tlsMinVersion = tls.VersionTLS12
}

// SetTLSMinVersion updates the minimum TLS version used in the TLS
// configuration.
//
// Supported versions are "1.0", "1.1", "1.2", and "1.3". If an empty string or
// an unknown version, the default version (1.2) will be used and an error will
// be returned.
func SetTLSMinVersion(version string) error {
	switch version {
	case "1.0":
		tlsMinVersion = tls.VersionTLS10
	case "1.1":
		tlsMinVersion = tls.VersionTLS11
	case "1.2":
		tlsMinVersion = tls.VersionTLS12
	case "1.3":
		tlsMinVersion = tls.VersionTLS13
	case "":
		return fmt.Errorf("TLS version is empty, use default 1.2")
	default:
		return fmt.Errorf("unknown TLS version %q, use default 1.2", version)
	}
	return nil
}

// TLSConfig returns the current TLS configuration.
//
// The returned configuration will use the minimum TLS version specified by
// SetTLSMinVersion and will skip verification of server certificates.
func TLSConfig() *tls.Config {
	// Create a copy of the base configuration.
	//
	// NOTE: do not return a pointer to the [tlsConfig] here, because
	// [tls.Config] contains a mutex inside. Using the same copy of the
	// configuration will cause multiple goroutines using the same mutex.
	return tlsConfig.Clone()
}

// TLSConfigWithServerName returns a TLS configuration with the specified
// ServerName.
//
// This function creates a copy of the base TLS configuration and sets the
// ServerName field for SNI (Server Name Indication).
func TLSConfigWithServerName(serverName string) *tls.Config {
	config := TLSConfig()
	config.ServerName = serverName

	return config
}
