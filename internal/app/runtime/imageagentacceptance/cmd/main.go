package main

import (
	"context"
	"fmt"
	"os"

	imageagentacceptanceruntime "task-processor/internal/app/runtime/imageagentacceptance"
)

func main() {
	if err := imageagentacceptanceruntime.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
