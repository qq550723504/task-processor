package main

import (
	"context"
	"log"

	"task-processor/internal/app/runtime/listing"
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	opts := listing.ParseFlags("amazon")
	opts.Version = appVersion
	opts.BuildTime = buildTime

	if err := listing.Run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}
