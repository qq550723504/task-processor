package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"task-processor/internal/app/runtime/sheinplatformrecovery"
)

func main() {
	var opts sheinplatformrecovery.Options
	flag.StringVar(&opts.Config, "config", "config/config-dev.yaml", "config file path")
	flag.Int64Var(&opts.StoreID, "store-id", 986, "store ID (must be 986)")
	flag.IntVar(&opts.ExpectedCount, "expected-count", 0, "required verified cohort count")
	flag.BoolVar(&opts.Execute, "execute", false, "apply the update; default is dry-run")
	flag.Parse()

	if err := sheinplatformrecovery.Run(context.Background(), opts); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
