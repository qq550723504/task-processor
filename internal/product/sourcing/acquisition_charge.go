package sourcing

import "context"

const (
	AcquisitionFundingMember     = "member_allocated"
	AcquisitionFundingEnterprise = "enterprise_unallocated"
)

// The Product owner retains the admitted membership and funding choice. The
// Resource ledger receives this immutable intent through application composition.
type AcquisitionChargeIntent struct {
	Scope       PublicationScope
	OperationID string
	MemberID    string
	Funding     string
	Fingerprint string
	Source      AcquisitionSource
}
type AcquisitionChargeProof struct {
	Intent         AcquisitionChargeIntent
	ReservationID  string
	State          string
	EvidenceID     string
	PublicationID  string
	InputHash      string
	ProductKey     string
	CatalogVersion uint64
	SnapshotHash   string
}
type AcquisitionChargeStore interface {
	RecordChargeIntent(context.Context, AcquisitionOperation, AcquisitionChargeIntent) error
	BindChargeReservation(context.Context, AcquisitionOperation, AcquisitionChargeIntent, string) error
	ReadChargeIntent(context.Context, string, string) (AcquisitionChargeIntent, error)
	ReadChargeProof(context.Context, string, string) (AcquisitionChargeProof, error)
}
