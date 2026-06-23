package listingcontrol

const defaultConfigPath = "config/config-prod.yaml"

type Options struct {
	Config    string
	AppConfig string
	LogLevel  string
	Force     bool
	Version   string
	BuildTime string
}

func (o Options) ConfigPath() string {
	return ResolveConfigPath(o.Config, o.AppConfig)
}

func ResolveConfigPath(configPath, appConfigPath string) string {
	if configPath != "" {
		return configPath
	}
	if appConfigPath != "" {
		return appConfigPath
	}
	return defaultConfigPath
}
