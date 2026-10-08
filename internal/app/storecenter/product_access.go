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
	QueryProductCategories(context.Context, storecenter.OfficialMerchantCredential) ([]model.Category, error)
	QueryProductFillStandards(context.Context, storecenter.OfficialMerchantCredential, int64) (model.FillStandards, error)
	QueryProductAttributes(context.Context, storecenter.OfficialMerchantCredential, int64) (model.AttributeTemplate, error)
	QueryProductLinkedRules(context.Context, storecenter.OfficialMerchantCredential, model.LinkedRulesRequest) ([]model.LinkedRules, error)
	QueryProductBrands(context.Context, storecenter.OfficialMerchantCredential) ([]model.Brand, error)
	QueryProductPublishPermission(context.Context, storecenter.OfficialMerchantCredential, string) (model.PublishPermission, error)
	PublishProduct(context.Context, storecenter.OfficialMerchantCredential, model.PublishProduct) (model.PublishResult, error)
	TransformProductImage(context.Context, storecenter.OfficialMerchantCredential, model.TransformImage) (model.TransformedImage, error)
}
type MerchantBinding struct {
	OrganizationID       string    `json:"organization_id"`
	StoreID              string    `json:"store_id"`
	Site                 string    `json:"site"`
	StoreVersion         int64     `json:"store_version"`
	ConnectionRevision   int64     `json:"connection_revision"`
	ApplicationRevision  string    `json:"application_revision"`
	SupplierIdentityHash string    `json:"supplier_identity_hash"`
	ServiceExpiresAt     time.Time `json:"service_expires_at"`
}
type OfficialProductAccess struct {
	reader        storecenter.ProductExecutionReader
	authorization storecenter.ProductExecutionAuthorizer
	provider      OfficialGoodsProvider
	protection    storecenter.OfficialCredentialProtection
	now           func() time.Time
}

// MerchantHandle has no serializable execution capability or credential. It is
// minted for one current process, original membership and bounded activity.
type MerchantHandle struct {
	owner                         *OfficialProductAccess
	subject                       storecenter.ProductExecutionSubject
	binding                       MerchantBinding
	connectionRef, credentialHash string
	expiresAt                     time.Time
	mutationSent                  atomic.Bool
}

