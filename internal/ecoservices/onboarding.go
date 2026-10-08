package ecoservices

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MerchantDetails is the bounded enterprise application contract, never a
// channel SDK DTO. Company identity comes from the approved application.
type IdentityDocument struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Number      string `json:"number"`
	Address     string `json:"address"`
	FrontFileID string `json:"frontFileId"`
	BackFileID  string `json:"backFileId"`
	ValidFrom   string `json:"validFrom"`
	ValidUntil  string `json:"validUntil"`
}
type MerchantDetails struct {
	LicenseFileID        string             `json:"licenseFileId"`
	Legal                IdentityDocument   `json:"legal"`
	SoleLegalBeneficiary bool               `json:"soleLegalBeneficiary"`
	Beneficiaries        []IdentityDocument `json:"beneficiaries"`
	ContactMobile        string             `json:"contactMobile"`
	AccountBank          string             `json:"accountBank"`
	AccountNumber        string             `json:"accountNumber"`
	BankBranchName       string             `json:"bankBranchName"`
	MerchantShortName    string             `json:"merchantShortName"`
	StoreName            string             `json:"storeName"`
	StoreURL             string             `json:"storeUrl"`
}
type MerchantProfile struct{ Version, PlatformMerchantID string }
type MerchantIntent struct {
	ID, ApplicationID, OrganizationID, ActorID, Key, OutRequestNo, Fingerprint string
	ApplicationVersion                                                         int64
	Profile                                                                    MerchantProfile
	SealedDetails                                                              []byte
	FileIDs                                                                    []string
	LicenseFileID                                                              string
}
type MerchantAttempt struct {
	Intent                          MerchantIntent
	CompanyName, RegistrationNumber string
	State, ClaimToken               string
	DispatchActorID                 string
	ClaimUntil                      time.Time
	Dispatched                      bool
	MediaIDs                        map[string]string
	SealedObservation               []byte
	UpdatedAt                       time.Time
}
type BankValidation struct {
	AccountName       string `json:"accountName"`
	AccountNumber     string `json:"accountNumber"`
	PayAmountMinor    int64  `json:"payAmountMinor,string"`
	DestinationNumber string `json:"destinationNumber"`
	DestinationName   string `json:"destinationName"`
	DestinationBank   string `json:"destinationBank"`
	City              string `json:"city"`
	Remark            string `json:"remark"`
	Deadline          string `json:"deadline"`
}
type MerchantObservation struct {
	Profile                                                          MerchantProfile
	OutRequestNo, ChannelApplicationID, State, SignState, MerchantID string
	SignURL, LegalValidationURL, Reason, VerificationVersion         string
	Bank                                                             *BankValidation
}
type MerchantView struct {
	VerificationPending bool            `json:"verificationPending"`
	ID                  string          `json:"id"`
	ApplicationID       string          `json:"applicationId"`
	State               string          `json:"state"`
	SignState           string          `json:"signState"`
	SignURL             string          `json:"signUrl"`
	LegalValidationURL  string          `json:"legalValidationUrl"`
	Reason              string          `json:"reason"`
	Bank                *BankValidation `json:"bank,omitempty"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}
type MerchantRepository interface {
	CheckMerchantApplicationVersion(context.Context, Scope, string, int64) error
	CreateMerchantAttempt(context.Context, MerchantIntent) (MerchantAttempt, error)
	ReadMerchantAttempt(context.Context, Scope, string) (MerchantAttempt, error)
	ClaimMerchantAttempt(context.Context, Scope, string) (MerchantAttempt, bool, error)
	SaveMerchantMedia(context.Context, MerchantAttempt, string, string) error
	MarkMerchantDispatched(context.Context, MerchantAttempt) error
	ObserveMerchant(context.Context, MerchantAttempt, MerchantObservation, []byte) error
	ReleaseMerchantClaim(context.Context, MerchantAttempt) error
}

func (s *MerchantOnboarding) Resume(ctx context.Context, scope Scope, appID string, version int64) (MerchantView, error) {
	if scope.Platform || !validText(scope.OrganizationID, 128) || !validText(scope.ActorID, 256) || !ValidID(appID) || version < 1 {
		return MerchantView{}, ErrInvalid
	}
	if !s.channel.NewMerchantApplicationsEnabled() {
		return MerchantView{}, ErrUnavailable
	}
	if err := s.authorizer.AuthorizeMerchantApplication(ctx, scope.OrganizationID, scope.ActorID); err != nil {
		return MerchantView{}, ErrForbidden
	}
	if err := s.repo.CheckMerchantApplicationVersion(ctx, scope, appID, version); err != nil {
		return MerchantView{}, err
	}
	return s.continueOriginal(ctx, scope, appID)
}

type MerchantOnboardingPort interface {
	MerchantProfile() MerchantProfile
	NewMerchantApplicationsEnabled() bool
	UploadMerchantImage(context.Context, File, []byte) (string, error)
	SubmitMerchant(context.Context, MerchantAttempt, MerchantDetails) error
	QueryMerchant(context.Context, MerchantAttempt) (MerchantObservation, error)
}
type MerchantProtection interface {
	Seal(string, string) ([]byte, error)
	Open(string, []byte) (string, error)
}
type MerchantAuthorizer interface {
	AuthorizeMerchantApplication(context.Context, string, string) error
}
type MerchantOnboarding struct {
	repo       MerchantRepository
	channel    MerchantOnboardingPort
	files      *FileService
	protection MerchantProtection
	authorizer MerchantAuthorizer
}

func NewMerchantOnboarding(r MerchantRepository, p MerchantOnboardingPort, f *FileService, secrets MerchantProtection, a MerchantAuthorizer) (*MerchantOnboarding, error) {
	if r == nil || p == nil || f == nil || secrets == nil || a == nil || !validText(p.MerchantProfile().Version, 128) || !validText(p.MerchantProfile().PlatformMerchantID, 32) {
		return nil, ErrUnavailable
	}
	return &MerchantOnboarding{r, p, f, secrets, a}, nil
}
func validDocumentAt(d IdentityDocument, beneficiary bool, now time.Time) bool {
	switch d.Type {
	case "IDENTIFICATION_TYPE_MAINLAND_IDCARD", "IDENTIFICATION_TYPE_OVERSEA_PASSPORT", "IDENTIFICATION_TYPE_HONGKONG", "IDENTIFICATION_TYPE_MACAO", "IDENTIFICATION_TYPE_TAIWAN", "IDENTIFICATION_TYPE_FOREIGN_RESIDENT", "IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT", "IDENTIFICATION_TYPE_TAIWAN_RESIDENT":
	default:
		return false
	}
	if !validText(d.Name, 128) || !validText(d.Number, 64) || !ValidID(d.FrontFileID) || d.BackFileID != "" && !ValidID(d.BackFileID) || d.Type != "IDENTIFICATION_TYPE_OVERSEA_PASSPORT" && d.BackFileID == "" || beneficiary && !validText(d.Address, 512) {
		return false
	}
	start, err := time.Parse("2006-01-02", d.ValidFrom)
	today := now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02")
	if err != nil || start.Year() < 1900 || start.Format("2006-01-02") >= today {
		return false
	}
	if d.ValidUntil != "长期" {
		end, err := time.Parse("2006-01-02", d.ValidUntil)
		if err != nil || !end.After(start) || end.Format("2006-01-02") < today {
			return false
		}
	}
	return true
}
func ValidateMerchantDetails(d MerchantDetails) ([]string, error) {
	now := time.Now()
	if !ValidID(d.LicenseFileID) || !validDocumentAt(d.Legal, false, now) || len(d.Beneficiaries) > 4 || d.SoleLegalBeneficiary && len(d.Beneficiaries) != 0 || !d.SoleLegalBeneficiary && len(d.Beneficiaries) == 0 || !validText(d.ContactMobile, 32) || !validText(d.AccountBank, 128) || !validText(d.AccountNumber, 64) || !validText(d.MerchantShortName, 64) || !validText(d.StoreName, 128) || len(d.BankBranchName) > 128 {
		return nil, ErrInvalid
	}
	u, err := url.Parse(d.StoreURL)
	if err != nil || len(d.StoreURL) > 256 || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, ErrInvalid
	}
	files := []string{d.LicenseFileID, d.Legal.FrontFileID}
	if d.Legal.BackFileID != "" {
		files = append(files, d.Legal.BackFileID)
	}
	for _, b := range d.Beneficiaries {
		if !validDocumentAt(b, true, now) {
			return nil, ErrInvalid
		}
		files = append(files, b.FrontFileID)
		if b.BackFileID != "" {
			files = append(files, b.BackFileID)
		}
	}
	seen := map[string]bool{}
	unique := []string{}
	for _, id := range files {
		if seen[id] {
			continue
		}
		unique = append(unique, id)
		seen[id] = true
	}
	return unique, nil
}
func (s *MerchantOnboarding) Submit(ctx context.Context, scope Scope, key, appID string, version int64, d MerchantDetails) (MerchantView, error) {
	if scope.Platform || !validText(scope.OrganizationID, 128) || !validText(scope.ActorID, 256) || !ValidID(key) || !ValidID(appID) || version < 1 {
		return MerchantView{}, ErrInvalid
	}
	ids, err := ValidateMerchantDetails(d)
	if err != nil {
		return MerchantView{}, err
	}
	if !s.channel.NewMerchantApplicationsEnabled() {
		return MerchantView{}, ErrUnavailable
	}
	if err = s.authorizer.AuthorizeMerchantApplication(ctx, scope.OrganizationID, scope.ActorID); err != nil {
		return MerchantView{}, ErrForbidden
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("ecoservices-merchant:"+scope.OrganizationID+":"+key)).String()
	intent := MerchantIntent{ID: id, ApplicationID: appID, ApplicationVersion: version, OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, Key: key, Profile: s.channel.MerchantProfile(), OutRequestNo: strings.ReplaceAll(id, "-", ""), FileIDs: ids, LicenseFileID: d.LicenseFileID}
	intent.Fingerprint = Fingerprint([]any{scope, key, appID, version, intent.Profile, d})
	plain, _ := json.Marshal(d)
	intent.SealedDetails, err = s.protection.Seal(id, string(plain))
	if err != nil {
		return MerchantView{}, ErrUnavailable
	}
	attempt, err := s.repo.CreateMerchantAttempt(ctx, intent)
	if err != nil {
		return MerchantView{}, err
	}
	return s.continueOriginal(ctx, scope, attempt.Intent.ApplicationID)
}
func (s *MerchantOnboarding) continueOriginal(ctx context.Context, scope Scope, appID string) (MerchantView, error) {
	attempt, claimed, err := s.repo.ClaimMerchantAttempt(ctx, scope, appID)
	if err != nil {
		return MerchantView{}, err
	}
	if !claimed {
		return s.view(attempt)
	}
	defer s.repo.ReleaseMerchantClaim(ctx, attempt)
	if attempt.Intent.Profile != s.channel.MerchantProfile() {
		return MerchantView{}, ErrUnavailable
	}
	// A marked intent is queried first. Only verified absence plus this live
	// human admission permits another dispatch of the SAME original intent.
	if attempt.Dispatched {
		obs, qerr := s.channel.QueryMerchant(ctx, attempt)
		if qerr != nil {
			return MerchantView{}, ErrUnavailable
		}
		if obs.State != "NOT_FOUND" {
			return s.accept(ctx, attempt, obs)
		}
		if !obs.Matches(attempt) {
			return MerchantView{}, ErrConflict
		}
		if attempt.State != "PREPARING" || len(attempt.SealedObservation) > 0 {
			return MerchantView{}, ErrConflict
		}
	}
	raw, err := s.protection.Open(attempt.Intent.ID, attempt.Intent.SealedDetails)
	var d MerchantDetails
	if err != nil || json.Unmarshal([]byte(raw), &d) != nil {
		return MerchantView{}, ErrConflict
	}
	originalScope := Scope{OrganizationID: attempt.Intent.OrganizationID, ActorID: attempt.Intent.ActorID}
	if Fingerprint([]any{originalScope, attempt.Intent.Key, appID, attempt.Intent.ApplicationVersion, attempt.Intent.Profile, d}) != attempt.Intent.Fingerprint {
		return MerchantView{}, ErrConflict
	}
	if _, err = ValidateMerchantDetails(d); err != nil {
		return MerchantView{}, err
	}
	for _, id := range attempt.Intent.FileIDs {
		if attempt.MediaIDs[id] != "" {
			continue
		}
		file, data, err := s.files.Download(ctx, scope, id)
		if err != nil {
			return MerchantView{}, err
		}
		if file.ParentKind != "APPLICATION" || file.ParentID != appID || file.SizeBytes > 2<<20 || file.ContentType != "image/png" && file.ContentType != "image/jpeg" {
			return MerchantView{}, ErrInvalid
		}
		media, err := s.channel.UploadMerchantImage(ctx, file, data)
		if err != nil {
			return MerchantView{}, ErrUnavailable
		}
		if !validText(media, 256) {
			return MerchantView{}, ErrConflict
		}
		if err = s.repo.SaveMerchantMedia(ctx, attempt, id, media); err != nil {
			return MerchantView{}, err
		}
		attempt.MediaIDs[id] = media
	}
	if err = s.authorizer.AuthorizeMerchantApplication(ctx, scope.OrganizationID, scope.ActorID); err != nil {
		return MerchantView{}, ErrForbidden
	}
	attempt.DispatchActorID = scope.ActorID
	if err = s.repo.MarkMerchantDispatched(ctx, attempt); err != nil {
		return MerchantView{}, err
	}
	attempt.Dispatched = true
	// Even a lost reply recovers by original number. No unverified SDK error is
	// interpreted as a definitive absence or permission to create a new intent.
	_ = s.channel.SubmitMerchant(ctx, attempt, d)
	obs, err := s.channel.QueryMerchant(ctx, attempt)
	if err != nil || obs.State == "NOT_FOUND" {
		return MerchantView{}, ErrUnavailable
	}
	return s.accept(ctx, attempt, obs)
}
func (o MerchantObservation) Matches(a MerchantAttempt) bool {
	return o.Profile == a.Intent.Profile && o.OutRequestNo == a.Intent.OutRequestNo && validText(o.VerificationVersion, 256)
}
func (s *MerchantOnboarding) accept(ctx context.Context, a MerchantAttempt, o MerchantObservation) (MerchantView, error) {
	if !o.Matches(a) {
		return MerchantView{}, ErrConflict
	}
	if o.State == "NOT_FOUND" {
		v, err := s.view(a)
		v.VerificationPending = true
		return v, err
	}
	raw, _ := json.Marshal(o)
	sealed, err := s.protection.Seal(a.Intent.ID+":observation", string(raw))
	if err != nil {
		return MerchantView{}, ErrUnavailable
	}
	if err = s.repo.ObserveMerchant(ctx, a, o, sealed); err != nil {
		return MerchantView{}, err
	}
	a.State = o.State
	a.SealedObservation = sealed
	a.UpdatedAt = time.Now().UTC()
	return s.view(a)
}
func (s *MerchantOnboarding) Read(ctx context.Context, scope Scope, appID string) (MerchantView, error) {
	if scope.Platform || !validText(scope.OrganizationID, 128) || !validText(scope.ActorID, 256) || !ValidID(appID) {
		return MerchantView{}, ErrInvalid
	}
	a, err := s.repo.ReadMerchantAttempt(ctx, scope, appID)
	if err != nil {
		return MerchantView{}, err
	}
	// Read does not create, submit or retry a channel application.
	if a.Dispatched && a.Intent.Profile == s.channel.MerchantProfile() {
		a, claimed, err := s.repo.ClaimMerchantAttempt(ctx, scope, appID)
		if err != nil {
			return MerchantView{}, err
		}
		if claimed {
			defer s.repo.ReleaseMerchantClaim(ctx, a)
			o, err := s.channel.QueryMerchant(ctx, a)
			if err != nil {
				v, viewErr := s.view(a)
				v.VerificationPending = true
				return v, viewErr
			}
			return s.accept(ctx, a, o)
		}
	}
	v, err := s.view(a)
	if a.Dispatched && a.Intent.Profile != s.channel.MerchantProfile() {
		v.VerificationPending = true
	}
	return v, err
}
func (s *MerchantOnboarding) view(a MerchantAttempt) (MerchantView, error) {
	v := MerchantView{ID: a.Intent.ID, ApplicationID: a.Intent.ApplicationID, State: a.State, UpdatedAt: a.UpdatedAt}
	if len(a.SealedObservation) == 0 {
		return v, nil
	}
	raw, err := s.protection.Open(a.Intent.ID+":observation", a.SealedObservation)
	var o MerchantObservation
	if err != nil || json.Unmarshal([]byte(raw), &o) != nil || !o.Matches(a) {
		return MerchantView{}, ErrConflict
	}
	v.SignState = o.SignState
	v.SignURL = o.SignURL
	v.LegalValidationURL = o.LegalValidationURL
	v.Reason = o.Reason
	v.Bank = o.Bank
	return v, nil
}
