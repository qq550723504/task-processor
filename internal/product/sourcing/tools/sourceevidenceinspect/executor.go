// Package sourceevidenceinspect exposes exact, read-only source evidence through
// the Commerce Tool invocation boundary. Sourcing remains the evidence owner.
package sourceevidenceinspect

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type Input struct {
	ProductKey     string `json:"product_key"`
	CatalogVersion string `json:"catalog_version"`
}

// SourceReader must be an admitted SRC-1 reader that freshly authorizes every
// read and verifies the durable publication. No mutation port is exposed here.
type SourceReader interface {
	Read(context.Context, string) (sourcing.PersistedPublication, error)
}

type Executor struct {
	snapshots catalog.VersionedSnapshotReader
	sources   SourceReader
}

func NewExecutor(snapshots catalog.VersionedSnapshotReader, sources SourceReader) (*Executor, error) {
	if nilPort(snapshots) || nilPort(sources) {
		return nil, errors.New("source evidence readers are required")
	}
	return &Executor{snapshots: snapshots, sources: sources}, nil
}

func (e *Executor) readExact(ctx context.Context, principal commercetool.Principal, input Input) (sourcing.PersistedPublication, error) {
	var empty sourcing.PersistedPublication
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) || !authidentity.IsBoundedIdentifier(identity.EffectiveOrganizationID) ||
		identity.UserID != principal.UserID || identity.TenantID != principal.TenantID || identity.EffectiveOrganizationID != principal.TenantID ||
		identity.TokenExpiresAt.IsZero() || !time.Now().Before(identity.TokenExpiresAt) {
		return empty, sourcing.ErrPublicationForbidden
	}
	version, err := strconv.ParseUint(input.CatalogVersion, 10, 64)
	if err != nil || version == 0 || version > math.MaxInt64 || strconv.FormatUint(version, 10) != input.CatalogVersion ||
		!authidentity.IsBoundedIdentifier(input.ProductKey) {
		return empty, sourcing.ErrInvalidSourcePublication
	}
	expected := catalog.SnapshotIdentity{TenantID: principal.TenantID, ProductKey: input.ProductKey}
	published, err := e.snapshots.GetSnapshot(ctx, expected, version)
	if err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if published.Identity != expected || published.Version != version || !authidentity.IsBoundedIdentifier(published.PublicationID) {
		return empty, sourcing.ErrSourcePublicationStateInvalid
	}
	persisted, err := e.sources.Read(ctx, published.PublicationID)
	if err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	r := persisted.Receipt
	if r.OrganizationID != expected.TenantID || r.ProductKey != expected.ProductKey || r.CatalogVersion != version ||
		r.PublicationID != published.PublicationID || r.CatalogPublicationID != published.PublicationID || !reflect.DeepEqual(persisted.Snapshot, published.Snapshot) {
		return empty, sourcing.ErrSourcePublicationStateInvalid
	}
	return persisted, nil
}

func (e *Executor) Execute(ctx context.Context, envelope commercetool.ExecutionEnvelope, raw json.RawMessage) (commercetool.ExecutionResult, error) {
	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		return commercetool.ExecutionResult{}, toolError(sourcing.ErrInvalidSourcePublication)
	}
	persisted, err := e.readExact(ctx, envelope.Principal(), input)
	if err != nil {
		return commercetool.ExecutionResult{}, toolError(err)
	}
	output, err := Project(persisted)
	if err != nil {
		return commercetool.ExecutionResult{}, toolError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, toolError(err)
	}
	return commercetool.ExecutionResult{Output: output}, nil
}

func toolError(cause error) error {
	code := commercetool.ErrorInternal
	switch {
	case errors.Is(cause, context.Canceled), errors.Is(cause, context.DeadlineExceeded):
		code = commercetool.ErrorDeadlineExceeded
	case errors.Is(cause, sourcing.ErrInvalidSourcePublication):
		code = commercetool.ErrorInvalidInput
	case errors.Is(cause, sourcing.ErrPublicationForbidden):
		code = commercetool.ErrorPermissionDenied
	case errors.Is(cause, sourcing.ErrSourcePublicationNotFound), errors.Is(cause, catalog.ErrSnapshotNotReady):
		code = commercetool.ErrorNotFound
	case errors.Is(cause, sourcing.ErrSourcePublicationUnavailable), errors.Is(cause, catalog.ErrRepositoryUnavailable):
		code = commercetool.ErrorDependencyUnavailable
	case errors.Is(cause, ErrProjectionTooLarge), errors.Is(cause, sourcing.ErrSourcePublicationTooLarge), errors.Is(cause, catalog.ErrSnapshotTooLarge):
		code = commercetool.ErrorFailedPrecondition
	case errors.Is(cause, sourcing.ErrSourcePublicationConflict):
		code = commercetool.ErrorConflict
	}
	return commercetool.NewError(code, "source evidence inspection failed", cause)
}

var _ commercetool.Executor = (*Executor)(nil)

func nilPort(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}
