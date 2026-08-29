package effectpolicy

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"task-processor/internal/imageagent"
)

func TestReserveProviderDecisionMatrix(t *testing.T) {
	reservation := providerPolicyReservation()
	accounting := providerAccounting(reservation.Policy)
	accounting.Committed.Images = 1
	accounting.Reserved.Images = 1

	claimed := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotBudgetReserved)
	notDispatched := providerAttempt(reservation, imageagent.SlotEffectV3ProviderNotDispatched, imageagent.SlotBudgetReleased)
	released := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotBudgetReleased)
	mismatch := claimed
	mismatch.InputFingerprint = "other-input"

	tests := []struct {
		name           string
		current        *imageagent.SlotEffectV3Attempt
		reservation    imageagent.SlotEffectV3Reservation
		accounting     AccountingSnapshot
		wantErr        error
		wantAcquired   bool
		wantChanged    bool
		wantAccounting bool
		wantPhase      imageagent.SlotEffectV3Phase
		wantBudget     imageagent.SlotBudgetStatus
		wantReserved   int64
	}{
		{name: "new reservation", reservation: reservation, accounting: accounting, wantAcquired: true, wantChanged: true, wantAccounting: true, wantPhase: imageagent.SlotEffectV3ProviderClaimed, wantBudget: imageagent.SlotBudgetReserved, wantReserved: 3},
		{name: "exact repeat", current: &claimed, reservation: reservation, accounting: accounting, wantPhase: imageagent.SlotEffectV3ProviderClaimed, wantBudget: imageagent.SlotBudgetReserved, wantReserved: 1},
		{name: "reservation mismatch", current: &mismatch, reservation: reservation, accounting: accounting, wantErr: imageagent.ErrRevisionConflict},
		{name: "provider not dispatched redispatch", current: &notDispatched, reservation: reservation, accounting: providerAccounting(reservation.Policy), wantAcquired: true, wantChanged: true, wantAccounting: true, wantPhase: imageagent.SlotEffectV3ProviderClaimed, wantBudget: imageagent.SlotBudgetReserved, wantReserved: 2},
		{name: "released budget reacquisition", current: &released, reservation: reservation, accounting: providerAccounting(reservation.Policy), wantAcquired: true, wantChanged: true, wantAccounting: true, wantPhase: imageagent.SlotEffectV3ProviderClaimed, wantBudget: imageagent.SlotBudgetReserved, wantReserved: 2},
		{name: "persisted policy mismatch", reservation: reservation, accounting: providerAccounting(imageagent.BudgetPolicy{Images: imageagent.Limit{Enabled: true, Value: 6}}), wantErr: imageagent.ErrRevisionConflict},
		{name: "budget exceeded", reservation: reservation, accounting: AccountingSnapshot{Policy: reservation.Policy, Committed: imageagent.UsageVector{Images: 4}}, wantErr: imageagent.ErrBudgetExceeded},
		{name: "quote validation", reservation: providerReservationWithInvalidQuote(), accounting: accounting, wantErr: imageagent.ErrValidation},
		{name: "accounting overflow", reservation: reservation, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: math.MaxInt64}}, wantErr: imageagent.ErrBudgetOverflow},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := ReserveProvider(test.current, test.reservation, test.accounting)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ReserveProvider() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Acquired != test.wantAcquired || decision.Changed != test.wantChanged || decision.AccountingChanged != test.wantAccounting {
				t.Fatalf("ReserveProvider() flags = acquired %t, changed %t, accounting changed %t", decision.Acquired, decision.Changed, decision.AccountingChanged)
			}
			if decision.Attempt.Phase != test.wantPhase || decision.Attempt.BudgetStatus != test.wantBudget {
				t.Fatalf("ReserveProvider() attempt phase/status = %q/%q, want %q/%q", decision.Attempt.Phase, decision.Attempt.BudgetStatus, test.wantPhase, test.wantBudget)
			}
			if decision.Accounting.Reserved.Images != test.wantReserved {
				t.Fatalf("ReserveProvider() reserved images = %d, want %d", decision.Accounting.Reserved.Images, test.wantReserved)
			}
		})
	}
}

