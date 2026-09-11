package canonicalinspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/catalog"
)

// MaxCatalogSnapshotBytes is the consumer-side bound applied by the injected
// persistence adapter before a snapshot is materialized.
const MaxCatalogSnapshotBytes = 8 << 20

type Executor struct {
	snapshots catalog.VersionedSnapshotReader
}

func NewExecutor(snapshots catalog.VersionedSnapshotReader) (*Executor, error) {
	if nilInterface(snapshots) {
		return nil, fmt.Errorf("catalog versioned snapshot reader is nil")
	}
	return &Executor{snapshots: snapshots}, nil
}

func (e *Executor) Execute(ctx context.Context, envelope commercetool.ExecutionEnvelope, raw json.RawMessage) (commercetool.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, deadlineError(err)
	}
	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		return commercetool.ExecutionResult{}, invalidInput(err)
	}
	productKey := strings.TrimSpace(input.ProductKey)
	version, err := parseCatalogVersion(input.CatalogVersion)
	if err != nil || productKey == "" || productKey != input.ProductKey || utf8.RuneCountInString(productKey) > 128 {
		return commercetool.ExecutionResult{}, invalidInput(err)
	}

	identity := catalog.SnapshotIdentity{TenantID: envelope.Principal().TenantID, ProductKey: productKey}
	published, err := e.snapshots.GetSnapshot(ctx, identity, version)
	if err != nil {
		return commercetool.ExecutionResult{}, mapCatalogError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, deadlineError(err)
	}
	if published.Identity != identity || published.Version != version ||
		published.PublicationID == "" || published.PublicationID != strings.TrimSpace(published.PublicationID) {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorInternal, "canonical snapshot state is invalid", nil)
	}

	output, err := Project(published)
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

func parseCatalogVersion(value string) (uint64, error) {
	if value == "" || value != strings.TrimSpace(value) {
		return 0, fmt.Errorf("catalog version is empty or padded")
	}
	version, err := strconv.ParseUint(value, 10, 64)
	if err != nil || version == 0 || version > math.MaxInt64 || strconv.FormatUint(version, 10) != value {
		return 0, fmt.Errorf("catalog version is outside the persistent domain")
	}
	return version, nil
}

func invalidInput(cause error) error {
	return commercetool.NewError(commercetool.ErrorInvalidInput, "canonical inspection input is invalid", cause)
}

func mapCatalogError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return deadlineError(err)
	case errors.Is(err, catalog.ErrSnapshotNotReady):
		return commercetool.NewError(commercetool.ErrorNotFound, "canonical product version is not available", err)
	case errors.Is(err, catalog.ErrSnapshotTooLarge):
		return commercetool.NewError(commercetool.ErrorFailedPrecondition, "canonical product snapshot exceeds size limit", err)
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
