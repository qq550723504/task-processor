package podapp

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/pod"
	"testing"
	"time"
)

type kernelFixture struct {
	permit *submission.SendPermit
	calls  int
	marked int
	status submission.ExecutionStatus
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

func (m *mutationFixture) Upload(context.Context, pod.Plan, []byte, *submission.SendPermit) (pod.ObjectReceipt, error) {
	m.calls++
	return pod.ObjectReceipt{}, pod.ErrUnknown
}
func (m *mutationFixture) CreateMaterial(context.Context, pod.Plan, pod.ObjectReceipt, *submission.SendPermit) (pod.MaterialReceipt, error) {
	m.calls++
	return pod.MaterialReceipt{}, pod.ErrUnknown
}
func (m *mutationFixture) Sync(context.Context, pod.Plan, []byte, *submission.SendPermit) error {
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
