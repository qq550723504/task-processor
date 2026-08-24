package listingkitidentitypreflight

import "flag"

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

func ParseFlags() Options {
	return ParseFlagsFrom(flag.CommandLine)
}

func ParseFlagsFrom(fs *flag.FlagSet, args ...string) Options {
	opts := Options{}
	fs.StringVar(&opts.Config, "config", "", "config path")
	fs.StringVar(&opts.LogLevel, "log-level", "info", "log level")
	if len(args) > 0 {
		_ = fs.Parse(args)
	} else if fs == flag.CommandLine {
		flag.Parse()
	}
	return opts
}
