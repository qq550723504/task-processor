package listingkitownerexceptions

import "flag"

type Options struct {
	Config        string
	Report        string
	ConfirmReport string
}

func (options Options) ConfigPath() string {
	if options.Config == "" {
		return "config/config-dev.yaml"
	}
	return options.Config
}

func ParseFlags() Options { return ParseFlagsFrom(flag.CommandLine) }

func ParseFlagsFrom(fs *flag.FlagSet, args ...string) Options {
	options := Options{}
	fs.StringVar(&options.Config, "config", "config/config-dev.yaml", "path to config file")
	fs.StringVar(&options.Report, "report", "", "approved owner reconciliation report path")
	fs.StringVar(&options.ConfirmReport, "confirm-report", "", "exact approved report fingerprint")
	if len(args) > 0 {
		_ = fs.Parse(args)
	} else if fs == flag.CommandLine {
		flag.Parse()
	}
	return options
}
