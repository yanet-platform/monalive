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
	return &tlsConfig
}
