package identitypreflight

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"task-processor/internal/listingkit/userdirectory"
)

func TestServiceRunSucceedsAndFetchesEachTenantDirectoryOnce(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: "tenant-a", UserID: "subject-x", RowCount: 2},
		{Table: "listing_category", TenantID: "tenant-a", UserID: "subject-x", RowCount: 1},
		{Table: "listing_kit_tasks", TenantID: "tenant-b", UserID: "subject-y", RowCount: 1},
	}}
	directory := &recordingUserDirectory{usersByTenant: map[string][]userdirectory.User{
		"tenant-a": {{Subject: "subject-x", TenantID: "tenant-a"}},
		"tenant-b": {{Subject: "subject-y", TenantID: "tenant-b"}},
	}}
	var output bytes.Buffer

	err := NewService(repository, directory, nil, &output).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want no findings", output.String())
	}
	if want := []string{"tenant-a", "tenant-b"}; !reflect.DeepEqual(directory.calls, want) {
		t.Fatalf("directory calls = %#v, want %#v", directory.calls, want)
	}
}

func TestServiceRunReportsUnknownOwnerWithTypedError(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: "tenant-a", UserID: "subject-x", RowCount: 3},
	}}
	directory := &recordingUserDirectory{}
	var output bytes.Buffer

	err := NewService(repository, directory, nil, &output).Run(context.Background())
	var unknownOwners *ErrUnknownOwners
	if !errors.As(err, &unknownOwners) {
		t.Fatalf("Run error = %v, want *ErrUnknownOwners", err)
	}
	if unknownOwners.Count != 1 {
		t.Fatalf("unknown owner count = %d, want 1", unknownOwners.Count)
	}
	want := "status=blocked table=listing_store tenant=sha256:80a707af7dc7 owner=sha256:c0f704a39dcc rows=3 reason=unknown_subject\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestServiceRunBlocksBlankOwnerWithoutDirectoryLookup(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: "tenant-a", UserID: "", RowCount: 3},
	}}
	directory := &recordingUserDirectory{}
	var output bytes.Buffer

	err := NewService(repository, directory, nil, &output).Run(context.Background())
	var unknownOwners *ErrUnknownOwners
	if !errors.As(err, &unknownOwners) || unknownOwners.Count != 1 {
		t.Fatalf("Run error = %v, want one blocking missing owner finding", err)
	}
	if len(directory.calls) != 0 {
		t.Fatalf("directory calls = %#v, want none", directory.calls)
	}
	want := "status=blocked table=listing_store tenant=sha256:80a707af7dc7 owner=sha256:e3b0c44298fc rows=3 reason=missing_subject\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestServiceRunRejectsSameSubjectFromWrongTenant(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_kit_tasks", TenantID: "tenant-a", UserID: "subject-x", RowCount: 1},
	}}
	directory := &recordingUserDirectory{usersByTenant: map[string][]userdirectory.User{
		"tenant-a": {{Subject: "subject-x", TenantID: "tenant-b"}},
	}}
	var output bytes.Buffer

	err := NewService(repository, directory, nil, &output).Run(context.Background())
	var unknownOwners *ErrUnknownOwners
	if !errors.As(err, &unknownOwners) {
		t.Fatalf("Run error = %v, want *ErrUnknownOwners", err)
	}
	if !strings.Contains(output.String(), "reason=unknown_subject") {
		t.Fatalf("output = %q, want unknown subject finding", output.String())
	}
}

func TestServiceRunSanitizesDirectoryFailureAndEmitsNoPartialReport(t *testing.T) {
	t.Parallel()

	const (
		tenantID = "tenant-secret"
		secret   = "token-secret private@example.test Private Name mocked-response-body"
	)
	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: tenantID, UserID: "subject-secret", RowCount: 1},
	}}
	directory := &recordingUserDirectory{errorsByTenant: map[string]error{
		tenantID: errors.New(secret + " " + tenantID),
	}}
	var output bytes.Buffer

	err := NewService(repository, directory, nil, &output).Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want directory failure")
	}
	if !strings.Contains(err.Error(), "load user directory") {
		t.Fatalf("error = %q, want operation context", err)
	}
	for _, raw := range []string{tenantID, "token-secret", "private@example.test", "Private Name", "mocked-response-body"} {
		if strings.Contains(err.Error(), raw) {
			t.Errorf("error contains secret %q: %q", raw, err)
		}
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want no partial report", output.String())
	}
}

