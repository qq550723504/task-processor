package ecoservices

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
	"testing"
)

func merchantApplication(t *testing.T, r *Repository, id string) e.Application {
	t.Helper()
	var row applicationRow
	if err := r.db.Where("id=?", id).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	app, err := applicationFact(row)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestMerchantRejectedCorrectionUsesOriginalNumber(t *testing.T) {
	r, s, p, _, scope, in, d := merchantServiceFixture(t)
	ctx := context.Background()
	p.state = "REJECTED"
	if _, err := s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, in.ApplicationVersion, d); err != nil {
		t.Fatal(err)
	}
	original, err := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	app := merchantApplication(t, r, in.ApplicationID)
	d.ExpectedRevisionVersion = 1
	d.AccountNumber = "corrected-private-bank"
	p.acknowledge = true
	p.state = "FINISH"
	view, err := s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, app.Version, d)
	if err != nil || view.State != "FINISH" {
		t.Fatal("rejected original cannot be corrected", err)
	}
	after, err := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	if p.submits != 2 || p.original != original.Intent.OutRequestNo || after.Intent.Fingerprint != original.Intent.Fingerprint {
		t.Fatal("correction replaced immutable original")
	}
	if after.Revision.Version != 2 || after.Revision.ID == original.Revision.ID || after.Acceptance == nil || after.Acceptance.ChannelApplicationID != "1001" || merchantApplication(t, r, in.ApplicationID).State != "ACTIVE" {
		t.Fatal("current accepted correction not qualified")
	}
	var versions int64
	if err = r.db.Model(&versionRow{}).Where("id=? AND kind=?", after.Intent.ID, "MERCHANT_DETAILS").Count(&versions).Error; err != nil || versions != 2 {
		t.Fatal("immutable versions lost", versions, err)
	}
	var root merchantIntentRow
	r.db.Where("id=?", original.Intent.ID).Take(&root)
	var preserved e.MerchantIntent
	if json.Unmarshal(root.Payload, &preserved) != nil || !bytes.Equal(preserved.SealedDetails, original.Intent.SealedDetails) || preserved.Key != original.Intent.Key {
		t.Fatal("original details overwritten")
	}
}

type lostMerchantAcceptance struct{ e.MerchantRepository }

func (r lostMerchantAcceptance) SaveMerchantAcceptance(context.Context, e.MerchantAttempt, e.MerchantSubmissionAcceptance, []byte) error {
	return e.ErrUnavailable
}

func TestMerchantCorrectionLostAcceptanceCannotConsumeOldQueryOrRedispatch(t *testing.T) {
	for _, saveLost := range []bool{false, true} {
		t.Run(map[bool]string{false: "response-lost", true: "durable-save-lost"}[saveLost], func(t *testing.T) {
			r, s, p, _, scope, in, d := merchantServiceFixture(t)
			ctx := context.Background()
			p.state = "REJECTED"
			if _, err := s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, in.ApplicationVersion, d); err != nil {
				t.Fatal(err)
			}
			d.ExpectedRevisionVersion = 1
			d.AccountNumber = "corrected-private-bank"
			app := merchantApplication(t, r, in.ApplicationID)
			if saveLost {
				p.acknowledge = true
				var err error
				s, err = e.NewMerchantOnboarding(lostMerchantAcceptance{r}, p, merchantCorrectionFiles(t, r), merchantProtection(t), &merchantAuthorizerFixture{})
				if err != nil {
					t.Fatal(err)
				}
				// Pre-upload the bounded fixture media; no external images are fetched.
				p.unknown = true
			}
			key := uuid.NewString()
			view, err := s.Submit(ctx, scope, key, in.ApplicationID, app.Version, d)
			if saveLost && !errors.Is(err, e.ErrUnavailable) || !saveLost && (err != nil || !view.VerificationPending) {
				t.Fatal("lost acknowledgement fabricated correlation", err, view)
			}
			after, err := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
			if err != nil {
				t.Fatal(err)
			}
			if !after.Dispatched || after.Acceptance != nil || len(after.SealedObservation) > 0 {
				t.Fatal("lost ack state not retained")
			}
			// Even a later signed FINISH or old REJECTED cannot substitute for the
			// missing current-version submission acceptance, including after restart.
			p.unknown = false
			p.state = "FINISH"
			queries := p.queries
			restarted, err := e.NewMerchantOnboarding(r, p, sFiles(t, r), merchantProtection(t), &merchantAuthorizerFixture{})
			if err != nil {
				t.Fatal(err)
			}
			view, err = restarted.Read(ctx, scope, in.ApplicationID)
			if err != nil || !view.VerificationPending || view.CanCorrect || view.SignURL != "" || p.queries != queries || merchantApplication(t, r, in.ApplicationID).State == "ACTIVE" {
				t.Fatal("unacknowledged correction consumed old query", err, view)
			}
			if _, err = restarted.Resume(ctx, scope, in.ApplicationID, merchantApplication(t, r, in.ApplicationID).Version); err != nil || p.submits != 2 {
				t.Fatal("unknown was resubmitted", err)
			}
			d.ExpectedRevisionVersion = 2
			d.AccountNumber = "third-bank"
			if _, err = restarted.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, merchantApplication(t, r, in.ApplicationID).Version, d); !errors.Is(err, e.ErrConflict) {
				t.Fatal("unknown allowed changed payload", err)
			}
		})
	}
}

