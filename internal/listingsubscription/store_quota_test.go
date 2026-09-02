package listingsubscription

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestStoreQuotaLedgerContractHasOnlyStoreAllocationOperations(t *testing.T) {
	ledgerType := reflect.TypeOf((*StoreQuotaLedger)(nil)).Elem()
	want := map[string]reflect.Type{
		"Reserve": reflect.TypeOf(func(context.Context, StoreQuotaReserveInput) (StoreQuotaReserveResult, error) {
			return StoreQuotaReserveResult{}, nil
		}),
		"RenewReservation": reflect.TypeOf(func(context.Context, StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error) {
			return StoreQuotaTransitionResult{}, nil
		}),
		"Commit": reflect.TypeOf(func(context.Context, StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error) {
			return StoreQuotaTransitionResult{}, nil
		}),
		"ReleaseReservation": reflect.TypeOf(func(context.Context, StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error) {
			return StoreQuotaTransitionResult{}, nil
		}),
		"Deallocate": reflect.TypeOf(func(context.Context, StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error) {
			return StoreQuotaTransitionResult{}, nil
		}),
		"GetByRequestKey": reflect.TypeOf(func(context.Context, string, string) (*StoreQuotaAllocation, error) { return nil, nil }),
		"Summary":         reflect.TypeOf(func(context.Context, string) (StoreQuotaSummary, error) { return StoreQuotaSummary{}, nil }),
	}
	if ledgerType.NumMethod() != len(want) {
		t.Fatalf("StoreQuotaLedger methods = %d, want %d", ledgerType.NumMethod(), len(want))
	}
	for name, signature := range want {
		method, ok := ledgerType.MethodByName(name)
		if !ok || method.Type != signature {
			t.Fatalf("StoreQuotaLedger.%s = %v, want %v", name, method.Type, signature)
		}
	}
}

func TestStoreQuotaInputRejectsNonCanonicalIdentity(t *testing.T) {
	input := StoreQuotaReserveInput{OrganizationID: " org-a", RequestKey: uuid.NewString(), ActorSubject: "actor-1"}
	if _, err := NormalizeAndValidateStoreQuotaReserveInput(input); !errors.Is(err, ErrStoreQuotaInvalidInput) {
		t.Fatalf("NormalizeAndValidateStoreQuotaReserveInput() error = %v, want invalid input", err)
	}
	input.OrganizationID = "org-a"
	input.RequestKey = "{" + input.RequestKey + "}"
	if _, err := NormalizeAndValidateStoreQuotaReserveInput(input); !errors.Is(err, ErrStoreQuotaInvalidInput) {
		t.Fatalf("noncanonical request key error = %v, want invalid input", err)
	}
}

func TestStoreQuotaNilRepositoryReturnsConfiguredError(t *testing.T) {
	ledger := NewGormStoreQuotaLedger(nil)
	input := StoreQuotaReserveInput{OrganizationID: "org-a", RequestKey: uuid.NewString(), ActorSubject: "actor-1"}
	if _, err := ledger.Reserve(context.Background(), input); !errors.Is(err, ErrStoreQuotaNotConfigured) {
		t.Fatalf("nil ledger Reserve() error = %v, want not configured", err)
	}
	if _, err := ledger.Summary(context.Background(), "org-a"); !errors.Is(err, ErrStoreQuotaNotConfigured) {
		t.Fatalf("nil ledger Summary() error = %v, want not configured", err)
	}
	if _, err := ledger.GetByRequestKey(context.Background(), "org-a", input.RequestKey); !errors.Is(err, ErrStoreQuotaNotConfigured) {
		t.Fatalf("nil ledger GetByRequestKey() error = %v, want not configured", err)
	}
}
