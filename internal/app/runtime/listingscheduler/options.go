package listingscheduler

const defaultConfigPath = "config/config-prod.yaml"

type Options struct {
	Config    string
	LogLevel  string
	Version   string
	BuildTime string
}

func (o Options) ConfigPath() string {
	return ResolveConfigPath(o.Config)
}

func ResolveConfigPath(configPath string) string {
	if configPath != "" {
		return configPath
	}
	return defaultConfigPath
}
