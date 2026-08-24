package main

import (
	"context"
	"log"

	"task-processor/internal/app/runtime/listingkitidentitypreflight"
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	opts := listingkitidentitypreflight.ParseFlags()
	opts.Version = appVersion
	opts.BuildTime = buildTime

	if err := listingkitidentitypreflight.Run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}
