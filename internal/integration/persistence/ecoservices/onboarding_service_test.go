package ecoservices

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/servicepayments"
	"testing"
)

type merchantChannelFixture struct {
	signURL                   string
	profile                   e.MerchantProfile
	state                     string
	unknown                   bool
	acknowledge               bool
	submits, uploads, queries int
	original                  string
}

func (p *merchantChannelFixture) MerchantProfile() e.MerchantProfile   { return p.profile }
func (p *merchantChannelFixture) NewMerchantApplicationsEnabled() bool { return true }
func (p *merchantChannelFixture) UploadMerchantImage(_ context.Context, f e.File, _ []byte) (string, error) {
	p.uploads++
	return "media-" + f.ID, nil
}
func (p *merchantChannelFixture) SubmitMerchant(_ context.Context, a e.MerchantAttempt, _ e.MerchantDetails) (e.MerchantSubmissionAcceptance, error) {
	p.submits++
	p.original = a.Intent.OutRequestNo
	if p.acknowledge {
		return e.MerchantSubmissionAcceptance{RevisionID: a.Revision.ID, RevisionVersion: a.Revision.Version, DetailsFingerprint: a.Revision.Input.Fingerprint, Profile: a.Intent.Profile, OutRequestNo: a.Intent.OutRequestNo, ChannelApplicationID: "1001", VerificationVersion: "verified-submission-fixture", SignedResponseDigest: strings.Repeat("a", 64)}, nil
	}
	return e.MerchantSubmissionAcceptance{}, e.ErrUnavailable
}
func (p *merchantChannelFixture) QueryMerchant(_ context.Context, a e.MerchantAttempt) (e.MerchantObservation, error) {
	p.queries++
	if p.unknown {
		return e.MerchantObservation{}, e.ErrUnavailable
	}
	return e.MerchantObservation{Profile: p.profile, OutRequestNo: a.Intent.OutRequestNo, ChannelApplicationID: "1001", State: p.state, SignState: "SIGNED", MerchantID: "1900000011", SignURL: p.signURL, VerificationVersion: "verified-original-fixture"}, nil
}

type merchantAuthorizerFixture struct{ calls, revokeAt int }

