// source-account-ownership-preflight produces read-only migration evidence. It
// does not backfill ownership, drain jobs or modify browser profiles.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"task-processor/internal/integration/persistence/sourceaccount/ownershipmigration"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("profile-root", "", "absolute existing 1688 account runtime profile root on this host")
	sourceID := flag.String("source-id", "", "non-secret business database environment/cluster identity")
	metadataID := flag.String("metadata-id", "", "non-secret authoritative ZITADEL environment/cluster identity")
	output := flag.String("receipt", "", "absolute new receipt file path outside the profile root (never overwritten)")
	timeout := flag.Duration("timeout", 2*time.Minute, "total deadline (maximum 10m)")
	flag.Parse()
	if flag.NArg() != 0 || *root == "" || *sourceID == "" || *metadataID == "" || *output == "" || *timeout <= 0 || *timeout > 10*time.Minute {
		return fmt.Errorf("profile-root, source-id, metadata-id, new receipt path and timeout in (0,10m] required")
	}
	sourceDSN := os.Getenv("SOURCE_ACCOUNT_PREFLIGHT_DSN")
	metadataDSN := os.Getenv("SOURCE_ACCOUNT_METADATA_DSN")
	if sourceDSN == "" || metadataDSN == "" {
		return fmt.Errorf("SOURCE_ACCOUNT_PREFLIGHT_DSN and SOURCE_ACCOUNT_METADATA_DSN must explicitly select the business and authoritative ZITADEL databases")
	}
	source, err := sql.Open("pgx", sourceDSN)
	if err != nil {
		return fmt.Errorf("invalid business database configuration")
	}
	defer source.Close()
	metadata, err := sql.Open("pgx", metadataDSN)
	if err != nil {
		return fmt.Errorf("invalid metadata database configuration")
	}
	defer metadata.Close()
	source.SetMaxOpenConns(1)
	metadata.SetMaxOpenConns(1)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	snapshot, err := ownershipmigration.ReadSnapshot(ctx, source, metadata, *sourceID, *metadataID)
	if err != nil {
		return err
	}
	receipt, err := ownershipmigration.Preflight(ctx, snapshot, *root)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = ownershipmigration.ValidateReceiptTarget(*root, *output, receipt); err != nil {
		return err
	}
	// WriteReceipt owns the publication success decision. It removes a just-linked
	// receipt if cancellation is observed before it returns; cancellation that
	// arrives after a nil return does not retroactively turn committed evidence
	// into a failed invocation.
	if err = ownershipmigration.WriteReceipt(ctx, *output, receipt); err != nil {
		return err
	}
	fmt.Printf("Preflight only: %d accounts; sha256 %s. Backfill, profile fleet validation and old-job drain are NOT certified.\n", len(receipt.Accounts), receipt.Digest)
	return nil
}
