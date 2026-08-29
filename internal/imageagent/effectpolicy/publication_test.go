package effectpolicy

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"task-processor/internal/imageagent"
)

func TestClaimPublicationDecisionMatrix(t *testing.T) {
	observedAt := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	request := publicationPolicyClaimRequest()
	active := publicationPolicyAttempt(request.Reservation, imageagent.SlotEffectV3PublicationClaimed)
	active.Publication = imageagent.PublicationClaim{Owner: "worker-a", Fence: 7, LeaseExpiresAt: observedAt.Add(time.Minute)}
	active.PublicationFingerprint = request.PublicationFingerprint
	active.FinalManifest = publicationPolicyFinalManifest(strings.ToUpper(publicationPolicySHA256))
	expired := cloneSlotEffectV3Attempt(active)
	expired.Publication.LeaseExpiresAt = observedAt
	complete := cloneSlotEffectV3Attempt(active)
	complete.Phase = imageagent.SlotEffectV3PublicationComplete
	conflictingManifest := request
	conflictingManifest.FinalManifest = cloneFinalManifestForPublicationPolicy(request.FinalManifest)
	conflictingManifest.FinalManifest.Assets[0].ObjectKey = "image-agent/final/tenant-a/run/other.png"
	conflictingFingerprint := request
	conflictingFingerprint.PublicationFingerprint = "other-publication"
	invalid := request
	invalid.Owner = ""

	tests := []struct {
		name         string
		current      imageagent.SlotEffectV3Attempt
		request      imageagent.PublicationClaimRequest
		wantErr      error
		wantChanged  bool
		wantAcquired bool
		wantOwner    string
		wantFence    int64
		wantExpiry   time.Time
	}{
		{name: "first claim starts fence one", current: publicationPolicyAttempt(request.Reservation, imageagent.SlotEffectV3ArtifactStaged), request: request, wantChanged: true, wantAcquired: true, wantOwner: "worker-b", wantFence: 1, wantExpiry: observedAt.Add(2 * time.Minute)},
		{name: "active lease replays normalized manifest", current: active, request: request, wantOwner: "worker-a", wantFence: 7, wantExpiry: observedAt.Add(time.Minute)},
		{name: "expired lease hands off and increments fence", current: expired, request: request, wantChanged: true, wantAcquired: true, wantOwner: "worker-b", wantFence: 8, wantExpiry: observedAt.Add(2 * time.Minute)},
		{name: "manifest conflict", current: active, request: conflictingManifest, wantErr: imageagent.ErrRevisionConflict},
		{name: "fingerprint conflict", current: active, request: conflictingFingerprint, wantErr: imageagent.ErrRevisionConflict},
		{name: "post completion claim is replay only", current: complete, request: request, wantOwner: "worker-a", wantFence: 7, wantExpiry: observedAt.Add(time.Minute)},
		{name: "invalid command precedes invalid persisted state", current: imageagent.SlotEffectV3Attempt{}, request: invalid, wantErr: imageagent.ErrValidation},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := ClaimPublication(test.current, test.request, observedAt)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ClaimPublication() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Changed != test.wantChanged || decision.Acquired != test.wantAcquired || decision.Claim.Owner != test.wantOwner || decision.Claim.Fence != test.wantFence || !decision.Claim.LeaseExpiresAt.Equal(test.wantExpiry) {
				t.Fatalf("ClaimPublication() decision = %+v, want changed=%t acquired=%t owner=%q fence=%d expiry=%s", decision, test.wantChanged, test.wantAcquired, test.wantOwner, test.wantFence, test.wantExpiry)
			}
			if decision.Attempt.FinalManifest.Assets[0].SHA256 != publicationPolicySHA256 {
				t.Fatalf("ClaimPublication() manifest SHA256 = %q, want normalized %q", decision.Attempt.FinalManifest.Assets[0].SHA256, publicationPolicySHA256)
			}
		})
	}
}