func TestRecordProviderNotDispatchedReleasesOnlyProvenReservation(t *testing.T) {
	reservation := providerPolicyReservation()
	reserved := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotBudgetReserved)
	alreadyReleased := providerAttempt(reservation, imageagent.SlotEffectV3ProviderNotDispatched, imageagent.SlotBudgetReleased)
	unproven := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotBudgetReleased)
	unbudgetedReservation := providerUnbudgetedReservation()
	unbudgeted := providerAttempt(unbudgetedReservation, imageagent.SlotEffectV3ProviderClaimed, "")
	mismatch := reserved
	mismatch.IdempotencyKey = "other-key"

	tests := []struct {
		name           string
		current        imageagent.SlotEffectV3Attempt
		reservation    imageagent.SlotEffectV3Reservation
		accounting     AccountingSnapshot
		wantErr        error
		wantChanged    bool
		wantAccounting bool
		wantPhase      imageagent.SlotEffectV3Phase
		wantBudget     imageagent.SlotBudgetStatus
		wantReserved   int64
	}{
		{name: "reserved budget is released", current: reserved, reservation: reservation, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 3, CostMicros: 80}}, wantChanged: true, wantAccounting: true, wantPhase: imageagent.SlotEffectV3ProviderNotDispatched, wantBudget: imageagent.SlotBudgetReleased, wantReserved: 1},
		{name: "exact repeat", current: alreadyReleased, reservation: reservation, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 1}}, wantPhase: imageagent.SlotEffectV3ProviderNotDispatched, wantBudget: imageagent.SlotBudgetReleased, wantReserved: 1},
		{name: "unproven released budget", current: unproven, reservation: reservation, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 3}}, wantErr: imageagent.ErrRevisionConflict},
		{name: "unbudgeted provider", current: unbudgeted, reservation: unbudgetedReservation, accounting: AccountingSnapshot{}, wantChanged: true, wantPhase: imageagent.SlotEffectV3ProviderNotDispatched},
		{name: "reservation mismatch", current: mismatch, reservation: reservation, accounting: providerAccounting(reservation.Policy), wantErr: imageagent.ErrRevisionConflict},
		{name: "policy drift still releases reserved budget", current: reserved, reservation: reservation, accounting: AccountingSnapshot{Policy: imageagent.BudgetPolicy{Images: imageagent.Limit{Enabled: true, Value: 6}}, Reserved: imageagent.UsageVector{Images: 3, CostMicros: 80}}, wantChanged: true, wantAccounting: true, wantPhase: imageagent.SlotEffectV3ProviderNotDispatched, wantBudget: imageagent.SlotBudgetReleased, wantReserved: 1},
		{name: "reserved accounting underflow", current: reserved, reservation: reservation, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 1}}, wantErr: imageagent.ErrBudgetOverflow},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := RecordProviderNotDispatched(test.current, test.reservation, test.accounting)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("RecordProviderNotDispatched() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Changed != test.wantChanged || decision.AccountingChanged != test.wantAccounting {
				t.Fatalf("RecordProviderNotDispatched() flags = changed %t, accounting changed %t", decision.Changed, decision.AccountingChanged)
			}
			if decision.Attempt.Phase != test.wantPhase || decision.Attempt.BudgetStatus != test.wantBudget || decision.Accounting.Reserved.Images != test.wantReserved {
				t.Fatalf("RecordProviderNotDispatched() result = phase %q, status %q, reserved %d", decision.Attempt.Phase, decision.Attempt.BudgetStatus, decision.Accounting.Reserved.Images)
			}
		})
	}
}

