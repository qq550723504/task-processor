//go:build integration

package membership

import (
	"context"
	"github.com/google/uuid"
	"strings"
	"sync"
	flow "task-processor/internal/organization/membership/inviteflow"
	"testing"
	"time"
)

func testInvitations(t *testing.T, ctx context.Context, repo *Repository) {
	t.Helper()
	store := repo.Invitations()
	now := time.Unix(time.Now().Unix(), 123456789).UTC()
	inv := flow.Invitation{ID: uuid.NewString(), ProjectID: "p", OrganizationID: "invite-org", CreatorID: "admin", Contact: "invite@example.test", Role: "listingkit_viewer", Fingerprint: strings.Repeat("a", 64), State: flow.Pending, Revision: 1, CreatedAt: now, ExpiresAt: now.Add(time.Hour), DeliveryState: "not_attempted"}
	saved, replay, err := store.Create(ctx, inv)
	if err != nil || replay {
		t.Fatalf("create %v %v", replay, err)
	}
	got, replay, err := store.Create(ctx, inv)
	if err != nil || !replay || got.Fingerprint != inv.Fingerprint {
		t.Fatal("replay", err)
	}
	attempt := uuid.NewString()
	if _, err = store.ClaimDelivery(ctx, inv.ID, attempt, now); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	winners := make(chan flow.Invitation, 8)
	for range 8 {
		wg.Go(func() {
			next, err := store.Change(ctx, inv.ID, saved.Revision, flow.Change{State: flow.Accepting, RecipientID: "recipient", DispatchID: uuid.NewString(), Now: now})
			if err == nil {
				winners <- next
			}
		})
	}
	wg.Wait()
	if len(winners) != 1 {
		t.Fatalf("dispatch winners %d", len(winners))
	}
	// Stale SMTP completion never rewrites accepting or its revision/recipient.
	got, err = store.FinishDelivery(ctx, inv.ID, attempt, "mail_server_accepted", now)
	if err != nil || got.State != flow.Accepting || got.RecipientID != "recipient" {
		t.Fatal("mail overwrote consent", err)
	}
	other := inv
	other.ID = uuid.NewString()
	other.CreatedAt = now.Add(2 * time.Hour)
	other.ExpiresAt = other.CreatedAt.Add(time.Hour)
	if _, _, err = store.Create(ctx, other); err != flow.ErrConflict {
		t.Fatal("UNKNOWN contact released", err)
	}
	restarted := repo.Invitations()
	got, err = restarted.Read(ctx, inv.ID)
	if err != nil || got.State != flow.Accepting || got.DispatchID == "" {
		t.Fatal("restart", err)
	}
	if _, err = restarted.ClaimDelivery(ctx, inv.ID, uuid.NewString(), now); err != flow.ErrConflict {
		t.Fatal("UNKNOWN resend", err)
	}
	if _, err = store.Change(ctx, inv.ID, got.Revision, flow.Change{State: flow.Accepted, RecipientID: "recipient", AuthorizationID: "confirmed-grant", Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Create(ctx, other); err != nil {
		t.Fatal("terminal contact retained", err)
	}
	page, err := store.List(ctx, "invite-org", 1, 0)
	if err != nil || page.Total != 2 || len(page.Items) != 1 {
		t.Fatal("count", err)
	}
	foreign, err := store.List(ctx, "foreign-org", 20, 0)
	if err != nil || foreign.Total != 0 {
		t.Fatal("org isolation", err)
	}
	changed := other
	changed.Fingerprint = strings.Repeat("b", 64)
	if _, _, err = store.Create(ctx, changed); err != flow.ErrConflict {
		t.Fatal("same key admitted different payload", err)
	}
	first, second := uuid.NewString(), uuid.NewString()
	if _, err = store.ClaimDelivery(ctx, other.ID, first, other.CreatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimDelivery(ctx, other.ID, second, other.CreatedAt.Add(31*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.FinishDelivery(ctx, other.ID, first, "mail_server_accepted", other.CreatedAt.Add(32*time.Second)); err != flow.ErrConflict {
		t.Fatal("late mail ACK admitted", err)
	}
	got, err = store.FinishDelivery(ctx, other.ID, second, "delivery_unknown", other.CreatedAt.Add(33*time.Second))
	if err != nil || got.DeliveryAttempts != 2 || got.DeliveryAttempt != second {
		t.Fatal("current attempt lost", err)
	}
	// An expired pending invitation releases only its own email reservation.
	renewed := other
	renewed.ID = uuid.NewString()
	renewed.CreatedAt = other.ExpiresAt.Add(time.Second)
	renewed.ExpiresAt = renewed.CreatedAt.Add(time.Hour)
	if _, _, err = store.Create(ctx, renewed); err != nil {
		t.Fatal("expired pending contact retained", err)
	}
	got, err = store.Read(ctx, other.ID)
	if err != nil || got.State != flow.Expired {
		t.Fatal("expiration not saved", err)
	}
}
