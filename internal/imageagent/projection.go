package imageagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProjectionInitialization is the single transaction that creates a run's
// immutable authorization inputs and first public snapshot.
type ProjectionInitialization struct {
	Scope        RunScope
	Run          Run
	Plan         Plan
	Catalog      AssetCatalog
	Snapshot     RunProjection
	CommitID     string
	EventType    string
	EventPayload json.RawMessage
}

type PlanProjectionMutation struct {
	ExpectedActiveRevision int64
	Plan                   Plan
}

type SlotProjectionMutation struct {
	PlanRevision int64
	Result       SlotResult
	Projection   SlotProjection
	Attempt      StepAttempt
}

// ProjectionCommit atomically applies the normalized mutation, stores the
// complete canonical snapshot, and appends its matching stream event.
type ProjectionCommit struct {
	Scope                     RunScope
	CommitID                  string
	ExpectedProjectionVersion int64
	Snapshot                  RunProjection
	EventType                 string
	EventPayload              json.RawMessage
	RunMutation               *RunMutation
	ExpectedRunVersion        int64
	PlanMutation              *PlanProjectionMutation
	SlotMutation              *SlotProjectionMutation
}

func ValidateProjectionSnapshot(scope RunScope, snapshot RunProjection) error {
	if strings.TrimSpace(scope.TenantID) == "" || strings.TrimSpace(scope.OwnerUserID) == "" || strings.TrimSpace(scope.RunID) == "" {
		return ErrRunNotFound
	}
	if snapshot.Run.TenantID != scope.TenantID || snapshot.Run.UserID != scope.OwnerUserID || snapshot.Run.ID != scope.RunID {
		return ErrRunNotFound
	}
	if snapshot.ProjectionVersion != snapshot.LastEventID || snapshot.ProjectionVersion < 0 {
		return fmt.Errorf("%w: projection cursor baseline is invalid", ErrRevisionConflict)
	}
	if err := ValidatePlan(snapshot.Plan); err != nil {
		return fmt.Errorf("validate projection plan: %w", err)
	}
	if snapshot.Plan.Revision != snapshot.Run.ActivePlanRevision {
		return ErrRevisionConflict
	}
	for _, slot := range snapshot.Slots {
		for _, candidate := range slot.Candidates {
			if _, err := ValidateSafeImageURL(candidate.URL); err != nil {
				return fmt.Errorf("slot %q candidate %q has unsafe URL: %w", slot.Slot.ID, candidate.AssetID, err)
			}
		}
	}
	return nil
}

func ScopeForRun(run Run) RunScope {
	return RunScope{TenantID: strings.TrimSpace(run.TenantID), OwnerUserID: strings.TrimSpace(run.UserID), RunID: strings.TrimSpace(run.ID)}
}
