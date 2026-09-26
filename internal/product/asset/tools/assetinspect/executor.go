package assetinspect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

const MaxCatalogSnapshotBytes = 8 << 20
const MaxInventoryBytes = 1 << 20

type Input struct {
	ProductKey     string `json:"product_key"`
	CatalogVersion string `json:"catalog_version"`
	TargetPlatform string `json:"target_platform"`
}

type Executor struct {
	snapshots catalog.VersionedSnapshotReader
	assets    asset.ApprovedInventoryReader
}

func NewExecutor(snapshots catalog.VersionedSnapshotReader, assets asset.ApprovedInventoryReader) (*Executor, error) {
	if nilPort(snapshots) || nilPort(assets) {
		return nil, errors.New("bounded exact readers are required")
	}
	return &Executor{snapshots: snapshots, assets: assets}, nil
}

func (e *Executor) Execute(ctx context.Context, envelope commercetool.ExecutionEnvelope, raw json.RawMessage) (commercetool.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, mapReadError(err)
	}
	var input Input
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = errors.New("trailing input")
		}
	}
	version, versionErr := strconv.ParseUint(input.CatalogVersion, 10, 64)
	if err != nil || versionErr != nil || version == 0 || version > math.MaxInt64 || strconv.FormatUint(version, 10) != input.CatalogVersion || !validIdentity(input.ProductKey) || !validIdentity(input.TargetPlatform) {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorInvalidInput, "asset inspection input is invalid", nil)
	}
	scope := asset.InventoryScope{TenantID: envelope.Principal().TenantID, ProductKey: input.ProductKey, SourceSnapshotVersion: version, TargetPlatform: input.TargetPlatform}
	identity := catalog.SnapshotIdentity{TenantID: scope.TenantID, ProductKey: scope.ProductKey}
	if asset.ValidateInventoryScope(scope) != nil {
		return commercetool.ExecutionResult{}, commercetool.NewError(commercetool.ErrorIdentityIntegrity, "asset inspection identity is invalid", nil)
	}
	published, err := e.snapshots.GetSnapshot(ctx, identity, version)
	if err != nil {
		return commercetool.ExecutionResult{}, mapReadError(err)
	}
	if published.Identity != identity || published.Version != version || !validIdentity(published.PublicationID) {
		return commercetool.ExecutionResult{}, mapReadError(errInvalidFacts)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, mapReadError(err)
	}
	inventory, err := e.assets.GetApprovedInventory(ctx, scope)
	if err != nil {
		return commercetool.ExecutionResult{}, mapReadError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, mapReadError(err)
	}
	output, err := project(published, inventory, scope)
	if err != nil {
		return commercetool.ExecutionResult{}, mapReadError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, mapReadError(err)
	}
	return commercetool.ExecutionResult{Output: output}, nil
}

func mapReadError(err error) error {
	code := commercetool.ErrorInternal
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code = commercetool.ErrorDeadlineExceeded
	case errors.Is(err, catalog.ErrSnapshotNotReady), errors.Is(err, asset.ErrApprovedAssetsNotReady), errors.Is(err, errMissingFacts):
		code = commercetool.ErrorNotFound
	case errors.Is(err, catalog.ErrSnapshotTooLarge), errors.Is(err, asset.ErrInventoryTooLarge), errors.Is(err, errProjectionTooLarge):
		code = commercetool.ErrorFailedPrecondition
	case errors.Is(err, catalog.ErrRepositoryUnavailable), errors.Is(err, asset.ErrRepositoryUnavailable):
		code = commercetool.ErrorDependencyUnavailable
	}
	return commercetool.NewError(code, "exact product asset facts are unavailable", err)
}

func nilPort(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan:
		return v.IsNil()
	}
	return false
}
