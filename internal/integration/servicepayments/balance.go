package servicepayments

import (
	"context"
	wechat "github.com/go-pay/gopay/wechat/v3"
	b "task-processor/internal/commercial/billing"
	"task-processor/internal/integration/paymentsecurity"
	"task-processor/internal/ledger/money"
)

func (p *WeChat) QueryServiceUnsplit(ctx context.Context, o b.ServicePurchaseOrder) (b.ServiceUnsplitObservation, error) {
	if !p.matches(o) || o.Payment == nil || o.Payment.TransactionID == "" {
		return b.ServiceUnsplitObservation{}, b.ErrConflict
	}
	response, err := p.client.V3EcommerceProfitShareUnsplitAmount(ctx, o.Payment.TransactionID)
	if err != nil || response == nil || response.Code != 0 || response.Response == nil {
		return b.ServiceUnsplitObservation{}, b.ErrReconciliationRequired
	}
	return p.unsplitResult(o, response.SignInfo)
}
func (p *WeChat) unsplitResult(o b.ServicePurchaseOrder, s *wechat.SignInfo) (b.ServiceUnsplitObservation, error) {
	var out b.ServiceUnsplitObservation
	if !p.matches(o) || o.Payment == nil || !paymentsecurity.VerifiedWeChatResponse(s, p.config.PublicKeyID, p.publicKey, p.now()) {
		return out, b.ErrReconciliationRequired
	}
	var body struct {
		TransactionID string `json:"transaction_id"`
		UnsplitAmount *int64 `json:"unsplit_amount"`
	}
	if paymentsecurity.StrictJSON([]byte(s.SignBody), &body) != nil || body.UnsplitAmount == nil {
		return out, b.ErrReconciliationRequired
	}
	at, err := providerResponseTime(s)
	if err != nil {
		return out, err
	}
	out = b.ServiceUnsplitObservation{ProfileVersion: o.Profile.Version, ProviderMerchantID: o.Source.ProviderMerchantID, TransactionID: body.TransactionID, UnsplitMinor: *body.UnsplitAmount, VerificationVersion: "wechat-v3:" + p.config.PublicKeyID, OccurredAt: at, ProofID: "wechat-unsplit:" + money.ServiceFingerprint(struct{ Profile, Transaction, Serial, Timestamp, Body string }{o.Profile.Version, body.TransactionID, s.HeaderSerial, s.HeaderTimestamp, s.SignBody})}
	if !out.Matches(o) {
		return b.ServiceUnsplitObservation{}, b.ErrConflict
	}
	return out, nil
}
