package billing

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"task-processor/internal/ledger/money"

	"github.com/google/uuid"
)

type walletTopUps struct {
	store      TopUpStore
	money      money.ProviderTopUpOwner
	alipay     TopUpProviderPort
	wechat     TopUpProviderPort
	policy     TopUpAmountPolicy
	protection TopUpPayloadProtection
	authorizer TopUpAuthorizer
}

func (s *Service) EnableWalletTopUps(store TopUpStore, owner money.ProviderTopUpOwner, alipay, wechat TopUpProviderPort, policy TopUpAmountPolicy, protection TopUpPayloadProtection, auth TopUpAuthorizer) error {
	if s == nil || store == nil || owner == nil || auth == nil {
		return ErrInvalid
	}
	if alipay != nil && (alipay.Merchant().Provider != PaymentAlipay || alipay.Merchant().Validate() != nil) {
		return ErrInvalid
	}
	if wechat != nil && (wechat.Merchant().Provider != PaymentWeChat || wechat.Merchant().Validate() != nil) {
		return ErrInvalid
	}
	s.topups = &walletTopUps{store: store, money: owner, alipay: alipay, wechat: wechat, policy: policy, protection: protection, authorizer: auth}
	return nil
}
func (t *walletTopUps) provider(p PaymentProvider) TopUpProviderPort {
	if p == PaymentAlipay {
		return t.alipay
	}
	if p == PaymentWeChat {
		return t.wechat
	}
	return nil
}
func (t *walletTopUps) originalProvider(a TopUpPaymentAttempt) (TopUpProviderPort, error) {
	p := t.provider(a.Merchant.Provider)
	if p == nil || p.Merchant() != a.Merchant {
		return nil, ErrPaymentMethodUnavailable
	}
	return p, nil
}

type TopUpChannelOption struct {
	Provider  PaymentProvider
	Product   string
	Available bool
	Reason    string
}
type TopUpOptions struct {
	Policy   TopUpAmountPolicy
	Channels []TopUpChannelOption
}

func (s *Service) WalletTopUpOptions() TopUpOptions {
	result := TopUpOptions{Channels: []TopUpChannelOption{{Provider: PaymentAlipay, Product: "PAGE_PAY", Reason: "PAYMENT_NOT_CONFIGURED"}, {Provider: PaymentWeChat, Product: "NATIVE", Reason: "PAYMENT_NOT_CONFIGURED"}}}
	if s == nil || s.topups == nil {
		return result
	}
	t := s.topups
	if t.policy.Validate() == nil {
		result.Policy = t.policy
	}
	for i := range result.Channels {
		p := t.provider(result.Channels[i].Provider)
		if p == nil {
			continue
		}
		if t.policy.Validate() != nil {
			result.Channels[i].Reason = "AMOUNT_POLICY_NOT_CONFIGURED"
			continue
		}
		if t.protection == nil {
			result.Channels[i].Reason = "PAYMENT_NOT_CONFIGURED"
			continue
		}
		if !p.Available() {
			result.Channels[i].Reason = "NEW_PAYMENTS_DISABLED"
			continue
		}
		result.Channels[i].Available = true
		result.Channels[i].Reason = ""
	}
	return result
}

