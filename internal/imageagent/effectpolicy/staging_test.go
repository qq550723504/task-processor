package effectpolicy

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"task-processor/internal/imageagent"
)

func TestPrepareStagingDecisionMatrix(t *testing.T) {
	reservation := stagingPolicyReservation()
	manifest := stagingPolicyManifest()
	normalizedFingerprint, err := imageagent.StagingManifestFingerprint(manifest)
	if err != nil {
		t.Fatalf("StagingManifestFingerprint() error = %v", err)
	}
	prepared := stagingPolicyAttempt(reservation, imageagent.SlotEffectV3StagingPrepared)
	prepared.StagingManifest = manifest
	prepared.StagingManifestFingerprint = normalizedFingerprint
	conflicting := stagingPolicyManifest()
	conflicting.Assets[0].ObjectKey = "image-agent/staging/tenant-a/run/replacement.png"
	invalidPersisted := prepared
	invalidPersisted.BlockedCode = "unexpected"
	reservationMismatch := reservation
	reservationMismatch.InputFingerprint = "other-input"

	tests := []struct {
		name        string
		current     imageagent.SlotEffectV3Attempt
		reservation imageagent.SlotEffectV3Reservation
		manifest    imageagent.StagingManifest
		wantErr     error
		wantChanged bool
		wantPhase   imageagent.SlotEffectV3Phase
		wantFP      string
	}{
		{name: "normalizes and prepares provider claim", current: stagingPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed), reservation: reservation, manifest: manifest, wantChanged: true, wantPhase: imageagent.SlotEffectV3StagingPrepared, wantFP: normalizedFingerprint},
		{name: "exact prepared repeat", current: prepared, reservation: reservation, manifest: manifest, wantPhase: imageagent.SlotEffectV3StagingPrepared, wantFP: normalizedFingerprint},
		{name: "conflicting prepared manifest", current: prepared, reservation: reservation, manifest: conflicting, wantErr: imageagent.ErrRevisionConflict},
		{name: "invalid phase", current: stagingPolicyAttempt(reservation, imageagent.SlotEffectV3ArtifactStaged), reservation: reservation, manifest: manifest, wantErr: imageagent.ErrRevisionConflict},
		{name: "invalid persisted policy", current: invalidPersisted, reservation: reservation, manifest: manifest, wantErr: imageagent.ErrInvalidPersistedPolicy},
		{name: "reservation mismatch", current: stagingPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed), reservation: reservationMismatch, manifest: manifest, wantErr: imageagent.ErrRevisionConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := PrepareStaging(test.current, test.reservation, test.manifest)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("PrepareStaging() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Changed != test.wantChanged || decision.Attempt.Phase != test.wantPhase || decision.Attempt.StagingManifestFingerprint != test.wantFP {
				t.Fatalf("PrepareStaging() decision = %+v, want changed=%t phase=%q fingerprint=%q", decision, test.wantChanged, test.wantPhase, test.wantFP)
			}
			if decision.Changed && decision.Attempt.StagingManifest.Assets[0].SHA256 != strings.ToLower(manifest.Assets[0].SHA256) {
				t.Fatalf("PrepareStaging() SHA256 = %q, want normalized %q", decision.Attempt.StagingManifest.Assets[0].SHA256, strings.ToLower(manifest.Assets[0].SHA256))
			}
		})
	}
}

