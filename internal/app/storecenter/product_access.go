package storecenterapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync/atomic"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/storecenter"
	"time"
)

var ErrProductAccessChanged = errors.New("official product merchant authorization changed or expired")

type OfficialGoodsProvider interface {
	Application() storecenter.OfficialApplication
	QueryProductSites(context.Context, storecenter.OfficialMerchantCredential) ([]model.MainSite, error)
	QueryProductWarehouses(context.Context, storecenter.OfficialMerchantCredential) ([]model.Warehouse, error)
	QueryProductCategories(context.Context, storecenter.OfficialMerchantCredential) ([]model.Category, error)
	QueryProductFillStandards(context.Context, storecenter.OfficialMerchantCredential, int64) (model.FillStandards, error)
	QueryProductAttributes(context.Context, storecenter.OfficialMerchantCredential, int64) (model.AttributeTemplate, error)
	QueryProductLinkedRules(context.Context, storecenter.OfficialMerchantCredential, model.LinkedRulesRequest) ([]model.LinkedRules, error)
	QueryProductBrands(context.Context, storecenter.OfficialMerchantCredential) ([]model.Brand, error)
	QueryProductPublishPermission(context.Context, storecenter.OfficialMerchantCredential, string) (model.PublishPermission, error)
	PublishProduct(context.Context, storecenter.OfficialMerchantCredential, model.PublishProduct) (model.PublishResult, error)
	TransformProductImage(context.Context, storecenter.OfficialMerchantCredential, model.TransformImage) (model.TransformedImage, error)
}
type MerchantBinding = storecenter.ProductMerchantBinding
type OfficialProductAccess struct {
	reader        storecenter.ProductExecutionReader
	authorization storecenter.ProductExecutionAuthorizer
	applications  *OfficialApplicationRegistry
	now           func() time.Time
}

// MerchantHandle has no serializable execution capability or credential. It is
// minted for one current process, original membership and bounded activity.
type MerchantHandle struct {
	owner                         *OfficialProductAccess
	entry                         officialApplicationEntry
	subject                       storecenter.ProductExecutionSubject
	binding                       MerchantBinding
	connectionRef, credentialHash string
	expiresAt                     time.Time
	mutationSent                  atomic.Bool
}

