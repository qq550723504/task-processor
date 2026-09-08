package sourceaccountownershiprehearsal

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"task-processor/internal/integration/persistence/sourceaccount/ownershipmigration"
)

func runChildStage(ctx context.Context, stage string, stdin io.Reader, stdout, stderr io.Writer) int {
	if !validStage(stage) {
		fmt.Fprintln(stderr, "invalid internal rehearsal stage")
		return ExitInvalidInput
	}
	nonce, err := newNonce()
	if err != nil {
		fmt.Fprintln(stderr, "internal rehearsal challenge failed")
		return ExitDependencyFailure
	}
	challenge := stageChallenge{Stage: stage, Nonce: nonce, ChildPID: os.Getpid(), ParentPID: os.Getppid()}
	if err = encodeFrame(stdout, challenge); err != nil {
		return ExitDependencyFailure
	}
	reader := bufio.NewReaderSize(stdin, maxProtocolFrameBytes+2)
	var grant stageGrant
	if err = decodeFrame(reader, &grant); err != nil || validateGrant(challenge, grant) != nil {
		fmt.Fprintln(stderr, "live parent grant rejected")
		return ExitAdmissionDenied
	}

	stageCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		// The parent keeps this anonymous pipe open until it has consumed the
		// result. EOF proves that the process holding the allocation disappeared.
		_, _ = reader.ReadByte()
		cancel()
	}()

	result := stageResult{Invocation: grant.Invocation, Stage: grant.Stage, Nonce: grant.Nonce}
	result.Code, result.Status, result.Preflight, result.Prepared, result.Replayed = executeChildStage(stageCtx, grant)
	if grant.Stage == stagePrepare && grant.DropResponseAfterWrite && result.Code == ExitSuccess {
		// Deliberately model a committed write whose response was lost. Only a
		// new receipt-read process is allowed to resolve this outcome.
		return ExitOutcomeUnknown
	}
	if err = encodeFrame(stdout, result); err != nil {
		if grant.Stage == stagePrepare {
			return ExitOutcomeUnknown
		}
		return ExitDependencyFailure
	}
	return result.Code
}

func executeChildStage(ctx context.Context, grant stageGrant) (int, string, *ownershipmigration.Receipt, *ownershipmigration.PreparedReceipt, bool) {
	source, err := openVerifiedDatabase(ctx, grant.SourceDSN, grant.Source)
	if err != nil {
		return ExitAdmissionDenied, "source-admission-denied", nil, nil, false
	}
	defer source.Close()

	if grant.Stage == stageInstallSchema {
		if err = ownershipmigration.InstallPreparedSchema(ctx, source); err != nil {
			return ExitDependencyFailure, "schema-install-failed", nil, nil, false
		}
		return ExitSuccess, "schema-installed", nil, nil, false
	}

	metadata, err := openVerifiedDatabase(ctx, grant.MetadataDSN, grant.Metadata)
	if err != nil {
		return ExitAdmissionDenied, "metadata-admission-denied", nil, nil, false
	}
	defer metadata.Close()

	if grant.Stage == stageInspect {
		receipt, inspectErr := freshPreflight(ctx, source, metadata, grant)
		if inspectErr != nil {
			return classifyPreflightError(inspectErr), "inspect-failed", nil, nil, false
		}
		if err = ownershipmigration.ValidateReceiptTarget(grant.ProfileRoot, grant.ReceiptPath, receipt); err != nil {
			return ExitAdmissionDenied, "receipt-target-rejected", nil, nil, false
		}
		if err = ownershipmigration.WriteReceipt(ctx, grant.ReceiptPath, receipt); err != nil {
			return ExitDependencyFailure, "receipt-write-failed", nil, nil, false
		}
		return ExitSuccess, "inspected", &receipt, nil, false
	}

	sourceGuard, err := acquireFreeze(ctx, source, "SELECT public.b2_hold_source_freeze()")
	if err != nil {
		return ExitDependencyFailure, "source-freeze-failed", nil, nil, false
	}
	defer sourceGuard.Rollback()
	metadataGuard, err := acquireFreeze(ctx, metadata, "SELECT projections.b2_hold_metadata_freeze()")
	if err != nil {
		return ExitDependencyFailure, "metadata-freeze-failed", nil, nil, false
	}
	defer metadataGuard.Rollback()

	current, err := freshPreflight(ctx, source, metadata, grant)
	if err != nil {
		return classifyPreflightError(err), "preflight-failed", nil, nil, false
	}
	if !samePinnedPreflight(current, grant.Preflight) {
		return ExitConflictOrDrift, "preflight-drift", nil, nil, false
	}
	request := ownershipmigration.PrepareRequest{
		ContractVersion: ownershipmigration.PreparedContractVersion,
		IdempotencyKey:  grant.IdempotencyKey,
		SourceID:        grant.SourceID,
		Preflight:       current,
	}
	preparer, err := ownershipmigration.NewPreparer(source)
	if err != nil {
		return ExitDependencyFailure, "preparer-init-failed", nil, nil, false
	}
	if grant.Stage == stageReceiptRead {
		prepared, found, readErr := preparer.ReadPreparedReceipt(ctx, request)
		if readErr != nil {
			return classifyPrepareError(readErr), "receipt-read-failed", nil, nil, false
		}
		if !found {
			return ExitNotPrepared, "not-prepared", nil, nil, false
		}
		return ExitSuccess, "prepared", nil, &prepared, true
	}

	prepared, replayed, prepareErr := preparer.Prepare(ctx, request)
	if prepareErr != nil {
		return classifyPrepareError(prepareErr), "prepare-failed", nil, nil, false
	}
	return ExitSuccess, "prepared", nil, &prepared, replayed
}