func TestCommitStagedDecisionMatrix(t *testing.T) {
	reservation := stagingPolicyReservation()
	manifest := stagingPolicyManifest()
	fingerprint, err := imageagent.StagingManifestFingerprint(manifest)
	if err != nil {
		t.Fatalf("StagingManifestFingerprint() error = %v", err)
	}
	prepared := stagingPolicyAttempt(reservation, imageagent.SlotEffectV3StagingPrepared)
	prepared.StagingManifest = manifest
	prepared.StagingManifestFingerprint = fingerprint
	staged := stagingPolicyAttempt(reservation, imageagent.SlotEffectV3ArtifactStaged)
	staged.StagingManifest = manifest
	staged.StagingManifestFingerprint = fingerprint
	invalidPersisted := prepared
	invalidPersisted.BlockedCode = "unexpected"
	reservationMismatch := reservation
	reservationMismatch.IdempotencyKey = "other-key"

	tests := []struct {
		name        string
		current     imageagent.SlotEffectV3Attempt
		reservation imageagent.SlotEffectV3Reservation
		fingerprint string
		wantErr     error
		wantChanged bool
		wantPhase   imageagent.SlotEffectV3Phase
	}{
		{name: "matching prepared commit", current: prepared, reservation: reservation, fingerprint: fingerprint, wantChanged: true, wantPhase: imageagent.SlotEffectV3ArtifactStaged},
		{name: "wrong fingerprint", current: prepared, reservation: reservation, fingerprint: "wrong-fingerprint", wantErr: imageagent.ErrRevisionConflict},
		{name: "staged repeat", current: staged, reservation: reservation, fingerprint: fingerprint, wantPhase: imageagent.SlotEffectV3ArtifactStaged},
		{name: "invalid phase", current: stagingPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed), reservation: reservation, fingerprint: fingerprint, wantErr: imageagent.ErrRevisionConflict},
		{name: "invalid persisted policy", current: invalidPersisted, reservation: reservation, fingerprint: fingerprint, wantErr: imageagent.ErrInvalidPersistedPolicy},
		{name: "reservation mismatch", current: prepared, reservation: reservationMismatch, fingerprint: fingerprint, wantErr: imageagent.ErrRevisionConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := CommitStaged(test.current, test.reservation, test.fingerprint)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CommitStaged() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Changed != test.wantChanged || decision.Attempt.Phase != test.wantPhase {
				t.Fatalf("CommitStaged() decision = %+v, want changed=%t phase=%q", decision, test.wantChanged, test.wantPhase)
			}
		})
	}
}

func TestStagingDecisionsDoNotMutateManifests(t *testing.T) {
	reservation := stagingPolicyReservation()
	manifest := stagingPolicyManifest()
	originalManifest := manifest
	originalManifest.Assets = append([]imageagent.StagedAssetRef(nil), manifest.Assets...)
	originalManifest.Assets[0].Operations = append([]string(nil), manifest.Assets[0].Operations...)
	current := stagingPolicyAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed)
	originalCurrent := cloneSlotEffectV3Attempt(current)

	prepared, err := PrepareStaging(current, reservation, manifest)
	if err != nil {
		t.Fatalf("PrepareStaging() error = %v", err)
	}
	if !reflect.DeepEqual(current, originalCurrent) || !reflect.DeepEqual(manifest, originalManifest) {
		t.Fatal("PrepareStaging() mutated caller-owned attempt or manifest")
	}
	manifest.Assets[0].Operations[0] = "crop"
	if prepared.Attempt.StagingManifest.Assets[0].Operations[0] != "resize" {
		t.Fatal("PrepareStaging() returned manifest aliases caller input")
	}
	originalPrepared := cloneSlotEffectV3Attempt(prepared.Attempt)
	committed, err := CommitStaged(prepared.Attempt, reservation, prepared.Attempt.StagingManifestFingerprint)
	if err != nil {
		t.Fatalf("CommitStaged() error = %v", err)
	}
	if !reflect.DeepEqual(prepared.Attempt, originalPrepared) {
		t.Fatal("CommitStaged() mutated caller-owned attempt")
	}
	committed.Attempt.StagingManifest.Assets[0].Operations[0] = "crop"
	if prepared.Attempt.StagingManifest.Assets[0].Operations[0] != "resize" {
		t.Fatal("CommitStaged() returned attempt aliases caller input")
	}
}

func stagingPolicyReservation() imageagent.SlotEffectV3Reservation {
	return imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-a"}, PlanRevision: 1, SlotID: "slot-a", Attempt: 1}, IdempotencyKey: "staging-key", InputFingerprint: "staging-input"}
}

func stagingPolicyAttempt(reservation imageagent.SlotEffectV3Reservation, phase imageagent.SlotEffectV3Phase) imageagent.SlotEffectV3Attempt {
	return imageagent.SlotEffectV3Attempt{Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Policy: reservation.Policy, Quote: cloneSlotUsageQuote(reservation.Quote), Phase: phase}
}

func stagingPolicyManifest() imageagent.StagingManifest {
	return imageagent.StagingManifest{Assets: []imageagent.StagedAssetRef{{ObjectKey: "image-agent/staging/tenant-a/run/asset.png", SHA256: strings.Repeat("A", 64), SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}, ProviderReceiptID: "receipt-1"}}}
}
