package listingcontrol

import "flag"

func ParseFlags() Options {
	return ParseFlagsFrom(flag.CommandLine)
}

func ParseFlagsFrom(fs *flag.FlagSet, args ...string) Options {
	opts := Options{}
	fs.StringVar(&opts.Config, "config", "", "config path")
	fs.StringVar(&opts.AppConfig, "app-config", "", "legacy config path")
	fs.StringVar(&opts.LogLevel, "log-level", "info", "log level")
	fs.BoolVar(&opts.Force, "force", false, "run even when listing control plane is disabled in config")
	if len(args) > 0 {
		_ = fs.Parse(args)
	} else if fs == flag.CommandLine {
		flag.Parse()
	}
	return opts
}
