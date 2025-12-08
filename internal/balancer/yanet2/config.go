package yanet2

type Config struct {
	BalancerModuleName string `yaml:"balancer_module_name"`
	DataplaneInstance  uint32 `yaml:"dataplane_instance"`
	Addr               string `yaml:"addr"`
}
