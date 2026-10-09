package dataacquisition

import (
	"context"
	"errors"
	"time"

	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
)

type Principal struct {
	Scope              collection.Scope
	CredentialID       string
	CredentialRevision int64
}
type Job struct {
	ID           string                      `json:"id"`
	Scope        collection.Scope            `json:"-"`
	CredentialID string                      `json:"credentialId,omitempty"`
	CommandKey   string                      `json:"commandKey"`
	InputHash    string                      `json:"-"`
	Query        Query                       `json:"query"`
	Funding      orgresource.ResourceFunding `json:"-"`
	State        string                      `json:"state"`
	Reason       string                      `json:"reason,omitempty"`
	Discovered   bool                        `json:"discovered"`
	Canceled     bool                        `json:"canceled"`
	Saved        int                         `json:"saved"`
	Failed       int                         `json:"failed"`
	Pending      int                         `json:"pending"`
	ConfirmedFen int64                       `json:"confirmedFen"`
	PendingFen   int64                       `json:"pendingFen"`
	BatchID      string                      `json:"batchId,omitempty"`
	CreatedAt    time.Time                   `json:"createdAt"`
	Deadline     time.Time                   `json:"deadline"`
}
type Item struct {
	ID            string                       `json:"id"`
	ASIN          string                       `json:"asin"`
	State         string                       `json:"state"`
	ClaimToken    string                       `json:"-"`
	Evidence      *Evidence                    `json:"-"`
	Source        *collection.Source           `json:"source,omitempty"`
	ReservationID string                       `json:"-"`
	ChargeState   orgresource.ReservationState `json:"chargeState,omitempty"`
	Reason        string                       `json:"reason,omitempty"`
}
type Provider interface {
	Discover(context.Context, Query) ([]string, error)
	Fetch(context.Context, string, string) (Evidence, error)
}
type LiveAccess interface {
	CheckExecution(context.Context, Principal, orgresource.ResourceFunding) error
	CheckRead(context.Context, Principal) error
}
type ExecutionStarter interface {
	EnsureExecution(context.Context, Job) error
}
type Charges interface {
	orgresource.ConsumerChargePort
	Lookup(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error)
}
type Repository interface {
	Admit(context.Context, Principal, string, Query, orgresource.ResourceFunding) (Job, error)
	Read(context.Context, collection.Scope, string) (Job, error)
	List(context.Context, collection.Scope, int) ([]Job, error)
	CheckActive(context.Context, Job) error
	Discover(context.Context, Job, []string) (Job, error)
	FailDiscovery(context.Context, Job, string) (Job, error)
	Items(context.Context, Job) ([]Item, error)
	Claim(context.Context, Job, string) (Item, error)
	BindReservation(context.Context, Job, string, orgresource.ConsumerChargeReceipt) (Item, error)
	PrepareEvidence(context.Context, Job, Item, Evidence) (Item, error)
	Publish(context.Context, Job, Item) (Item, error)
	Fence(context.Context, Job, Item, string) (Item, error)
	RecordCharge(context.Context, Job, string, orgresource.ConsumerChargeReceipt) error
	Finish(context.Context, Job) (Job, error)
	Cancel(context.Context, collection.Scope, string, string) (Job, error)
	ChargeIntent(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeIntent, error)
	ChargeProof(context.Context, orgresource.ConsumerChargeReceipt) (orgresource.ConsumerChargeProof, error)
}
type Service struct {
	repo     Repository
	live     LiveAccess
	provider Provider
	charges  Charges
	starter  ExecutionStarter
	now      func() time.Time
}