func TestMerchantCorrectionCurrentVersionFencesOldWorkersAndProofs(t *testing.T) {
	r, s, p, _, scope, in, d := merchantServiceFixture(t)
	ctx := context.Background()
	p.state = "REJECTED"
	firstKey := uuid.NewString()
	if _, err := s.Submit(ctx, scope, firstKey, in.ApplicationID, in.ApplicationVersion, d); err != nil {
		t.Fatal(err)
	}
	original, claimed, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	originalDetails := d
	app := merchantApplication(t, r, in.ApplicationID)
	d.ExpectedRevisionVersion = 1
	d.ContactMobile = "13900139000"
	key := uuid.NewString()
	if _, err = s.Submit(ctx, scope, key, in.ApplicationID, app.Version, d); !errors.Is(err, e.ErrConflict) {
		t.Fatal("live previous worker not fenced", err)
	}
	if err = r.ReleaseMerchantClaim(ctx, original); err != nil {
		t.Fatal(err)
	}
	p.acknowledge = true
	p.state = "REJECTED"
	if _, err = s.Submit(ctx, scope, key, in.ApplicationID, app.Version, d); err != nil {
		t.Fatal(err)
	}
	current, claimed, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	for _, err = range []error{r.SaveMerchantMedia(ctx, original, in.FileIDs[0], "late"), r.MarkMerchantDispatched(ctx, original), r.ObserveMerchant(ctx, original, merchantObservation(original, "FINISH", "SIGNED", "1900000011"), []byte("late-proof")), r.ReleaseMerchantClaim(ctx, original)} {
		if !errors.Is(err, e.ErrConflict) {
			t.Fatal("old version wrote current facts", err)
		}
	}
	proof := *current.Acceptance
	proof.DetailsFingerprint = "wrong"
	if err = r.SaveMerchantAcceptance(ctx, current, proof, []byte("wrong")); !errors.Is(err, e.ErrConflict) {
		t.Fatal("wrong acceptance consumed", err)
	}
	o := merchantObservation(current, "FINISH", "SIGNED", "1900000011")
	if err = r.ObserveMerchant(ctx, current, o, []byte("old-query")); !errors.Is(err, e.ErrConflict) {
		t.Fatal("query without current durable association consumed", err)
	}
	o = current.BindQuery(o)
	o.ChannelApplicationID = "foreign"
	if err = r.ObserveMerchant(ctx, current, o, []byte("wrong-id")); !errors.Is(err, e.ErrConflict) {
		t.Fatal("foreign applyment bound", err)
	}
	if err = r.ReleaseMerchantClaim(ctx, current); err != nil {
		t.Fatal(err)
	}
	submits := p.submits
	if _, err = s.Submit(ctx, scope, firstKey, in.ApplicationID, in.ApplicationVersion, originalDetails); err != nil || p.submits != submits {
		t.Fatal("old exact receipt did not replay safely", err)
	}
	if _, err = s.Submit(ctx, scope, key, in.ApplicationID, app.Version, d); err != nil || p.submits != submits {
		t.Fatal("same correction repeated", err)
	}
	changed := d
	changed.AccountBank = "changed"
	if _, err = s.Submit(ctx, scope, key, in.ApplicationID, app.Version, changed); !errors.Is(err, e.ErrConflict) {
		t.Fatal("same key changed details", err)
	}
	// A current acknowledged terminal rejection permits the next revision;
	// applyment ID is deliberately unchanged by the fixture.
	d.ExpectedRevisionVersion = 2
	d.AccountBank = "corrected-bank"
	p.state = "FINISH"
	if _, err = s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, merchantApplication(t, r, in.ApplicationID).Version, d); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantCorrectionLicenseRequiresPlatformReviewAndAgreement(t *testing.T) {
	r, s, p, _, scope, in, d := merchantServiceFixture(t)
	ctx := context.Background()
	p.state = "REJECTED"
	if _, err := s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, in.ApplicationVersion, d); err != nil {
		t.Fatal(err)
	}
	d.ExpectedRevisionVersion = 1
	d.LicenseFileID = in.FileIDs[1]
	view, err := s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, merchantApplication(t, r, in.ApplicationID).Version, d)
	if err != nil || view.State != "PREPARING" || p.submits != 1 {
		t.Fatal("unreviewed license dispatched", err)
	}
	app := merchantApplication(t, r, in.ApplicationID)
	if app.State != "SUBMITTED" || app.AgreementAccepted || app.FileIDs[0] != d.LicenseFileID {
		t.Fatal("license escaped renewed platform review")
	}
	if _, err = s.Resume(ctx, scope, app.ID, app.Version); !errors.Is(err, e.ErrNotQualified) {
		t.Fatal("unsigned correction dispatched", err)
	}
	svc, _ := e.NewService(r, nil, 180)
	result, err := svc.Mutate(ctx, e.Command{Scope: e.Scope{Platform: true, ActorID: "reviewer"}, Kind: "application_review", Key: uuid.NewString(), ID: app.ID, Version: app.Version, Reason: "corrected license checked"})
	if err != nil {
		t.Fatal(err)
	}
	result, err = svc.Mutate(ctx, e.Command{Scope: scope, Kind: "agreement_accept", Key: uuid.NewString(), ID: app.ID, Version: result.Application.Version, AgreementVersion: e.PolicyVersion})
	if err != nil {
		t.Fatal(err)
	}
	p.acknowledge = true
	p.state = "FINISH"
	view, err = s.Resume(ctx, scope, app.ID, result.Application.Version)
	if err != nil || view.State != "FINISH" || p.submits != 2 {
		t.Fatal("reviewed correction cannot continue", err)
	}
}

