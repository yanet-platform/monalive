package keepalived

func LoadConfig(path string, v any) error {
	cfgRoot, err := parseFile(path)
	if err != nil {
		return err
	}

	return configDecode(v, cfgRoot)
}
