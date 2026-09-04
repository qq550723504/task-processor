package main

import (
	"context"
	"log"
	"os"

	"task-processor/internal/app/runtime/storehistorymigrate"
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	options := storehistorymigrate.ParseFlags()
	options.Version = appVersion
	options.BuildTime = buildTime
	if err := storehistorymigrate.Run(context.Background(), options, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
