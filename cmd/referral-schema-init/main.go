package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	app "task-processor/internal/app/referralregistration"
	"time"
)

func run(args []string, output io.Writer, install func(context.Context, string) error) int {
	fail := func() int { _, _ = fmt.Fprintln(output, "referral schema initialization failed"); return 1 }
	flags := flag.NewFlagSet("referral-schema-init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("dsn-file", "", "explicit connection file")
	timeout := flags.Duration("timeout", 30*time.Second, "bounded installation timeout")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *path == "" || *timeout <= 0 || *timeout > time.Minute || install == nil {
		return fail()
	}
	f, err := os.Open(*path)
	if err != nil {
		return fail()
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return fail()
	}
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(data) > 65536 || strings.TrimSpace(string(data)) == "" {
		return fail()
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if install(ctx, strings.TrimSpace(string(data))) != nil {
		return fail()
	}
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stderr, app.InstallSchema)) }