func merchantCorrectionFiles(t *testing.T, r *Repository) *e.FileService {
	t.Helper()
	var rows []fileRow
	if err := r.db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	objects := &objectFixture{files: map[string][]byte{}, metadata: map[string]e.File{}}
	for _, f := range rows {
		objects.files[f.ObjectKey] = []byte("immutable-original-image-" + f.ID)
	}
	files, err := e.NewFileService(r, objects)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

type merchantReadBeforePointerChange struct {
	e.MerchantRepository
	old e.MerchantAttempt
}

func (r merchantReadBeforePointerChange) ReadMerchantAttempt(context.Context, e.Scope, string) (e.MerchantAttempt, error) {
	return r.old, nil
}

func TestMerchantReadUsesCurrentBusyClaimAfterRevisionSwitch(t *testing.T) {
	r, s, p, _, scope, in, d := merchantServiceFixture(t)
	ctx := context.Background()
	p.state = "REJECTED"
	p.signURL = "https://pay.weixin.qq.com/public/old-sign"
	if _, err := s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, in.ApplicationVersion, d); err != nil {
		t.Fatal(err)
	}
	old, err := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	d.ExpectedRevisionVersion = 1
	d.AccountNumber = "corrected-bank"
	if _, err = s.Submit(ctx, scope, uuid.NewString(), in.ApplicationID, merchantApplication(t, r, in.ApplicationID).Version, d); err != nil {
		t.Fatal(err)
	}
	current, claimed, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	deferredRead, err := e.NewMerchantOnboarding(merchantReadBeforePointerChange{r, old}, p, sFiles(t, r), merchantProtection(t), &merchantAuthorizerFixture{})
	if err != nil {
		t.Fatal(err)
	}
	queries := p.queries
	view, err := deferredRead.Read(ctx, scope, in.ApplicationID)
	if err != nil || view.RevisionVersion != current.Revision.Version || !view.VerificationPending || view.SignURL != "" || view.CanCorrect || p.queries != queries {
		t.Fatal("busy current revision returned old consent/status", view, err)
	}
	view, err = s.Resume(ctx, scope, in.ApplicationID, merchantApplication(t, r, in.ApplicationID).Version)
	if err != nil || !view.VerificationPending || view.RevisionVersion != current.Revision.Version || view.SignURL != "" {
		t.Fatal("busy unacknowledged correction fabricated a verified view", view, err)
	}
}
