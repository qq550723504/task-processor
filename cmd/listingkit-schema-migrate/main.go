package main

import (
	"context"
	"log"

	"task-processor/internal/app/runtime/listingkitschemamigrate"
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	opts := listingkitschemamigrate.ParseFlags()
	opts.Version = appVersion
	opts.BuildTime = buildTime

	if err := listingkitschemamigrate.Run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}
