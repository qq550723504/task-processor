package ecoservices

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
)

func correctedApplicationCommand(t *testing.T, r *Repository, s *e.Service) (e.Application, e.Command, string) {
	t.Helper()
	ctx := context.Background()
	oldFile, replacement := uuid.NewString(), uuid.NewString()
	for _, id := range []string{oldFile, replacement} {
		if err := r.db.Create(&fileRow{ID: id, OrganizationID: "provider", ParentKind: "APPLICATION", State: "CONFIRMED", ContentType: "image/png", SizeBytes: 68}).Error; err != nil {
			t.Fatal(err)
		}
	}
	scope := e.Scope{OrganizationID: "provider", ActorID: "provider-user"}
	first, err := s.Mutate(ctx, e.Command{Scope: scope, Key: uuid.NewString(), Kind: "application_submit", Application: &e.Application{CompanyName: "original company", RegistrationNumber: "original-registration", Categories: []e.Category{e.CompanyRegistration}, Regions: []string{"China"}, FileIDs: []string{oldFile}}})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := s.Mutate(ctx, e.Command{Scope: e.Scope{Platform: true, ActorID: "reviewer"}, Key: uuid.NewString(), Kind: "application_reject", ID: first.Application.ID, Version: first.Application.Version, Reason: "replace unclear license"})
	if err != nil {
		t.Fatal(err)
	}
	c := e.Command{Scope: scope, Key: uuid.NewString(), Kind: "application_submit", Version: rejected.Application.Version, Application: &e.Application{CompanyName: "corrected company", RegistrationNumber: "corrected-registration", Categories: []e.Category{e.CompanyRegistration}, Regions: []string{"China"}, FileIDs: []string{replacement}}}
	return *rejected.Application, c, oldFile
}

func TestRejectedApplicationCorrectionKeepsOriginalAndRequiresNewReview(t *testing.T) {
	r, s := fixture(t)
	rejected, c, oldFile := correctedApplicationCommand(t, r, s)
	ctx := context.Background()
	result, err := s.Mutate(ctx, c)
	if err != nil {
		t.Fatal("rejected provider cannot submit corrected evidence", err)
	}
	a := result.Application
	if a.ID != rejected.ID || a.Version != rejected.Version+1 || a.State != "SUBMITTED" || a.CompanyName != "corrected company" || a.AgreementAccepted || a.MerchantID != "" || a.OnboardingState != "NOT_STARTED" || a.ReviewReason != "" {
		t.Fatalf("correction inherited authority or changed original identity: %+v", a)
	}
	var history versionRow
	if err := r.db.Where("id=? AND kind='APPLICATION' AND version=?", rejected.ID, rejected.Version).Take(&history).Error; err != nil {
		t.Fatal(err)
	}
	var old e.Application
	if err := json.Unmarshal(history.Payload, &old); err != nil || old.State != "REJECTED" || old.ReviewReason != rejected.ReviewReason || len(old.FileIDs) != 1 || old.FileIDs[0] != oldFile {
		t.Fatal("original reviewed evidence or reason was replaced", err)
	}
	if file, err := r.ReadFile(ctx, c.Scope, oldFile); err != nil || file.ParentID != rejected.ID {
		t.Fatal("historical original file was detached", err)
	}
	again, err := s.Mutate(ctx, c)
	if err != nil || again.Application.ID != a.ID || again.Application.Version != a.Version {
		t.Fatal("correction replay created another version", err)
	}
	changed := c
	changed.Application = &e.Application{}
	*changed.Application = *c.Application
	changed.Application.CompanyName = "different correction"
	if _, err := s.Mutate(ctx, changed); !errors.Is(err, e.ErrConflict) {
		t.Fatal("same-key correction changed the original payload", err)
	}
	if _, err := s.Mutate(ctx, e.Command{Scope: e.Scope{Platform: true, ActorID: "reviewer"}, Key: uuid.NewString(), Kind: "application_review", ID: a.ID, Version: rejected.Version, Reason: "stale approval"}); !errors.Is(err, e.ErrConflict) {
		t.Fatal("stale reviewed version approved the correction", err)
	}
	approved, err := s.Mutate(ctx, e.Command{Scope: e.Scope{Platform: true, ActorID: "reviewer"}, Key: uuid.NewString(), Kind: "application_review", ID: a.ID, Version: a.Version, Reason: "reviewed corrected evidence"})
	if err != nil || approved.Application.State != "APPROVED" || approved.Application.AgreementAccepted {
		t.Fatal("corrected version cannot reach a fresh approval", err)
	}
}

func TestApplicationCorrectionRejectsMissingStaleAndForeignVersion(t *testing.T) {
	for _, variant := range []string{"missing", "stale", "foreign"} {
		t.Run(variant, func(t *testing.T) {
			r, s := fixture(t)
			_, c, _ := correctedApplicationCommand(t, r, s)
			switch variant {
			case "missing":
				c.Version = 0
			case "stale":
				c.Version--
			case "foreign":
				c.Scope.OrganizationID = "stranger"
			}
			if _, err := s.Mutate(context.Background(), c); !errors.Is(err, e.ErrConflict) {
				t.Fatalf("%s correction was accepted: %v", variant, err)
			}
		})
	}
}

func TestApplicationCorrectionCannotReplaceApprovedOrChannelApplication(t *testing.T) {
	for _, state := range []string{"SUBMITTED", "APPROVED", "ACTIVE", "CHANNEL_STARTED"} {
		t.Run(state, func(t *testing.T) {
			r, s := fixture(t)
			rejected, c, _ := correctedApplicationCommand(t, r, s)
			if state == "CHANNEL_STARTED" {
				rejected.OnboardingState = "AUDITING"
			} else {
				rejected.State = state
			}
			if err := r.db.Save(applicationRecord(rejected)).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.Mutate(context.Background(), c); !errors.Is(err, e.ErrConflict) {
				t.Fatal("correction replaced an ineligible existing application", err)
			}
		})
	}
}
