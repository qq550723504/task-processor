package ecoservices

import (
	"context"
	"errors"
	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
	"testing"
)

func TestQualificationWorkflowPreservesOriginalApplicationAndDoesNotActivateMerchant(t *testing.T) {
	r, _ := fixture(t)
	assertQualificationWorkflow(t, r)
}

func assertQualificationWorkflow(t *testing.T, r *Repository) {
	t.Helper()
	s, err := e.NewQualificationService(r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rejected, correction, _ := correctedApplicationCommand(t, r, s)
	result, err := s.Mutate(ctx, correction)
	if err != nil || result.Application.ID != rejected.ID {
		t.Fatal("correction failed", err)
	}
	app := result.Application
	result, err = s.Mutate(ctx, e.Command{Scope: e.Scope{Platform: true, ActorID: "reviewer"}, Key: uuid.NewString(), Kind: "application_review", ID: app.ID, Version: app.Version, Reason: "reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	app = result.Application
	result, err = s.Mutate(ctx, e.Command{Scope: correction.Scope, Key: uuid.NewString(), Kind: "agreement_accept", ID: app.ID, Version: app.Version, AgreementVersion: e.PolicyVersion})
	if err != nil || result.Application.State != "APPROVED" || !result.Application.AgreementAccepted || result.Application.MerchantID != "" || result.Application.OnboardingState != "NOT_STARTED" {
		t.Fatalf("agreement invented merchant activation: %+v %v", result.Application, err)
	}
	if err := r.VerifyQualificationRuntime(ctx); err != nil {
		t.Fatal("retained qualification rejected", err)
	}
	if _, err := s.Mutate(ctx, e.Command{Kind: "listing_create"}); !errors.Is(err, e.ErrUnavailable) {
		t.Fatal("premerchant draft admitted", err)
	}
}

func TestQualificationRuntimeRejectsRetainedFinancialAndMerchantFacts(t *testing.T) {
	for _, model := range []any{
		&listingRow{ID: uuid.NewString(), ProviderOrganizationID: "provider"},
		&requestRow{ID: uuid.NewString(), BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider"},
		&financialRow{ID: uuid.NewString(), RequestID: "request", OrderID: "order"},
		&merchantIntentRow{ID: uuid.NewString()},
		&merchantProgressRow{ID: uuid.NewString()},
		&merchantBindingRow{ApplicationID: uuid.NewString(), OrganizationID: "provider", MerchantID: "original", OriginalAttemptID: "attempt"},
		applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "ACTIVE", OnboardingState: "FINISH"}),
		applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "APPROVED", MerchantID: "original", OnboardingState: "NOT_STARTED"}),
		applicationRecord(e.Application{ID: uuid.NewString(), OrganizationID: "provider", State: "APPROVED", OnboardingState: "PREPARING"}),
		&fileRow{ID: uuid.NewString(), ParentKind: "REQUEST"},
		&operationRow{OrganizationID: "provider", Kind: "confirm_quote", Key: uuid.NewString()},
		&versionRow{ID: uuid.NewString(), Kind: "REQUEST", Version: 1},
	} {
		r, _ := fixture(t)
		if err := r.db.Create(model).Error; err != nil {
			t.Fatal(err)
		}
		if err := r.VerifyQualificationRuntime(context.Background()); err == nil {
			t.Fatalf("retained %T admitted without original channel", model)
		}
	}
	r, _ := fixture(t)
	if err := r.db.Migrator().DropTable(&financialRow{}); err != nil {
		t.Fatal(err)
	}
	if r.VerifyQualificationRuntime(context.Background()) == nil {
		t.Fatal("failed state read allowed startup")
	}
}