func (s *Service) createWalletTopUp(ctx context.Context, req CreateWalletTopUpOrderRequest) (Order, error) {
	if s == nil || s.topups == nil {
		return Order{}, ErrFeatureUnavailable
	}
	t := s.topups
	if !isCanonicalIdentifier(req.OrganizationID) || !isCanonicalIdentifier(req.ActorID) || req.Currency != CurrencyCNY || req.AmountMinor <= 0 || strings.TrimSpace(req.IdempotencyKey) == "" || len(req.IdempotencyKey) > 192 || (req.Provider != PaymentAlipay && req.Provider != PaymentWeChat) {
		return Order{}, ErrInvalid
	}
	if err := t.authorizer.AuthorizeTopUp(ctx, req.OrganizationID, req.ActorID, false); err != nil {
		return Order{}, err
	}
	if previous, err := t.store.FindTopUpAttempt(ctx, req.OrganizationID, req.IdempotencyKey); err == nil {
		if previous.ActorID != req.ActorID || previous.Merchant.Provider != req.Provider || previous.AmountMinor != req.AmountMinor || previous.Currency != req.Currency {
			return Order{}, ErrConflict
		}
		return s.ReadOrder(ctx, req.OrganizationID, previous.OrderID)
	} else if !errors.Is(err, ErrNotFound) {
		return Order{}, err
	}
	p := t.provider(req.Provider)
	if p == nil || !p.Available() || t.protection == nil || t.policy.Validate() != nil {
		return Order{}, ErrPaymentMethodUnavailable
	}
	if req.AmountMinor < t.policy.MinMinor || req.AmountMinor > t.policy.MaxMinor {
		return Order{}, ErrInvalid
	}
	now := money.NormalizeTimestamp(s.now())
	a, err := t.store.CreateTopUpAttempt(ctx, req, p.Merchant(), now, now.Add(t.policy.PaymentWindow))
	if err != nil {
		return Order{}, err
	}
	return s.ReadOrder(ctx, req.OrganizationID, a.OrderID)
}
func (s *Service) ReadTopUpAttempt(ctx context.Context, org, order string) (TopUpPaymentAttempt, error) {
	if s == nil || s.topups == nil {
		return TopUpPaymentAttempt{}, ErrFeatureUnavailable
	}
	return s.topups.store.ReadTopUpAttempt(ctx, org, order)
}