func TestRenewPublicationDecisionMatrix(t *testing.T) {
	observedAt := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	reservation := publicationPolicyReservation()
	active := publicationPolicyAttempt(reservation, imageagent.SlotEffectV3PublicationClaimed)
	active.Publication = imageagent.PublicationClaim{Owner: "worker-a", Fence: 3, LeaseExpiresAt: observedAt.Add(time.Minute)}
	renewal := imageagent.PublicationLeaseRenewal{Identity: reservation.Identity, Owner: "worker-a", Fence: 3, LeaseDuration: 2 * time.Minute}

	tests := []struct {
		name       string
		current    imageagent.SlotEffectV3Attempt
		renewal    imageagent.PublicationLeaseRenewal
		at         time.Time
		wantErr    error
		wantExpiry time.Time
	}{
		{name: "renews active owner and fence", current: active, renewal: renewal, at: observedAt, wantExpiry: observedAt.Add(2 * time.Minute)},
		{name: "stale owner", current: active, renewal: func() imageagent.PublicationLeaseRenewal { value := renewal; value.Owner = "worker-b"; return value }(), at: observedAt, wantErr: imageagent.ErrRevisionConflict},
		{name: "stale fence", current: active, renewal: func() imageagent.PublicationLeaseRenewal { value := renewal; value.Fence = 2; return value }(), at: observedAt, wantErr: imageagent.ErrRevisionConflict},
		{name: "at expiry", current: active, renewal: renewal, at: active.Publication.LeaseExpiresAt, wantErr: imageagent.ErrRevisionConflict},
		{name: "after expiry", current: active, renewal: renewal, at: active.Publication.LeaseExpiresAt.Add(time.Nanosecond), wantErr: imageagent.ErrRevisionConflict},
		{name: "wrong phase", current: publicationPolicyAttempt(reservation, imageagent.SlotEffectV3ArtifactStaged), renewal: renewal, at: observedAt, wantErr: imageagent.ErrRevisionConflict},
		{name: "invalid renewal", current: imageagent.SlotEffectV3Attempt{}, renewal: func() imageagent.PublicationLeaseRenewal { value := renewal; value.Fence = 0; return value }(), at: observedAt, wantErr: imageagent.ErrValidation},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := RenewPublication(test.current, test.renewal, test.at)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("RenewPublication() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if !decision.Changed || decision.Claim.Owner != renewal.Owner || decision.Claim.Fence != renewal.Fence || !decision.Claim.LeaseExpiresAt.Equal(test.wantExpiry) {
				t.Fatalf("RenewPublication() decision = %+v, want changed owner/fence and expiry %s", decision, test.wantExpiry)
			}
		})
	}
}

func TestCompletePublicationDecisionMatrix(t *testing.T) {
	request := publicationPolicyClaimRequest()
	current := publicationPolicyAttempt(request.Reservation, imageagent.SlotEffectV3PublicationClaimed)
	current.Publication = imageagent.PublicationClaim{Owner: "worker-a", Fence: 4, LeaseExpiresAt: time.Date(2026, time.August, 29, 10, 1, 0, 0, time.UTC)}
	current.PublicationFingerprint = request.PublicationFingerprint
	current.FinalManifest = publicationPolicyTwoAssetFinalManifest(strings.ToUpper(publicationPolicySHA256))
	completion := publicationPolicyCompletion(t, request.Reservation, current.Publication, request.PublicationFingerprint)
	completed := cloneSlotEffectV3Attempt(current)
	completed.Phase = imageagent.SlotEffectV3PublicationComplete
	completed.ResultFingerprint = completion.ResultFingerprint
	completed.Published = completion.Published
	completed.Published.Candidates = append([]imageagent.SlotEffectV3AssetCandidate(nil), completion.Published.Candidates...)
	completed.Published.Candidates[0].DurableAsset.SHA256 = strings.ToUpper(completed.Published.Candidates[0].DurableAsset.SHA256)

	tests := []struct {
		name        string
		current     imageagent.SlotEffectV3Attempt
		completion  imageagent.PublicationCompletion
		wantErr     error
		wantChanged bool
	}{
		{name: "ordered final manifest bijection completes", current: current, completion: completion, wantChanged: true},
		{name: "exact normalized completion repeats", current: completed, completion: completion},
		{name: "stale owner", current: current, completion: func() imageagent.PublicationCompletion { value := completion; value.Owner = "worker-b"; return value }(), wantErr: imageagent.ErrRevisionConflict},
		{name: "stale fence", current: current, completion: func() imageagent.PublicationCompletion { value := completion; value.Fence = 3; return value }(), wantErr: imageagent.ErrRevisionConflict},
		{name: "conflicting completed result", current: completed, completion: func() imageagent.PublicationCompletion {
			value := completion
			value.ResultFingerprint = "other-result"
			return value
		}(), wantErr: imageagent.ErrRevisionConflict},
		{name: "reordered final manifest is rejected", current: current, completion: publicationPolicyReorderedCompletion(t, completion), wantErr: imageagent.ErrRevisionConflict},
		{name: "invalid completion precedes invalid persisted state", current: imageagent.SlotEffectV3Attempt{}, completion: func() imageagent.PublicationCompletion { value := completion; value.Owner = ""; return value }(), wantErr: imageagent.ErrValidation},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := CompletePublication(test.current, test.completion)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CompletePublication() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Changed != test.wantChanged || decision.Attempt.Phase != imageagent.SlotEffectV3PublicationComplete {
				t.Fatalf("CompletePublication() decision = %+v, want changed=%t complete", decision, test.wantChanged)
			}
			if decision.Attempt.Published.Candidates[0].DurableAsset.SHA256 != publicationPolicySHA256 {
				t.Fatalf("CompletePublication() published SHA256 = %q, want normalized %q", decision.Attempt.Published.Candidates[0].DurableAsset.SHA256, publicationPolicySHA256)
			}
		})
	}
}

