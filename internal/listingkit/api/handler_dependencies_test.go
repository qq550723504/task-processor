package api

import (
	"testing"

	"task-processor/internal/listingsubscription"
)

func TestWithDependenciesConfiguresSubscriptionState(t *testing.T) {
	t.Parallel()

	repo := listingsubscription.NewMemRepository()
	service, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}

	users := []string{"platform-user"}
	roles := []string{"platform-role"}

	h, err := NewHandler(&stubGenerationTaskService{}, WithDependencies(HandlerDependencies{
		Subscription: SubscriptionDependencies{
			Service:            service,
			PlatformAdminUsers: users,
			PlatformAdminRoles: roles,
		},
	}))
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	if h.subscriptionService != service {
		t.Fatal("expected subscription service to be attached")
	}
	if h.subscriptionHandler == nil {
		t.Fatal("expected subscription handler to be initialized")
	}
	if len(h.platformAdminUsers) != 1 || h.platformAdminUsers[0] != "platform-user" {
		t.Fatalf("platform admin users = %#v", h.platformAdminUsers)
	}
	if len(h.platformAdminRoles) != 1 || h.platformAdminRoles[0] != "platform-role" {
		t.Fatalf("platform admin roles = %#v", h.platformAdminRoles)
	}

	users[0] = "mutated-user"
	roles[0] = "mutated-role"
	if h.platformAdminUsers[0] != "platform-user" {
		t.Fatalf("platform admin users should be copied, got %#v", h.platformAdminUsers)
	}
	if h.platformAdminRoles[0] != "platform-role" {
		t.Fatalf("platform admin roles should be copied, got %#v", h.platformAdminRoles)
	}
}
