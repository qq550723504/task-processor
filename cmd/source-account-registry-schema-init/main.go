package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	schemainit "task-processor/internal/app/runtime/sourceaccountregistryschemainit"
)

func main() {
	configPath := flag.String("config", "config/config-dev.yaml", "database config file path")
	flag.Parse()
	if err := schemainit.Run(context.Background(), *configPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "initialize source account registry schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("source account registry schema initialized using %s\n", *configPath)
}
