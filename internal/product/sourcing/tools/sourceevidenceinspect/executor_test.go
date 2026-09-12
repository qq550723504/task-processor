package sourceevidenceinspect

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type snapshotStub struct {
	value    catalog.PublishedSnapshot
	err      error
	calls    int
	identity catalog.SnapshotIdentity
	version  uint64
}

func (s *snapshotStub) GetSnapshot(_ context.Context, identity catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	s.calls++
	s.identity = identity
	s.version = version
	return s.value, s.err
}

type sourceStub struct {
	value       sourcing.PersistedPublication
	err         error
	calls       int
	publication string
}

func (s *sourceStub) Read(_ context.Context, publication string) (sourcing.PersistedPublication, error) {
	s.calls++
	s.publication = publication
	return s.value, s.err
}

func bindingFixture() (context.Context, commercetool.Principal, *snapshotStub, *sourceStub) {
	identity := authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org", Roles: []string{"listingkit_operator"}, TokenExpiresAt: time.Now().Add(time.Hour)}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	p := commercetool.Principal{UserID: "actor", TenantID: "org", Roles: identity.Roles}
	c := &snapshotStub{value: catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"}, Version: 9007199254740993, PublicationID: "publication", Snapshot: catalog.ProductSnapshot{Title: "exact title"}}}
	s := &sourceStub{value: sourcing.PersistedPublication{Receipt: sourcing.PublicationReceipt{OrganizationID: "org", ProductKey: "product", CatalogVersion: c.value.Version, PublicationID: "publication", CatalogPublicationID: "publication"}, Snapshot: c.value.Snapshot}}
	return ctx, p, c, s
}

func TestReadExactUsesServerPublicationAndExactLargeVersion(t *testing.T) {
	ctx, p, c, s := bindingFixture()
	e, err := NewExecutor(c, s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.readExact(ctx, p, Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
	if err != nil || !reflect.DeepEqual(got, s.value) {
		t.Fatalf("readExact=%#v, %v", got, err)
	}
	if c.calls != 1 || c.identity != c.value.Identity || c.version != c.value.Version || s.calls != 1 || s.publication != "publication" {
		t.Fatalf("exact calls: catalog=%#v source=%#v", c, s)
	}
}

func TestReadExactRejectsMixedFacts(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*snapshotStub, *sourceStub)
	}{
		{"catalog org", func(c *snapshotStub, s *sourceStub) { c.value.Identity.TenantID = "other" }},
		{"catalog product", func(c *snapshotStub, s *sourceStub) { c.value.Identity.ProductKey = "other" }},
		{"catalog version", func(c *snapshotStub, s *sourceStub) { c.value.Version++ }},
		{"catalog publication", func(c *snapshotStub, s *sourceStub) { c.value.PublicationID = "" }},
		{"source org", func(c *snapshotStub, s *sourceStub) { s.value.Receipt.OrganizationID = "other" }},
		{"source product", func(c *snapshotStub, s *sourceStub) { s.value.Receipt.ProductKey = "other" }},
		{"source version", func(c *snapshotStub, s *sourceStub) { s.value.Receipt.CatalogVersion++ }},
		{"source publication", func(c *snapshotStub, s *sourceStub) { s.value.Receipt.PublicationID = "other" }},
		{"source catalog publication", func(c *snapshotStub, s *sourceStub) { s.value.Receipt.CatalogPublicationID = "other" }},
		{"source snapshot", func(c *snapshotStub, s *sourceStub) { s.value.Snapshot.Title = "tampered" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, p, c, s := bindingFixture()
			tc.mutate(c, s)
			e, _ := NewExecutor(c, s)
			got, err := e.readExact(ctx, p, Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
			if err == nil || !reflect.DeepEqual(got, sourcing.PersistedPublication{}) {
				t.Fatalf("mixed facts leaked: %#v %v", got, err)
			}
		})
	}
}

func TestReadExactRejectsIdentityMismatchBeforeReading(t *testing.T) {
	for _, kind := range []string{"missing", "actor", "org", "expired"} {
		t.Run(kind, func(t *testing.T) {
			ctx, p, c, s := bindingFixture()
			switch kind {
			case "missing":
				ctx = context.Background()
			case "actor":
				p.UserID = "other"
			case "org":
				p.TenantID = "other"
			case "expired":
				id, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
				id.TokenExpiresAt = time.Now().Add(-time.Minute)
				ctx = authidentity.WithAuthenticatedIdentity(ctx, id)
			}
			e, _ := NewExecutor(c, s)
			_, err := e.readExact(ctx, p, Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
			if err == nil || c.calls != 0 || s.calls != 0 {
				t.Fatalf("identity bypass: %v calls=%d/%d", err, c.calls, s.calls)
			}
		})
	}
}

func TestReadExactRejectsNoncanonicalVersionsAndKeys(t *testing.T) {
	for _, version := range []string{"", "0", "01", "1.0", "+1", " 1", "9223372036854775808", "18446744073709551615"} {
		ctx, p, c, s := bindingFixture()
		e, _ := NewExecutor(c, s)
		_, err := e.readExact(ctx, p, Input{ProductKey: "product", CatalogVersion: version})
		if err == nil || c.calls != 0 || s.calls != 0 {
			t.Fatalf("version %q accepted: %v", version, err)
		}
	}
	for _, key := range []string{"", " product", "product "} {
		ctx, p, c, s := bindingFixture()
		e, _ := NewExecutor(c, s)
		_, err := e.readExact(ctx, p, Input{ProductKey: key, CatalogVersion: "1"})
		if err == nil || c.calls != 0 {
			t.Fatalf("key %q accepted: %v", key, err)
		}
	}
}

func TestReadExactPropagatesMissingRevokedCorruptWithoutFallback(t *testing.T) {
	for _, cause := range []error{sourcing.ErrPublicationForbidden, sourcing.ErrSourcePublicationNotFound, sourcing.ErrSourcePublicationStateInvalid, sourcing.ErrSourcePublicationUnavailable, context.Canceled, context.DeadlineExceeded} {
		ctx, p, c, s := bindingFixture()
		s.err = cause
		e, _ := NewExecutor(c, s)
		got, err := e.readExact(ctx, p, Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
		if err == nil || !errors.Is(err, cause) || c.calls != 1 || s.calls != 1 || !reflect.DeepEqual(got, sourcing.PersistedPublication{}) {
			t.Fatalf("source failure=%v got=%#v err=%v", cause, got, err)
		}
	}
	ctx, p, c, s := bindingFixture()
	c.err = catalog.ErrSnapshotNotReady
	e, _ := NewExecutor(c, s)
	_, err := e.readExact(ctx, p, Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
	if !errors.Is(err, catalog.ErrSnapshotNotReady) || c.calls != 1 || s.calls != 0 {
		t.Fatalf("missing exact fallback: %v", err)
	}
}

func TestReadExactCanceledDoesNotRead(t *testing.T) {
	ctx, p, c, s := bindingFixture()
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	e, _ := NewExecutor(c, s)
	_, err := e.readExact(ctx, p, Input{ProductKey: "product", CatalogVersion: "9007199254740993"})
	if !errors.Is(err, context.Canceled) || c.calls != 0 || s.calls != 0 {
		t.Fatalf("cancel: %v", err)
	}
}

func TestNewExecutorRejectsNilPorts(t *testing.T) {
	_, _, c, s := bindingFixture()
	var nilCatalog *snapshotStub
	var nilSource *sourceStub
	for _, ports := range []struct {
		catalog catalog.VersionedSnapshotReader
		source  SourceReader
	}{{nil, s}, {c, nil}, {nilCatalog, s}, {c, nilSource}} {
		if _, err := NewExecutor(ports.catalog, ports.source); err == nil {
			t.Fatal("nil port accepted")
		}
	}
}