func NewOfficialProductAccess(reader storecenter.ProductExecutionReader, authorization storecenter.ProductExecutionAuthorizer, provider OfficialGoodsProvider, protection storecenter.OfficialCredentialProtection) (*OfficialProductAccess, error) {
	if reader == nil || authorization == nil || provider == nil || protection == nil {
		return nil, storecenter.ErrOfficialConnectionUnavailable
	}
	return &OfficialProductAccess{reader, authorization, provider, protection, time.Now}, nil
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
	credential, err := a.protection.Open(material.Attempt, material.Attempt.KeyID, material.Attempt.Ciphertext)
	if err != nil || !a.validMaterial(subject, storeID, material, credential) {
		return nil, ErrProductAccessChanged
	}
	binding := merchantBinding(subject, storeID, material, credential)
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
	return &MerchantHandle{owner: a, subject: subject, binding: binding, connectionRef: material.Connection.AttemptID, credentialHash: privateCredentialHash(material), expiresAt: expiresAt}, nil
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
	credential, err := h.owner.protection.Open(material.Attempt, material.Attempt.KeyID, material.Attempt.Ciphertext)
	if err != nil || !h.owner.validMaterial(h.subject, h.binding.StoreID, material, credential) || merchantBinding(h.subject, h.binding.StoreID, material, credential) != h.binding || !h.owner.now().Before(h.expiresAt) {
		return storecenter.OfficialMerchantCredential{}, ErrProductAccessChanged
	}
	return credential, nil
}
func (a *OfficialProductAccess) validMaterial(subject storecenter.ProductExecutionSubject, storeID string, m storecenter.ProductExecutionMaterial, c storecenter.OfficialMerchantCredential) bool {
	application := a.provider.Application()
	return m.Platform == storecenter.PlatformShein && m.StoreVersion > 0 && m.Connection.Version > 0 && m.Connection.Status == storecenter.ConnectionStatusConnected && m.Attempt.State == "verified" && m.Attempt.OrganizationID == subject.OrganizationID && m.Attempt.StoreID == storeID && m.Attempt.AttemptID == m.Connection.AttemptID && m.Attempt.AppID == application.AppID && m.Attempt.AppVersion == application.Version && c.AppID == application.AppID && c.SupplierID != "" && c.OpenKeyID != "" && c.SecretKey != "" && a.now().Before(m.ServiceExpiresAt)
}
func merchantBinding(subject storecenter.ProductExecutionSubject, storeID string, m storecenter.ProductExecutionMaterial, c storecenter.OfficialMerchantCredential) MerchantBinding {
	hash := sha256.Sum256([]byte(c.AppID + "\x00" + c.SupplierID + "\x00" + c.OpenKeyID))
	return MerchantBinding{OrganizationID: subject.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: m.StoreVersion, ConnectionRevision: m.Connection.Version, ApplicationRevision: m.Attempt.AppVersion, SupplierIdentityHash: hex.EncodeToString(hash[:]), ServiceExpiresAt: m.ServiceExpiresAt.UTC()}
}
func (h *MerchantHandle) Publish(ctx context.Context, input model.PublishProduct) (model.PublishResult, error) {
	credential, err := h.credential(ctx)
	if err != nil || h.subject.Purpose != storecenter.ProductPurposePublish || !h.mutationSent.CompareAndSwap(false, true) {
		return model.PublishResult{}, ErrProductAccessChanged
	}
	return h.owner.provider.PublishProduct(ctx, credential, input)
}
func (h *MerchantHandle) TransformImage(ctx context.Context, input model.TransformImage) (model.TransformedImage, error) {
	credential, err := h.credential(ctx)
	if err != nil || h.subject.Purpose != storecenter.ProductPurposeImage || !h.mutationSent.CompareAndSwap(false, true) {
		return model.TransformedImage{}, ErrProductAccessChanged
	}
	return h.owner.provider.TransformProductImage(ctx, credential, input)
}
func (h *MerchantHandle) Sites(ctx context.Context) ([]model.MainSite, error) {
	c, e := h.credential(ctx)
	if e != nil {
		return nil, e
	}
	return h.owner.provider.QueryProductSites(ctx, c)
}
func (h *MerchantHandle) Categories(ctx context.Context) ([]model.Category, error) {
	c, e := h.credential(ctx)
	if e != nil {
		return nil, e
	}
	return h.owner.provider.QueryProductCategories(ctx, c)
}
func (h *MerchantHandle) FillStandards(ctx context.Context, id int64) (model.FillStandards, error) {
	c, e := h.credential(ctx)
	if e != nil {
		return model.FillStandards{}, e
	}
	return h.owner.provider.QueryProductFillStandards(ctx, c, id)
}
func (h *MerchantHandle) Attributes(ctx context.Context, id int64) (model.AttributeTemplate, error) {
	c, e := h.credential(ctx)
	if e != nil {
		return model.AttributeTemplate{}, e
	}
	return h.owner.provider.QueryProductAttributes(ctx, c, id)
}
func (h *MerchantHandle) LinkedRules(ctx context.Context, input model.LinkedRulesRequest) ([]model.LinkedRules, error) {
	c, e := h.credential(ctx)
	if e != nil {
		return nil, e
	}
	return h.owner.provider.QueryProductLinkedRules(ctx, c, input)
}
func (h *MerchantHandle) Brands(ctx context.Context) ([]model.Brand, error) {
	c, e := h.credential(ctx)
	if e != nil {
		return nil, e
	}
	return h.owner.provider.QueryProductBrands(ctx, c)
}
func (h *MerchantHandle) PublishPermission(ctx context.Context, brand string) (model.PublishPermission, error) {
	c, e := h.credential(ctx)
	if e != nil {
		return model.PublishPermission{}, e
	}
	return h.owner.provider.QueryProductPublishPermission(ctx, c, brand)
}
func privateCredentialHash(material storecenter.ProductExecutionMaterial) string {
	hash := sha256.Sum256([]byte(material.Attempt.KeyID + "\x00" + material.Attempt.Ciphertext))
	return hex.EncodeToString(hash[:])
}
