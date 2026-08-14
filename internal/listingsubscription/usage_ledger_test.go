package listingsubscription

import (
	"errors"
	"testing"
)

func TestReserveUsageInputNormalizesIdentifiersAndClonesMetadata(t *testing.T) {
	metadata := map[string]string{" source ": " studio "}

	got, err := NormalizeAndValidateReserveUsageInput(ReserveUsageInput{
		TenantID:       " tenant-17 ",
		ModuleCode:     " studio ",
		Metric:         " studio_design_jobs_succeeded ",
		Quantity:       1,
		PeriodKey:      " 2026-08 ",
		SourceType:     " design_job ",
		SourceID:       " job-42 ",
		IdempotencyKey: " request-42 ",
		Metadata:       metadata,
	})
	if err != nil {
		t.Fatalf("NormalizeAndValidateReserveUsageInput() error = %v", err)
	}

	if got.TenantID != "tenant-17" || got.ModuleCode != "studio" || got.Metric != "studio_design_jobs_succeeded" || got.PeriodKey != "2026-08" || got.SourceType != "design_job" || got.SourceID != "job-42" || got.IdempotencyKey != "request-42" {
		t.Fatalf("normalized input = %+v, want trimmed identifiers", got)
	}
	metadata[" source "] = "mutated"
	if got.Metadata[" source "] != " studio " {
		t.Fatalf("metadata = %#v, want an independent copy", got.Metadata)
	}
}

func TestReserveUsageInputRejectsInvalidValues(t *testing.T) {
	valid := ReserveUsageInput{
		TenantID:       "tenant-17",
		ModuleCode:     "studio",
		Metric:         "studio_design_jobs_succeeded",
		Quantity:       1,
		PeriodKey:      "2026-08",
		SourceType:     "design_job",
		SourceID:       "job-42",
		IdempotencyKey: "request-42",
	}
	tests := []struct {
		name  string
		input ReserveUsageInput
		field string
	}{
		{name: "blank tenant", input: withReserveUsageInput(valid, func(in *ReserveUsageInput) { in.TenantID = " \t" }), field: "tenant_id"},
		{name: "blank module", input: withReserveUsageInput(valid, func(in *ReserveUsageInput) { in.ModuleCode = " \t" }), field: "module_code"},
		{name: "blank metric", input: withReserveUsageInput(valid, func(in *ReserveUsageInput) { in.Metric = " \t" }), field: "metric"},
		{name: "blank idempotency key", input: withReserveUsageInput(valid, func(in *ReserveUsageInput) { in.IdempotencyKey = " \t" }), field: "idempotency_key"},
		{name: "zero quantity", input: withReserveUsageInput(valid, func(in *ReserveUsageInput) { in.Quantity = 0 }), field: "quantity"},
		{name: "negative job quantity", input: withReserveUsageInput(valid, func(in *ReserveUsageInput) { in.Quantity = -1 }), field: "quantity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeAndValidateReserveUsageInput(tt.input)
			if !errors.Is(err, ErrUsageInvalidInput) {
				t.Fatalf("NormalizeAndValidateReserveUsageInput() error = %v, want ErrUsageInvalidInput", err)
			}
			var validationErr *UsageValidationError
			if !errors.As(err, &validationErr) || validationErr.Field != tt.field {
				t.Fatalf("validation error = %#v, want field %q", err, tt.field)
			}
		})
	}
	t.Run("known count metrics require one", func(t *testing.T) {
		input := valid
		input.Quantity = 2
		_, err := NormalizeAndValidateReserveUsageInput(input)
		if !errors.Is(err, ErrUsageInvalidInput) {
			t.Fatalf("NormalizeAndValidateReserveUsageInput() error = %v, want ErrUsageInvalidInput", err)
		}
		var validationErr *UsageValidationError
		if !errors.As(err, &validationErr) || validationErr.Field != "quantity" {
			t.Fatalf("validation error = %#v, want quantity", err)
		}
	})
}

func TestUsageLedgerRejectsStorageDeltaBelowZero(t *testing.T) {
	err := ValidateProjectedUsage("storage_bytes_current", 5, 3, -9)
	if !errors.Is(err, ErrUsageQuotaExceeded) {
		t.Fatalf("ValidateProjectedUsage() error = %v, want ErrUsageQuotaExceeded", err)
	}
	var quotaErr *UsageQuotaError
	if !errors.As(err, &quotaErr) {
		t.Fatalf("ValidateProjectedUsage() error = %T, want *UsageQuotaError", err)
	}
	if quotaErr.Metric != "storage_bytes_current" || quotaErr.CommittedUsage != 5 || quotaErr.ReservedUsage != 3 || quotaErr.Quantity != -9 {
		t.Fatalf("quota error = %+v, want metric and usage context", quotaErr)
	}
}

func TestUsageLedgerValidatesLifecycleTransitions(t *testing.T) {
	valid := []struct {
		from UsageEventStatus
		to   UsageEventStatus
	}{
		{UsageEventReserved, UsageEventCommitted},
		{UsageEventReserved, UsageEventReleased},
		{UsageEventCommitted, UsageEventReversed},
	}
	for _, tt := range valid {
		if err := ValidateUsageEventTransition(tt.from, tt.to); err != nil {
			t.Errorf("ValidateUsageEventTransition(%q, %q) error = %v", tt.from, tt.to, err)
		}
	}

	invalid := []struct {
		from UsageEventStatus
		to   UsageEventStatus
	}{
		{UsageEventReleased, UsageEventCommitted},
		{UsageEventCommitted, UsageEventReleased},
		{UsageEventReversed, UsageEventReversed},
	}
	for _, tt := range invalid {
		err := ValidateUsageEventTransition(tt.from, tt.to)
		if !errors.Is(err, ErrUsageInvalidTransition) {
			t.Errorf("ValidateUsageEventTransition(%q, %q) error = %v, want ErrUsageInvalidTransition", tt.from, tt.to, err)
		}
	}
}

func withReserveUsageInput(input ReserveUsageInput, mutate func(*ReserveUsageInput)) ReserveUsageInput {
	mutate(&input)
	return input
}
