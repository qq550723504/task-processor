package membership

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/authz"
)

type memoryReceipts struct {
	mu           sync.Mutex
	op           *Operation
	loseDispatch bool
	ackMode      string
}

func (m *memoryReceipts) Begin(_ context.Context, op Operation) (Operation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.op != nil {
		if m.op.Scope == op.Scope && m.op.Key == op.Key && m.op.Fingerprint == op.Fingerprint {
			return *m.op, nil
		}
		return Operation{}, ErrConflict
	}
	m.op = &op
	return op, nil
}
func (m *memoryReceipts) Read(_ context.Context, scope OperationScope, key string) (Operation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.op == nil || m.op.Key != key || m.op.Scope != scope {
		return Operation{}, ErrNotFound
	}
	return *m.op, nil
}
func (m *memoryReceipts) Apply(_ context.Context, scope OperationScope, key string, revision int64, change OperationChange) (Operation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.op == nil || m.op.Key != key || m.op.Scope != scope || m.op.Revision != revision {
		return Operation{}, ErrConflict
	}
	if change.Event == EventAcknowledge && m.ackMode == "before" {
		return Operation{}, ErrUnavailable
	}
	next, err := Transition(*m.op, change)
	if err != nil {
		return Operation{}, err
	}
	m.op = &next
	if change.Event == EventAcknowledge && m.ackMode == "after" {
		return Operation{}, ErrUnavailable
	}
	if m.loseDispatch && change.Event == EventDispatch {
		return Operation{}, ErrUnavailable
	}
	return next, nil
}

type writerStub struct {
	calls int
	err   error
}

func (w *writerStub) Write(_ context.Context, op Operation) (Acknowledgment, error) {
	w.calls++
	return Acknowledgment{ID: op.AuthorizationID, At: time.Now().UTC().Format(time.RFC3339Nano)}, w.err
}

func TestLostResponseAndLostDispatchCommitNeverResend(t *testing.T) {
	for _, loseCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "provider response lost", true: "dispatch commit lost"}[loseCommit], func(t *testing.T) {
			a, _ := authz.NewListingKitAuthorizer(nil, nil)
			member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}, State: "active"}
			d := &directoryStub{page: Page{Items: []Member{member}, Total: 1}}
			s := NewService(d, a, "project")
			store := &memoryReceipts{loseDispatch: loseCommit}
			provider := &writerStub{err: errors.New("lost response")}
			refresh := func(ctx context.Context) (context.Context, error) { return ctx, nil }
			commands := NewCommands(s, store, provider, refresh)
			key := uuid.NewString()
			input := CommandInput{Kind: CommandRole, AuthorizationID: "grant", ExpectedVersion: observedVersion(member), Role: "listingkit_operator"}
			_, _ = commands.Execute(scopedContext("listingkit_admin"), key, input)
			want := 1
			if loseCommit {
				want = 0
			}
			if provider.calls != want {
				t.Fatalf("sends=%d want=%d", provider.calls, want)
			}
			// Reconstruct the application and retry the original command.
			rebuilt := NewCommands(s, store, provider, refresh)
			receipt, err := rebuilt.Execute(scopedContext("listingkit_admin"), key, input)
			if err != nil || receipt.Phase != PhaseDispatched || provider.calls != want {
				t.Fatalf("replay=%+v %v sends=%d", receipt, err, provider.calls)
			}
			input.Role = "listingkit_admin"
			if _, err := rebuilt.Execute(scopedContext("listingkit_admin"), key, input); err != ErrConflict {
				t.Fatalf("different payload=%v", err)
			}
		})
	}
}

func TestRevokeBeforeDispatchPreventsProviderWrite(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}}
	s := NewService(&directoryStub{page: Page{Items: []Member{member}, Total: 1}}, a, "project")
	provider := &writerStub{}
	calls := 0
	commands := NewCommands(s, &memoryReceipts{}, provider, func(ctx context.Context) (context.Context, error) {
		calls++
		if calls > 1 {
			return nil, ErrPermission
		}
		return ctx, nil
	})
	_, err := commands.Execute(scopedContext("listingkit_admin"), uuid.NewString(), CommandInput{Kind: CommandRemove, AuthorizationID: "grant", ExpectedVersion: observedVersion(member)})
	if err != ErrPermission || provider.calls != 0 {
		t.Fatalf("err=%v sends=%d", err, provider.calls)
	}
}

func TestAcknowledgmentPersistenceFailureUsesOnlyDurableEvidenceAfterRebuild(t *testing.T) {
	for _, mode := range []string{"before", "after"} {
		t.Run(mode, func(t *testing.T) {
			a, _ := authz.NewListingKitAuthorizer(nil, nil)
			member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}, State: "active"}
			s := NewService(&directoryStub{page: Page{Items: []Member{member}, Total: 1}}, a, "project")
			store := &memoryReceipts{ackMode: mode}
			provider := &writerStub{}
			refresh := func(ctx context.Context) (context.Context, error) { return ctx, nil }
			key := uuid.NewString()
			input := CommandInput{Kind: CommandRemove, AuthorizationID: "grant", ExpectedVersion: observedVersion(member)}
			op, err := NewCommands(s, store, provider, refresh).Execute(scopedContext("listingkit_admin"), key, input)
			if err != nil || op.Phase != PhaseDispatched {
				t.Fatalf("lost completion response=%+v %v", op, err)
			}
			op, err = NewCommands(s, store, provider, refresh).Execute(scopedContext("listingkit_admin"), key, input)
			want := PhaseDispatched
			if mode == "after" {
				want = PhaseCompleted
			}
			if err != nil || op.Phase != want || provider.calls != 1 {
				t.Fatalf("rebuild=%+v %v sends=%d", op, err, provider.calls)
			}
		})
	}
}

type delayedWriter struct {
	started, release chan struct{}
	calls            int
}

func (w *delayedWriter) Write(_ context.Context, op Operation) (Acknowledgment, error) {
	w.calls++
	close(w.started)
	<-w.release
	return Acknowledgment{ID: op.AuthorizationID, At: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}
func TestOutstandingHTTPHoldsTargetWhileVerifyAndNewKeyRace(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}, State: "active"}
	s := NewService(&directoryStub{page: Page{Items: []Member{member}, Total: 1}}, a, "project")
	store := &memoryReceipts{}
	writer := &delayedWriter{started: make(chan struct{}), release: make(chan struct{})}
	c := NewCommands(s, store, writer, func(ctx context.Context) (context.Context, error) { return ctx, nil })
	key := uuid.NewString()
	input := CommandInput{Kind: CommandRemove, AuthorizationID: "grant", ExpectedVersion: observedVersion(member)}
	done := make(chan error, 1)
	go func() { _, err := c.Execute(scopedContext("listingkit_admin"), key, input); done <- err }()
	<-writer.started
	op, verifyErr := c.Verify(scopedContext("listingkit_admin"), key)
	_, conflictErr := c.Execute(scopedContext("listingkit_admin"), uuid.NewString(), input)
	close(writer.release)
	err := <-done
	if verifyErr != nil || op.Phase != PhaseDispatched || conflictErr != ErrConflict || err != nil || writer.calls != 1 {
		t.Fatalf("verify=%+v %v new=%v final=%v sends=%d", op, verifyErr, conflictErr, err, writer.calls)
	}
}