func openVerifiedDatabase(ctx context.Context, dsn string, expected databaseIdentity) (*sql.DB, error) {
	db, err := openDatabase(ctx, dsn)
	if err != nil {
		return nil, err
	}
	actual, err := readDatabaseIdentity(ctx, db)
	if err != nil || actual != expected {
		_ = db.Close()
		return nil, errors.New("synthetic PostgreSQL database identity mismatch")
	}
	return db, nil
}

func freshPreflight(ctx context.Context, source, metadata *sql.DB, grant stageGrant) (ownershipmigration.Receipt, error) {
	if !filepath.IsAbs(grant.ProfileRoot) || !filepath.IsAbs(grant.ReceiptPath) {
		return ownershipmigration.Receipt{}, errors.New("absolute rehearsal paths required")
	}
	snapshot, err := ownershipmigration.ReadSnapshot(ctx, source, metadata, grant.SourceID, grant.MetadataID)
	if err != nil {
		return ownershipmigration.Receipt{}, snapshotReadError{err: err}
	}
	if snapshot.AccountObservation.Database != grant.Source.Database || snapshot.MetadataObservation.Database != grant.Metadata.Database {
		return ownershipmigration.Receipt{}, errors.New("snapshot database identity mismatch")
	}
	return ownershipmigration.Preflight(ctx, snapshot, grant.ProfileRoot)
}

type snapshotReadError struct {
	err error
}

func (err snapshotReadError) Error() string { return "synthetic rehearsal snapshot read failed" }
func (err snapshotReadError) Unwrap() error { return err.err }

func classifyPreflightError(err error) int {
	var readError snapshotReadError
	if errors.As(err, &readError) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ExitDependencyFailure
	}
	return ExitConflictOrDrift
}

func samePinnedPreflight(current, pinned ownershipmigration.Receipt) bool {
	return pinned.Version == 1 && pinned.Stage == "preflight_only" &&
		pinned.SnapshotConsistency == "separate_non_atomic_snapshots" &&
		current.Digest == pinned.Digest &&
		current.AccountObservation.SourceID == pinned.AccountObservation.SourceID &&
		current.AccountObservation.Database == pinned.AccountObservation.Database &&
		current.MetadataObservation.SourceID == pinned.MetadataObservation.SourceID &&
		current.MetadataObservation.Database == pinned.MetadataObservation.Database
}

func acquireFreeze(ctx context.Context, db *sql.DB, statement string) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: false})
	if err != nil {
		return nil, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		_ = tx.Rollback()
		return nil, errors.New("rehearsal deadline required")
	}
	timeout := time.Until(deadline)
	if timeout <= 0 {
		_ = tx.Rollback()
		return nil, context.DeadlineExceeded
	}
	if timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}
	timeoutSetting := strconv.FormatInt(max(timeout.Milliseconds(), 1), 10) + "ms"
	if _, err = tx.ExecContext(ctx, `SELECT set_config('lock_timeout', $1, true), set_config('statement_timeout', $1, true), set_config('idle_in_transaction_session_timeout', $1, true)`, timeoutSetting); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, statement); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}
