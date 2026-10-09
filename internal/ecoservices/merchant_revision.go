package ecoservices

// The root intent keeps the original identity and number. Every dispatch reads
// this immutable, sealed details version; a missing version fails closed.
type MerchantDetailsRevision struct {
	ID                         string
	Version                    int64
	Input                      MerchantIntent
	ApprovalFingerprint        string
	ReviewedApplicationVersion int64
	AgreementVersion           string
}

// Revision fields associate a verified response with the actual local SDK call.
// They are not fields returned by WeChat.
type MerchantSubmissionAcceptance struct {
	RevisionID                                                                    string
	RevisionVersion                                                               int64
	DetailsFingerprint                                                            string
	Profile                                                                       MerchantProfile
	OutRequestNo, ChannelApplicationID, VerificationVersion, SignedResponseDigest string
}

func (p MerchantSubmissionAcceptance) Matches(a MerchantAttempt) bool {
	return p.RevisionID == a.Revision.ID && p.RevisionVersion == a.Revision.Version &&
		p.DetailsFingerprint == a.Revision.Input.Fingerprint && p.Profile == a.Intent.Profile &&
		p.OutRequestNo == a.Intent.OutRequestNo && validText(p.ChannelApplicationID, 32) &&
		validText(p.VerificationVersion, 256) && len(p.SignedResponseDigest) == 64
}

func (a MerchantAttempt) CanObserve() bool {
	return a.Revision.Version == 1 || a.Acceptance != nil && a.Acceptance.Matches(a)
}

func (a MerchantAttempt) BindQuery(o MerchantObservation) MerchantObservation {
	o.RevisionID = a.Revision.ID
	o.RevisionVersion = a.Revision.Version
	if a.Acceptance != nil {
		o.AcceptanceFingerprint = Fingerprint(*a.Acceptance)
	}
	return o
}
