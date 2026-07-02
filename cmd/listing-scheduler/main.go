package main

import (
	"context"
	"log"

	"task-processor/internal/app/runtime/listingscheduler"
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	opts := listingscheduler.ParseFlags()
	opts.Version = appVersion
	opts.BuildTime = buildTime

	if err := listingscheduler.Run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}
