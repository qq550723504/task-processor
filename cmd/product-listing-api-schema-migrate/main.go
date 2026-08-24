package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	productlistingschemamigrate "task-processor/internal/app/runtime/productlistingschemamigrate"
)

func main() {
	configPath := flag.String("config", "config/config-dev.yaml", "config file path")
	flag.Parse()

	if err := productlistingschemamigrate.Run(context.Background(), *configPath); err != nil {
		exitf("migrate product listing API schema: %v", err)
	}

	fmt.Printf("product-listing-api schema migration completed using %s\n", *configPath)
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
