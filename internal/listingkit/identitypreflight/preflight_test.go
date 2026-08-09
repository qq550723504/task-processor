package identitypreflight

import (
	"bytes"
	"context"
	"errors"
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

	err := NewService(repository, directory, &output).Run(context.Background())
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

	err := NewService(repository, directory, &output).Run(context.Background())
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

func TestServiceRunRejectsSameSubjectFromWrongTenant(t *testing.T) {
	t.Parallel()

	repository := stubOwnerRepository{owners: []PersistedOwner{
		{Table: "listing_kit_tasks", TenantID: "tenant-a", UserID: "subject-x", RowCount: 1},
	}}
	directory := &recordingUserDirectory{usersByTenant: map[string][]userdirectory.User{
		"tenant-a": {{Subject: "subject-x", TenantID: "tenant-b"}},
	}}
	var output bytes.Buffer

	err := NewService(repository, directory, &output).Run(context.Background())
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

	err := NewService(repository, directory, &output).Run(context.Background())
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

	_ = NewService(repository, directory, &output).Run(context.Background())
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

	err := NewService(repository, directory, &output).Run(context.Background())
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
