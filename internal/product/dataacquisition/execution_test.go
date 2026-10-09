package dataacquisition

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
)

type itemRepo struct {
	Repository
	item   Item
	fenced int
}

func (r *itemRepo) CheckActive(context.Context, Job) error { return nil }
func (r *itemRepo) BindReservation(_ context.Context, _ Job, _ string, c orgresource.ConsumerChargeReceipt) (Item, error) {
	r.item.ReservationID = c.ReservationID
	r.item.ChargeState = c.State
	return r.item, nil
}
func (r *itemRepo) Fence(_ context.Context, _ Job, _ string, _ string) (Item, error) {
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

func (a *revokedAccess) CheckExecution(context.Context, collection.Scope, orgresource.ResourceFunding) error {
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