func NewOfficialProductAccess(reader storecenter.ProductExecutionReader, authorization storecenter.ProductExecutionAuthorizer, applications *OfficialApplicationRegistry) (*OfficialProductAccess, error) {
	if reader == nil || authorization == nil || applications == nil {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	return &OfficialProductAccess{reader, authorization, applications, time.Now}, nil
}
func (a *OfficialProductAccess) Authorize(ctx context.Context, subject storecenter.ProductExecutionSubject, storeID string, expected *MerchantBinding) (*MerchantHandle, error) {
	if a == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrProductAccessChanged
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	material, err := a.reader.ReadProductExecution(ctx, subject, storeID, a.authorization, a.now())
	if err != nil {
		return nil, ErrProductAccessChanged
	}
	entry, err := a.applications.resolve(material.Attempt.AppID, material.Attempt.AppVersion)
	if err != nil {
		return nil, ErrProductAccessChanged
	}
	if _, ok := entry.provider.(OfficialGoodsProvider); !ok {
		return nil, ErrProductAccessChanged
	}
	credential, err := entry.protection.Open(material.Attempt, material.Attempt.KeyID, material.Attempt.Ciphertext)
	if err != nil || !a.validMaterial(subject, storeID, material, credential, entry) {
		return nil, ErrProductAccessChanged
	}
	binding := merchantBinding(subject, storeID, material, credential, entry.mode)
	if expected != nil && *expected != binding {
		return nil, ErrProductAccessChanged
	}
	expiresAt := a.now().Add(10 * time.Second)
	if deadline, _ := ctx.Deadline(); deadline.Before(expiresAt) {
		expiresAt = deadline
	}
	if !a.now().Before(expiresAt) {
		return nil, ErrProductAccessChanged
	}
	return &MerchantHandle{owner: a, entry: entry, subject: subject, binding: binding, connectionRef: material.Connection.AttemptID, credentialHash: privateCredentialHash(material), expiresAt: expiresAt}, nil
}
func (h *MerchantHandle) Binding() MerchantBinding {
	if h == nil {
		return MerchantBinding{}
	}
	return h.binding
}
func (h *MerchantHandle) credential(ctx context.Context) (storecenter.OfficialMerchantCredential, error) {
	if h == nil || h.owner == nil || ctx == nil || ctx.Err() != nil || !h.owner.now().Before(h.expiresAt) {
		return storecenter.OfficialMerchantCredential{}, ErrProductAccessChanged
	}
	material, err := h.owner.reader.ReadProductExecution(ctx, h.subject, h.binding.StoreID, h.owner.authorization, h.owner.now())
	if err != nil || material.Connection.AttemptID != h.connectionRef || privateCredentialHash(material) != h.credentialHash {
		return storecenter.OfficialMerchantCredential{}, ErrProductAccessChanged
	}
	entry, err := h.owner.applications.resolve(material.Attempt.AppID, material.Attempt.AppVersion)
	if err != nil || entry.application != h.entry.application || entry.mode != h.entry.mode {
		return storecenter.OfficialMerchantCredential{}, ErrProductAccessChanged
	}
	credential, err := entry.protection.Open(material.Attempt, material.Attempt.KeyID, material.Attempt.Ciphertext)
	if err != nil || !h.owner.validMaterial(h.subject, h.binding.StoreID, material, credential, entry) || merchantBinding(h.subject, h.binding.StoreID, material, credential, entry.mode) != h.binding || !h.owner.now().Before(h.expiresAt) {
		return storecenter.OfficialMerchantCredential{}, ErrProductAccessChanged
	}
	return credential, nil
}
func (a *OfficialProductAccess) validMaterial(subject storecenter.ProductExecutionSubject, storeID string, m storecenter.ProductExecutionMaterial, c storecenter.OfficialMerchantCredential, entry officialApplicationEntry) bool {
	application := entry.application
	return m.Platform == storecenter.PlatformShein && m.StoreVersion > 0 && m.Connection.Version > 0 && m.Connection.Status == storecenter.ConnectionStatusConnected && m.Attempt.State == "verified" && m.Attempt.OrganizationID == subject.OrganizationID && m.Attempt.StoreID == storeID && m.Attempt.AttemptID == m.Connection.AttemptID && m.Attempt.AppID == application.AppID && m.Attempt.AppVersion == application.Version && c.AppID == application.AppID && c.SupplierID != "" && c.OpenKeyID != "" && c.SecretKey != "" && a.now().Before(m.ServiceExpiresAt)
}
func merchantBinding(subject storecenter.ProductExecutionSubject, storeID string, m storecenter.ProductExecutionMaterial, c storecenter.OfficialMerchantCredential, mode storecenter.OfficialApplicationType) MerchantBinding {
	hash := sha256.Sum256([]byte(c.AppID + "\x00" + c.SupplierID + "\x00" + c.OpenKeyID))
	return MerchantBinding{OrganizationID: subject.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: m.StoreVersion, ConnectionRevision: m.Connection.Version, ApplicationRevision: m.Attempt.AppVersion, ApplicationID: m.Attempt.AppID, ApplicationType: mode, SupplierIdentityHash: hex.EncodeToString(hash[:]), ServiceExpiresAt: m.ServiceExpiresAt.UTC()}
}
func (h *MerchantHandle) Publish(ctx context.Context, input model.PublishProduct) (model.PublishResult, error) {
	return callMerchant(ctx, h, storecenter.ProductPurposePublish, func(ctx context.Context, c storecenter.OfficialMerchantCredential) (model.PublishResult, error) {
		return h.entry.provider.(OfficialGoodsProvider).PublishProduct(ctx, c, input)
	})
}
func (h *MerchantHandle) TransformImage(ctx context.Context, input model.TransformImage) (model.TransformedImage, error) {
	return callMerchant(ctx, h, storecenter.ProductPurposeImage, func(ctx context.Context, c storecenter.OfficialMerchantCredential) (model.TransformedImage, error) {
		return h.entry.provider.(OfficialGoodsProvider).TransformProductImage(ctx, c, input)
	})
}
func (h *MerchantHandle) Sites(ctx context.Context) ([]model.MainSite, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) ([]model.MainSite, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductSites(ctx, c)
	})
}
func (h *MerchantHandle) Categories(ctx context.Context) ([]model.Category, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) ([]model.Category, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductCategories(ctx, c)
	})
}
func (h *MerchantHandle) Warehouses(ctx context.Context) ([]model.Warehouse, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) ([]model.Warehouse, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductWarehouses(ctx, c)
	})
}
func (h *MerchantHandle) FillStandards(ctx context.Context, id int64) (model.FillStandards, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) (model.FillStandards, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductFillStandards(ctx, c, id)
	})
}
func (h *MerchantHandle) Attributes(ctx context.Context, id int64) (model.AttributeTemplate, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) (model.AttributeTemplate, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductAttributes(ctx, c, id)
	})
}
func (h *MerchantHandle) LinkedRules(ctx context.Context, input model.LinkedRulesRequest) ([]model.LinkedRules, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) ([]model.LinkedRules, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductLinkedRules(ctx, c, input)
	})
}
func (h *MerchantHandle) Brands(ctx context.Context) ([]model.Brand, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) ([]model.Brand, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductBrands(ctx, c)
	})
}
func (h *MerchantHandle) PublishPermission(ctx context.Context, brand string) (model.PublishPermission, error) {
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) (model.PublishPermission, error) {
		return h.entry.provider.(OfficialGoodsProvider).QueryProductPublishPermission(ctx, c, brand)
	})
}
func (h *MerchantHandle) QuerySPU(ctx context.Context, spu string) (model.ProductReadback, error) {
	if h == nil || h.subject.Purpose != storecenter.ProductPurposePublish {
		return model.ProductReadback{}, ErrProductAccessChanged
	}
	return callMerchant(ctx, h, "", func(ctx context.Context, c storecenter.OfficialMerchantCredential) (model.ProductReadback, error) {
		provider, ok := h.entry.provider.(interface {
			QueryProductSPU(context.Context, storecenter.OfficialMerchantCredential, string) (model.ProductReadback, error)
		})
		if !ok {
			return model.ProductReadback{}, ErrProductAccessChanged
		}
		return provider.QueryProductSPU(ctx, c, spu)
	})
}
func callMerchant[T any](ctx context.Context, h *MerchantHandle, mutationPurpose string, call func(context.Context, storecenter.OfficialMerchantCredential) (T, error)) (T, error) {
	var zero T
	if h == nil || h.owner == nil || ctx == nil || ctx.Err() != nil {
		return zero, ErrProductAccessChanged
	}
	ctx, cancel := context.WithDeadline(ctx, h.expiresAt)
	defer cancel()
	credential, err := h.credential(ctx)
	if err != nil {
		return zero, err
	}
	if mutationPurpose != "" && (h.subject.Purpose != mutationPurpose || !h.mutationSent.CompareAndSwap(false, true)) {
		return zero, ErrProductAccessChanged
	}
	return call(ctx, credential)
}
func privateCredentialHash(material storecenter.ProductExecutionMaterial) string {
	hash := sha256.Sum256([]byte(material.Attempt.KeyID + "\x00" + material.Attempt.Ciphertext))
	return hex.EncodeToString(hash[:])
}