func TestServiceRunOrdersFindingsDeterministically(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: "tenant-b", UserID: "subject-z", RowCount: 3},
		{Table: "listing_store", TenantID: "tenant-a", UserID: "subject-x", RowCount: 1},
		{Table: "listing_category", TenantID: "tenant-a", UserID: "subject-y", RowCount: 2},
	}}
	directory := &recordingUserDirectory{}
	var output bytes.Buffer

	_ = NewService(repository, directory, nil, &output).Run(context.Background())
	want := "" +
		"status=blocked table=listing_category tenant=sha256:80a707af7dc7 owner=sha256:1ffc71997096 rows=2 reason=unknown_subject\n" +
		"status=blocked table=listing_store tenant=sha256:80a707af7dc7 owner=sha256:c0f704a39dcc rows=1 reason=unknown_subject\n" +
		"status=blocked table=listing_store tenant=sha256:df6b6a5f230e owner=sha256:695654276845 rows=3 reason=unknown_subject\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestServiceRunRedactsAllRawIdentityAndResponseValues(t *testing.T) {
	t.Parallel()

	const (
		tenantID = "tenant-private@example.test"
		userID   = "subject-token-secret-Private Name-mocked-response-body"
	)
	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: tenantID, UserID: userID, RowCount: 7},
	}}
	directory := &recordingUserDirectory{}
	var output bytes.Buffer

	err := NewService(repository, directory, nil, &output).Run(context.Background())
	var unknownOwners *ErrUnknownOwners
	if !errors.As(err, &unknownOwners) {
		t.Fatalf("Run error = %v, want *ErrUnknownOwners", err)
	}
	report := output.String()
	if !strings.Contains(report, "status=blocked table=listing_store") || !strings.Contains(report, "rows=7 reason=unknown_subject") {
		t.Fatalf("output = %q, want safe finding fields", report)
	}
	for _, raw := range []string{tenantID, userID, "token-secret", "private@example.test", "Private Name", "mocked-response-body"} {
		if strings.Contains(report, raw) {
			t.Errorf("output contains raw value %q: %q", raw, report)
		}
	}
}

func TestFingerprintTrimsAndUsesFirstSixSHA256Bytes(t *testing.T) {
	t.Parallel()

	if got, want := fingerprint("  subject-x  "), "sha256:c0f704a39dcc"; got != want {
		t.Fatalf("fingerprint = %q, want %q", got, want)
	}
}

func TestServiceRunResolvesLegacyNumericTenantBeforeDirectoryComparison(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: "101", TenantDomain: TenantDomainLegacyNumeric, UserID: "subject-x", RowCount: 2},
	}}
	resolver := &recordingLegacyTenantResolver{organizations: map[int64]string{101: "org-X"}}
	directory := &recordingUserDirectory{usersByTenant: map[string][]userdirectory.User{
		"org-X": {{Subject: "subject-x", TenantID: "org-X"}},
	}}
	var output bytes.Buffer

	err := NewService(repository, directory, resolver, &output).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want no findings", output.String())
	}
	if want := []int64{101}; !reflect.DeepEqual(resolver.calls, want) {
		t.Fatalf("resolver calls = %#v, want %#v", resolver.calls, want)
	}
	if want := []string{"org-X"}; !reflect.DeepEqual(directory.calls, want) {
		t.Fatalf("directory calls = %#v, want %#v", directory.calls, want)
	}
}

func TestServiceRunRejectsLegacyOwnerSubjectFromDifferentOrganization(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: "101", TenantDomain: TenantDomainLegacyNumeric, UserID: "subject-x", RowCount: 1},
	}}
	resolver := &recordingLegacyTenantResolver{organizations: map[int64]string{101: "org-X"}}
	directory := &recordingUserDirectory{usersByTenant: map[string][]userdirectory.User{
		"org-X": {{Subject: "subject-x", TenantID: "org-Y"}},
	}}
	var output bytes.Buffer

	err := NewService(repository, directory, resolver, &output).Run(context.Background())
	var unknownOwners *ErrUnknownOwners
	if !errors.As(err, &unknownOwners) || unknownOwners.Count != 1 {
		t.Fatalf("Run error = %v, want one unknown owner", err)
	}
	if !strings.Contains(output.String(), "reason=unknown_subject") {
		t.Fatalf("output = %q, want unknown subject finding", output.String())
	}
	for _, raw := range []string{"101", "org-X", "org-Y", "subject-x"} {
		if strings.Contains(output.String(), raw) {
			t.Errorf("output contains raw identifier %q: %q", raw, output.String())
		}
	}
}

