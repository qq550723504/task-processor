package canonicalinspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"task-processor/internal/commercetool"
	listingtask "task-processor/internal/listing/task"
	"task-processor/internal/product/catalog"
)

type snapshotReader interface {
	catalog.SnapshotReader
	catalog.VersionedSnapshotReader
}

// MaxCatalogSnapshotBytes is the B0 consumer-side materialization bound. It is
// deliberately not a global Catalog publication or repository invariant.
const MaxCatalogSnapshotBytes = 8 << 20

type Executor struct {
	subjects     listingtask.CanonicalSubjectReader
	snapshots    snapshotReader
	tenantAdmins listingtask.TenantAdminChecker
}

func NewExecutor(subjects listingtask.CanonicalSubjectReader, snapshots snapshotReader, tenantAdmins listingtask.TenantAdminChecker) (*Executor, error) {
	if nilInterface(subjects) {
		return nil, fmt.Errorf("canonical subject reader is nil")
	}
	if nilInterface(snapshots) {
		return nil, fmt.Errorf("catalog snapshot reader is nil")
	}
	if nilInterface(tenantAdmins) {
		return nil, fmt.Errorf("tenant admin checker is nil")
	}
	return &Executor{subjects: subjects, snapshots: snapshots, tenantAdmins: tenantAdmins}, nil
}

func (e *Executor) Execute(ctx context.Context, envelope commercetool.ExecutionEnvelope, raw json.RawMessage) (commercetool.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, deadlineError(err)
	}
	var input Input
	if err := json.Unmarshal(raw, &input); err != nil || listingtask.ValidateTaskID(input.TaskID) != nil {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorInvalidInput, "canonical inspection input is invalid", err)
	}
	if input.TaskID != envelope.Metadata().BusinessTaskID {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorInvalidInput, "business task binding does not match input", nil)
	}

	principal := envelope.Principal()
	actor := listingtask.Actor{TenantID: principal.TenantID, UserID: principal.UserID, Roles: append([]string(nil), principal.Roles...)}
	if err := listingtask.ValidateActor(actor); err != nil {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorIdentityIntegrity, "verified principal is invalid", err)
	}
	subject, err := e.subjects.ReadCanonicalSubject(ctx, actor, input.TaskID)
	if err != nil {
		return commercetool.ExecutionResult{}, mapSubjectError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, deadlineError(err)
	}
	if subject.TaskID != input.TaskID || !listingtask.CanReadCanonicalSubject(actor, subject, e.tenantAdmins) {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorNotFound, "canonical product is not available", nil)
	}
	if subject.ProductKey == "" {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorFailedPrecondition, "canonical product is not ready", nil)
	}

	identity := catalog.SnapshotIdentity{TenantID: subject.TenantID, ProductKey: subject.ProductKey}
	var published catalog.PublishedSnapshot
	if subject.SnapshotVersion > 0 {
		published, err = e.snapshots.GetSnapshot(ctx, identity, subject.SnapshotVersion)
	} else {
		published, err = e.snapshots.GetCurrentSnapshot(ctx, identity)
	}
	if err != nil {
		return commercetool.ExecutionResult{}, mapCatalogError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, deadlineError(err)
	}
	if published.Identity != identity || published.Version == 0 ||
		(subject.SnapshotVersion > 0 && published.Version != subject.SnapshotVersion) {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorInternal, "canonical snapshot state is invalid", nil)
	}

	output, err := Project(subject, published)
	if err != nil {
		if errors.Is(err, ErrProjectionTooLarge) {
			return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorFailedPrecondition, "canonical product projection exceeds size limit", err)
		}
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorInternal, "canonical product projection failed", err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, deadlineError(err)
	}
	return commercetool.ExecutionResult{Output: output}, nil
}

func mapSubjectError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return deadlineError(err)
	case errors.Is(err, listingtask.ErrInvalidTaskID):
		return commercetool.NewError(commercetool.ErrorInvalidInput, "canonical inspection input is invalid", err)
	case errors.Is(err, listingtask.ErrInvalidActor):
		return commercetool.NewError(commercetool.ErrorIdentityIntegrity, "verified principal is invalid", err)
	case errors.Is(err, listingtask.ErrCanonicalSubjectNotFound):
		return commercetool.NewError(commercetool.ErrorNotFound, "canonical product is not available", err)
	case errors.Is(err, listingtask.ErrCanonicalSubjectNotReady):
		return commercetool.NewError(commercetool.ErrorFailedPrecondition, "canonical product is not ready", err)
	case errors.Is(err, listingtask.ErrCanonicalSubjectUnavailable):
		return commercetool.NewError(commercetool.ErrorDependencyUnavailable, "canonical task repository is unavailable", err)
	default:
		return commercetool.NewError(commercetool.ErrorInternal, "canonical task lookup failed", err)
	}
}

func mapCatalogError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return deadlineError(err)
	case errors.Is(err, catalog.ErrSnapshotNotReady):
		return commercetool.NewError(commercetool.ErrorFailedPrecondition, "canonical product is not ready", err)
	case errors.Is(err, catalog.ErrRepositoryUnavailable):
		return commercetool.NewError(commercetool.ErrorDependencyUnavailable, "canonical product repository is unavailable", err)
	default:
		return commercetool.NewError(commercetool.ErrorInternal, "canonical snapshot lookup failed", err)
	}
}

func deadlineError(cause error) error {
	return commercetool.NewError(commercetool.ErrorDeadlineExceeded, "canonical inspection deadline exceeded", cause)
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var _ commercetool.Executor = (*Executor)(nil)
