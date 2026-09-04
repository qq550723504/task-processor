package storecenter

import (
	"context"
	"errors"
	"strings"
	"time"
)

type LegacyHistoryStatus string

const (
	HistoryFound           LegacyHistoryStatus = "found"
	HistoryConfirmedAbsent LegacyHistoryStatus = "confirmed_absent"
	HistoryUnavailable     LegacyHistoryStatus = "unavailable"
)

var ErrInvalidLegacyHistoryResolution = errors.New("invalid legacy service history resolution")
var ErrInvalidLegacyHistoryFreeze = errors.New("invalid legacy service history freeze")

// LegacyServiceHistoryResolution is the only accepted input for Store service
// history backfill. A missing history row is not represented by zero values;
// it must be an explicit CONFIRMED_ABSENT result with a source snapshot token.
type LegacyServiceHistoryResolution struct {
	Status              LegacyHistoryStatus
	SourceIdentity      string
	SourceSnapshotToken string
	ServiceStartedAt    *time.Time
	ServiceExpiresAt    *time.Time
	RetryAfter          time.Duration
	Cause               error
}

// LegacyServiceHistoryResolver owns lookup semantics for the old paid-service
// authority. Backfill must retain the returned freeze through final
// authority handoff; it cannot replace this contract with a local NULL check.
type LegacyServiceHistoryResolver interface {
	Resolve(ctx context.Context, store StoreSnapshot) (LegacyServiceHistoryResolution, LegacyServiceHistoryFreeze, error)
}

// LegacyServiceHistoryFreeze is deliberately explicit because a source token
// alone is not a concurrency fence. Handoff must complete before Release.
type LegacyServiceHistoryFreeze interface {
	SourceSnapshotToken() string
	Handoff(ctx context.Context) error
	Release(ctx context.Context) error
}

func ValidateLegacyServiceHistoryFreeze(resolution LegacyServiceHistoryResolution, freeze LegacyServiceHistoryFreeze) error {
	if err := ValidateLegacyServiceHistoryResolution(resolution); err != nil {
		return err
	}
	switch resolution.Status {
	case HistoryFound, HistoryConfirmedAbsent:
		if freeze == nil || strings.TrimSpace(freeze.SourceSnapshotToken()) != strings.TrimSpace(resolution.SourceSnapshotToken) {
			return ErrInvalidLegacyHistoryFreeze
		}
	case HistoryUnavailable:
		if freeze != nil {
			return ErrInvalidLegacyHistoryFreeze
		}
	}
	return nil
}

func ValidateLegacyServiceHistoryResolution(resolution LegacyServiceHistoryResolution) error {
	identity := strings.TrimSpace(resolution.SourceIdentity)
	token := strings.TrimSpace(resolution.SourceSnapshotToken)
	if len(identity) > 256 || len(token) > 256 || (identity == "" && token != "") || (identity != "" && token == "") {
		return ErrInvalidLegacyHistoryResolution
	}
	switch resolution.Status {
	case HistoryFound:
		if identity == "" || !validHistoryPeriod(resolution.ServiceStartedAt, resolution.ServiceExpiresAt) || resolution.RetryAfter != 0 || resolution.Cause != nil {
			return ErrInvalidLegacyHistoryResolution
		}
	case HistoryConfirmedAbsent:
		if identity == "" || resolution.ServiceStartedAt != nil || resolution.ServiceExpiresAt != nil || resolution.RetryAfter != 0 || resolution.Cause != nil {
			return ErrInvalidLegacyHistoryResolution
		}
	case HistoryUnavailable:
		if resolution.ServiceStartedAt != nil || resolution.ServiceExpiresAt != nil || resolution.Cause == nil || resolution.RetryAfter <= 0 {
			return ErrInvalidLegacyHistoryResolution
		}
	default:
		return ErrInvalidLegacyHistoryResolution
	}
	return nil
}

func validHistoryPeriod(startedAt, expiresAt *time.Time) bool {
	return startedAt != nil && expiresAt != nil && !startedAt.IsZero() && !expiresAt.IsZero() && expiresAt.After(*startedAt)
}
