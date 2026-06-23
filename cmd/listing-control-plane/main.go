package main

import (
	"context"
	"log"

	"task-processor/internal/app/runtime/listingcontrol"
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	opts := listingcontrol.ParseFlags()
	opts.Version = appVersion
	opts.BuildTime = buildTime

	if err := listingcontrol.Run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}