func (s *Service) CheckoutTopUp(ctx context.Context, org, actor, order string, version int64) (CheckoutAction, error) {
	var out CheckoutAction
	if s == nil || s.topups == nil {
		return out, ErrFeatureUnavailable
	}
	t := s.topups
	if err := t.authorizer.AuthorizeTopUp(ctx, org, actor, false); err != nil {
		return out, err
	}
	a, err := t.store.ReadTopUpAttempt(ctx, org, order)
	if err != nil {
		return out, err
	}
	if a.ActorID != actor {
		return out, ErrAuthorizationRevoked
	}
	if a.Version != version {
		return out, ErrConflict
	}
	if !s.now().Before(a.ExpiresAt) || a.CloseRequestedAt != nil || a.Phase == TopUpClosedUnpaid || a.Phase == TopUpCompleted || a.Phase == TopUpPaidPendingCredit || a.NeedsReconcile {
		return out, ErrReconciliationRequired
	}
	p, err := t.originalProvider(a)
	if err != nil {
		return out, err
	}
	if !p.Available() || t.protection == nil {
		return out, ErrPaymentMethodUnavailable
	}
	if len(a.CheckoutCiphertext) > 0 {
		payload, err := t.protection.Open(a.Fingerprint(), a.CheckoutCiphertext)
		if err != nil {
			return out, ErrReconciliationRequired
		}
		out = CheckoutAction{Kind: a.CheckoutKind, Payload: payload, OrderID: a.OrderID, AttemptID: a.AttemptID, Provider: a.Merchant.Provider, ExpiresAt: a.ExpiresAt, RequestFingerprint: a.Fingerprint()}
		if !out.Matches(a) {
			return CheckoutAction{}, ErrConflict
		}
		return out, nil
	}
	if a.CheckoutAdmittedAt != nil {
		return out, ErrReconciliationRequired
	}
	now := money.NormalizeTimestamp(s.now())
	a.CheckoutAdmittedAt = &now
	a.Phase = TopUpAwaitingPayment
	a.LeaseToken = uuid.NewString()
	a.LeaseUntil = now.Add(30 * time.Second)
	a.NextCheckAt = now.Add(30 * time.Second)
	a, err = t.store.SaveTopUpAttempt(ctx, a)
	if err != nil {
		return out, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err = p.CreateOrReadCheckout(requestCtx, a)
	if errors.Is(err, ErrCheckoutNotDispatched) {
		a.Phase = TopUpClosedUnpaid
		a.ClosedAt = &now
		a.LeaseToken = ""
		a.LeaseUntil = time.Time{}
		if _, saveErr := t.store.SaveTopUpAttempt(ctx, a); saveErr != nil {
			return CheckoutAction{}, ErrReconciliationRequired
		}
		return CheckoutAction{}, ErrPaymentMethodUnavailable
	}
	if err != nil || !out.Matches(a) {
		a.Phase = TopUpReconciliationRequired
		a.LastSafeError = "CHECKOUT_RESULT_UNKNOWN"
		a.LeaseToken = ""
		a.LeaseUntil = time.Time{}
		_, _ = t.store.SaveTopUpAttempt(ctx, a)
		return CheckoutAction{}, ErrReconciliationRequired
	}
	a.CheckoutCiphertext, err = t.protection.Seal(a.Fingerprint(), out.Payload)
	if err != nil {
		return CheckoutAction{}, ErrReconciliationRequired
	}
	a.CheckoutKind = out.Kind
	a.LeaseToken = ""
	a.LeaseUntil = time.Time{}
	if _, err = t.store.SaveTopUpAttempt(ctx, a); err != nil {
		return CheckoutAction{}, ErrReconciliationRequired
	}
	return out, nil
}

func (s *Service) RecordVerifiedPaymentObservation(ctx context.Context, o ProviderObservation) error {
	if s == nil || s.topups == nil {
		return ErrFeatureUnavailable
	}
	return s.topups.store.RecordTopUpObservation(ctx, o)
}
func (s *Service) CancelTopUp(ctx context.Context, org, actor, order string, version int64) error {
	if s == nil || s.topups == nil {
		return ErrFeatureUnavailable
	}
	t := s.topups
	if err := t.authorizer.AuthorizeTopUp(ctx, org, actor, false); err != nil {
		return err
	}
	a, err := t.store.ReadTopUpAttempt(ctx, org, order)
	if err != nil {
		return err
	}
	if a.Version != version {
		return ErrConflict
	}
	if a.Phase == TopUpCompleted || a.Phase == TopUpPaidPendingCredit {
		return ErrConflict
	}
	if a.Phase == TopUpClosedUnpaid {
		return nil
	}
	now := money.NormalizeTimestamp(s.now())
	a.CloseRequestedAt = &now
	a.NextCheckAt = now
	if a.CheckoutAdmittedAt == nil && !a.NeedsReconcile {
		a.Phase = TopUpClosedUnpaid
		a.ClosedAt = &now
	}
	if _, err = t.store.SaveTopUpAttempt(ctx, a); err != nil {
		return err
	}
	return s.ReconcileTopUpOrder(ctx, org, order)
}

func (s *Service) ReconcileTopUpOrder(ctx context.Context, org, order string) error {
	if s == nil || s.topups == nil {
		return ErrFeatureUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	t := s.topups
	a, err := t.store.ReadTopUpAttempt(ctx, org, order)
	if err != nil {
		return err
	}
	if a.LeaseToken != "" && s.now().Before(a.LeaseUntil) {
		return ErrReconciliationRequired
	}
	if (a.Phase == TopUpCompleted || a.Phase == TopUpClosedUnpaid) && !a.NeedsReconcile {
		return nil
	}
	// A CAS lease fences all network responses and completion writes. Advancing
	// next_check_at before work prevents unavailable channels starving the queue.
	a.LeaseToken = uuid.NewString()
	a.LeaseUntil = s.now().Add(30 * time.Second)
	a.NextCheckAt = s.now().Add(30 * time.Second)
	if a.Phase == TopUpCompleted {
		a.Phase = TopUpPaidPendingCredit
	}
	a, err = t.store.SaveTopUpAttempt(ctx, a)
	if err != nil {
		return err
	}
	lease := a.LeaseToken
	observations, err := t.store.ReadTopUpObservations(ctx, a)
	if err != nil {
		return s.deferTopUp(ctx, a, "EVIDENCE_UNAVAILABLE")
	}
	paid, hasPaid, err := selectTopUpPayment(a, observations)
	if err != nil {
		return s.deferTopUp(ctx, a, "EVIDENCE_CONFLICT")
	}
	normalPending := a.CheckoutAdmittedAt == nil && len(observations) == 0
	if !hasPaid && a.CheckoutAdmittedAt != nil {
		p, err := t.originalProvider(a)
		if err != nil {
			return s.deferTopUp(ctx, a, "PROVIDER_UNAVAILABLE")
		}
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		o, queryErr := p.QueryPayment(requestCtx, a)
		cancel()
		if queryErr == nil && o.Validate() == nil && o.Matches(a) {
			// Page Pay creates a signed URL locally; the trade need not exist until
			// the user opens it. A verified absence preserves only that saved URL.
			unvisitedPagePay := o.State == "NOT_FOUND" && a.Merchant.Provider == PaymentAlipay && a.CheckoutKind == "REDIRECT"
			normalPending = (o.State == "UNPAID" || unvisitedPagePay) && len(a.CheckoutCiphertext) > 0
			if err := t.store.RecordTopUpObservation(ctx, o); err != nil {
				return s.deferTopUp(ctx, a, "EVIDENCE_STORE_UNAVAILABLE")
			}
			a, err = t.store.ReadTopUpAttempt(ctx, org, order)
			if err != nil {
				return err
			}
			if a.LeaseToken != lease || !s.now().Before(a.LeaseUntil) {
				return ErrConflict
			}
			observations, err = t.store.ReadTopUpObservations(ctx, a)
			if err != nil {
				return err
			}
			paid, hasPaid, err = selectTopUpPayment(a, observations)
			if err != nil {
				return s.deferTopUp(ctx, a, "EVIDENCE_CONFLICT")
			}
		}
	}
	if hasPaid {
		// A signed refund hint is not an individual reversal. Resolve its original
		// request first so a known refund cannot become briefly spendable credit.
		resolved := map[string]bool{}
		for _, o := range observations {
			if o.Kind == "REFUND" && (o.State == "REFUNDED" || o.State == "REFUND_CLOSED") {
				resolved[o.RefundRequestID] = true
			}
		}
		for _, hint := range observations {
			if hint.Kind != "REFUND" || hint.State != "REFUND_PENDING" || resolved[hint.RefundRequestID] {
				continue
			}
			p, providerErr := t.originalProvider(a)
			if providerErr != nil {
				return s.deferTopUp(ctx, a, "REFUND_PROVIDER_UNAVAILABLE")
			}
			requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			o, queryErr := p.QueryRefund(requestCtx, TopUpRefundIntent{Merchant: a.Merchant, MerchantOrderID: a.MerchantOrderID, TradeID: hint.TradeID, ProviderRequestID: hint.RefundRequestID, TotalMinor: a.AmountMinor})
			cancel()
			if queryErr != nil || o.Validate() != nil || !o.Matches(a) || o.Kind != "REFUND" || o.TradeID != paid.TradeID || o.RefundRequestID != hint.RefundRequestID || (o.State != "REFUNDED" && o.State != "REFUND_CLOSED") {
				return s.deferTopUp(ctx, a, "REFUND_RESULT_UNKNOWN")
			}
			if err := t.store.RecordTopUpObservation(ctx, o); err != nil {
				return s.deferTopUp(ctx, a, "EVIDENCE_STORE_UNAVAILABLE")
			}
			a, err = t.store.ReadTopUpAttempt(ctx, org, order)
			if err != nil {
				return err
			}
			if a.LeaseToken != lease || !s.now().Before(a.LeaseUntil) {
				return ErrConflict
			}
			observations = append(observations, o)
			resolved[o.RefundRequestID] = true
		}
		input := a.MoneyInput(paid)
		unresolvedAggregate := false
		remainingPrincipal := a.AmountMinor
		confirmedRefunds := map[string]bool{}
		for _, o := range observations {
			if o.Kind == "PAYMENT" && o.State == "PAID_REFUND_UNKNOWN" {
				unresolvedAggregate = true
			}
			if o.Kind != "REFUND" || o.State != "REFUNDED" {
				continue
			}
			if !o.Matches(a) || o.TradeID != paid.TradeID {
				return s.deferTopUp(ctx, a, "REFUND_BINDING_CONFLICT")
			}
			key := o.RefundKey(input.Payment.PaymentID)
			if !confirmedRefunds[key.ReversalID] {
				remainingPrincipal -= min(remainingPrincipal, o.AmountMinor)
				confirmedRefunds[key.ReversalID] = true
			}
			input.KnownReversals = append(input.KnownReversals, money.OrganizationWalletReversal{ReversalID: key.ReversalID, PaymentID: key.PaymentID, Kind: key.Kind, OrganizationID: a.OrganizationID, CommercialOrderID: a.OrderID, Currency: a.Currency, AmountMinor: o.AmountMinor, OccurredAt: o.OccurredAt, ProviderReference: "provider-refund:" + money.TopUpFingerprint([]string{string(o.Merchant.Provider), o.Merchant.Environment, o.Merchant.MerchantID, o.RefundRequestID})})
		}
		// A trade-level refund state does not enumerate individual refund IDs or
		// amounts. Partial receipts cannot prove that all refunds are known. Keep
		// reconciliation until original receipts cover the entire principal, at
		// which point M1 can atomically post with no spendable net credit.
		if unresolvedAggregate && remainingPrincipal > 0 {
			return s.deferTopUp(ctx, a, "REFUND_DETAILS_UNKNOWN")
		}
		a.Phase = TopUpPaidPendingCredit
		a.NeedsReconcile = false
		a.LastSafeError = ""
		a, err = t.store.SaveTopUpAttempt(ctx, a)
		if err != nil {
			return err
		}
		receipt, err := t.money.AcceptAndPostProviderTopUp(ctx, input)
		if err != nil {
			receipt, err = t.money.ReadTopUpPosting(ctx, a.OrganizationID, a.OrderID, input.Payment.PaymentID)
			if err != nil {
				return s.deferTopUp(ctx, a, "POSTING_RESULT_UNKNOWN")
			}
			if len(input.KnownReversals) > 0 {
				return s.deferTopUp(ctx, a, "REVERSAL_RESULT_UNKNOWN")
			}
		}
		if receipt.Validate() != nil || receipt.RequestFingerprint != input.Fingerprint() || receipt.PaymentID != input.Payment.PaymentID || receipt.Binding != input.Binding {
			return s.deferTopUp(ctx, a, "POSTING_RECEIPT_CONFLICT")
		}
		_, err = t.store.CompleteTopUpOrder(ctx, a, receipt)
		return err
	}
	confirmedClosed := false
	for _, o := range observations {
		if o.Kind == "REFUND" {
			normalPending = false
		}
		if o.Kind == "PAYMENT" && o.State == "CLOSED" {
			confirmedClosed = true
		}
	}
	if confirmedClosed || a.CloseRequestedAt != nil || !s.now().Before(a.ExpiresAt) {
		now := money.NormalizeTimestamp(s.now())
		if a.CloseRequestedAt == nil {
			a.CloseRequestedAt = &now
			a, err = t.store.SaveTopUpAttempt(ctx, a)
			if err != nil {
				return err
			}
		}
		closed := a.CheckoutAdmittedAt == nil || confirmedClosed
		if !closed {
			p, err := t.originalProvider(a)
			if err != nil {
				return s.deferTopUp(ctx, a, "PROVIDER_UNAVAILABLE")
			}
			requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			o, closeErr := p.ClosePayment(requestCtx, a)
			cancel()
			if closeErr == nil && o.Validate() == nil && o.Matches(a) {
				if err := t.store.RecordTopUpObservation(ctx, o); err != nil {
					return err
				}
				a, err = t.store.ReadTopUpAttempt(ctx, org, order)
				if err != nil {
					return err
				}
				if a.LeaseToken != lease || !s.now().Before(a.LeaseUntil) {
					return ErrConflict
				}
				if o.State == "PAID" || o.State == "PAID_REFUND_UNKNOWN" {
					a.LeaseToken = ""
					a.LeaseUntil = time.Time{}
					if _, err = t.store.SaveTopUpAttempt(ctx, a); err != nil {
						return err
					}
					return s.ReconcileTopUpOrder(ctx, org, order)
				}
				closed = o.State == "CLOSED"
			}
		}
		if closed {
			a.Phase = TopUpClosedUnpaid
			a.ClosedAt = &now
			a.CheckoutCiphertext = nil
			a.CheckoutKind = ""
			a.NeedsReconcile = false
			a.LeaseToken = ""
			a.LeaseUntil = time.Time{}
			_, err = t.store.SaveTopUpAttempt(ctx, a)
			return err
		}
	}
	if normalPending && a.CloseRequestedAt == nil && s.now().Before(a.ExpiresAt) {
		a.Phase = TopUpCreated
		if a.CheckoutAdmittedAt != nil {
			a.Phase = TopUpAwaitingPayment
		}
		a.NeedsReconcile = false
		a.LastSafeError = ""
		a.RetryCount = 0
		a.LeaseToken = ""
		a.LeaseUntil = time.Time{}
		_, err = t.store.SaveTopUpAttempt(ctx, a)
		return err
	}
	return s.deferTopUp(ctx, a, "PAYMENT_NOT_CONFIRMED")
}

func selectTopUpPayment(a TopUpPaymentAttempt, observations []ProviderObservation) (ProviderObservation, bool, error) {
	var paid ProviderObservation
	found := false
	for _, o := range observations {
		if !o.Matches(a) {
			return paid, false, ErrConflict
		}
		if o.Kind == "PAYMENT" && (o.State == "PAID" || o.State == "PAID_REFUND_UNKNOWN") {
			if found && (paid.TradeID != o.TradeID || !paid.OccurredAt.Equal(o.OccurredAt)) {
				return paid, false, ErrConflict
			}
			paid = o
			found = true
		}
	}
	return paid, found, nil
}
func (s *Service) deferTopUp(ctx context.Context, a TopUpPaymentAttempt, code string) error {
	a.Phase = TopUpReconciliationRequired
	a.LastSafeError = code
	a.RetryCount++
	a.LeaseToken = ""
	a.LeaseUntil = time.Time{}
	a.NeedsReconcile = false
	delay := 30 * time.Second * time.Duration(1<<min(a.RetryCount, 4))
	a.NextCheckAt = s.now().Add(delay)
	_, err := s.topups.store.SaveTopUpAttempt(ctx, a)
	if err != nil {
		return err
	}
	return ErrReconciliationRequired
}

func (s *Service) ApproveTopUpRefund(ctx context.Context, actor, order, key, reason string, amount, version int64) (TopUpRefundIntent, error) {
	var out TopUpRefundIntent
	if s == nil || s.topups == nil {
		return out, ErrFeatureUnavailable
	}
	t := s.topups
	if !isCanonicalIdentifier(actor) || !isCanonicalIdentifier(order) || strings.TrimSpace(key) == "" || len(key) > 192 || strings.TrimSpace(reason) == "" || len(reason) > 500 || amount <= 0 {
		return out, ErrInvalid
	}
	if err := t.authorizer.AuthorizeTopUp(ctx, "", actor, true); err != nil {
		return out, err
	}
	a, err := t.store.ReadTopUpForRefund(ctx, order)
	if err != nil {
		return out, err
	}
	if a.Version != version || a.Phase != TopUpCompleted {
		return out, ErrConflict
	}
	receipt, err := t.money.ReadTopUpPosting(ctx, a.OrganizationID, a.OrderID, a.PaymentID)
	if err != nil || receipt.Validate() != nil {
		return out, ErrReconciliationRequired
	}
	requestID := strings.ReplaceAll(uuid.NewString(), "-", "")
	o := ProviderObservation{Merchant: a.Merchant, RefundRequestID: requestID}
	refundID := o.RefundKey(a.PaymentID).ReversalID
	out = TopUpRefundIntent{RefundID: refundID, OrderID: a.OrderID, OrganizationID: a.OrganizationID, ActorID: actor, IdempotencyKey: key, AmountMinor: amount, TotalMinor: a.AmountMinor, Reason: reason, Merchant: a.Merchant, MerchantOrderID: a.MerchantOrderID, TradeID: receipt.Binding.TradeID, PaymentID: a.PaymentID, ProviderRequestID: requestID, State: "PREPARED", Version: 1, CreatedAt: money.NormalizeTimestamp(s.now()), NextCheckAt: s.now()}
	out, err = t.store.CreateTopUpRefund(ctx, out)
	if err != nil {
		return out, err
	}
	if out.State == "CONFIRMED" || out.State == "RELEASED" || out.State == "REJECTED" {
		return out, nil
	}
	if out.Dispatched {
		return s.reconcileTopUpRefund(ctx, out)
	}
	if _, err = t.money.PrepareTopUpRefund(ctx, out.HoldInput()); err != nil {
		return out, err
	}
	if err = t.authorizer.AuthorizeTopUp(ctx, "", actor, true); err != nil {
		return s.releaseUndispatchedTopUpRefund(ctx, out, err)
	}
	p, err := t.originalProvider(a)
	if err != nil {
		return s.releaseUndispatchedTopUpRefund(ctx, out, err)
	}
	if _, err = t.money.AdmitTopUpRefund(ctx, out.HoldInput()); err != nil {
		return s.releaseUndispatchedTopUpRefund(ctx, out, err)
	}
	out.Dispatched = true
	out.State = "UNKNOWN"
	out.LeaseToken = uuid.NewString()
	out.LeaseUntil = s.now().Add(30 * time.Second)
	out.NextCheckAt = s.now().Add(30 * time.Second)
	out, err = t.store.SaveTopUpRefund(ctx, out)
	if err != nil {
		return out, ErrReconciliationRequired
	}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	observation, callErr := p.Refund(requestCtx, out)
	cancel()
	if callErr == nil {
		if err = s.acceptTopUpRefundObservation(ctx, a, out, observation); err != nil {
			return out, err
		}
	}
	out.LeaseToken = ""
	out.LeaseUntil = time.Time{}
	out, err = t.store.SaveTopUpRefund(ctx, out)
	if err != nil {
		return out, err
	}
	return s.reconcileTopUpRefund(ctx, out)
}

// A money admission may have committed despite a lost response. Only the money
// owner can prove this hold was never admitted; no local flag authorizes release.
func (s *Service) releaseUndispatchedTopUpRefund(ctx context.Context, r TopUpRefundIntent, cause error) (TopUpRefundIntent, error) {
	if _, err := s.topups.money.ReleaseTopUpRefundHold(ctx, r.HoldInput(), false); err != nil {
		return r, cause
	}
	r.State = "RELEASED"
	updated, err := s.topups.store.SaveTopUpRefund(ctx, r)
	if err != nil {
		return r, err
	}
	return updated, cause
}

func (s *Service) acceptTopUpRefundObservation(ctx context.Context, a TopUpPaymentAttempt, r TopUpRefundIntent, o ProviderObservation) error {
	if o.Validate() != nil || !o.Matches(a) || o.Kind != "REFUND" || o.RefundRequestID != r.ProviderRequestID || o.TradeID != r.TradeID || o.AmountMinor != r.AmountMinor || o.RefundKey(r.PaymentID).ReversalID != r.RefundID {
		return ErrConflict
	}
	return s.RecordVerifiedPaymentObservation(ctx, o)
}
func (s *Service) reconcileTopUpRefund(ctx context.Context, r TopUpRefundIntent) (TopUpRefundIntent, error) {
	t := s.topups
	if r.LeaseToken != "" && s.now().Before(r.LeaseUntil) {
		return r, ErrReconciliationRequired
	}
	r.LeaseToken = uuid.NewString()
	r.LeaseUntil = s.now().Add(30 * time.Second)
	r.NextCheckAt = s.now().Add(time.Minute)
	var err error
	r, err = t.store.SaveTopUpRefund(ctx, r)
	if err != nil {
		return r, err
	}
	a, err := t.store.ReadTopUpAttempt(ctx, r.OrganizationID, r.OrderID)
	if err != nil {
		return r, err
	}
	// Money readback always precedes a channel query or any new dispatch.
	if receipt, readErr := t.money.ReadTopUpReversal(ctx, r.OrganizationID, r.OrderID, r.HoldInput().Key); readErr == nil {
		if receipt.Validate() != nil || receipt.HoldState != "CONFIRMED" || receipt.Key != r.HoldInput().Key || receipt.OrganizationID != r.OrganizationID || receipt.CommercialOrderID != r.OrderID || receipt.ProviderAmountMinor != r.AmountMinor {
			return r, ErrConflict
		}
		r.State = "CONFIRMED"
	} else if !errors.Is(readErr, money.ErrNotFound) {
		return r, ErrReconciliationRequired
	} else {
		hold, holdErr := t.money.ReadTopUpRefundHold(ctx, r.HoldInput())
		if holdErr == nil && hold.Input == r.HoldInput() && hold.State == "RELEASED" {
			// Release has no reversal receipt. Recover its committed money outcome
			// directly when the billing projection was lost, without channel calls.
			r.State = "RELEASED"
			r.LeaseToken = ""
			r.LeaseUntil = time.Time{}
			return t.store.SaveTopUpRefund(ctx, r)
		}
		if holdErr != nil || hold.Input != r.HoldInput() || hold.State != "RESERVED" || !hold.Dispatched {
			// A restart never creates or admits a hold. Only the money owner's
			// durable admission can recover a lost billing projection.
			r.LeaseToken = ""
			r.LeaseUntil = time.Time{}
			r.NextCheckAt = s.now().Add(15 * time.Minute)
			_, _ = t.store.SaveTopUpRefund(ctx, r)
			return r, ErrReconciliationRequired
		}
		if !r.Dispatched {
			r.Dispatched = true
			r.State = "UNKNOWN"
			r, err = t.store.SaveTopUpRefund(ctx, r)
			if err != nil {
				return r, err
			}
		}
		p, providerErr := t.originalProvider(a)
		if providerErr != nil {
			return r, providerErr
		}
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		o, queryErr := p.QueryRefund(requestCtx, r)
		cancel()
		if errors.Is(queryErr, ErrRefundReplayAllowed) {
			hold, holdErr := t.money.ReadTopUpRefundHold(ctx, r.HoldInput())
			if holdErr != nil || hold.Input != r.HoldInput() || hold.State != "RESERVED" || !hold.Dispatched {
				return r, ErrReconciliationRequired
			}
			// Authorization and budget were frozen at the original money admission.
			// This CAS fences a stale worker before replaying that exact capability;
			// no new intent, hold, request ID, amount or admission is created.
			if !s.now().Before(r.LeaseUntil) {
				return r, ErrConflict
			}
			r, err = t.store.SaveTopUpRefund(ctx, r)
			if err != nil {
				return r, err
			}
			requestCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
			o, queryErr = p.Refund(requestCtx, r)
			cancel()
		}
		if queryErr == nil && s.acceptTopUpRefundObservation(ctx, a, r, o) == nil {
			if o.State == "REFUNDED" {
				_, err = t.money.AcceptProviderTopUpReversal(ctx, money.OrganizationWalletReversal{ReversalID: r.RefundID, PaymentID: r.PaymentID, Kind: money.WalletReversalRefund, OrganizationID: r.OrganizationID, CommercialOrderID: r.OrderID, Currency: CurrencyCNY, AmountMinor: r.AmountMinor, OccurredAt: o.OccurredAt, ProviderReference: "provider-refund:" + money.TopUpFingerprint([]string{string(o.Merchant.Provider), o.Merchant.Environment, o.Merchant.MerchantID, o.RefundRequestID})})
				if err == nil {
					r.State = "CONFIRMED"
				}
			} else if o.State == "REFUND_CLOSED" {
				if _, err = t.money.ReleaseTopUpRefundHold(ctx, r.HoldInput(), true); err == nil {
					r.State = "RELEASED"
				}
			}
		}
	}
	r.LeaseToken = ""
	r.LeaseUntil = time.Time{}
	updated, saveErr := t.store.SaveTopUpRefund(ctx, r)
	if saveErr != nil {
		return r, saveErr
	}
	return updated, err
}

func (s *Service) ReconcileRecoverableTopUps(ctx context.Context) error {
	if s == nil || s.topups == nil {
		return nil
	}
	// Independent channel workers prevent one unavailable channel starving the
	// other. Each worker uses bounded rows and every attempt advances its cursor.
	var wg sync.WaitGroup
	failures := make(chan error, 4)
	for _, provider := range []PaymentProvider{PaymentAlipay, PaymentWeChat} {
		wg.Go(func() {
			rows, err := s.topups.store.ListRecoverableTopUps(ctx, provider, s.now(), 25)
			if err != nil {
				failures <- err
				return
			}
			for _, a := range rows {
				if ctx.Err() != nil {
					return
				}
				_ = s.ReconcileTopUpOrder(ctx, a.OrganizationID, a.OrderID)
			}
		})
		wg.Go(func() {
			refunds, err := s.topups.store.ListRecoverableTopUpRefunds(ctx, provider, s.now(), 25)
			if err != nil {
				failures <- err
				return
			}
			for _, r := range refunds {
				if ctx.Err() != nil {
					return
				}
				_, _ = s.reconcileTopUpRefund(ctx, r)
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		return err
	}
	return ctx.Err()
}