func TestSettleProviderDecisionMatrix(t *testing.T) {
	reservation := providerPolicyReservation()
	receipt := imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 1, CostMicros: 30}, ProviderRequestIDs: []string{"request-1"}, CostBasis: imageagent.UsageCostActual}
	reserved := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotBudgetReserved)
	committed := reserved
	committed.BudgetStatus = imageagent.SlotBudgetCommitted
	committed.Receipt = receipt
	startedAt := time.Date(2026, time.August, 29, 8, 0, 0, 0, time.UTC)
	observedAt := startedAt.Add(5 * time.Minute)

	tests := []struct {
		name           string
		current        imageagent.SlotEffectV3Attempt
		receipt        imageagent.SlotUsageReceipt
		accounting     AccountingSnapshot
		observedAt     time.Time
		wantErr        error
		wantChanged    bool
		wantAccounting bool
		wantReserved   int64
		wantCommitted  imageagent.UsageVector
		wantElapsed    time.Duration
	}{
		{name: "settlement", current: reserved, receipt: receipt, accounting: AccountingSnapshot{Policy: reservation.Policy, Committed: imageagent.UsageVector{Images: 2, CostMicros: 10}, Reserved: imageagent.UsageVector{Images: 4, CostMicros: 80}, Elapsed: time.Minute, StartedAt: startedAt}, observedAt: observedAt, wantChanged: true, wantAccounting: true, wantReserved: 2, wantCommitted: imageagent.UsageVector{Images: 3, CostMicros: 40}, wantElapsed: 5 * time.Minute},
		{name: "settlement preserves greater elapsed", current: reserved, receipt: receipt, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: reservation.Quote.Maximum, Elapsed: 7 * time.Minute, StartedAt: startedAt}, observedAt: observedAt, wantChanged: true, wantAccounting: true, wantCommitted: receipt.Actual, wantElapsed: 7 * time.Minute},
		{name: "committed exact repeat", current: committed, receipt: receipt, accounting: AccountingSnapshot{Policy: reservation.Policy, Committed: receipt.Actual, Elapsed: time.Minute}, observedAt: observedAt, wantCommitted: receipt.Actual, wantElapsed: time.Minute},
		{name: "committed conflicting repeat", current: committed, receipt: imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 2}, CostBasis: imageagent.UsageCostActual}, accounting: providerAccounting(reservation.Policy), observedAt: observedAt, wantErr: imageagent.ErrRevisionConflict},
		{name: "invalid cost basis", current: reserved, receipt: imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 1}}, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: reservation.Quote.Maximum}, observedAt: observedAt, wantErr: imageagent.ErrValidation},
		{name: "receipt exceeds quote", current: reserved, receipt: imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 3}, CostBasis: imageagent.UsageCostActual}, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: reservation.Quote.Maximum}, observedAt: observedAt, wantErr: imageagent.ErrRevisionConflict},
		{name: "reserved accounting underflow", current: reserved, receipt: receipt, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 1, CostMicros: 80}}, observedAt: observedAt, wantErr: imageagent.ErrBudgetOverflow},
		{name: "committed accounting overflow", current: reserved, receipt: receipt, accounting: AccountingSnapshot{Policy: reservation.Policy, Committed: imageagent.UsageVector{Images: math.MaxInt64}, Reserved: reservation.Quote.Maximum}, observedAt: observedAt, wantErr: imageagent.ErrBudgetOverflow},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := SettleProvider(test.current, reservation, test.receipt, test.accounting, test.observedAt)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("SettleProvider() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Changed != test.wantChanged || decision.AccountingChanged != test.wantAccounting {
				t.Fatalf("SettleProvider() flags = changed %t, accounting changed %t", decision.Changed, decision.AccountingChanged)
			}
			if decision.Attempt.BudgetStatus != imageagent.SlotBudgetCommitted {
				t.Fatalf("SettleProvider() budget status = %q", decision.Attempt.BudgetStatus)
			}
			if decision.Accounting.Reserved.Images != test.wantReserved || decision.Accounting.Committed != test.wantCommitted || decision.Accounting.Elapsed != test.wantElapsed {
				t.Fatalf("SettleProvider() accounting = %#v", decision.Accounting)
			}
		})
	}
}