func NewService(repo Repository, live LiveAccess, provider Provider, charges Charges, starter ExecutionStarter) (*Service, error) {
	if repo == nil || live == nil || provider == nil || charges == nil || starter == nil {
		return nil, ErrUnavailable
	}
	return &Service{repo, live, provider, charges, starter, time.Now}, nil
}
func (s *Service) Start(ctx context.Context, p Principal, command string, q Query, funding orgresource.ResourceFunding, maximumCostFen int64) (Job, error) {
	q, err := NormalizeQuery(q)
	if err != nil || !collection.ValidID(command) || p.Scope.Validate() != nil || maximumCostFen != int64(q.Limit)*PriceFen {
		return Job{}, ErrInvalid
	}
	if err = s.live.CheckExecution(ctx, p, funding); err != nil {
		return Job{}, err
	}
	job, err := s.repo.Admit(ctx, p, command, q, funding)
	if err != nil {
		return Job{}, err
	}
	if err = s.starter.EnsureExecution(ctx, job); err != nil {
		return job, ErrUnavailable
	}
	return job, nil
}
func (s *Service) Read(ctx context.Context, p Principal, id string) (Job, error) {
	if p.Scope.Validate() != nil || !collection.ValidID(id) {
		return Job{}, ErrInvalid
	}
	if err := s.live.CheckRead(ctx, p); err != nil {
		return Job{}, err
	}
	job, err := s.repo.Read(ctx, p.Scope, id)
	if err != nil {
		return Job{}, err
	}
	if p.CredentialID != "" && job.CredentialID != p.CredentialID {
		return Job{}, ErrNotFound
	}
	if err = s.starter.EnsureExecution(ctx, job); err != nil {
		return job, ErrUnavailable
	}
	return job, nil
}
func identity(job Job, item Item) orgresource.ConsumerChargeIdentity {
	return orgresource.ConsumerChargeIdentity{OrganizationID: job.Scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: item.ID}
}
func (s *Service) check(ctx context.Context, job Job) error {
	if err := s.live.CheckExecution(ctx, Principal{Scope: job.Scope, CredentialID: job.CredentialID}, job.Funding); err != nil {
		return err
	}
	return s.repo.CheckActive(ctx, job)
}

// ProcessItem always looks up the original reservation before considering a new
// reserve. Revocation permits only original binding, terminal fencing and settle.
func (s *Service) ProcessItem(ctx context.Context, job Job, item Item, stopReason string) error {
	chargeID := identity(job, item)
	receipt, lookupErr := s.charges.Lookup(ctx, chargeID)
	if lookupErr == nil {
		var err error
		item, err = s.repo.BindReservation(ctx, job, item.ID, receipt)
		if err != nil {
			return err
		}
	} else if !errors.Is(lookupErr, orgresource.ErrReservationNotFound) {
		return lookupErr
	}
	if item.State == "SAVED" || item.State == "FAILED" {
		return s.settle(ctx, job, item)
	}
	if stopReason == "" {
		if s.now().After(job.Deadline) {
			stopReason = "deadline"
		} else if err := s.check(ctx, job); err != nil {
			if errors.Is(err, ErrForbidden) {
				stopReason = "access_revoked"
			} else {
				return err
			}
		}
	}
	if stopReason != "" {
		fenced, err := s.repo.Fence(ctx, job, item, stopReason)
		if err != nil {
			return err
		}
		return s.settle(ctx, job, fenced)
	}
	if lookupErr != nil {
		var err error
		receipt, err = s.charges.Reserve(ctx, chargeID)
		if err != nil {
			if errors.Is(err, orgresource.ErrInsufficientBalance) || errors.Is(err, orgresource.ErrResourceDebtOutstanding) {
				fenced, fenceErr := s.repo.Fence(ctx, job, item, "resource_unavailable")
				if fenceErr != nil {
					return fenceErr
				}
				return s.settle(ctx, job, fenced)
			}
			return err
		}
		item, err = s.repo.BindReservation(ctx, job, item.ID, receipt)
		if err != nil {
			return err
		}
	}
	if item.ReservationID == "" || item.ChargeState != orgresource.ReservationReserved {
		return ErrUnknown
	}
	if err := s.check(ctx, job); err != nil {
		return err
	}
	if item.Evidence == nil {
		claimed, err := s.repo.Claim(ctx, job, item.ID)
		if err != nil {
			return err
		}
		fetchContext, cancel := context.WithTimeout(ctx, 30*time.Second)
		evidence, fetchErr := s.provider.Fetch(fetchContext, job.Query.Site, item.ASIN)
		cancel()
		if fetchErr != nil || evidence.Site != job.Query.Site || evidence.ASIN != item.ASIN || evidence.Validate() != nil {
			fenced, err := s.repo.Fence(ctx, job, claimed, "provider_rejected")
			if err != nil {
				return err
			}
			return s.settle(ctx, job, fenced)
		}
		item, err = s.repo.PrepareEvidence(ctx, job, claimed, evidence)
		if err != nil {
			return err
		}
	}
	if err := s.check(ctx, job); err != nil {
		return err
	}
	saved, err := s.repo.Publish(ctx, job, item)
	if err != nil {
		return err
	}
	return s.settle(ctx, job, saved)
}
func (s *Service) settle(ctx context.Context, job Job, item Item) error {
	if item.ReservationID == "" {
		return nil
	}
	receipt, err := s.charges.Reconcile(ctx, identity(job, item))
	if err != nil {
		return err
	}
	return s.repo.RecordCharge(ctx, job, item.ID, receipt)
}

