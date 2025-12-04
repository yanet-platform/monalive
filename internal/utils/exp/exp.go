// Package exp provides experimental features.
package exp

import "sync"

// Config is the configuration for experimental features.
type Config struct {
	// Enabled is the flag to enable experimental features.
	Enabled bool `yaml:"enabled"`
	// ReplaceMHWith is value that will be used to replace MH. If the value is
	// set and the LVSSheduler of the virtual service is set to MH, LVSSheduler
	// value will be replaced by new value.
	ReplaceMHWith string `yaml:"replace_mh_with"`
	// EnableTLSSNI is the flag to enable TLS SNI in HTTPs and gRPC checks. If
	// set, virtualhost will be used as a value.
	EnableTLSSNI bool `yaml:"enable_tls_sni"`
}

var experimentalFeaturesOnce sync.Once // used to initialize experimental features only once

// ExperimentalFeatures applies experimental features configuration.
func ExperimentalFeatures(config Config) {
	experimentalFeaturesOnce.Do(func() {
		experimentalFeatures(config)
	})
}

func experimentalFeatures(config Config) {
	if !config.Enabled {
		return
	}

	setReplaceMHWith(config.ReplaceMHWith)

	if config.EnableTLSSNI {
		enableTLSSNI()
	}
}