func TestReleaseAndUnknownProviderBudgetDecisionMatrix(t *testing.T) {
	reservation := providerPolicyReservation()
	reserved := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotBudgetReserved)
	released := reserved
	released.BudgetStatus = imageagent.SlotBudgetReleased
	unknown := reserved
	unknown.BudgetStatus = imageagent.SlotBudgetUnknown

	tests := []struct {
		name           string
		apply          func(imageagent.SlotEffectV3Attempt, imageagent.SlotEffectV3Reservation, AccountingSnapshot) (AccountingDecision, error)
		current        imageagent.SlotEffectV3Attempt
		accounting     AccountingSnapshot
		wantErr        error
		wantStatus     imageagent.SlotBudgetStatus
		wantChanged    bool
		wantAccounting bool
		wantReserved   int64
	}{
		{name: "release", apply: ReleaseProviderBudget, current: reserved, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 3, CostMicros: 80}}, wantStatus: imageagent.SlotBudgetReleased, wantChanged: true, wantAccounting: true, wantReserved: 1},
		{name: "release exact repeat", apply: ReleaseProviderBudget, current: released, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 1}}, wantStatus: imageagent.SlotBudgetReleased, wantReserved: 1},
		{name: "release conflicting state", apply: ReleaseProviderBudget, current: unknown, accounting: providerAccounting(reservation.Policy), wantErr: imageagent.ErrRevisionConflict},
		{name: "release underflow", apply: ReleaseProviderBudget, current: reserved, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 1}}, wantErr: imageagent.ErrBudgetOverflow},
		{name: "unknown", apply: MarkProviderBudgetUnknown, current: reserved, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 3, CostMicros: 80}}, wantStatus: imageagent.SlotBudgetUnknown, wantChanged: true, wantReserved: 3},
		{name: "unknown underflow", apply: MarkProviderBudgetUnknown, current: reserved, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 1, CostMicros: 80}}, wantErr: imageagent.ErrBudgetOverflow},
		{name: "unknown exact repeat", apply: MarkProviderBudgetUnknown, current: unknown, accounting: AccountingSnapshot{Policy: reservation.Policy, Reserved: imageagent.UsageVector{Images: 3}}, wantStatus: imageagent.SlotBudgetUnknown, wantReserved: 3},
		{name: "unknown conflicting state", apply: MarkProviderBudgetUnknown, current: released, accounting: providerAccounting(reservation.Policy), wantErr: imageagent.ErrRevisionConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := test.apply(test.current, reservation, test.accounting)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("budget decision error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if decision.Attempt.BudgetStatus != test.wantStatus || decision.Changed != test.wantChanged || decision.AccountingChanged != test.wantAccounting || decision.Accounting.Reserved.Images != test.wantReserved {
				t.Fatalf("budget decision = status %q, changed %t, accounting changed %t, reserved %d", decision.Attempt.BudgetStatus, decision.Changed, decision.AccountingChanged, decision.Accounting.Reserved.Images)
			}
		})
	}
}

func TestReserveProviderExactRepeatPreservesNonNilEmptySlices(t *testing.T) {
	reservation := providerUnbudgetedReservation()
	current := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, "")
	current.StagingManifest.Assets = []imageagent.StagedAssetRef{}
	current.FinalManifest.Assets = []imageagent.PublishedAssetRef{}
	current.Published.Candidates = []imageagent.SlotEffectV3AssetCandidate{}
	current.Quote.Operations = []imageagent.SlotUsageOperation{}
	current.Receipt.ProviderRequestIDs = []string{}

	decision, err := ReserveProvider(&current, reservation, AccountingSnapshot{})
	if err != nil {
		t.Fatalf("ReserveProvider() error = %v", err)
	}
	if decision.Changed {
		t.Fatal("exact repeat unexpectedly changed")
	}
	if decision.Attempt.StagingManifest.Assets == nil || decision.Attempt.FinalManifest.Assets == nil ||
		decision.Attempt.Published.Candidates == nil || decision.Attempt.Quote.Operations == nil || decision.Attempt.Receipt.ProviderRequestIDs == nil {
		t.Fatalf("exact repeat lost non-nil empty slice state: %#v", decision.Attempt)
	}
}

