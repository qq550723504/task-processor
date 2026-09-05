package storecenter

import (
	"context"
	"errors"
	"testing"
)

type sourcingStoreReader struct {
	store   *Store
	err     error
	calls   int
	org, id string
}

func (r *sourcingStoreReader) Get(_ context.Context, org, id string) (*Store, error) {
	r.calls++
	r.org, r.id = org, id
	return r.store, r.err
}

func TestSourcingStoreAccessFailsClosed(t *testing.T) {
	const id = "f16fd962-d190-4fcf-a4ab-3c8f473f40de"
	for _, test := range []struct {
		name  string
		store *Store
		err   error
		want  bool
	}{
		{name: "active", store: &Store{id: id, organizationID: "org-a", platform: PlatformShein, lifecycleStatus: StoreStatusActive}, want: true},
		{name: "cross organization", store: &Store{id: id, organizationID: "org-b", platform: PlatformShein, lifecycleStatus: StoreStatusActive}},
		{name: "wrong store", store: &Store{id: "other", organizationID: "org-a", platform: PlatformShein, lifecycleStatus: StoreStatusActive}},
		{name: "wrong platform", store: &Store{id: id, organizationID: "org-a", platform: "amazon", lifecycleStatus: StoreStatusActive}},
		{name: "disabled", store: &Store{id: id, organizationID: "org-a", platform: PlatformShein, lifecycleStatus: StoreStatusDisabled}},
		{name: "provisioning", store: &Store{id: id, organizationID: "org-a", platform: PlatformShein, lifecycleStatus: StoreStatusProvisioning}},
		{name: "deleting", store: &Store{id: id, organizationID: "org-a", platform: PlatformShein, lifecycleStatus: StoreStatusDeleting}},
		{name: "missing"}, {name: "repository failure", err: errors.New("unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &sourcingStoreReader{store: test.store, err: test.err}
			err := ValidateSourcingStoreAccess(context.Background(), reader, "org-a", id, PlatformShein)
			if (err == nil) != test.want {
				t.Fatalf("access error = %v", err)
			}
			if reader.org != "org-a" || reader.id != id {
				t.Fatal("repository read was not organization scoped")
			}
		})
	}
	reader := &sourcingStoreReader{}
	if ValidateSourcingStoreAccess(context.Background(), reader, "", id, PlatformShein) == nil || reader.calls != 0 {
		t.Fatal("invalid organization reached repository")
	}
	if ValidateSourcingStoreAccess(context.Background(), nil, "org-a", id, PlatformShein) == nil {
		t.Fatal("missing dependency accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ValidateSourcingStoreAccess(ctx, reader, "org-a", id, PlatformShein); !errors.Is(err, context.Canceled) || reader.calls != 0 {
		t.Fatal("canceled access reached repository")
	}
}

func TestSourcingStoreAccessRechecksLifecycleOnReplay(t *testing.T) {
	store := &Store{id: "f16fd962-d190-4fcf-a4ab-3c8f473f40de", organizationID: "org-a", platform: PlatformShein, lifecycleStatus: StoreStatusActive}
	reader := &sourcingStoreReader{store: store}
	if err := ValidateSourcingStoreAccess(context.Background(), reader, "org-a", store.id, PlatformShein); err != nil {
		t.Fatal(err)
	}
	store.lifecycleStatus = StoreStatusDisabled
	if ValidateSourcingStoreAccess(context.Background(), reader, "org-a", store.id, PlatformShein) == nil || reader.calls != 2 {
		t.Fatal("revoked store access reused")
	}
}

func TestSourcingStoreAccessRejectsTypedNilDependency(t *testing.T) {
	var reader *sourcingStoreReader
	if err := ValidateSourcingStoreAccess(context.Background(), reader, "org-a", "f16fd962-d190-4fcf-a4ab-3c8f473f40de", PlatformShein); !errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("typed nil: %v", err)
	}
}
