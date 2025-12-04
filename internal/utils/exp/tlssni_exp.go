package exp

var tlsSNIEnabled bool

func init() {
	tlsSNIEnabled = false
}

func enableTLSSNI() {
	tlsSNIEnabled = true
}

// TLSSNIEnabled returns true if TLS SNI is enabled.
func TLSSNIEnabled() bool {
	return tlsSNIEnabled
}
