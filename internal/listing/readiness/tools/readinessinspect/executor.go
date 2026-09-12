package readinessinspect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type Executor struct {
	products catalog.VersionedSnapshotReader
	assets   asset.ApprovedInventoryReader
}

func NewExecutor(products catalog.VersionedSnapshotReader, assets asset.ApprovedInventoryReader) (*Executor, error) {
	if nilInterface(products) || nilInterface(assets) {
		return nil, fmt.Errorf("exact readers are required")
	}
	return &Executor{products: products, assets: assets}, nil
}

func (e *Executor) Execute(ctx context.Context, envelope commercetool.ExecutionEnvelope, raw json.RawMessage) (commercetool.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, readError(err)
	}
	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		return commercetool.ExecutionResult{}, invalidInput()
	}
	version, err := strconv.ParseUint(input.CatalogVersion, 10, 64)
	if err != nil || version == 0 || version > 1<<63-1 || strconv.FormatUint(version, 10) != input.CatalogVersion || !boundedText(input.ProductKey, 128) || !boundedText(input.TargetPlatform, 128) {
		return commercetool.ExecutionResult{}, invalidInput()
	}
	id := catalog.SnapshotIdentity{TenantID: envelope.Principal().TenantID, ProductKey: input.ProductKey}
	product, err := e.products.GetSnapshot(ctx, id, version)
	if err != nil {
		return commercetool.ExecutionResult{}, readError(err)
	}
	if product.Identity != id || product.Version != version {
		return commercetool.ExecutionResult{}, readError(ErrCorrupt)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, readError(err)
	}
	inventory, err := e.assets.GetApprovedInventory(ctx, asset.InventoryScope{TenantID: id.TenantID, ProductKey: id.ProductKey, TargetPlatform: input.TargetPlatform, SourceSnapshotVersion: version})
	var exact *asset.ApprovedAssetInventory
	if err == nil {
		exact = &inventory
	} else if !errors.Is(err, asset.ErrApprovedAssetsNotReady) {
		return commercetool.ExecutionResult{}, readError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, readError(err)
	}
	output, err := Project(product, exact, input.TargetPlatform)
	if err != nil {
		return commercetool.ExecutionResult{}, readError(err)
	}
	if err := ctx.Err(); err != nil {
		return commercetool.ExecutionResult{}, readError(err)
	}
	return commercetool.ExecutionResult{Output: output}, nil
}

func readError(err error) error {
	code := commercetool.ErrorInternal
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code = commercetool.ErrorDeadlineExceeded
	case errors.Is(err, catalog.ErrSnapshotNotReady):
		code = commercetool.ErrorNotFound
	case errors.Is(err, catalog.ErrSnapshotTooLarge), errors.Is(err, asset.ErrInventoryTooLarge), errors.Is(err, ErrTooLarge):
		code = commercetool.ErrorFailedPrecondition
	case errors.Is(err, catalog.ErrRepositoryUnavailable), errors.Is(err, asset.ErrRepositoryUnavailable):
		code = commercetool.ErrorDependencyUnavailable
	}
	return commercetool.NewError(code, "exact input diagnostics unavailable", err)
}

func invalidInput() error {
	return commercetool.NewError(commercetool.ErrorInvalidInput, "invalid exact input diagnostic request", nil)
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return v.IsNil()
	}
	return false
}
