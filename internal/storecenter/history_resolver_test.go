package storecenter

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestValidateLegacyServiceHistoryResolutionKeepsThreeOutcomesDistinct(t *testing.T) {
	started := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC)
	expires := started.Add(30 * 24 * time.Hour)
	tests := []struct {
		name       string
		resolution LegacyServiceHistoryResolution
		valid      bool
	}{
		{
			name:       "found carries exact period and source token",
			resolution: LegacyServiceHistoryResolution{Status: HistoryFound, SourceIdentity: "legacy-store:42", SourceSnapshotToken: "version:7", ServiceStartedAt: &started, ServiceExpiresAt: &expires},
			valid:      true,
		},
		{
			name:       "confirmed absent carries source token without invented period",
			resolution: LegacyServiceHistoryResolution{Status: HistoryConfirmedAbsent, SourceIdentity: "legacy-store:42", SourceSnapshotToken: "version:7"},
			valid:      true,
		},
		{
			name:       "unavailable carries retry information",
			resolution: LegacyServiceHistoryResolution{Status: HistoryUnavailable, RetryAfter: time.Minute, Cause: errors.New("history timeout")},
			valid:      true,
		},
		{name: "found without token", resolution: LegacyServiceHistoryResolution{Status: HistoryFound, SourceIdentity: "legacy-store:42", ServiceStartedAt: &started, ServiceExpiresAt: &expires}},
		{name: "found without timestamps", resolution: LegacyServiceHistoryResolution{Status: HistoryFound, SourceIdentity: "legacy-store:42", SourceSnapshotToken: "version:7"}},
		{name: "confirmed absent with invented timestamps", resolution: LegacyServiceHistoryResolution{Status: HistoryConfirmedAbsent, SourceIdentity: "legacy-store:42", SourceSnapshotToken: "version:7", ServiceStartedAt: &started, ServiceExpiresAt: &expires}},
		{name: "unavailable without cause", resolution: LegacyServiceHistoryResolution{Status: HistoryUnavailable, RetryAfter: time.Minute}},
		{name: "unavailable with zero retry", resolution: LegacyServiceHistoryResolution{Status: HistoryUnavailable, Cause: errors.New("history timeout")}},
		{name: "unknown status", resolution: LegacyServiceHistoryResolution{Status: LegacyHistoryStatus("unknown")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLegacyServiceHistoryResolution(tt.resolution)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateLegacyServiceHistoryResolution() error = %v, valid = %t", err, tt.valid)
			}
			if !tt.valid && !errors.Is(err, ErrInvalidLegacyHistoryResolution) {
				t.Fatalf("error = %v, want ErrInvalidLegacyHistoryResolution", err)
			}
		})
	}
}

func TestLegacyHistoryResolverContractExposesResolverAndFreezeBoundaries(t *testing.T) {
	var _ LegacyServiceHistoryResolver = recordingHistoryResolver{}
	var _ LegacyServiceHistoryFreeze = recordingHistoryFreeze{}
	started := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC)
	expires := started.Add(time.Hour)
	found := LegacyServiceHistoryResolution{Status: HistoryFound, SourceIdentity: "legacy-store:42", SourceSnapshotToken: "version:1", ServiceStartedAt: &started, ServiceExpiresAt: &expires}
	if err := ValidateLegacyServiceHistoryFreeze(found, recordingHistoryFreeze{}); err != nil {
		t.Fatalf("matching freeze rejected: %v", err)
	}
	if err := ValidateLegacyServiceHistoryFreeze(found, nil); !errors.Is(err, ErrInvalidLegacyHistoryFreeze) {
		t.Fatalf("missing freeze error = %v, want ErrInvalidLegacyHistoryFreeze", err)
	}
	if err := ValidateLegacyServiceHistoryFreeze(found, mismatchedHistoryFreeze{}); !errors.Is(err, ErrInvalidLegacyHistoryFreeze) {
		t.Fatalf("mismatched freeze error = %v, want ErrInvalidLegacyHistoryFreeze", err)
	}
	if err := ValidateLegacyServiceHistoryFreeze(LegacyServiceHistoryResolution{Status: HistoryUnavailable, RetryAfter: time.Minute, Cause: errors.New("timeout")}, recordingHistoryFreeze{}); !errors.Is(err, ErrInvalidLegacyHistoryFreeze) {
		t.Fatalf("unavailable freeze error = %v, want ErrInvalidLegacyHistoryFreeze", err)
	}
}

type recordingHistoryResolver struct{}

func (recordingHistoryResolver) Resolve(_ context.Context, _ StoreSnapshot) (LegacyServiceHistoryResolution, LegacyServiceHistoryFreeze, error) {
	started := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC)
	expires := started.Add(time.Hour)
	return LegacyServiceHistoryResolution{Status: HistoryFound, SourceIdentity: "legacy-store:42", SourceSnapshotToken: "version:1", ServiceStartedAt: &started, ServiceExpiresAt: &expires}, recordingHistoryFreeze{}, nil
}

type recordingHistoryFreeze struct{}

func (recordingHistoryFreeze) SourceSnapshotToken() string     { return "version:1" }
func (recordingHistoryFreeze) Handoff(_ context.Context) error { return nil }
func (recordingHistoryFreeze) Release(_ context.Context) error { return nil }

type mismatchedHistoryFreeze struct{}

func (mismatchedHistoryFreeze) SourceSnapshotToken() string     { return "version:other" }
func (mismatchedHistoryFreeze) Handoff(_ context.Context) error { return nil }
func (mismatchedHistoryFreeze) Release(_ context.Context) error { return nil }
