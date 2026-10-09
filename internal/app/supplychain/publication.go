package supplychainapp

import (
	"context"
	"errors"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/listing/submission"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/collection"
	"time"
)

// PublicationView only contains confirmed platform facts. It is not a fresh
// official query, an approval status, or an authority to send again.
type PublicationView struct {
	SourceID   string               `json:"sourceId"`
	StoreID    string               `json:"storeId"`
	RecordID   string               `json:"recordId,omitempty"`
	ReceiptID  string               `json:"receiptId,omitempty"`
	Product    *model.PublishResult `json:"product,omitempty"`
	ObservedAt *time.Time           `json:"observedAt,omitempty"`
}

func (a *Application) Publication(ctx context.Context, sourceID, storeID string) (PublicationView, error) {
	value := PublicationView{SourceID: sourceID, StoreID: storeID}
	if ctx == nil || !collection.ValidID(sourceID) || !collection.ValidID(storeID) {
		return value, preparation.ErrInvalid
	}
	if a == nil || a.PublicationReceipts == nil || a.PublicationStores == nil {
		return value, record.ErrUnavailable
	}
	scope, err := a.Authorization.Authorize(ctx, preparation.PermissionRead)
	if err != nil {
		return value, err
	}
	proof, err := a.Sources.Select(ctx, sourceID)
	if err != nil {
		return value, err
	}
	owner, source, _, err := proof.Read(ctx)
	if err != nil || owner != scope {
		return value, preparation.ErrForbidden
	}
	merchant, err := a.PublicationStores.RulesMerchant(ctx, scope, storeID, nil)
	if err != nil {
		return value, record.ErrForbidden
	}
	binding := merchant.Binding()
	if binding.OrganizationID != scope.OrganizationID || binding.StoreID != storeID || binding.Site != "shein-us" {
		return value, record.ErrConflict
	}
	subject, err := submission.ProductSubjectID(source.Source.ProductKey, binding.Site)
	if err != nil {
		return value, err
	}
	receipt, err := a.PublicationReceipts.FindOfficialTarget(ctx, scope.OrganizationID, submission.ExecutionTarget{Platform: "shein", StoreID: storeID, SubjectID: subject})
	if errors.Is(err, submission.ErrExecutionNotFound) {
		return value, nil
	}
	if err != nil {
		return value, err
	}
	if submission.ValidateOfficialReceipt(receipt) != nil || receipt.Kind != "publish" || receipt.ProductKey != source.Source.ProductKey || receipt.Binding.SupplierIdentityHash != binding.SupplierIdentityHash || receipt.Binding.StoreID != storeID || receipt.Target.SubjectID != subject {
		return value, record.ErrConflict
	}
	if _, _, _, err = proof.Read(ctx); err != nil {
		return value, err
	}
	value.Product = &receipt.Product
	value.ReceiptID = receipt.ID
	value.ObservedAt = &receipt.ObservedAt
	if receipt.Owner == scope {
		value.RecordID = receipt.RecordID
	}
	return value, nil
}
