package supplychainapp

import (
	"context"
	"errors"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/listing/submission"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"time"
)

type OperationStarter interface {
	Ensure(context.Context, string, string) error
}
type OperationTargetCreator interface {
	CreateForExecution(context.Context, collection.Scope, string, record.TargetInput) (record.TargetReceipt, error)
}
type OperationUploader interface {
	Upload(context.Context, collection.Scope, string, string) (UploadResult, error)
}
type OperationOptimizer interface {
	Optimize(context.Context, preparation.Operation, preparation.OperationItem, string) (string, error)
}
type OperationActivities struct {
	Operations *preparation.OperationService
	Repository preparation.OperationRepository
	Sources    record.TargetExecutionSourceSelector
	Products   record.EffectiveTargetProductReader
	Targets    record.TargetRepository
	Creator    OperationTargetCreator
	Uploader   OperationUploader
	Optimizer  OperationOptimizer
}
type OperationPage struct {
	Items      []preparation.OperationItem `json:"items"`
	NextCursor string                      `json:"nextCursor,omitempty"`
	Cancelled  bool                        `json:"cancelled"`
}
type OperationExecution struct {
	OrganizationID string `json:"organizationId"`
	OperationID    string `json:"operationId"`
	After          string `json:"after,omitempty"`
}

func (a *OperationActivities) List(ctx context.Context, in OperationExecution) (OperationPage, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	proof, err := a.Operations.AuthorizeExecution(ctx, in.OrganizationID, in.OperationID)
	if err != nil {
		return OperationPage{}, err
	}
	op, err := proof.Read(ctx)
	if err != nil {
		return OperationPage{}, err
	}
	if op.Status == preparation.OperationCancelled || op.Status == preparation.OperationCompleted {
		return OperationPage{Items: []preparation.OperationItem{}, Cancelled: op.Status == preparation.OperationCancelled}, nil
	}
	page, err := a.Repository.ListOperationItems(ctx, op.Owner, op.ID, collection.Query{Limit: 100, After: in.After})
	return OperationPage{Items: page.Items, NextCursor: page.NextCursor}, err
}
func (a *OperationActivities) Process(ctx context.Context, in OperationExecution, sourceID string) (preparation.OperationItem, error) {
	if a == nil || a.Operations == nil || a.Repository == nil || !collection.ValidID(sourceID) {
		return preparation.OperationItem{}, preparation.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	proof, err := a.Operations.AuthorizeExecution(ctx, in.OrganizationID, in.OperationID)
	if err != nil {
		return preparation.OperationItem{}, err
	}
	op, err := proof.Read(ctx)
	if err != nil {
		return preparation.OperationItem{}, err
	}
	item, err := a.Repository.BeginOperationItem(ctx, proof, sourceID)
	if err != nil {
		return item, err
	}
	if preparation.ItemTerminal(item.Status) {
		return item, nil
	}
	key := preparation.ItemCommandID(op.ID, item.SourceID, op.Input.Action)
	result := item
	switch op.Input.Action {
	case preparation.OperationAdapt:
		receipt, e := a.adapt(ctx, op, item, key)
		if e != nil {
			err = e
			break
		}
		result.ResultReference = receipt.Record.ID
		result.Status = preparation.ItemSucceeded
		if !receipt.Record.Result.ReadyForUpload {
			result.Status = preparation.ItemMissing
			result.Note = "适配资料已保存，请补全必填信息并确认图片"
		}
	case preparation.OperationUpload:
		if a.Uploader == nil {
			return item, preparation.ErrUnavailable
		}
		uploaded, e := a.Uploader.Upload(ctx, op.Owner, key, item.RecordID)
		if e != nil {
			err = e
			break
		}
		result.ResultReference = uploaded.AttemptID
		switch uploaded.Status {
		case submission.ExecutionSucceeded:
			result.Status = preparation.ItemSucceeded
		case submission.ExecutionOutcomeUnknown:
			result.Status = preparation.ItemUnknown
		default:
			result.Status = preparation.ItemFailed
		}
		result.Note = uploaded.Message
	case preparation.OperationOptimize:
		if a.Optimizer == nil {
			return item, preparation.ErrUnavailable
		}
		if item.RecordID == "" {
			receipt, e := a.adapt(ctx, op, item, preparation.ItemCommandID(op.ID, item.SourceID, preparation.OperationAdapt))
			if e != nil {
				return item, e
			}
			proof, e = a.Operations.AuthorizeExecution(ctx, in.OrganizationID, in.OperationID)
			if e != nil {
				return item, e
			}
			item, e = a.Repository.BindOperationTarget(ctx, proof, item.SourceID, receipt.Record.ID, receipt.Record.Revision)
			if e != nil {
				return item, e
			}
			result = item
		}
		result.ResultReference, err = a.Optimizer.Optimize(ctx, op, item, key)
		if err == nil {
			result.Status = preparation.ItemReview
			result.Note = "优化结果须人工审核并应用后才会进入适配资料"
		}
	default:
		return item, preparation.ErrInvalid
	}
	if err != nil {
		// Transient failures and lost responses retry the same durable command.
		// Revocation stops progress until the original member regains authority.
		switch {
		case errors.Is(err, record.ErrNotReady):
			result.Status = preparation.ItemMissing
			result.Note = "当前资料或图片未满足上传条件"
		case errors.Is(err, record.ErrConflict), errors.Is(err, submission.ErrExecutionIntentConflict):
			result.Status = preparation.ItemFailed
			result.Note = "资料、店铺连接或原操作版本已变化，请核对原结果"
		case errors.Is(err, preparation.ErrUnknown):
			result.Status = preparation.ItemUnknown
			result.Note = "智能体原运行结果待核实，不会自动重新调用"
		default:
			return item, err
		}
	}
	if len([]rune(result.Note)) > 300 {
		result.Note = string([]rune(result.Note)[:300])
	}
	proof, err = a.Operations.AuthorizeExecution(ctx, in.OrganizationID, in.OperationID)
	if err != nil {
		return item, err
	}
	if err = a.Repository.FinishOperationItem(ctx, proof, result); err != nil {
		return item, err
	}
	return result, nil
}
func (a *OperationActivities) adapt(ctx context.Context, op preparation.Operation, item preparation.OperationItem, key string) (record.TargetReceipt, error) {
	if a.Targets == nil || a.Creator == nil || a.Sources == nil || a.Products == nil {
		return record.TargetReceipt{}, preparation.ErrUnavailable
	}
	prior, err := a.Targets.ReadTargetCommand(ctx, op.Owner, key)
	if err == nil {
		return prior, nil
	}
	if !errors.Is(err, record.ErrNotFound) {
		return record.TargetReceipt{}, err
	}
	selected, err := a.Sources.SelectForExecution(ctx, op.Owner, item.SourceID)
	if err != nil {
		return record.TargetReceipt{}, err
	}
	scope, source, original, err := selected.Read(ctx)
	if err != nil || scope != op.Owner || source.PreparationID != op.Input.PreparationID {
		return record.TargetReceipt{}, preparation.ErrForbidden
	}
	input := record.TargetInput{SourceID: source.ID, StoreID: op.Input.StoreID, EffectiveVersion: original.Version, Draft: goods.OfficialDraftInput{}}
	// Preserve explicit manual fields on re-adaptation; source mapping never
	// invents a brand code, platform attribute ID, warehouse, stock or price.
	head, e := a.Targets.ReadTargetHead(ctx, op.Owner, record.TargetIdentity(op.Owner, source.ID, op.Input.StoreID))
	if e == nil {
		input = head.Input
		input.ExpectedRevision = head.Revision
	} else if errors.Is(e, record.ErrNotFound) {
		input.Draft.Product.Names = []model.LanguageContent{{Language: "en", Name: original.Snapshot.Title}}
		input.Draft.Product.Descriptions = []model.LanguageContent{{Language: "en", Name: original.Snapshot.Description}}
		skc := model.ProductSKC{SKUs: []model.ProductSKU{}}
		for _, variant := range original.Snapshot.Variants {
			skc.SKUs = append(skc.SKUs, model.ProductSKU{SupplierSKU: variant.SKU, SaleAttributes: []model.AttributeValue{}})
		}
		if len(skc.SKUs) == 0 {
			skc.SKUs = []model.ProductSKU{{SaleAttributes: []model.AttributeValue{}}}
		}
		input.Draft.Product.SKCs = []model.ProductSKC{skc}
		input.Draft.Product.Attributes = []model.AttributeValue{}
	} else {
		return record.TargetReceipt{}, e
	}
	if op.Input.CategoryID > 0 {
		input.Draft.Product.CategoryID = op.Input.CategoryID
	}
	return a.Creator.CreateForExecution(ctx, op.Owner, key, input)
}

type OperationApplication struct {
	Service    *preparation.OperationService
	Repository preparation.OperationRepository
	Starter    OperationStarter
}

func (a OperationApplication) EnsureExecution(ctx context.Context, id string) (preparation.Operation, error) {
	if ctx == nil || a.Service == nil || a.Repository == nil || a.Starter == nil {
		return preparation.Operation{}, preparation.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	proof, err := a.Service.RequestAccess(ctx, id)
	if err != nil {
		return preparation.Operation{}, err
	}
	op, err := proof.Read(ctx)
	if err != nil {
		return preparation.Operation{}, err
	}
	if op.Status == preparation.OperationCompleted || op.Status == preparation.OperationCancelled {
		return op, nil
	}
	started := a.Starter.Ensure(ctx, op.Owner.OrganizationID, op.ID)
	// Refresh request authority after a slow/lost Temporal response. This only
	// changes execution progress, never the fixed source set or provider fence.
	proof, err = a.Service.RequestAccess(ctx, id)
	if err != nil {
		return op, err
	}
	status := "started"
	if started != nil {
		status = "start_unknown"
	}
	if err = a.Repository.MarkExecution(ctx, proof, status); err != nil {
		return op, err
	}
	op, err = a.Service.Read(ctx, id)
	if err != nil {
		return op, err
	}
	if started != nil {
		return op, preparation.ErrUnknown
	}
	return op, nil
}
