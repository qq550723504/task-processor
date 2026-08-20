package a1688

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/listingkit"
	"task-processor/internal/sourceaccount"
)

func TestTaskCommandServicePublicSourceSkipsAccountValidation(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	storeValidator := validStoreAccessValidator()
	accountValidator := &sourceAccountAccessValidatorFake{}

	_, err := NewTaskCommandService(creator, storeValidator, accountValidator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL: "https://detail.1688.com/offer/901.html", Product: commandProduct1688("901"),
		TenantID: "101", UserID: "user-1", SourceAccountID: 0, SheinStoreID: 168811, Platforms: []string{"shein"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if len(accountValidator.calls) != 0 {
		t.Fatalf("source account validator calls = %d, want 0 for public source", len(accountValidator.calls))
	}
	if len(storeValidator.calls) != 1 || storeValidator.calls[0].platform != "SHEIN" {
		t.Fatalf("store validator calls = %+v, want only SHEIN target validation", storeValidator.calls)
	}
}

func TestTaskCommandServiceAccountSourceUsesDedicatedValidator(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	storeValidator := validStoreAccessValidator()
	accountValidator := &sourceAccountAccessValidatorFake{}

	_, err := NewTaskCommandService(creator, storeValidator, accountValidator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL: "https://detail.1688.com/offer/902.html", Product: commandProduct1688("902"),
		TenantID: "101", UserID: "user-1", SourceAccountID: 3001, SheinStoreID: 168811, Platforms: []string{"shein"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if len(accountValidator.calls) != 1 || accountValidator.calls[0] != 3001 {
		t.Fatalf("source account validator calls = %v, want [3001]", accountValidator.calls)
	}
	if len(storeValidator.calls) != 1 || storeValidator.calls[0].platform != "SHEIN" {
		t.Fatalf("store validator calls = %+v, want only SHEIN target validation", storeValidator.calls)
	}
}

func TestTaskCommandServiceMapsDisabledSourceAccountToStableHandoffError(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	storeValidator := validStoreAccessValidator()
	accountValidator := &sourceAccountAccessValidatorFake{err: sourceaccount.NewDisabledError()}

	_, err := NewTaskCommandService(creator, storeValidator, accountValidator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL: "https://detail.1688.com/offer/903.html", Product: commandProduct1688("903"),
		TenantID: "101", UserID: "user-1", SourceAccountID: 3001, SheinStoreID: 168811, Platforms: []string{"shein"},
	})
	if got := listingkit.StoreAccessErrorCode(err); got != sourceaccount.SourceAccountDisabled {
		t.Fatalf("StoreAccessErrorCode() = %q, want %q", got, sourceaccount.SourceAccountDisabled)
	}
}

type sourceAccountAccessValidatorFake struct {
	calls []int64
	err   error
}

func (v *sourceAccountAccessValidatorFake) ValidateSourceAccountAccess(_ context.Context, _, accountID int64) (sourceaccount.Access, error) {
	v.calls = append(v.calls, accountID)
	if v.err != nil {
		return sourceaccount.Access{}, v.err
	}
	return sourceaccount.Access{ID: accountID, TenantID: 101, Platform: sourceaccount.PlatformAlibaba1688, Enabled: true}, nil
}

var _ sourceaccount.AccessValidator = (*sourceAccountAccessValidatorFake)(nil)

func TestTaskCommandServiceRejectsNegativeSourceAccountAsInvalidRequest(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	storeValidator := validStoreAccessValidator()

	_, err := NewTaskCommandService(creator, storeValidator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL: "https://detail.1688.com/offer/904.html", Product: commandProduct1688("904"),
		TenantID: "101", UserID: "user-1", SourceAccountID: -1, SheinStoreID: 168811, Platforms: []string{"shein"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid source_account_id") {
		t.Fatalf("error = %v, want invalid source_account_id request error", err)
	}
	if len(storeValidator.calls) != 0 {
		t.Fatalf("store validator calls = %+v, want no store validation for invalid source account", storeValidator.calls)
	}
}

func TestTaskCommandServiceReturnsSourceUnavailableWhenValidatorMissing(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	storeValidator := validStoreAccessValidator()
	_, err := NewTaskCommandService(creator, storeOnlyAccessValidator{delegate: storeValidator}).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL: "https://detail.1688.com/offer/905.html", Product: commandProduct1688("905"),
		TenantID: "101", UserID: "user-1", SourceAccountID: 3001, SheinStoreID: 168811, Platforms: []string{"shein"},
	})
	if got := listingkit.StoreAccessErrorCode(err); got != sourceaccount.SourceAccountUnavailable {
		t.Fatalf("StoreAccessErrorCode() = %q, want %q", got, sourceaccount.SourceAccountUnavailable)
	}
}

type storeOnlyAccessValidator struct{ delegate *storeAccessValidatorFake }

func (v storeOnlyAccessValidator) ValidateStoreAccess(ctx context.Context, tenantID, storeID int64, platform string) (listingkit.StoreAccess, error) {
	return v.delegate.ValidateStoreAccess(ctx, tenantID, storeID, platform)
}
