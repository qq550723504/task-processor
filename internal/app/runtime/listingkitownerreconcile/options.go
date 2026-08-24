package listingkitownerreconcile

import "flag"

type Options struct {
	Config                string
	Output                string
	SQLOutput             string
	SchemaOutput          string
	BackfillOutput        string
	SafeBackfillOutput    string
	ManualReviewOutput    string
	UnresolvedTasksJSON   string
	UnresolvedTasksCSV    string
	UnresolvedStudioJSON  string
	UnresolvedStudioCSV   string
	UnresolvedSummaryJSON string
	Execute               bool
	ConfirmReport         string
	BatchSize             int
}

func (options Options) ConfigPath() string {
	if options.Config == "" {
		return "config/config-dev.yaml"
	}
	return options.Config
}

func ParseFlags() Options {
	return ParseFlagsFrom(flag.CommandLine)
}

func ParseFlagsFrom(fs *flag.FlagSet, args ...string) Options {
	options := Options{}
	fs.StringVar(&options.Config, "config", "config/config-dev.yaml", "path to config file")
	fs.StringVar(&options.Output, "output", ".local/tmp/listingkit-owner-scope-dry-run.json", "redacted JSON report path")
	fs.StringVar(&options.SQLOutput, "sql-output", ".local/tmp/listingkit-owner-scope-dry-run.sql", "read-only SQL preview path")
	fs.StringVar(&options.SchemaOutput, "schema-output", ".local/tmp/listingkit-owner-scope-schema.sql", "schema preview path")
	fs.StringVar(&options.BackfillOutput, "backfill-output", ".local/tmp/listingkit-owner-scope-backfill.sql", "backfill preview path")
	fs.StringVar(&options.SafeBackfillOutput, "safe-backfill-output", ".local/tmp/listingkit-owner-scope-safe-backfill.sql", "safe backfill preview path")
	fs.StringVar(&options.ManualReviewOutput, "manual-review-output", ".local/tmp/listingkit-owner-scope-manual-review.json", "manual review report path")
	fs.StringVar(&options.UnresolvedTasksJSON, "unresolved-tasks-json", ".local/tmp/listingkit-owner-scope-unresolved-tasks.json", "unresolved task report path")
	fs.StringVar(&options.UnresolvedTasksCSV, "unresolved-tasks-csv", ".local/tmp/listingkit-owner-scope-unresolved-tasks.csv", "unresolved task CSV path")
	fs.StringVar(&options.UnresolvedStudioJSON, "unresolved-studio-json", ".local/tmp/listingkit-owner-scope-unresolved-studio-sessions.json", "unresolved studio report path")
	fs.StringVar(&options.UnresolvedStudioCSV, "unresolved-studio-csv", ".local/tmp/listingkit-owner-scope-unresolved-studio-sessions.csv", "unresolved studio CSV path")
	fs.StringVar(&options.UnresolvedSummaryJSON, "unresolved-summary-json", ".local/tmp/listingkit-owner-scope-unresolved-summary.json", "unresolved summary report path")
	fs.BoolVar(&options.Execute, "execute", false, "apply a confirmed unique backfill (disabled until explicit confirmation)")
	fs.StringVar(&options.ConfirmReport, "confirm-report", "", "exact report fingerprint required for execute mode")
	fs.IntVar(&options.BatchSize, "batch-size", 500, "maximum rows per backfill transaction")
	if len(args) > 0 {
		_ = fs.Parse(args)
	} else if fs == flag.CommandLine {
		flag.Parse()
	}
	return options
}
