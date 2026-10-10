package dataacquisition

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
)

type itemRepo struct {
	Repository
	item   Item
	fenced int
}

type outageStarter struct {
	calls int
	err   error
}

func (s *outageStarter) EnsureExecution(context.Context, Job) error {
	s.calls++
	return s.err
}

type admissionRepository struct {
	*resultRepository
}

func (r *admissionRepository) Admit(context.Context, Principal, string, Query, orgresource.ResourceFunding) (Job, error) {
	return r.job, nil
}

func TestCommittedAdmissionPreservesUnknownAndTerminalReplaySkipsStartup(t *testing.T) {
	scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}
	q := Query{Site: "us", Mode: "asin", ASINs: []string{"B000123456"}, Limit: 1}
	command := uuid.NewString()
	repo := &admissionRepository{resultRepository: &resultRepository{job: Job{ID: uuid.NewString(), Scope: scope, CommandKey: command, State: "ADMITTED"}}}
	starter := &outageStarter{err: ErrUnavailable}
	svc := &Service{repo: repo, live: &revokedAccess{}, starter: starter}
	job, err := svc.Start(context.Background(), Principal{Scope: scope}, command, q, orgresource.FundingEnterprise, 5)
	require.ErrorIs(t, err, ErrUnknown, "the durable job must be recovered through its original command")
	require.Equal(t, repo.job.ID, job.ID)
	starter.err = nil
	recovered, err := svc.Start(context.Background(), Principal{Scope: scope}, command, q, orgresource.FundingEnterprise, 5)
	require.NoError(t, err)
	require.Equal(t, job.ID, recovered.ID)
	for _, state := range []string{"SUCCEEDED", "PARTIAL", "FAILED", "CANCELED"} {
		t.Run(state, func(t *testing.T) {
			repo.job.State = state
			starter.err = ErrUnavailable
			before := starter.calls
			replayed, err := svc.Start(context.Background(), Principal{Scope: scope}, command, q, orgresource.FundingEnterprise, 5)
			require.NoError(t, err)
			require.Equal(t, state, replayed.State)
			require.Equal(t, before, starter.calls, "terminal command replay must not depend on Temporal")
		})
	}
}

func TestReadOnlyRestartsNonterminalJobsAndKeepsAccessChecks(t *testing.T) {
	for _, state := range []string{"ADMITTED", "RUNNING", "SUCCEEDED", "PARTIAL", "FAILED", "CANCELED"} {
		t.Run(state, func(t *testing.T) {
			scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}
			repo := &resultRepository{job: Job{ID: uuid.NewString(), Scope: scope, CredentialID: uuid.NewString(), State: state}}
			starter := &outageStarter{err: ErrUnavailable}
			access := &revokedAccess{}
			svc := &Service{repo: repo, live: access, starter: starter}
			principal := Principal{Scope: scope, CredentialID: repo.job.CredentialID}
			job, err := svc.Read(context.Background(), principal, repo.job.ID)
			if state == "ADMITTED" || state == "RUNNING" {
				require.ErrorIs(t, err, ErrUnavailable)
				require.Equal(t, 1, starter.calls)
			} else {
				require.NoError(t, err)
				require.Equal(t, state, job.State)
				require.Zero(t, starter.calls)
			}
			principal.CredentialID = uuid.NewString()
			_, err = svc.Read(context.Background(), principal, repo.job.ID)
			require.ErrorIs(t, err, ErrNotFound)
			access.denied = true
			_, err = svc.Read(context.Background(), principal, repo.job.ID)
			require.ErrorIs(t, err, ErrForbidden)
		})
	}
}

func (r *itemRepo) CheckActive(context.Context, Job) error { return nil }
func (r *itemRepo) BindReservation(_ context.Context, _ Job, _ string, c orgresource.ConsumerChargeReceipt) (Item, error) {
	r.item.ReservationID = c.ReservationID
	r.item.ChargeState = c.State
	return r.item, nil
}
func (r *itemRepo) Fence(_ context.Context, _ Job, _ Item, _ string) (Item, error) {
	r.item.State = "FAILED"
	r.fenced++
	return r.item, nil
}
func (r *itemRepo) RecordCharge(context.Context, Job, string, orgresource.ConsumerChargeReceipt) error {
	return nil
}

type lostCharges struct {
	receipt           orgresource.ConsumerChargeReceipt
	reserves, settles int
}

func (c *lostCharges) Lookup(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	if c.reserves == 0 {
		return c.receipt, orgresource.ErrReservationNotFound
	}
	return c.receipt, nil
}
func (c *lostCharges) Reserve(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	c.reserves++
	return orgresource.ConsumerChargeReceipt{}, ErrUnknown
}
func (c *lostCharges) Reconcile(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	c.settles++
	r := c.receipt
	r.State = orgresource.ReservationReleased
	return r, nil
}

type revokedAccess struct{ denied bool }

func (a *revokedAccess) CheckRead(context.Context, Principal) error {
	if a.denied {
		return ErrForbidden
	}
	return nil
}

func (a *revokedAccess) CheckExecution(context.Context, Principal, orgresource.ResourceFunding) error {
	if a.denied {
		return ErrForbidden
	}
	return nil
}
func TestReserveResponseLossThenRevocationBindsOriginalAndReleasesWithoutFetch(t *testing.T) {
	r := &itemRepo{item: Item{ID: uuid.NewString(), ASIN: "B000123456", State: "PREPARED"}}
	c := &lostCharges{receipt: orgresource.ConsumerChargeReceipt{ReservationID: uuid.NewString(), State: orgresource.ReservationReserved}}
	a := &revokedAccess{}
	s := &Service{repo: r, charges: c, live: a, now: time.Now}
	job := Job{ID: uuid.NewString(), Scope: collection.Scope{OrganizationID: "org", ActorID: "actor", MemberID: "original"}, Funding: orgresource.FundingEnterprise, Deadline: time.Now().Add(time.Hour)}
	if err := s.ProcessItem(context.Background(), job, r.item, ""); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	a.denied = true
	if err := s.ProcessItem(context.Background(), job, r.item, ""); err != nil {
		t.Fatal(err)
	}
	if c.reserves != 1 || c.settles != 1 || r.fenced != 1 || r.item.ReservationID != c.receipt.ReservationID {
		t.Fatalf("reserves %d settles %d fenced %d", c.reserves, c.settles, r.fenced)
	}
	// No provider is installed: an accidental Fetch would panic this test.
}
