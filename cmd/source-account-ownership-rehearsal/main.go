// source-account-ownership-rehearsal runs only a fixed, disposable B2 rehearsal.
// It has no external or production database mode.
package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"task-processor/internal/app/runtime/sourceaccountownershiprehearsal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	os.Exit(sourceaccountownershiprehearsal.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
