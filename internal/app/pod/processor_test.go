package podapp

import (
	"context"
	"errors"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type kernelFixture struct {
	permit      *submission.SendPermit
	calls       int
	marked      int
	status      submission.ExecutionStatus
	intentError error
}

func (k *kernelFixture) ReadIntent(context.Context, submission.ExecutionScope, string) (submission.ExecutionAttempt, error) {
	return submission.ExecutionAttempt{Status: k.status}, k.intentError
}

func TestPreSendRetryRequiresDefinitiveAbsenceOfOriginalAttempt(t *testing.T) {
	o := pod.Operation{Plan: pod.Plan{OperationID: "12752596-6056-4316-9f2f-380c97df9675", Scope: collection.Scope{"org", "actor", "member"}}}
	for _, tc := range []struct {
		name  string
		err   error
		step  string
		retry bool
	}{
		{"no attempt", submission.ErrExecutionNotFound, pod.StepOSS, true},
		{"existing attempt", nil, pod.StepOSS, false},
		{"unknown database result", submission.ErrExecutionUnavailable, pod.StepOSS, false},
		{"after OSS", submission.ErrExecutionNotFound, pod.StepMaterial, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := &kernelFixture{intentError: tc.err, status: submission.ExecutionOutcomeUnknown}
			p := Processor{Kernel: k}
			result := p.preSendFailure(context.Background(), o, tc.step)
			require.Equal(t, tc.retry, result.NotStarted)
			require.Equal(t, !tc.retry, result.Unknown)
			require.Zero(t, k.calls)
		})
	}
}

func (k *kernelFixture) Acquire(context.Context, submission.AcquireExecutionCommand) (submission.ExecutionAcquisition, error) {
	k.calls++
	return submission.ExecutionAcquisition{Permit: k.permit, Attempt: submission.ExecutionAttempt{Status: k.status}}, nil
}
func (k *kernelFixture) MarkUnknown(context.Context, submission.ExecutionClaim, submission.UnknownReason) (submission.ExecutionAttempt, error) {
	k.marked++
	return submission.ExecutionAttempt{}, nil
}

type mutationFixture struct{ calls int }

func (m *mutationFixture) Upload(context.Context, pod.Plan, []byte, *pod.MutationPermit) (pod.ObjectReceipt, error) {
	m.calls++
	return pod.ObjectReceipt{}, pod.ErrUnknown
}
func (m *mutationFixture) CreateMaterial(context.Context, pod.Plan, pod.ObjectReceipt, *pod.MutationPermit) (pod.MaterialReceipt, error) {
	m.calls++
	return pod.MaterialReceipt{}, pod.ErrUnknown
}
func (m *mutationFixture) Sync(context.Context, pod.Plan, []byte, *pod.MutationPermit) error {
	m.calls++
	return pod.ErrUnknown
}
func TestReplayedKernelAttemptNeverSendsEvenWhileClaimed(t *testing.T) {
	m := &mutationFixture{}
	k := &kernelFixture{status: submission.ExecutionClaimed}
	p := &Processor{Kernel: k, Mutations: m}
	command := submission.AcquireExecutionCommand{}
	_, e := p.send(context.Background(), pod.Operation{}, pod.StepOSS, command, []byte("artwork"), func(context.Context) error { return nil })
	require.ErrorIs(t, e, pod.ErrUnknown)
	require.Equal(t, 0, m.calls)
	k.permit = &submission.SendPermit{AttemptID: "attempt", ClaimToken: "token", LeaseExpiresAt: time.Now().Add(time.Minute)}
	_, e = p.send(context.Background(), pod.Operation{}, pod.StepOSS, command, []byte("artwork"), func(context.Context) error { return nil })
	require.ErrorIs(t, e, pod.ErrUnknown)
	require.Equal(t, 1, m.calls)
	require.Equal(t, 1, k.marked)
	_, e = p.send(context.Background(), pod.Operation{}, pod.StepOSS, command, []byte("artwork"), func(context.Context) error { return errors.New("revoked before send") })
	require.ErrorIs(t, e, pod.ErrForbidden)
	require.Equal(t, 1, m.calls)
	require.Equal(t, 2, k.marked)
}
