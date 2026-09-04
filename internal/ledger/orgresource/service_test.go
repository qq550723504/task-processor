package orgresource

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGrantWelcomeStoreRenewalPeriodRejectsUntrustedCallerBeforeSideEffects(t *testing.T) {
	verifier := &recordingEligibilityVerifier{}
	executor := &recordingWelcomeGrantExecutor{}
	authorizer := &recordingPrincipalAuthorizer{err: ErrForbidden}
	service, err := NewService(executor, verifier, authorizer)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.GrantWelcomeStoreRenewalPeriod(context.Background(), GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a",
		OperationID:    "operation-a",
		Principal:      Principal{ID: "browser-user", Kind: PrincipalTenantHuman},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("GrantWelcomeStoreRenewalPeriod() error = %v, want ErrForbidden", err)
	}
	if authorizer.calls != 1 || verifier.calls != 0 || executor.calls != 0 || executor.replayCalls != 0 {
		t.Fatalf("untrusted call reached side effects: auth=%d verifier=%d execute=%d replay=%d", authorizer.calls, verifier.calls, executor.calls, executor.replayCalls)
	}
}

func TestGrantWelcomeStoreRenewalPeriodBuildsFixedApprovedCommand(t *testing.T) {
	approvedAt := time.Date(2026, 9, 4, 1, 2, 3, 0, time.UTC)
	verifier := &recordingEligibilityVerifier{approval: WelcomeGrantApproval{
		OrganizationID: "org-a",
		EvidenceID:     "bootstrap:org-a",
		ApprovedAt:     approvedAt,
	}}
	executor := &recordingWelcomeGrantExecutor{result: WelcomeGrantResult{Snapshot: WelcomeGrantSnapshot{
		OperationID:    "operation-a",
		OrganizationID: "org-a",
		ResourceType:   ResourceStoreRenewalPeriod,
		Quantity:       "1",
		BalanceAfter:   "1",
		SourceType:     SourceOnboardingWelcomeStorePeriod,
		SourceIdentity: "org-a",
	}}}
	service, err := NewService(executor, verifier, &recordingPrincipalAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: " org-a ",
		OperationID:    " operation-a ",
		Principal:      Principal{ID: "onboarding", Kind: PrincipalTrustedProvisioning},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Snapshot.Quantity != "1" || result.Snapshot.ResourceType != ResourceStoreRenewalPeriod {
		t.Fatalf("result = %#v", result)
	}
	if verifier.calls != 1 || executor.calls != 1 {
		t.Fatalf("calls: verifier=%d executor=%d", verifier.calls, executor.calls)
	}
	got := executor.input
	if got.OrganizationID != "org-a" || got.OperationID != "operation-a" {
		t.Fatalf("identity fields = %#v", got)
	}
	if got.ResourceType != ResourceStoreRenewalPeriod || got.Quantity != 1 {
		t.Fatalf("fixed value fields = %#v", got)
	}
	if got.SourceType != SourceOnboardingWelcomeStorePeriod || got.SourceIdentity != "org-a" {
		t.Fatalf("source fields = %#v", got)
	}
	if got.ApprovalEvidenceID != "bootstrap:org-a" || !got.ApprovedAt.Equal(approvedAt) {
		t.Fatalf("approval fields = %#v", got)
	}
	if got.RequestFingerprint == "" {
		t.Fatal("request fingerprint is empty")
	}
}

func TestGrantWelcomeStoreRenewalPeriodFailsClosedOnApprovalScopeMismatch(t *testing.T) {
	verifier := &recordingEligibilityVerifier{approval: WelcomeGrantApproval{
		OrganizationID: "org-b",
		EvidenceID:     "bootstrap:org-b",
	}}
	executor := &recordingWelcomeGrantExecutor{}
	service, err := NewService(executor, verifier, &recordingPrincipalAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.GrantWelcomeStoreRenewalPeriod(context.Background(), GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a",
		OperationID:    "operation-a",
		Principal:      Principal{ID: "onboarding", Kind: PrincipalTrustedProvisioning},
	})
	if !errors.Is(err, ErrWelcomeGrantNotApproved) {
		t.Fatalf("GrantWelcomeStoreRenewalPeriod() error = %v, want ErrWelcomeGrantNotApproved", err)
	}
	if executor.calls != 0 {
		t.Fatalf("scope-mismatched approval reached executor %d times", executor.calls)
	}
}

func TestGrantWelcomeStoreRenewalPeriodDoesNotWriteWhenEligibilityAuthorityFails(t *testing.T) {
	verifier := &recordingEligibilityVerifier{err: errors.New("eligibility unavailable")}
	executor := &recordingWelcomeGrantExecutor{}
	service, err := NewService(executor, verifier, &recordingPrincipalAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.GrantWelcomeStoreRenewalPeriod(context.Background(), GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a",
		OperationID:    "operation-a",
		Principal:      Principal{ID: "onboarding", Kind: PrincipalTrustedProvisioning},
	})
	if err == nil {
		t.Fatal("GrantWelcomeStoreRenewalPeriod() error = nil, want eligibility failure")
	}
	if verifier.calls != 1 || executor.calls != 0 || executor.replayCalls != 1 {
		t.Fatalf("calls: verifier=%d execute=%d replay=%d", verifier.calls, executor.calls, executor.replayCalls)
	}
}

func TestGrantWelcomeStoreRenewalPeriodReplaysDurableResultBeforeEligibilityCheck(t *testing.T) {
	verifier := &recordingEligibilityVerifier{err: errors.New("eligibility authority unavailable")}
	executor := &recordingWelcomeGrantExecutor{
		replayFound: true,
		replayResult: WelcomeGrantResult{Snapshot: WelcomeGrantSnapshot{
			OperationID: "operation-a", OrganizationID: "org-a", Quantity: "1", BalanceAfter: "1",
		}, Replayed: true},
	}
	service, err := NewService(executor, verifier, &recordingPrincipalAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.GrantWelcomeStoreRenewalPeriod(context.Background(), GrantWelcomeStoreRenewalPeriodInput{
		OrganizationID: "org-a",
		OperationID:    "operation-a",
		Principal:      Principal{ID: "onboarding", Kind: PrincipalTrustedProvisioning},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Snapshot.BalanceAfter != "1" {
		t.Fatalf("result = %#v", result)
	}
	if verifier.calls != 0 || executor.calls != 0 || executor.replayCalls != 1 {
		t.Fatalf("calls: verifier=%d execute=%d replay=%d", verifier.calls, executor.calls, executor.replayCalls)
	}
}

type recordingEligibilityVerifier struct {
	approval WelcomeGrantApproval
	err      error
	calls    int
}

type recordingPrincipalAuthorizer struct {
	err   error
	calls int
}

func (a *recordingPrincipalAuthorizer) AuthorizeWelcomeGrant(context.Context, Principal) error {
	a.calls++
	return a.err
}

func (v *recordingEligibilityVerifier) VerifyWelcomeGrantEligibility(context.Context, string) (WelcomeGrantApproval, error) {
	v.calls++
	return v.approval, v.err
}

type recordingWelcomeGrantExecutor struct {
	input        WelcomeGrantExecution
	result       WelcomeGrantResult
	err          error
	calls        int
	replayResult WelcomeGrantResult
	replayFound  bool
	replayErr    error
	replayCalls  int
}

func (e *recordingWelcomeGrantExecutor) ReplayWelcomeGrant(_ context.Context, _ WelcomeGrantReplay) (WelcomeGrantResult, bool, error) {
	e.replayCalls++
	return e.replayResult, e.replayFound, e.replayErr
}

func (e *recordingWelcomeGrantExecutor) ExecuteWelcomeGrant(_ context.Context, input WelcomeGrantExecution) (WelcomeGrantResult, error) {
	e.calls++
	e.input = input
	return e.result, e.err
}