func TestPublicationDecisionsDoNotMutateInputs(t *testing.T) {
	observedAt := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	request := publicationPolicyClaimRequest()
	current := publicationPolicyAttempt(request.Reservation, imageagent.SlotEffectV3ArtifactStaged)
	originalCurrent := cloneSlotEffectV3Attempt(current)
	originalRequest := request
	originalRequest.FinalManifest = cloneFinalManifestForPublicationPolicy(request.FinalManifest)

	claimed, err := ClaimPublication(current, request, observedAt)
	if err != nil {
		t.Fatalf("ClaimPublication() error = %v", err)
	}
	if !reflect.DeepEqual(current, originalCurrent) || !reflect.DeepEqual(request, originalRequest) {
		t.Fatal("ClaimPublication() mutated caller-owned attempt or request")
	}
	claimed.Attempt.FinalManifest.Assets[0].Operations[0] = "crop"
	if request.FinalManifest.Assets[0].Operations[0] != "resize" {
		t.Fatal("ClaimPublication() returned a manifest alias")
	}

	renewal := imageagent.PublicationLeaseRenewal{Identity: request.Reservation.Identity, Owner: claimed.Claim.Owner, Fence: claimed.Claim.Fence, LeaseDuration: time.Minute}
	originalClaimed := cloneSlotEffectV3Attempt(claimed.Attempt)
	renewed, err := RenewPublication(claimed.Attempt, renewal, observedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("RenewPublication() error = %v", err)
	}
	if !reflect.DeepEqual(claimed.Attempt, originalClaimed) {
		t.Fatal("RenewPublication() mutated caller-owned attempt")
	}

	completion := publicationPolicyCompletion(t, request.Reservation, renewed.Claim, request.PublicationFingerprint)
	renewed.Attempt.FinalManifest = publicationPolicyTwoAssetFinalManifest(publicationPolicySHA256)
	originalRenewed := cloneSlotEffectV3Attempt(renewed.Attempt)
	originalCompletion := completion
	originalCompletion.Published.Candidates = append([]imageagent.SlotEffectV3AssetCandidate(nil), completion.Published.Candidates...)
	completed, err := CompletePublication(renewed.Attempt, completion)
	if err != nil {
		t.Fatalf("CompletePublication() error = %v", err)
	}
	if !reflect.DeepEqual(renewed.Attempt, originalRenewed) || !reflect.DeepEqual(completion, originalCompletion) {
		t.Fatal("CompletePublication() mutated caller-owned attempt or completion")
	}
	completed.Attempt.Published.Candidates[0].AssetID = "changed"
	if completion.Published.Candidates[0].AssetID != "candidate-a" {
		t.Fatal("CompletePublication() returned a published-result alias")
	}
}

const publicationPolicySHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func publicationPolicyReservation() imageagent.SlotEffectV3Reservation {
	return imageagent.SlotEffectV3Reservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-a"}, PlanRevision: 1, SlotID: "slot-a", Attempt: 1}, IdempotencyKey: "publication-key", InputFingerprint: "publication-input"}
}

func publicationPolicyAttempt(reservation imageagent.SlotEffectV3Reservation, phase imageagent.SlotEffectV3Phase) imageagent.SlotEffectV3Attempt {
	return imageagent.SlotEffectV3Attempt{Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint, Policy: reservation.Policy, Quote: cloneSlotUsageQuote(reservation.Quote), Phase: phase}
}

func publicationPolicyClaimRequest() imageagent.PublicationClaimRequest {
	return imageagent.PublicationClaimRequest{Reservation: publicationPolicyReservation(), Owner: "worker-b", LeaseDuration: 2 * time.Minute, PublicationFingerprint: "publication-fingerprint", FinalManifest: publicationPolicyFinalManifest(strings.ToUpper(publicationPolicySHA256))}
}

func publicationPolicyFinalManifest(sha string) imageagent.FinalManifest {
	return imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{{ObjectKey: "image-agent/final/tenant-a/run/asset-a.png", SHA256: sha, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}}}}
}

func publicationPolicyTwoAssetFinalManifest(firstSHA string) imageagent.FinalManifest {
	return imageagent.FinalManifest{Assets: []imageagent.PublishedAssetRef{
		{ObjectKey: "image-agent/final/tenant-a/run/asset-a.png", SHA256: firstSHA, SizeBytes: 42, ContentType: "image/png", Width: 1200, Height: 1200, SourceAssetID: "source-1", Operations: []string{"resize"}},
		{ObjectKey: "image-agent/final/tenant-a/run/asset-b.png", SHA256: strings.Repeat("b", 64), SizeBytes: 84, ContentType: "image/png", Width: 800, Height: 800, SourceAssetID: "source-2", Operations: []string{"resize"}},
	}}
}

func publicationPolicyCompletion(t *testing.T, reservation imageagent.SlotEffectV3Reservation, claim imageagent.PublicationClaim, publicationFingerprint string) imageagent.PublicationCompletion {
	t.Helper()
	published := imageagent.SlotEffectV3PublishedResult{SlotID: reservation.Identity.SlotID, Attempt: reservation.Identity.Attempt, Candidates: []imageagent.SlotEffectV3AssetCandidate{
		{AssetID: "candidate-a", SourceAssetID: "source-1", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset-a.png", SHA256: strings.ToUpper(publicationPolicySHA256)}},
		{AssetID: "candidate-b", SourceAssetID: "source-2", DurableAsset: imageagent.DurableAssetIdentity{ObjectKey: "image-agent/final/tenant-a/run/asset-b.png", SHA256: strings.Repeat("b", 64)}},
	}}
	fingerprint, err := imageagent.SlotEffectV3PublishedResultFingerprint(published)
	if err != nil {
		t.Fatalf("SlotEffectV3PublishedResultFingerprint() error = %v", err)
	}
	return imageagent.PublicationCompletion{Reservation: reservation, Owner: claim.Owner, Fence: claim.Fence, PublicationFingerprint: publicationFingerprint, ResultFingerprint: fingerprint, Published: published}
}

func publicationPolicyReorderedCompletion(t *testing.T, completion imageagent.PublicationCompletion) imageagent.PublicationCompletion {
	t.Helper()
	completion.Published.Candidates = append([]imageagent.SlotEffectV3AssetCandidate(nil), completion.Published.Candidates...)
	completion.Published.Candidates[0], completion.Published.Candidates[1] = completion.Published.Candidates[1], completion.Published.Candidates[0]
	fingerprint, err := imageagent.SlotEffectV3PublishedResultFingerprint(completion.Published)
	if err != nil {
		t.Fatalf("SlotEffectV3PublishedResultFingerprint() error = %v", err)
	}
	completion.ResultFingerprint = fingerprint
	return completion
}

func cloneFinalManifestForPublicationPolicy(manifest imageagent.FinalManifest) imageagent.FinalManifest {
	manifest.Assets = clonePublishedAssetRefs(manifest.Assets)
	return manifest
}
