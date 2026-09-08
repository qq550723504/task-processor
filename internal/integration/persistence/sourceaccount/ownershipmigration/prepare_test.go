package ownershipmigration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"
)

func TestValidatePrepareRequestAcceptsFrozenContractWithoutMutation(t *testing.T) {
	request := validPrepareTestRequest("migration-1")
	want := clonePrepareRequest(t, request)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	validated, err := validatePrepareRequest(ctx, request)
	if err != nil {
		t.Fatalf("validatePrepareRequest() error = %v", err)
	}
	if validated.requestSHA256 == "" {
		t.Fatal("request fingerprint is empty")
	}
	if !reflect.DeepEqual(request, want) {
		t.Fatalf("request mutated:\n got: %#v\nwant: %#v", request, want)
	}
}

func TestValidatePrepareRequestRejectsInvalidAuthorityAndMappingEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PrepareRequest)
	}{
		{name: "missing deadline"},
		{name: "deadline over maximum"},
		{name: "wrong contract version", mutate: func(r *PrepareRequest) { r.ContractVersion++ }},
		{name: "empty key", mutate: func(r *PrepareRequest) { r.IdempotencyKey = "" }},
		{name: "oversized key", mutate: func(r *PrepareRequest) { r.IdempotencyKey = string(make([]byte, 129)) }},
		{name: "source identity mismatch", mutate: func(r *PrepareRequest) { r.SourceID = "different-source" }},
		{name: "tampered A digest", mutate: func(r *PrepareRequest) { r.Preflight.Digest = sixtyFourPrepareHex('f') }},
		{name: "wrong A stage", mutate: func(r *PrepareRequest) {
			r.Preflight.Stage = "prepared_only"
			r.Preflight.Digest = receiptDigest(r.Preflight)
		}},
		{name: "empty accounts", mutate: func(r *PrepareRequest) {
			r.Preflight.Accounts = nil
			r.Preflight.Digest = receiptDigest(r.Preflight)
		}},
		{name: "mapping missing", mutate: func(r *PrepareRequest) {
			r.Preflight.Metadata = r.Preflight.Metadata[1:]
			r.Preflight.Digest = receiptDigest(r.Preflight)
		}},
		{name: "mapping ambiguous", mutate: func(r *PrepareRequest) {
			r.Preflight.Metadata = append(r.Preflight.Metadata, OrganizationMetadata{OrganizationID: "org-c", Value: []byte("101"), Sequence: 3})
			sort.Slice(r.Preflight.Metadata, func(i, j int) bool {
				return r.Preflight.Metadata[i].OrganizationID < r.Preflight.Metadata[j].OrganizationID
			})
			r.Preflight.Digest = receiptDigest(r.Preflight)
		}},
		{name: "removed mapping", mutate: func(r *PrepareRequest) {
			r.Preflight.Metadata[0].OwnerRemoved = true
			r.Preflight.Digest = receiptDigest(r.Preflight)
		}},
		{name: "wrong account owner", mutate: func(r *PrepareRequest) {
			r.Preflight.Accounts[0].OrganizationID = "org-b"
			r.Preflight.Digest = receiptDigest(r.Preflight)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validPrepareTestRequest("migration-1")
			if test.mutate != nil {
				test.mutate(&request)
			}
			ctx := context.Background()
			cancel := func() {}
			switch test.name {
			case "missing deadline":
			case "deadline over maximum":
				ctx, cancel = context.WithTimeout(ctx, maxPrepareDeadline+time.Minute)
			default:
				ctx, cancel = context.WithTimeout(ctx, time.Minute)
			}
			defer cancel()
			if _, err := validatePrepareRequest(ctx, request); err == nil {
				t.Fatal("validatePrepareRequest() error = nil")
			}
		})
	}
}

func validPrepareTestRequest(key string) PrepareRequest {
	observedAt := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
	receipt := Receipt{
		Version:             1,
		Stage:               "preflight_only",
		SnapshotConsistency: "separate_non_atomic_snapshots",
		AccountObservation:  Observation{SourceID: "issue362/source", Database: "issue362", At: observedAt},
		MetadataObservation: Observation{SourceID: "issue362/zitadel", Database: "zitadel", At: observedAt},
		Accounts: []AccountEvidence{
			{OrganizationID: "org-a", ProfileDirectory: prepareTestProfileDirectory("101", "1"), Previous: LegacyAccount{ID: 1, TenantID: 101, Platform: "1688", ProfileRef: "profile-a", Status: 0, Deleted: 0}},
			{OrganizationID: "org-b", ProfileDirectory: prepareTestProfileDirectory("202", "2"), Previous: LegacyAccount{ID: 2, TenantID: 202, Platform: "1688", ProfileRef: "profile-b", Status: 1, Deleted: 0}},
		},
		Metadata: []OrganizationMetadata{
			{OrganizationID: "org-a", Value: []byte("101"), Sequence: 1},
			{OrganizationID: "org-b", Value: []byte("202"), Sequence: 2},
		},
	}
	receipt.Digest = receiptDigest(receipt)
	return PrepareRequest{ContractVersion: PreparedContractVersion, IdempotencyKey: key, SourceID: receipt.AccountObservation.SourceID, Preflight: receipt}
}

func prepareTestProfileDirectory(parts ...string) string {
	root := string(filepath.Separator) + "profiles"
	if runtime.GOOS == "windows" {
		root = `C:\profiles`
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

func clonePrepareRequest(t *testing.T, request PrepareRequest) PrepareRequest {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var cloned PrepareRequest
	if err = json.Unmarshal(data, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func sixtyFourPrepareHex(value byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = value
	}
	return string(result)
}
