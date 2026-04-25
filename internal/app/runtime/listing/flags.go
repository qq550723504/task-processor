package listing

import "flag"

func ParseFlags(platform string) Options {
	opts := Options{Platform: platform}
	flag.StringVar(&opts.Config, "config", "", "config path")
	flag.StringVar(&opts.AppConfig, "app-config", "", "legacy config path")
	flag.StringVar(&opts.LogLevel, "log-level", "info", "log level")
	flag.Parse()
	return opts
}