func TestProviderDecisionsDoNotMutateInputs(t *testing.T) {
	reservation := providerPolicyReservation()
	current := providerAttempt(reservation, imageagent.SlotEffectV3ProviderClaimed, imageagent.SlotBudgetReserved)
	current.Receipt.ProviderRequestIDs = []string{"original-receipt"}
	accounting := AccountingSnapshot{Policy: reservation.Policy, Reserved: reservation.Quote.Maximum}
	receipt := imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: 1}, ProviderRequestIDs: []string{"original-request"}, CostBasis: imageagent.UsageCostActual}
	wantReservation := cloneProviderReservation(reservation)
	wantCurrent := cloneSlotEffectV3Attempt(current)
	wantReceipt := cloneProviderReceipt(receipt)
	wantAccounting := accounting

	reservationDecision, err := ReserveProvider(nil, reservation, accounting)
	if err != nil {
		t.Fatalf("ReserveProvider() error = %v", err)
	}
	reservationDecision.Attempt.Quote.Operations[0].Name = "mutated-reservation-operation"

	decision, err := SettleProvider(current, reservation, receipt, accounting, time.Now().UTC())
	if err != nil {
		t.Fatalf("SettleProvider() error = %v", err)
	}
	decision.Attempt.Quote.Operations[0].Name = "mutated-operation"
	decision.Attempt.Receipt.ProviderRequestIDs[0] = "mutated-request"

	if !reflect.DeepEqual(reservation, wantReservation) {
		t.Fatalf("reservation mutated: got %#v, want %#v", reservation, wantReservation)
	}
	if !reflect.DeepEqual(current, wantCurrent) {
		t.Fatalf("current attempt mutated: got %#v, want %#v", current, wantCurrent)
	}
	if !reflect.DeepEqual(receipt, wantReceipt) {
		t.Fatalf("receipt mutated: got %#v, want %#v", receipt, wantReceipt)
	}
	if accounting != wantAccounting {
		t.Fatalf("accounting mutated: got %#v, want %#v", accounting, wantAccounting)
	}
	if reservation.Quote.Operations[0].Name != "generate" || receipt.ProviderRequestIDs[0] != "original-request" {
		t.Fatal("decision aliases reservation or receipt slices")
	}
}

func providerPolicyReservation() imageagent.SlotEffectV3Reservation {
	return imageagent.SlotEffectV3Reservation{
		Identity: imageagent.SlotExternalEffectIdentity{
			RunScope:     imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-a"},
			PlanRevision: 1,
			SlotID:       "slot-a",
			Attempt:      1,
		},
		IdempotencyKey:   "provider-key-a",
		InputFingerprint: "input-a",
		Policy: imageagent.BudgetPolicy{
			Images:     imageagent.Limit{Enabled: true, Value: 5},
			CostMicros: imageagent.Limit{Enabled: true, Value: 200},
		},
		Quote: imageagent.SlotUsageQuote{
			Maximum: imageagent.UsageVector{Images: 2, CostMicros: 80},
			Operations: []imageagent.SlotUsageOperation{{
				Name: "generate", Provider: "provider-a", Model: "model-a", PricingVersion: "price-v1",
				Fingerprint: "operation-a", Maximum: imageagent.UsageVector{Images: 2, CostMicros: 80}, MaximumOutputs: 2,
			}},
			PricingVersion: "price-v1",
			Fingerprint:    "quote-a",
		},
	}
}

func providerUnbudgetedReservation() imageagent.SlotEffectV3Reservation {
	reservation := providerPolicyReservation()
	reservation.Policy = imageagent.BudgetPolicy{}
	reservation.Quote = imageagent.SlotUsageQuote{}
	return reservation
}

func providerReservationWithInvalidQuote() imageagent.SlotEffectV3Reservation {
	reservation := providerPolicyReservation()
	reservation.Quote.Operations = nil
	return reservation
}

func providerAccounting(policy imageagent.BudgetPolicy) AccountingSnapshot {
	return AccountingSnapshot{Policy: policy}
}

func providerAttempt(reservation imageagent.SlotEffectV3Reservation, phase imageagent.SlotEffectV3Phase, status imageagent.SlotBudgetStatus) imageagent.SlotEffectV3Attempt {
	return imageagent.SlotEffectV3Attempt{
		Identity: reservation.Identity, IdempotencyKey: reservation.IdempotencyKey, InputFingerprint: reservation.InputFingerprint,
		Phase: phase, BudgetStatus: status, Policy: reservation.Policy, Quote: cloneProviderQuote(reservation.Quote),
	}
}

func cloneProviderReservation(reservation imageagent.SlotEffectV3Reservation) imageagent.SlotEffectV3Reservation {
	reservation.Quote = cloneProviderQuote(reservation.Quote)
	return reservation
}

func cloneProviderQuote(quote imageagent.SlotUsageQuote) imageagent.SlotUsageQuote {
	quote.Operations = append([]imageagent.SlotUsageOperation(nil), quote.Operations...)
	return quote
}

func cloneProviderReceipt(receipt imageagent.SlotUsageReceipt) imageagent.SlotUsageReceipt {
	receipt.ProviderRequestIDs = append([]string(nil), receipt.ProviderRequestIDs...)
	return receipt
}
