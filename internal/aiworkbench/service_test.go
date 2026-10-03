package aiworkbench

import (
	"context"
	"errors"
	"testing"
)

var errReceiptRevoked = errors.New("receipt access revoked")

type confirmRepository struct {
	lookupFound  bool
	replayCalls  int
	confirmCalls int
	task         BusinessTask
}

type messageRepository struct {
	confirmRepository
	command     PlanningCommand
	appendCalls int
}

func (r *messageRepository) ReplayCommand(context.Context, Scope, string, string, MessageInput) (PlanningCommand, bool, error) {
	return r.command, true, nil
}

func (r *messageRepository) AppendUser(context.Context, Scope, string, string, MessageInput, PlanPreparer) (PlanningCommand, bool, error) {
	r.appendCalls++
	return r.command, true, nil
}
func (r *messageRepository) HistoryForCommand(context.Context, Scope, string) ([]Message, PlanningCommand, error) {
	return []Message{{Author: AuthorUser, Content: "goal"}}, r.command, nil
}
func (r *messageRepository) CompletePlan(context.Context, Scope, string, PlanTerminal) (PlanningCommand, bool, error) {
	r.command.State = PlanningComplete
	return r.command, false, nil
}

type messagePlanner struct {
	admissionCalls int
	decideCalls    int
	readyTerminal  bool
}

func (*messagePlanner) AuthorizeReceipt(context.Context, Scope) error { return nil }
func (p *messagePlanner) Admission(context.Context, Scope, MessageInput) (PlanPreparer, error) {
	p.admissionCalls++
	return nil, ErrUnavailable
}
func (p *messagePlanner) Decide(context.Context, PlanningCommand, []Message) (PlanTerminal, error) {
	if !p.readyTerminal {
		panic("terminal receipt must not redispatch")
	}
	p.decideCalls++
	return PlanTerminal{Mode: PlanClarify, AssistantText: "What style?"}, nil
}
func (*messagePlanner) FailureState(context.Context, PlanningCommand, error) PlanningState {
	panic("terminal receipt must not finalize again")
}

func TestMessageReplaysTerminalReceiptBeforeMutableAdmission(t *testing.T) {
	repo := &messageRepository{command: PlanningCommand{State: PlanningFailedBeforeDispatch}}
	plan := &messagePlanner{}
	service := Service{Store: repo, Plan: plan}
	command, err := service.Message(context.Background(), Scope{OrganizationID: "org", ActorID: "actor"},
		"conversation", "key", MessageInput{Content: "goal", OperationID: "operation", TargetPlatform: "shein"})
	if err != nil || command.State != PlanningFailedBeforeDispatch || plan.admissionCalls != 0 || repo.appendCalls != 0 {
		t.Fatalf("terminal replay consulted mutable admission: command=%+v err=%v admission=%d append=%d", command, err, plan.admissionCalls, repo.appendCalls)
	}
}

func TestMessageCanDispatchStoredReadyTurnWithoutMutableAdmission(t *testing.T) {
	repo := &messageRepository{command: PlanningCommand{State: PlanningReadyToDispatch,
		ConversationID: "conversation", RequestFingerprint: "wire-fingerprint"}}
	plan := &messagePlanner{readyTerminal: true}
	service := Service{Store: repo, Plan: plan}
	command, err := service.Message(context.Background(), Scope{OrganizationID: "org", ActorID: "actor"},
		"conversation", "key", MessageInput{Content: "goal", OperationID: "operation", TargetPlatform: "shein"})
	if err != nil || command.State != PlanningComplete || plan.admissionCalls != 0 || plan.decideCalls != 1 || repo.appendCalls != 0 {
		t.Fatalf("ready replay did not use frozen turn: command=%+v err=%v admission=%d decide=%d append=%d", command, err, plan.admissionCalls, plan.decideCalls, repo.appendCalls)
	}
}

func (r *confirmRepository) AppendUser(context.Context, Scope, string, string, MessageInput, PlanPreparer) (PlanningCommand, bool, error) {
	panic("unused")
}
func (r *confirmRepository) ReplayCommand(context.Context, Scope, string, string, MessageInput) (PlanningCommand, bool, error) {
	panic("unused")
}
func (r *confirmRepository) HistoryForCommand(context.Context, Scope, string) ([]Message, PlanningCommand, error) {
	panic("unused")
}
func (r *confirmRepository) GetCommand(context.Context, Scope, string) (PlanningCommand, error) {
	panic("unused")
}
func (r *confirmRepository) CompletePlan(context.Context, Scope, string, PlanTerminal) (PlanningCommand, bool, error) {
	panic("unused")
}
func (r *confirmRepository) FinalizePlanning(context.Context, Scope, string, PlanningState) (PlanningCommand, error) {
	panic("unused")
}
func (r *confirmRepository) LookupTask(context.Context, Scope, string, string, string) (BusinessTask, bool, error) {
	return r.task, r.lookupFound, nil
}
func (r *confirmRepository) GetProposal(context.Context, Scope, string) (ExecutionProposal, error) {
	return ExecutionProposal{ConversationID: "conversation"}, nil
}
func (r *confirmRepository) Confirm(context.Context, Scope, string, string, string, PreparedTask) (BusinessTask, bool, error) {
	r.confirmCalls++
	return r.task, false, nil
}
func (r *confirmRepository) ReplayOrFail(context.Context, Scope, string, string, string, error) (BusinessTask, bool, error) {
	r.replayCalls++
	return r.task, true, nil
}

type confirmExecution struct {
	preflightErr error
	revoked      bool
	startRevokes bool
}

func (x *confirmExecution) AuthorizeReceipt(context.Context, Scope) error {
	if x.revoked {
		return errReceiptRevoked
	}
	return nil
}
func (x *confirmExecution) Prepare(context.Context, ExecutionProposal, string) (PreparedTask, error) {
	return PreparedTask{}, x.preflightErr
}
func (x *confirmExecution) Start(context.Context, BusinessTask) error {
	x.revoked = x.startRevokes
	return nil
}

func TestConfirmDoesNotReleaseReceiptAfterAccessRevocation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		found        bool
		preflightErr error
		startRevokes bool
	}{
		{name: "existing receipt", found: true},
		{name: "preflight failure after competitor commits", preflightErr: ErrRevisionMismatch},
		{name: "after durable create and start", startRevokes: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &confirmRepository{lookupFound: tc.found, task: BusinessTask{ID: "task"}}
			execute := &confirmExecution{revoked: tc.found, preflightErr: tc.preflightErr, startRevokes: tc.startRevokes}
			if tc.preflightErr != nil {
				execute.revoked = true
			}
			service := Service{Store: store, Execute: execute}
			task, _, err := service.Confirm(context.Background(), Scope{}, "conversation", "proposal", "key")
			if !errors.Is(err, errReceiptRevoked) || task.ID != "" {
				t.Fatalf("revoked receipt access must not release task: task=%+v err=%v", task, err)
			}
			if tc.preflightErr != nil && store.replayCalls != 0 {
				t.Fatalf("replay-or-fail called after revocation: %d", store.replayCalls)
			}
		})
	}
}
