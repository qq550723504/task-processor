// Package ecoservicesbilling translates immutable source proofs across owners.
package ecoservicesbilling

import (
	"context"
	"task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
)

type CheckoutAuthorizer interface {
	AuthorizeServicePurchase(context.Context, string, string) error
}
type Source struct {
	Repository e.Repository
	Authorizer CheckoutAuthorizer
}

func command(in e.FinancialCommand) billing.ServicePurchaseCommand {
	return billing.ServicePurchaseCommand{ID: in.ID, RequestID: in.RequestID, OrderID: in.OrderID, Kind: in.Kind, SourceProofID: in.SourceProofID, ActorID: in.ActorID, BuyerOrganizationID: in.BuyerOrganizationID, ProviderOrganizationID: in.ProviderOrganizationID, ProviderMerchantID: in.MerchantID, QuoteVersion: in.Quote.Version, AmountMinor: in.AmountMinor, DeliveryDays: in.Quote.DeliveryDays, PolicyVersion: in.PolicyVersion}
}
func (s Source) OriginalServicePurchase(ctx context.Context, order string) (billing.ServicePurchaseCommand, error) {
	in, err := s.Repository.OriginalFinancialCommand(ctx, order)
	if err != nil {
		return billing.ServicePurchaseCommand{}, err
	}
	return command(in), nil
}
func (s Source) VerifyServiceCommand(ctx context.Context, c billing.ServicePurchaseCommand) error {
	if c.Validate() != nil {
		return billing.ErrInvalid
	}
	original, err := s.Repository.FinancialCommand(ctx, c.ID)
	if err != nil {
		return err
	}
	if command(original).Fingerprint() != c.Fingerprint() {
		return billing.ErrConflict
	}
	return nil
}
func (s Source) CanDispatchServiceCommand(ctx context.Context, c billing.ServicePurchaseCommand) error {
	if err := s.VerifyServiceCommand(ctx, c); err != nil {
		return err
	}
	page, err := s.Repository.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: c.BuyerOrganizationID}, Kind: "requests", ID: c.RequestID, Page: 1, PageSize: 1})
	if err != nil {
		return err
	}
	if len(page.Requests) != 1 {
		return billing.ErrNotFound
	}
	r := page.Requests[0]
	if c.Kind == "SETTLE" && (r.State != "ACCEPTED" || r.AcceptanceID != c.SourceProofID || r.FinancialFence) {
		return billing.ErrOrderCancelled
	}
	if c.Kind == "REFUND" && (r.Refund == nil || r.Refund.State != "APPROVED" || r.Refund.AmountMinor != c.AmountMinor) {
		return billing.ErrOrderCancelled
	}
	return nil
}
func (s Source) AdmitServiceCommand(ctx context.Context, c billing.ServicePurchaseCommand, operation string) error {
	original, err := s.Repository.FinancialCommand(ctx, c.ID)
	if err != nil {
		return err
	}
	if command(original).Fingerprint() != c.Fingerprint() {
		return billing.ErrConflict
	}
	original.DispatchOperationID = operation
	_, err = s.Repository.AdmitFinancialCommand(ctx, original)
	return err
}
func (s Source) AuthorizeServiceCheckout(ctx context.Context, org, actor, order string) error {
	if s.Authorizer == nil {
		return billing.ErrAuthorizationRevoked
	}
	if err := s.Authorizer.AuthorizeServicePurchase(ctx, org, actor); err != nil {
		return err
	}
	original, err := s.Repository.OriginalFinancialCommand(ctx, order)
	if err != nil {
		return err
	}
	if original.BuyerOrganizationID != org {
		return billing.ErrNotFound
	}
	page, err := s.Repository.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: org, ActorID: actor}, Kind: "requests", ID: original.RequestID, Page: 1, PageSize: 1})
	if err != nil {
		return err
	}
	if len(page.Requests) != 1 || page.Requests[0].State != "ORDER_PENDING" || page.Requests[0].FinancialFence {
		return billing.ErrOrderCancelled
	}
	if err := s.Repository.VerifyProviderQualification(ctx, original.ProviderOrganizationID, original.MerchantID); err != nil {
		return billing.ErrAuthorizationRevoked
	}
	return nil
}

type Trading struct{ Purchases *billing.ServicePurchases }

func (t Trading) ReadServiceRefundableAmount(ctx context.Context, order string) (int64, error) {
	return t.Purchases.ReadServiceRefundableAmount(ctx, order)
}

func (t Trading) ExecuteServiceCommand(ctx context.Context, in e.FinancialCommand) (e.FinancialResult, error) {
	r, err := t.Purchases.Execute(ctx, command(in))
	return e.FinancialResult{OrderID: r.OrderID, PaymentReceiptID: r.PaymentReceiptID, ReceiptID: r.ReceiptID, State: r.State, Reason: r.Reason, FundsExpireAt: r.FundsExpireAt, Revision: r.Revision, FullRefund: r.FullRefund}, err
}
