package aiworkbench

import (
	"context"
	"errors"
)

// Repository contains only Workbench-owned facts. Other owners are reached
// through the two narrow ports below; no cross-owner SQL transaction exists.
type Repository interface {
	AppendUser(context.Context, Scope, string, string, MessageInput, PlanPreparer) (PlanningCommand, bool, error)
	HistoryForCommand(context.Context, Scope, string) ([]Message, PlanningCommand, error)
	GetCommand(context.Context, Scope, string) (PlanningCommand, error)
	CompletePlan(context.Context, Scope, string, PlanTerminal) (PlanningCommand, bool, error)
	FinalizePlanning(context.Context, Scope, string, PlanningState) (PlanningCommand, error)
	LookupTask(context.Context, Scope, string, string, string) (BusinessTask, bool, error)
	GetProposal(context.Context, Scope, string) (ExecutionProposal, error)
	Confirm(context.Context, Scope, string, string, string, PreparedTask) (BusinessTask, bool, error)
	ReplayOrFail(context.Context, Scope, string, string, string, error) (BusinessTask, bool, error)
}

// PlanningPort checks current permissions and work selection outside T0. Its
// returned PlanPreparer must be pure: Store calls it under a Conversation lock.
type PlanningPort interface {
	Admission(context.Context, Scope, MessageInput) (PlanPreparer, error)
	Decide(context.Context, PlanningCommand, []Message) (PlanTerminal, error)
	FailureState(context.Context, PlanningCommand, error) PlanningState
}

type ExecutionPort interface {
	AuthorizeReceipt(context.Context, Scope) error
	Prepare(context.Context, ExecutionProposal, string) (PreparedTask, error)
	Start(context.Context, BusinessTask) error
}

type Service struct {
	Store   Repository
	Plan    PlanningPort
	Execute ExecutionPort
}

func (s *Service) Message(ctx context.Context, scope Scope, conversationID, key string, input MessageInput) (PlanningCommand, error) {
	if s == nil || s.Store == nil || s.Plan == nil {
		return PlanningCommand{}, ErrUnavailable
	}
	prepare, err := s.Plan.Admission(ctx, scope, input)
	if err != nil {
		// AppendUser looks up the scoped wire receipt before invoking prepare.
		// An existing turn remains readable when a mutable Product/model route
		// has since become unavailable; a new turn still fails before T0.
		prepare = func([]Message, string) (PreparedPlan, error) { return PreparedPlan{}, err }
	}
	command, _, err := s.Store.AppendUser(ctx, scope, conversationID, key, input, prepare)
	if err != nil || command.State != PlanningReadyToDispatch {
		return command, err
	}
	history, frozen, err := s.Store.HistoryForCommand(ctx, scope, key)
	if err != nil {
		return PlanningCommand{}, err
	}
	if frozen.ConversationID != conversationID || frozen.RequestFingerprint != command.RequestFingerprint {
		return PlanningCommand{}, ErrUnavailable
	}
	terminal, err := s.Plan.Decide(ctx, frozen, history)
	if err != nil {
		state := s.Plan.FailureState(ctx, frozen, err)
		if state == PlanningReadyToDispatch {
			return s.Store.GetCommand(ctx, scope, key)
		}
		if state != PlanningFailedBeforeDispatch && state != PlanningUnknown {
			state = PlanningUnknown
		}
		updated, finalErr := s.Store.FinalizePlanning(ctx, scope, key, state)
		if finalErr != nil {
			return PlanningCommand{}, finalErr
		}
		return updated, nil
	}
	updated, _, err := s.Store.CompletePlan(ctx, scope, key, terminal)
	return updated, err
}

// Confirm preserves T1 receipt precedence. Every first-create preflight
// rejection takes the Store's serialized replay-or-fail exit before returning.
func (s *Service) Confirm(ctx context.Context, scope Scope, conversationID, proposalID, key string) (BusinessTask, bool, error) {
	if s == nil || s.Store == nil || s.Execute == nil {
		return BusinessTask{}, false, ErrUnavailable
	}
	if err := s.Execute.AuthorizeReceipt(ctx, scope); err != nil {
		return BusinessTask{}, false, err
	}
	current, found, err := s.Store.LookupTask(ctx, scope, conversationID, proposalID, key)
	if err != nil {
		return BusinessTask{}, false, err
	}
	if found {
		if err := s.Execute.AuthorizeReceipt(ctx, scope); err != nil {
			return BusinessTask{}, false, err
		}
		return current, true, nil
	}
	proposal, err := s.Store.GetProposal(ctx, scope, proposalID)
	if err == nil && proposal.ConversationID != conversationID {
		err = ErrNotFound
	}
	var prepared PreparedTask
	if err == nil {
		prepared, err = s.Execute.Prepare(ctx, proposal, key)
	}
	if err != nil {
		if accessErr := s.Execute.AuthorizeReceipt(ctx, scope); accessErr != nil {
			return BusinessTask{}, false, accessErr
		}
		resolved, replay, exitErr := s.Store.ReplayOrFail(ctx, scope, conversationID, proposalID, key, err)
		if accessErr := s.Execute.AuthorizeReceipt(ctx, scope); accessErr != nil {
			return BusinessTask{}, false, accessErr
		}
		return resolved, replay, exitErr
	}
	task, replay, err := s.Store.Confirm(ctx, scope, conversationID, proposalID, key, prepared)
	if err != nil || replay {
		if accessErr := s.Execute.AuthorizeReceipt(ctx, scope); accessErr != nil {
			return BusinessTask{}, false, accessErr
		}
		return task, replay, err
	}
	// T1 is durable now. A T2 failure is visible through the exact Task receipt;
	// the caller can use the explicit start action without minting another key.
	startErr := s.Execute.Start(ctx, task)
	if accessErr := s.Execute.AuthorizeReceipt(ctx, scope); accessErr != nil {
		return BusinessTask{}, false, accessErr
	}
	if startErr != nil {
		return task, false, errors.Join(ErrUnavailable, startErr)
	}
	return task, false, nil
}