// Run is invoked only for an original durable job by the bounded Temporal
// activity. Product facts decide progress; activity replay never changes input.
func (s *Service) Run(ctx context.Context, scope collection.Scope, id string) error {
	job, err := s.repo.Read(ctx, scope, id)
	if err != nil {
		return err
	}
	if job.Scope != scope {
		return ErrForbidden
	}
	stop := ""
	if job.Canceled {
		stop = "canceled"
	}
	if s.now().After(job.Deadline) {
		stop = "deadline"
	}
	if !job.Discovered && stop == "" {
		if err = s.check(ctx, job); err != nil {
			if errors.Is(err, ErrForbidden) {
				stop = "access_revoked"
			} else {
				return err
			}
		}
		if stop == "" {
			discoveryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			ids, discoverErr := s.provider.Discover(discoveryCtx, job.Query)
			cancel()
			switch {
			case errors.Is(discoverErr, ErrSourceChallenge):
				stop = "provider_challenged"
			case errors.Is(discoverErr, ErrSourceUnsupported):
				stop = "provider_unsupported"
			case discoverErr != nil:
				return discoverErr
			default:
				job, err = s.repo.Discover(ctx, job, ids)
				if err != nil {
					return err
				}
			}
		}
	}
	if !job.Discovered && stop != "" {
		job, err = s.repo.FailDiscovery(ctx, job, stop)
		if err != nil {
			return err
		}
	}
	items, err := s.repo.Items(ctx, job)
	if err != nil {
		return err
	}
	var failures []error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.ProcessItem(ctx, job, item, stop); err != nil {
			failures = append(failures, err)
		}
	}
	if _, err = s.repo.Finish(ctx, job); err != nil {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}

type ChargeOwner struct{ Repository Repository }

func (o ChargeOwner) ReadChargeIntent(ctx context.Context, id orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeIntent, error) {
	if o.Repository == nil || id.Consumer != orgresource.ConsumerAmazonData {
		return orgresource.ConsumerChargeIntent{}, orgresource.ErrOwnerScopeMismatch
	}
	return o.Repository.ChargeIntent(ctx, id)
}
func (o ChargeOwner) ReadChargeProof(ctx context.Context, r orgresource.ConsumerChargeReceipt) (orgresource.ConsumerChargeProof, error) {
	if o.Repository == nil || r.Intent.Identity.Consumer != orgresource.ConsumerAmazonData {
		return orgresource.ConsumerChargeProof{}, orgresource.ErrOwnerScopeMismatch
	}
	return o.Repository.ChargeProof(ctx, r)
}
