package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"task-processor/internal/app/productsourcing"
)

func main() {
	manifest := flag.String("config", "", "absolute private initialization manifest path")
	confirmed := flag.String("confirm-empty-database", "", "exact pre-created empty database name")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := productsourcing.InitializeAcquisitionDatabase(ctx, *manifest, *confirmed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Product acquisition schema and runtime grants initialized")
}
