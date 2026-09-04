package storehistorymigrate

import "flag"

func ParseFlags() Options {
	return ParseFlagsFrom(flag.CommandLine)
}

func ParseFlagsFrom(flagSet *flag.FlagSet, arguments ...string) Options {
	options := Options{}
	flagSet.StringVar(&options.Config, "config", "", "config path")
	flagSet.StringVar(&options.LogLevel, "log-level", "info", "log level")
	flagSet.StringVar(&options.Action, "action", defaultAction, "migration action: verify (read-only), backfill (one bounded batch), or constraints (explicit Phase E)")
	flagSet.StringVar(&options.Manifest, "manifest", "", "approved no-authoritative-history-source JSON manifest path")
	flagSet.IntVar(&options.BatchSize, "batch-size", defaultBatchSize, "backfill batch size (1-1000)")
	flagSet.DurationVar(&options.ConstraintLockTimeout, "constraint-lock-timeout", defaultConstraintLockTimeout, "PostgreSQL Phase E lock timeout")
	flagSet.DurationVar(&options.ConstraintStatementTimeout, "constraint-statement-timeout", defaultConstraintStatementTimeout, "PostgreSQL Phase E statement timeout")
	if len(arguments) > 0 {
		_ = flagSet.Parse(arguments)
	} else if flagSet == flag.CommandLine {
		flag.Parse()
	}
	return options
}
