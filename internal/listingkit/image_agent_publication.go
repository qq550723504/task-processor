package listingkit

import (
	"context"
	"errors"
)

var ErrImageAgentPublicationConflict = errors.New("image agent publication idempotency conflict")

type ImageAgentPublicationAcknowledgement struct {
	TaskID            string
	RunID             string
	PlanRevision      int64
	ResultDigest      string
	IdempotencyKey    string
	CandidateAssetIDs []string
}

type ImageAgentPublicationCommit struct {
	TenantID        string
	OwnerUserID     string
	TaskID          string
	IdempotencyKey  string
	Fingerprint     string
	Acknowledgement ImageAgentPublicationAcknowledgement
}

type ImageAgentPublicationTransactionRepository interface {
	CommitImageAgentPublication(context.Context, ImageAgentPublicationCommit, TaskResultMutation) (ImageAgentPublicationAcknowledgement, error)
}
