package sourceaccountownershiprehearsal

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"task-processor/internal/integration/persistence/sourceaccount/ownershipmigration"
)

func TestDecodeFrameIsBoundedAndRejectsTrailingInput(t *testing.T) {
	var value map[string]string
	if err := decodeFrame(strings.NewReader("{\"stage\":\"inspect\"}\n"), &value); err != nil {
		t.Fatal(err)
	}
	if value["stage"] != "inspect" {
		t.Fatalf("decoded stage = %q", value["stage"])
	}
	if err := decodeFrame(strings.NewReader("{\"stage\":\"inspect\"} trailing\n"), &value); err == nil {
		t.Fatal("decodeFrame accepted trailing data")
	}
	if err := decodeFrame(bytes.NewReader(bytes.Repeat([]byte{'x'}, maxProtocolFrameBytes+1)), &value); err == nil {
		t.Fatal("decodeFrame accepted an oversized frame")
	}
}

func TestGrantBindsTheOneShotChildAndDatabaseIdentities(t *testing.T) {
	challenge := stageChallenge{Stage: stagePrepare, Nonce: "fresh", ChildPID: 17, ParentPID: 11}
	grant := stageGrant{
		Invocation:     "invocation",
		Stage:          stagePrepare,
		Nonce:          "fresh",
		ChildPID:       17,
		ParentPID:      11,
		Container:      "container",
		Source:         databaseIdentity{SystemIdentifier: "system", Database: "source_b2", OID: 101},
		Metadata:       databaseIdentity{SystemIdentifier: "system", Database: "metadata_b2", OID: 102},
		SourceDSN:      "postgres://b2_prepare:secret@127.0.0.1:5432/source_b2?sslmode=disable",
		MetadataDSN:    "postgres://b2_prepare:secret@127.0.0.1:5432/metadata_b2?sslmode=disable",
		SourceID:       "issue364/rehearsal/source",
		MetadataID:     "issue364/rehearsal/metadata",
		ProfileRoot:    filepath.Join(t.TempDir(), "profiles"),
		ReceiptPath:    filepath.Join(t.TempDir(), "receipt.json"),
		IdempotencyKey: "issue364-b2-rehearsal-v1",
		Preflight: ownershipmigration.Receipt{
			Version: 1, Stage: "preflight_only", SnapshotConsistency: "separate_non_atomic_snapshots",
			AccountObservation:  ownershipmigration.Observation{SourceID: "issue364/rehearsal/source", Database: "source_b2"},
			MetadataObservation: ownershipmigration.Observation{SourceID: "issue364/rehearsal/metadata", Database: "metadata_b2"},
			Digest:              strings.Repeat("a", 64),
		},
	}
	if err := validateGrant(challenge, grant); err != nil {
		t.Fatalf("validateGrant(valid) error = %v", err)
	}

	mutations := []func(*stageGrant){
		func(v *stageGrant) { v.Stage = stageReceiptRead },
		func(v *stageGrant) { v.Nonce = "copied" },
		func(v *stageGrant) { v.ChildPID++ },
		func(v *stageGrant) { v.ParentPID++ },
		func(v *stageGrant) { v.Invocation = "" },
		func(v *stageGrant) { v.Container = "" },
		func(v *stageGrant) { v.Source.OID = 0 },
		func(v *stageGrant) { v.Metadata.SystemIdentifier = "other" },
		func(v *stageGrant) { v.SourceDSN = "" },
		func(v *stageGrant) {
			v.SourceDSN = "postgres://issue364_admin:secret@127.0.0.1:5432/source_b2?sslmode=disable"
		},
		func(v *stageGrant) { v.SourceDSN = "postgres://b2_prepare@127.0.0.1:5432/source_b2?sslmode=disable" },
		func(v *stageGrant) {
			v.MetadataDSN = "postgres://b2_prepare:secret@192.0.2.1:5432/metadata_b2?sslmode=disable"
		},
		func(v *stageGrant) { v.SourceID = "" },
		func(v *stageGrant) { v.ProfileRoot = "relative" },
		func(v *stageGrant) { v.IdempotencyKey = "" },
		func(v *stageGrant) { v.Preflight.Digest = "copied" },
	}
	for index, mutate := range mutations {
		changed := grant
		mutate(&changed)
		if err := validateGrant(challenge, changed); err == nil {
			t.Fatalf("validateGrant mutation %d accepted", index)
		}
	}

	receiptChallenge := challenge
	receiptChallenge.Stage = stageReceiptRead
	receiptGrant := grant
	receiptGrant.Stage = stageReceiptRead
	receiptGrant.SourceDSN = strings.Replace(receiptGrant.SourceDSN, "b2_prepare", "b2_receipt", 1)
	receiptGrant.MetadataDSN = strings.Replace(receiptGrant.MetadataDSN, "b2_prepare", "b2_receipt", 1)
	receiptGrant.DropResponseAfterWrite = true
	if err := validateGrant(receiptChallenge, receiptGrant); err == nil {
		t.Fatal("receipt-read grant accepted prepare-only response-loss injection")
	}
}

func TestPrepareErrorsAreUnknownUnlessTheyAreProvenDefinite(t *testing.T) {
	if got := classifyPrepareError(ownershipmigration.ErrIdempotencyConflict); got != ExitConflictOrDrift {
		t.Fatalf("classify conflict = %d", got)
	}
	if got := classifyPrepareError(context.Canceled); got != ExitOutcomeUnknown {
		t.Fatalf("classify cancellation after Prepare invocation = %d", got)
	}
	if got := classifyPrepareError(errors.New("connection lost")); got != ExitOutcomeUnknown {
		t.Fatalf("classify transport failure = %d", got)
	}
}
