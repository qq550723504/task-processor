package ecoservices

import (
	"context"
	"errors"
	"github.com/google/uuid"
	e "task-processor/internal/ecoservices"
	"testing"
	"time"
)

func merchantFixture(t *testing.T, r *Repository, org string) (e.Scope, e.MerchantIntent) {
	t.Helper()
	scope := e.Scope{OrganizationID: org, ActorID: org + "-actor"}
	app := e.Application{ID: uuid.NewString(), OrganizationID: org, CompanyName: "企业 " + org, RegistrationNumber: "approved-registration", State: "APPROVED", Version: 3, AgreementAccepted: true, AgreementVersion: e.PolicyVersion, OnboardingState: "NOT_STARTED", UpdatedAt: time.Now().UTC()}
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	app.FileIDs = ids
	if err := r.db.Create(applicationRecord(app)).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if err := r.db.Create(&fileRow{ID: id, OrganizationID: org, ParentKind: "APPLICATION", ParentID: app.ID, State: "CONFIRMED", ContentType: "image/png", SizeBytes: 123}).Error; err != nil {
			t.Fatal(err)
		}
	}
	id := uuid.NewString()
	return scope, e.MerchantIntent{ID: id, Key: uuid.NewString(), ApplicationID: app.ID, ApplicationVersion: app.Version, OrganizationID: org, ActorID: scope.ActorID, OutRequestNo: "original" + org, Profile: e.MerchantProfile{Version: "original-profile", PlatformMerchantID: "1900000000"}, SealedDetails: []byte("sealed-private-original"), Fingerprint: "original-fingerprint", FileIDs: ids, LicenseFileID: ids[0]}
}
func merchantObservation(a e.MerchantAttempt, state, sign, merchant string) e.MerchantObservation {
	return e.MerchantObservation{Profile: a.Intent.Profile, OutRequestNo: a.Intent.OutRequestNo, ChannelApplicationID: "1001", State: state, SignState: sign, MerchantID: merchant, VerificationVersion: "verified-channel-query"}
}
func TestMerchantOriginalIntentRequiresApprovedAgreementAndExactFiles(t *testing.T) {
	r, _ := fixture(t)
	ctx := context.Background()
	scope, in := merchantFixture(t, r, "provider")
	a, err := r.CreateMerchantAttempt(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := r.CreateMerchantAttempt(ctx, in)
	if err != nil || replay.Intent.ID != a.Intent.ID {
		t.Fatal("original replay lost", err)
	}
	altered := in
	altered.Fingerprint = "changed"
	if _, err := r.CreateMerchantAttempt(ctx, altered); !errors.Is(err, e.ErrConflict) {
		t.Fatal("changed original accepted", err)
	}
	if _, err := r.ReadMerchantAttempt(ctx, e.Scope{OrganizationID: "foreign", ActorID: "foreign"}, in.ApplicationID); !errors.Is(err, e.ErrNotFound) {
		t.Fatal("foreign original exposed", err)
	}
	next := in
	next.ID = uuid.NewString()
	next.Key = uuid.NewString()
	if _, err := r.CreateMerchantAttempt(ctx, next); !errors.Is(err, e.ErrConflict) {
		t.Fatal("second original accepted", err)
	}
	if _, _, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID); err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []string{"unapproved", "unsigned-agreement", "stale-version", "foreign-file", "unreviewed-license"} {
		t.Run(wrong, func(t *testing.T) {
			r, _ := fixture(t)
			_, in := merchantFixture(t, r, wrong)
			var row applicationRow
			r.db.Where("id=?", in.ApplicationID).Take(&row)
			app, _ := applicationFact(row)
			switch wrong {
			case "unapproved":
				app.State = "SUBMITTED"
			case "unsigned-agreement":
				app.AgreementAccepted = false
			case "stale-version":
				in.ApplicationVersion--
			case "foreign-file":
				r.db.Model(&fileRow{}).Where("id=?", in.FileIDs[1]).Update("organization_id", "foreign")
			case "unreviewed-license":
				app.FileIDs = app.FileIDs[1:]
			}
			r.db.Save(applicationRecord(app))
			if _, err := r.CreateMerchantAttempt(ctx, in); err == nil {
				t.Fatal("invalid channel admission accepted")
			}
		})
	}
}
func TestMerchantOnlyExactFinishSignedBindsAndFrozenPreservesOriginal(t *testing.T) {
	r, _ := fixture(t)
	ctx := context.Background()
	scope, in := merchantFixture(t, r, "provider")
	a, err := r.CreateMerchantAttempt(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	a, ok, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	for _, id := range in.FileIDs {
		if err = r.SaveMerchantMedia(ctx, a, id, "media-"+id); err != nil {
			t.Fatal(err)
		}
	}
	a.DispatchActorID = scope.ActorID
	if err = r.MarkMerchantDispatched(ctx, a); err != nil {
		t.Fatal(err)
	}
	a.Dispatched = true
	needsign := merchantObservation(a, "NEED_SIGN", "UNSIGNED", "1900000011")
	if err = r.ObserveMerchant(ctx, a, needsign, []byte("sealed-needsign")); err != nil {
		t.Fatal(err)
	}
	var row applicationRow
	r.db.Where("id=?", in.ApplicationID).Take(&row)
	if row.State == "ACTIVE" || row.MerchantID != "" {
		t.Fatal("NEED_SIGN activated provider")
	}
	unsigned := merchantObservation(a, "FINISH", "UNSIGNED", "1900000011")
	if err = r.ObserveMerchant(ctx, a, unsigned, []byte("sealed-unsigned")); !errors.Is(err, e.ErrConflict) {
		t.Fatal("unsigned finish accepted", err)
	}
	foreign := merchantObservation(a, "FINISH", "SIGNED", "1900000011")
	foreign.OutRequestNo = "foreign"
	if err = r.ObserveMerchant(ctx, a, foreign, []byte("sealed-foreign")); !errors.Is(err, e.ErrConflict) {
		t.Fatal("foreign application accepted", err)
	}
	finish := merchantObservation(a, "FINISH", "SIGNED", "1900000011")
	if err = r.ObserveMerchant(ctx, a, finish, []byte("sealed-finish")); err != nil {
		t.Fatal(err)
	}
	r.db.Where("id=?", in.ApplicationID).Take(&row)
	if row.State != "ACTIVE" || row.MerchantID != finish.MerchantID {
		t.Fatal("verified finish did not qualify")
	}
	frozen := merchantObservation(a, "FROZEN", "SIGNED", "1900000011")
	if err = r.ObserveMerchant(ctx, a, frozen, []byte("sealed-frozen")); err != nil {
		t.Fatal(err)
	}
	r.db.Where("id=?", in.ApplicationID).Take(&row)
	if row.State == "ACTIVE" || row.MerchantID != finish.MerchantID {
		t.Fatal("frozen failed to block new service or deleted original funds binding")
	}
	// Another organization cannot claim the same recipient, even with FINISH.
	s2, in2 := merchantFixture(t, r, "other")
	a2, err := r.CreateMerchantAttempt(ctx, in2)
	if err != nil {
		t.Fatal(err)
	}
	a2, _, _ = r.ClaimMerchantAttempt(ctx, s2, in2.ApplicationID)
	for _, id := range in2.FileIDs {
		if err = r.SaveMerchantMedia(ctx, a2, id, "media-"+id); err != nil {
			t.Fatal(err)
		}
	}
	a2.DispatchActorID = s2.ActorID
	if err = r.MarkMerchantDispatched(ctx, a2); err != nil {
		t.Fatal(err)
	}
	a2.Dispatched = true
	o2 := merchantObservation(a2, "FINISH", "SIGNED", finish.MerchantID)
	if err = r.ObserveMerchant(ctx, a2, o2, []byte("sealed-second")); !errors.Is(err, e.ErrConflict) {
		t.Fatal("recipient assigned to unrelated organization", err)
	}
}

func TestFrozenProviderBlocksNewCatalogRequestsAndCheckoutQualification(t *testing.T) {
	r, service := fixture(t)
	ctx := context.Background()
	provider, in := merchantFixture(t, r, "provider")
	var row applicationRow
	r.db.Where("id=?", in.ApplicationID).Take(&row)
	app, _ := applicationFact(row)
	app.State = "ACTIVE"
	app.MerchantID = "1900000011"
	app.OnboardingState = "FINISH"
	r.db.Save(applicationRecord(app))
	listing := e.Listing{ID: uuid.NewString(), ProviderOrganizationID: provider.OrganizationID, State: "PUBLISHED", Title: "服务", Category: e.StoreOpening, Version: 1}
	r.db.Create(listingRecord(listing))
	buyer := e.Scope{OrganizationID: "buyer", ActorID: "buyer"}
	page, err := service.Read(ctx, e.Query{Scope: buyer, Kind: "catalog", Page: 1, PageSize: 20})
	if err != nil || page.Total != 1 {
		t.Fatal("qualified listing hidden", err)
	}
	app.State = "APPROVED"
	app.OnboardingState = "FROZEN"
	r.db.Save(applicationRecord(app))
	page, err = service.Read(ctx, e.Query{Scope: buyer, Kind: "catalog", Page: 1, PageSize: 20})
	if err != nil || page.Total != 0 {
		t.Fatal("frozen provider still orderable in catalog", err)
	}
	if _, err := service.Mutate(ctx, e.Command{Scope: buyer, Key: uuid.NewString(), Kind: "request_create", ID: listing.ID, Description: "new request"}); !errors.Is(err, e.ErrNotQualified) {
		t.Fatal("frozen provider accepted new request", err)
	}
	if err := r.VerifyProviderQualification(ctx, provider.OrganizationID, app.MerchantID); !errors.Is(err, e.ErrNotQualified) {
		t.Fatal("frozen provider allowed new checkout", err)
	}
}