func (a *merchantAuthorizerFixture) AuthorizeMerchantApplication(context.Context, string, string) error {
	a.calls++
	if a.revokeAt > 0 && a.calls >= a.revokeAt {
		return e.ErrForbidden
	}
	return nil
}
func merchantServiceFixture(t *testing.T) (*Repository, *e.MerchantOnboarding, *merchantChannelFixture, *merchantAuthorizerFixture, e.Scope, e.MerchantIntent, e.MerchantDetails) {
	t.Helper()
	r, _ := fixture(t)
	scope, in := merchantFixture(t, r, "provider")
	objects := &objectFixture{files: map[string][]byte{}, metadata: map[string]e.File{}}
	for _, id := range in.FileIDs {
		data := []byte("immutable-original-image-" + id)
		key := "original-private/" + id
		objects.files[key] = data
		r.db.Model(&fileRow{}).Where("id=?", id).Updates(map[string]any{"size_bytes": len(data), "sha256": e.FileDigest(data), "object_key": key, "filename": id + ".png"})
	}
	files, err := e.NewFileService(r, objects)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := servicepayments.NewMerchantPayloadProtection(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	channel := &merchantChannelFixture{profile: in.Profile, state: "NEED_SIGN"}
	authorizer := &merchantAuthorizerFixture{}
	s, err := e.NewMerchantOnboarding(r, channel, files, protection, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	d := e.MerchantDetails{LicenseFileID: in.FileIDs[0], Legal: e.IdentityDocument{Type: "IDENTIFICATION_TYPE_MAINLAND_IDCARD", Name: "private-legal-name", Number: "private-identity-number", FrontFileID: in.FileIDs[1], BackFileID: in.FileIDs[2], ValidFrom: "2020-01-01", ValidUntil: "长期"}, SoleLegalBeneficiary: true, ContactMobile: "13800138000", AccountBank: "工商银行", AccountNumber: "private-bank-number", MerchantShortName: "原企业服务", StoreName: "原企业网站", StoreURL: "https://provider.example/"}
	return r, s, channel, authorizer, scope, in, d
}
func TestMerchantLostReplyRecoversOriginalAndPrivatePayloadSurvivesRestart(t *testing.T) {
	r, s, p, _, scope, in, d := merchantServiceFixture(t)
	ctx := context.Background()
	key := uuid.NewString()
	p.unknown = true
	if _, err := s.Submit(ctx, scope, key, in.ApplicationID, in.ApplicationVersion, d); !errors.Is(err, e.ErrUnavailable) {
		t.Fatal("unknown fabricated success", err)
	}
	if _, err := s.Submit(ctx, scope, key, in.ApplicationID, in.ApplicationVersion, d); !errors.Is(err, e.ErrUnavailable) || p.submits != 1 {
		t.Fatal("unknown repeated external application", err)
	}
	a, err := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{d.Legal.Name, d.Legal.Number, d.AccountNumber, d.ContactMobile} {
		if bytes.Contains(a.Intent.SealedDetails, []byte(secret)) {
			t.Fatal("PII persisted as plaintext")
		}
	}
	if p.original != a.Intent.OutRequestNo || p.uploads != 3 {
		t.Fatal("original media/application changed")
	}
	p.unknown = false
	p.state = "FINISH"
	// A new instance with the original key recovers by query; no human checkout
	// admission and no new submission is required to observe the existing fact.
	restarted, err := e.NewMerchantOnboarding(r, p, sFiles(t, r), merchantProtection(t), &merchantAuthorizerFixture{revokeAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	view, err := restarted.Read(ctx, scope, in.ApplicationID)
	if err != nil || view.State != "FINISH" || p.submits != 1 {
		t.Fatal("restart lost original query recovery", err)
	}
	var row applicationRow
	r.db.Where("id=?", in.ApplicationID).Take(&row)
	if row.State != "ACTIVE" {
		t.Fatal("verified finish not projected")
	}
}
func sFiles(t *testing.T, r *Repository) *e.FileService {
	t.Helper()
	s, err := e.NewFileService(r, &objectFixture{files: map[string][]byte{}, metadata: map[string]e.File{}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func merchantProtection(t *testing.T) e.MerchantProtection {
	t.Helper()
	p, err := servicepayments.NewMerchantPayloadProtection(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestMerchantPermissionRevokedAfterMediaStopsExternalApplication(t *testing.T) {
	r, s, p, a, scope, in, d := merchantServiceFixture(t)
	a.revokeAt = 2
	if _, err := s.Submit(context.Background(), scope, uuid.NewString(), in.ApplicationID, in.ApplicationVersion, d); !errors.Is(err, e.ErrForbidden) || p.submits != 0 || p.queries != 0 {
		t.Fatal("revoked actor dispatched new channel application", err)
	}
	attempt, err := r.ReadMerchantAttempt(context.Background(), scope, in.ApplicationID)
	if err != nil || attempt.Dispatched {
		t.Fatal("revocation fabricated dispatch", err)
	}
}
func TestMerchantHumanCanResumeSealedOriginalAfterPageReload(t *testing.T) {
	r, s, p, a, scope, in, d := merchantServiceFixture(t)
	a.revokeAt = 2
	ctx := context.Background()
	key := uuid.NewString()
	if _, err := s.Submit(ctx, scope, key, in.ApplicationID, in.ApplicationVersion, d); !errors.Is(err, e.ErrForbidden) {
		t.Fatal(err)
	}
	original, _ := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	p.state = "FINISH"
	resumed, err := e.NewMerchantOnboarding(r, p, sFiles(t, r), merchantProtection(t), &merchantAuthorizerFixture{})
	if err != nil {
		t.Fatal(err)
	}
	scope.ActorID = "new-authorized-operator"
	view, err := resumed.Resume(ctx, scope, in.ApplicationID, merchantApplication(t, r, in.ApplicationID).Version)
	if err != nil || view.State != "FINISH" || p.submits != 1 || p.original != original.Intent.OutRequestNo || p.uploads != 3 {
		t.Fatal("reload cannot continue encrypted original without new intent", err)
	}
	after, _ := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	if after.Intent.ID != original.Intent.ID || after.Intent.Key != key || after.Intent.ActorID != original.Intent.ActorID {
		t.Fatal("resume changed immutable original")
	}
}