func TestServiceRunFailsClosedWhenLegacyTenantCannotBeResolved(t *testing.T) {
	t.Parallel()

	const resolverSecret = "resolver-secret private@example.test"
	tests := []struct {
		name     string
		resolver *recordingLegacyTenantResolver
	}{
		{name: "unmapped", resolver: &recordingLegacyTenantResolver{}},
		{name: "resolver failure", resolver: &recordingLegacyTenantResolver{errors: map[int64]error{101: errors.New(resolverSecret + " 101")}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := stubOwnerRepository{owners: []PersistedOwner{
				{Table: "listing_kit_tasks", TenantID: "org-direct", UserID: "subject-direct", RowCount: 1},
				{Table: "listing_store", TenantID: "101", TenantDomain: TenantDomainLegacyNumeric, UserID: "subject-secret", RowCount: 1},
			}}
			directory := &recordingUserDirectory{}
			var output bytes.Buffer

			err := NewService(repository, directory, test.resolver, &output).Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), "resolve tenant organization") {
				t.Fatalf("Run error = %v, want tenant organization resolution failure", err)
			}
			for _, raw := range []string{"101", resolverSecret, "private@example.test"} {
				if strings.Contains(err.Error(), raw) {
					t.Errorf("error contains sensitive value %q: %q", raw, err)
				}
			}
			if len(directory.calls) != 0 {
				t.Fatalf("directory calls = %#v, want none", directory.calls)
			}
			if output.Len() != 0 {
				t.Fatalf("output = %q, want no partial report", output.String())
			}
		})
	}
}

func TestServiceRunCachesOrganizationResolutionPerDistinctLegacyTenant(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_store", TenantID: "101", TenantDomain: TenantDomainLegacyNumeric, UserID: "subject-x", RowCount: 1},
		{Table: "listing_category", TenantID: "0101", TenantDomain: TenantDomainLegacyNumeric, UserID: "subject-x", RowCount: 1},
		{Table: "listing_filter_rule", TenantID: "202", TenantDomain: TenantDomainLegacyNumeric, UserID: "subject-y", RowCount: 1},
	}}
	resolver := &recordingLegacyTenantResolver{organizations: map[int64]string{101: "org-X", 202: "org-Y"}}
	directory := &recordingUserDirectory{usersByTenant: map[string][]userdirectory.User{
		"org-X": {{Subject: "subject-x", TenantID: "org-X"}},
		"org-Y": {{Subject: "subject-y", TenantID: "org-Y"}},
	}}

	err := NewService(repository, directory, resolver, io.Discard).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if want := []int64{101, 202}; !reflect.DeepEqual(resolver.calls, want) {
		t.Fatalf("resolver calls = %#v, want %#v", resolver.calls, want)
	}
}

type stubOwnerRepository struct {
	owners []PersistedOwner
	err    error
}

func (repository stubOwnerRepository) List(context.Context) ([]PersistedOwner, error) {
	return append([]PersistedOwner(nil), repository.owners...), repository.err
}

type recordingUserDirectory struct {
	usersByTenant  map[string][]userdirectory.User
	errorsByTenant map[string]error
	calls          []string
}

func (directory *recordingUserDirectory) ListByTenant(_ context.Context, tenantID string) ([]userdirectory.User, error) {
	directory.calls = append(directory.calls, tenantID)
	if err := directory.errorsByTenant[tenantID]; err != nil {
		return nil, err
	}
	return append([]userdirectory.User(nil), directory.usersByTenant[tenantID]...), nil
}

type recordingLegacyTenantResolver struct {
	organizations map[int64]string
	errors        map[int64]error
	calls         []int64
}

func (resolver *recordingLegacyTenantResolver) ResolveOrganizationID(_ context.Context, legacyTenantID int64) (string, bool, error) {
	resolver.calls = append(resolver.calls, legacyTenantID)
	if err := resolver.errors[legacyTenantID]; err != nil {
		return "", false, err
	}
	organizationID, ok := resolver.organizations[legacyTenantID]
	return organizationID, ok, nil
}
