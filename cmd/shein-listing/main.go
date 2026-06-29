package main

import (
	"context"
	"log"
	"os"

	"task-processor/internal/app/runtime/listing"
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "recover-paused-tasks" {
		opts, err := listing.ParsePausedTaskRecoveryFlags("shein", os.Args[2:])
		if err != nil {
			log.Fatal(err)
		}
		opts.Version = appVersion
		opts.BuildTime = buildTime
		if err := listing.RunPausedTaskRecovery(context.Background(), opts); err != nil {
			log.Fatal(err)
		}
		return
	}

	opts := listing.ParseFlags("shein")
	opts.Version = appVersion
	opts.BuildTime = buildTime

	if err := listing.Run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}
