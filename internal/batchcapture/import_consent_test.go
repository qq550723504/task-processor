package batchcapture

import (
	"errors"
	"path/filepath"
	"testing"
)

// unapprovedQueueFile writes a one-item queue with no confirmed scope. It exists
// here rather than with the fixture helpers because these tests are pure local
// logic: they must run in CI, where no browser is available.
func unapprovedQueueFile(t *testing.T, url string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("consent-batch")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: url, State: ItemQueued})
	if err := queue.Save(path); err != nil {
		t.Fatalf("save queue: %v", err)
	}
	return path
}

// TestImportOneRefusesAnUnapprovedQueueWithoutAConfirmationChannel is the fail-closed
// half of design section 4 D1.4: when nobody can be asked, an unconfirmed batch must
// stop before the browser is touched instead of adopting whatever identity the live
// session happens to carry. It is the F5-1 regression guard.
func TestImportOneRefusesAnUnapprovedQueueWithoutAConfirmationChannel(t *testing.T) {
	url := "https://detail.1688.com/offer/981645030344.html"
	path := unapprovedQueueFile(t, url)

	_, err := ImportOne(nil, ImportOptions{
		QueuePath: path,
		Approved:  AppScope{ActorID: "declared-actor", OrganizationID: "declared-org"},
	})
	if !errors.Is(err, ErrScopeUnconfirmed) {
		t.Fatalf("err=%v, want ErrScopeUnconfirmed", err)
	}
	if errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("an absent confirmation was reported as a disagreement: %v", err)
	}
	stored, loadErr := Load(path)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.ScopeApproved {
		t.Fatalf("the run approved the batch without a person: %+v", stored)
	}
	if stored.Items[0].State != ItemQueued {
		t.Fatalf("state %q, want queued so the item stays re-doable", stored.Items[0].State)
	}
	if stored.Items[0].ApprovedActorID != "" || stored.Items[0].ApprovedOrganizationID != "" {
		t.Fatalf("the declared flags were written as an approved scope: %+v", stored.Items[0])
	}
}

// TestImportOneRejectsAnApprovedQueueUnderAnotherDeclaredScope keeps the already
// approved path from being loosened by the pre-approval work: once a scope exists it
// is authoritative, and a run started for another identity is refused before the
// browser starts and without asking anyone.
func TestImportOneRejectsAnApprovedQueueUnderAnotherDeclaredScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("consent-batch")
	if err := queue.ApproveScope("approved-actor", "approved-org"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	queue.Items = append(queue.Items, Item{Seq: 1, URL: "https://detail.1688.com/offer/981645030344.html", State: ItemQueued})
	if err := queue.Save(path); err != nil {
		t.Fatalf("save queue: %v", err)
	}

	asked := false
	_, err := ImportOne(nil, ImportOptions{
		QueuePath: path,
		Approved:  AppScope{ActorID: "other-actor", OrganizationID: "other-org"},
		ConfirmScope: func(AppScope) (bool, error) {
			asked = true
			return true, nil
		},
	})
	if !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("err=%v, want ErrScopeMismatch", err)
	}
	if asked {
		t.Fatalf("a person was asked about a scope that was already confirmed")
	}
	stored, loadErr := Load(path)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if !stored.ScopeMatches("approved-actor", "approved-org") {
		t.Fatalf("the confirmed scope was rewritten: %+v", stored)
	}
	if stored.Items[0].State != ItemQueued {
		t.Fatalf("state %q, want queued", stored.Items[0].State)
	}
}
