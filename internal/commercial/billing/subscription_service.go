package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"task-processor/internal/ledger/money"
)

func (s *Service) ReconcileRecoverableSubscriptionOrders(ctx context.Context, limit int) error {
	if !s.subscriptionPurchasesEnabled() || limit < 1 || limit > 50 {
		return ErrFeatureUnavailable
	}
	orders, err := s.subscriptionOrders.ListRecoverableSubscriptionOrders(ctx, limit)
	if err != nil {
		return err
	}
	var failures []error
	for _, order := range orders {
		_, reconcileErr := s.ReconcileSubscriptionOrder(ctx, order.OrganizationID, order.OrderID)
		if reconcileErr != nil && !errors.Is(reconcileErr, ErrInsufficientFunds) && !errors.Is(reconcileErr, ErrActiveSubscriptionExists) && !errors.Is(reconcileErr, ErrPlanChanged) && !errors.Is(reconcileErr, ErrAuthorizationRevoked) {
			failures = append(failures, fmt.Errorf("reconcile subscription order %s: %w", order.OrderID, reconcileErr))
		}
	}
	return errors.Join(failures...)
}

// EnableSubscriptionPurchases wires the existing commercial, money, and
// subscription owners together. It deliberately does not create a second
// catalog, wallet, subscription, or entitlement owner.
func (s *Service) EnableSubscriptionPurchases(port SubscriptionPurchasePort, authorizer SubscriptionPurchaseRecoveryAuthorizer) error {
	if s == nil || s.subscriptionOffers == nil || s.subscriptionQuotes == nil || s.subscriptionOrders == nil || port == nil || authorizer == nil {
		return ErrFeatureUnavailable
	}
	s.subscriptions = port
	s.subscriptionAuthorizer = authorizer
	return nil
}

func (s *Service) ListSubscriptionOffers(ctx context.Context, organizationID string) ([]SubscriptionOfferView, error) {
	if !s.subscriptionPurchasesEnabled() {
		return nil, ErrFeatureUnavailable
	}
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return nil, ErrInvalid
	}
	state, err := s.subscriptions.ReadPurchasedSubscriptionState(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if state.BlocksPurchase && strings.TrimSpace(state.PlanCode) == "" {
		return nil, ErrFeatureUnavailable
	}
	offers, err := s.subscriptionOffers.ListSubscriptionOffers(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]SubscriptionOfferView, 0, len(offers))
	for _, offer := range offers {
		plan, resolveErr := s.subscriptions.ResolvePurchasablePlan(ctx, offer.PlanCode)
		if resolveErr != nil || plan.PlanCode != offer.PlanCode || strings.TrimSpace(plan.Fingerprint) == "" {
			views = append(views, SubscriptionOfferView{Offer: offer, Availability: SubscriptionOfferUnavailable})
			continue
		}
		availability := SubscriptionOfferAvailable
		switch {
		case state.BlocksPurchase && state.PlanCode == offer.PlanCode:
			availability = SubscriptionOfferCurrentPlan
		case state.BlocksPurchase:
			availability = SubscriptionOfferActiveConflict
		case offer.SettlementMode == SettlementExternalPayment:
			availability = SubscriptionOfferPaymentUnavailable
		}
		views = append(views, SubscriptionOfferView{Offer: offer, PlanName: plan.DisplayName, Availability: availability})
	}
	return views, nil
}

func (s *Service) CreateSubscriptionQuote(ctx context.Context, request SubscriptionQuoteRequest) (Quote, error) {
	if !s.subscriptionPurchasesEnabled() {
		return Quote{}, ErrFeatureUnavailable
	}
	request.OrganizationID = strings.TrimSpace(request.OrganizationID)
	request.OfferID = strings.TrimSpace(request.OfferID)
	if request.OrganizationID == "" || request.OfferID == "" {
		return Quote{}, ErrInvalid
	}
	offer, err := s.offers.ReadOffer(ctx, request.OfferID)
	if err != nil {
		return Quote{}, err
	}
	if offer.ProductKind != ProductSubscriptionPlan {
		return Quote{}, ErrOfferUnavailable
	}
	plan, err := s.subscriptions.ResolvePurchasablePlan(ctx, offer.PlanCode)
	if err != nil {
		return Quote{}, err
	}
	if strings.TrimSpace(plan.PlanCode) == "" || plan.PlanCode != offer.PlanCode || strings.TrimSpace(plan.Fingerprint) == "" {
		return Quote{}, ErrOfferUnavailable
	}
	return s.subscriptionQuotes.CreateSubscriptionQuote(ctx, request, offer, plan)
}

func (s *Service) CreateSubscriptionOrder(ctx context.Context, request CreateSubscriptionOrderRequest) (Order, error) {
	if !s.subscriptionPurchasesEnabled() {
		return Order{}, ErrFeatureUnavailable
	}
	request.OrganizationID = strings.TrimSpace(request.OrganizationID)
	request.ActorID = strings.TrimSpace(request.ActorID)
	request.QuoteID = strings.TrimSpace(request.QuoteID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.OrganizationID == "" || request.ActorID == "" || request.QuoteID == "" || request.IdempotencyKey == "" {
		return Order{}, ErrInvalid
	}
	order, found, err := s.subscriptionOrders.FindSubscriptionOrderByIdempotency(ctx, request.OrganizationID, request.IdempotencyKey)
	if err != nil {
		return Order{}, err
	}
	if found {
		if order.QuoteID != request.QuoteID || order.ActorID != request.ActorID {
			return Order{}, ErrConflict
		}
		if order.Status == OrderFulfilled {
			return order, nil
		}
		if order.Status == OrderCancelled {
			return order, subscriptionCancellationError(order.FailureCode)
		}
		return s.executeSubscriptionOrder(ctx, order)
	}
	quote, err := s.quotes.ReadQuote(ctx, request.OrganizationID, request.QuoteID)
	if err != nil {
		return Order{}, err
	}
	if quote.ProductKind != ProductSubscriptionPlan {
		return Order{}, ErrInvalid
	}
	if quote.SettlementMode == SettlementExternalPayment {
		return Order{}, ErrPaymentMethodUnavailable
	}
	order, err = s.subscriptionOrders.CreatePendingSubscriptionOrder(ctx, request, quote)
	if err != nil {
		return Order{}, err
	}
	return s.executeSubscriptionOrder(ctx, order)
}

func (s *Service) ReconcileSubscriptionOrder(ctx context.Context, organizationID, orderID string) (Order, error) {
	if !s.subscriptionPurchasesEnabled() {
		return Order{}, ErrFeatureUnavailable
	}
	order, err := s.reader.ReadOrder(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(orderID))
	if err != nil {
		return Order{}, err
	}
	if order.Kind != OrderSubscriptionPurchase {
		return Order{}, ErrNotFound
	}
	if order.Status == OrderFulfilled {
		return order, nil
	}
	if order.Status == OrderCancelled {
		return order, subscriptionCancellationError(order.FailureCode)
	}
	return s.executeSubscriptionOrder(ctx, order)
}

func (s *Service) executeSubscriptionOrder(ctx context.Context, order Order) (Order, error) {
	if order.TerminalIntent == SubscriptionOrderTerminalIntentCancel {
		return s.finishRevokedSubscriptionOrder(ctx, order)
	}
	if order.ActivationOutcome != "" {
		return s.finishSubscriptionActivation(ctx, order, activationResultFromOrder(order))
	}
	if order.SettlementMode == SettlementWallet && order.WalletReservationID == "" {
		resolved, err := s.ensureSubscriptionReserve(ctx, order)
		if err != nil {
			return resolved, err
		}
		order = resolved
		if order.Status == OrderFulfilled {
			return order, nil
		}
	}
	if order.SettlementMode != SettlementZeroPrice && order.SettlementMode != SettlementWallet {
		return order, ErrPaymentMethodUnavailable
	}
	return s.ensureSubscriptionActivation(ctx, order)
}

func (s *Service) ensureSubscriptionReserve(ctx context.Context, order Order) (Order, error) {
	if order.PendingEffect == PendingSubscriptionOrderEffectNone {
		admitted, err := s.admitSubscriptionEffect(ctx, order, PendingSubscriptionOrderEffectReserve)
		if err != nil {
			return admitted, err
		}
		order = admitted
		if order.Status == OrderFulfilled {
			return order, nil
		}
	}
	if order.PendingEffect != PendingSubscriptionOrderEffectReserve {
		return s.markSubscriptionReconciliation(ctx, order)
	}
	input := money.ReserveWalletFundsInput{OperationID: order.OrderID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, Currency: order.Currency, AmountMinor: order.AmountMinor}
	_, reserveErr := s.wallet.ReserveCommercialPurchase(ctx, input)
	decision, decisionErr := s.wallet.ReadCommercialPurchaseReserveDecision(ctx, order.OrganizationID, order.OrderID, order.OrderID)
	if decisionErr != nil {
		_ = reserveErr
		return s.markSubscriptionReconciliation(ctx, order)
	}
	if !matchesReserveDecision(order, decision) {
		return s.markSubscriptionReconciliation(ctx, order)
	}
	switch decision.Outcome {
	case money.WalletReserveDecisionRejectedInsufficientFunds:
		_, activationErr := s.subscriptions.ReadPurchasedSubscriptionActivation(ctx, order.OrganizationID, order.OrderID)
		if !errors.Is(activationErr, ErrSubscriptionActivationNotFound) {
			return s.markSubscriptionReconciliation(ctx, order)
		}
		order.PendingEffect = PendingSubscriptionOrderEffectNone
		order.PendingEffectAdmittedAt = nil
		order.Status = OrderCancelled
		order.FailureCode = OrderFailureInsufficientFunds
		if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
			return order, ErrReconciliationRequired
		}
		return order, ErrInsufficientFunds
	case money.WalletReserveDecisionReserved:
		reservation, err := s.wallet.ReadCommercialPurchaseReservation(ctx, order.OrganizationID, order.OrderID, decision.ReservationID)
		if err != nil || !matchesReservation(order, decision.ReservationID, reservation) || reservation.State != money.WalletReservationReserved {
			return s.markSubscriptionReconciliation(ctx, order)
		}
		order.WalletReservationID = reservation.ReservationID
		order.WalletReservationState = reservation.State
		order.PendingEffect = PendingSubscriptionOrderEffectNone
		order.PendingEffectAdmittedAt = nil
		order.Status = OrderFundsReserved
		if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
			return order, ErrReconciliationRequired
		}
		return order, nil
	default:
		return s.markSubscriptionReconciliation(ctx, order)
	}
}

func (s *Service) ensureSubscriptionActivation(ctx context.Context, order Order) (Order, error) {
	result, err := s.subscriptions.ReadPurchasedSubscriptionActivation(ctx, order.OrganizationID, order.OrderID)
	if err == nil {
		return s.finishSubscriptionActivation(ctx, order, result)
	}
	if !errors.Is(err, ErrSubscriptionActivationNotFound) {
		return s.markSubscriptionReconciliation(ctx, order)
	}
	if order.PendingEffect == PendingSubscriptionOrderEffectNone {
		admitted, admitErr := s.admitSubscriptionEffect(ctx, order, PendingSubscriptionOrderEffectActivate)
		if admitErr != nil {
			return admitted, admitErr
		}
		order = admitted
		if order.Status == OrderFulfilled {
			return order, nil
		}
	}
	if order.PendingEffect != PendingSubscriptionOrderEffectActivate {
		return s.markSubscriptionReconciliation(ctx, order)
	}
	request := activationRequestForOrder(order)
	result, err = s.subscriptions.ActivatePurchasedSubscription(ctx, request)
	if err != nil {
		result, err = s.subscriptions.ReadPurchasedSubscriptionActivation(ctx, order.OrganizationID, order.OrderID)
		if err != nil {
			return s.markSubscriptionReconciliation(ctx, order)
		}
	}
	return s.finishSubscriptionActivation(ctx, order, result)
}

func (s *Service) finishSubscriptionActivation(ctx context.Context, order Order, result SubscriptionActivationResult) (Order, error) {
	if !matchesActivationResult(order, result) {
		return s.markSubscriptionReconciliation(ctx, order)
	}
	copyActivationProof(&order, result)
	order.PendingEffect = PendingSubscriptionOrderEffectNone
	order.PendingEffectAdmittedAt = nil
	if result.Outcome == SubscriptionActivationRejected {
		order.Status = OrderReconciliationRequired
		order.FailureCode = ""
		if order.SettlementMode == SettlementZeroPrice {
			order.Status = OrderCancelled
			order.FailureCode = orderFailureForActivation(result.FailureCode)
			if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
				return order, ErrReconciliationRequired
			}
			return order, subscriptionActivationError(result.FailureCode)
		}
		if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
			return order, ErrReconciliationRequired
		}
		released, err := s.wallet.ReleaseCommercialPurchase(ctx, money.ReleaseWalletReservationInput{OperationID: "release:" + order.OrderID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, ReservationID: order.WalletReservationID, Reason: "subscription_activation_rejected"})
		if err != nil {
			released, err = s.wallet.ReadCommercialPurchaseReservation(ctx, order.OrganizationID, order.OrderID, order.WalletReservationID)
		}
		if err != nil || released.State != money.WalletReservationReleased || !matchesReservationIdentity(order, released) {
			return order, ErrReconciliationRequired
		}
		order.WalletReservationID = ""
		order.WalletReservationState = ""
		order.Status = OrderCancelled
		order.FailureCode = orderFailureForActivation(result.FailureCode)
		if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
			return order, ErrReconciliationRequired
		}
		return order, subscriptionActivationError(result.FailureCode)
	}
	order.Status = OrderFulfilling
	order.FailureCode = ""
	if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
		return order, ErrReconciliationRequired
	}
	if order.SettlementMode == SettlementWallet {
		committed, err := s.wallet.CommitCommercialPurchase(ctx, money.CommitWalletReservationInput{OperationID: "commit:" + order.OrderID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, ReservationID: order.WalletReservationID})
		if err != nil {
			committed, err = s.wallet.ReadCommercialPurchaseReservation(ctx, order.OrganizationID, order.OrderID, order.WalletReservationID)
		}
		if err != nil || committed.State != money.WalletReservationCommitted || !matchesReservationIdentity(order, committed) {
			return s.markSubscriptionReconciliation(ctx, order)
		}
		order.WalletReservationState = committed.State
	}
	order.Status = OrderFulfilled
	if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
		return order, ErrReconciliationRequired
	}
	return order, nil
}

func (s *Service) admitSubscriptionEffect(ctx context.Context, order Order, effect PendingSubscriptionOrderEffect) (Order, error) {
	authorization, err := s.subscriptionAuthorizer.ReauthorizeCommercialPurchase(ctx, order.OrganizationID, order.ActorID)
	if err != nil || authorization.OrganizationID != order.OrganizationID || authorization.ActorID != order.ActorID || authorization.ObservedAt.IsZero() {
		return s.markSubscriptionReconciliation(ctx, order)
	}
	if !authorization.Allowed {
		return s.beginRevokedSubscriptionCancellation(ctx, order)
	}
	admitted, won, err := s.subscriptionOrders.AdmitSubscriptionOrderEffect(ctx, order.OrganizationID, order.OrderID, order.Version, effect, s.now().UTC())
	if err != nil {
		return order, ErrReconciliationRequired
	}
	if !won {
		fresh, readErr := s.reader.ReadOrder(ctx, order.OrganizationID, order.OrderID)
		if readErr != nil {
			return order, ErrReconciliationRequired
		}
		if fresh.Status == OrderCancelled {
			return fresh, subscriptionCancellationError(fresh.FailureCode)
		}
		if fresh.Status == OrderFulfilled {
			return fresh, nil
		}
		// Another writer changed the admission state. Its effect or terminal
		// intent must be recovered from the fresh durable order, not advanced
		// through this caller's stale phase.
		return fresh, ErrReconciliationRequired
	}
	return admitted, nil
}

func (s *Service) beginRevokedSubscriptionCancellation(ctx context.Context, order Order) (Order, error) {
	if order.PendingEffect != PendingSubscriptionOrderEffectNone || order.ActivationOutcome == SubscriptionActivationActivated {
		return s.markSubscriptionReconciliation(ctx, order)
	}
	now := s.now().UTC()
	order.TerminalIntent = SubscriptionOrderTerminalIntentCancel
	order.TerminalIntentReason = OrderFailureAuthorizationRevoked
	order.TerminalIntentAt = &now
	order.Status = OrderReconciliationRequired
	if order.WalletReservationID == "" {
		order.Status = OrderCancelled
		order.FailureCode = OrderFailureAuthorizationRevoked
	}
	if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
		return order, ErrReconciliationRequired
	}
	return s.finishRevokedSubscriptionOrder(ctx, order)
}

func (s *Service) finishRevokedSubscriptionOrder(ctx context.Context, order Order) (Order, error) {
	if order.WalletReservationID != "" {
		released, err := s.wallet.ReleaseCommercialPurchase(ctx, money.ReleaseWalletReservationInput{OperationID: "release:" + order.OrderID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, ReservationID: order.WalletReservationID, Reason: "authorization_revoked"})
		if err != nil {
			released, err = s.wallet.ReadCommercialPurchaseReservation(ctx, order.OrganizationID, order.OrderID, order.WalletReservationID)
		}
		if err != nil || released.State != money.WalletReservationReleased || !matchesReservationIdentity(order, released) {
			return order, ErrReconciliationRequired
		}
		order.WalletReservationID = ""
		order.WalletReservationState = ""
		order.Status = OrderCancelled
		order.FailureCode = OrderFailureAuthorizationRevoked
		if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
			return order, ErrReconciliationRequired
		}
	}
	return order, ErrAuthorizationRevoked
}

func (s *Service) markSubscriptionReconciliation(ctx context.Context, order Order) (Order, error) {
	order.Status = OrderReconciliationRequired
	order.FailureCode = ""
	if err := s.persistSubscriptionOrder(ctx, &order); err != nil {
		return order, ErrReconciliationRequired
	}
	return order, ErrReconciliationRequired
}

func (s *Service) persistSubscriptionOrder(ctx context.Context, order *Order) error {
	order.UpdatedAt = s.now().UTC()
	if err := s.subscriptionOrders.PersistSubscriptionOrder(ctx, *order); err != nil {
		return err
	}
	order.Version++
	return nil
}

func (s *Service) subscriptionPurchasesEnabled() bool {
	return s != nil && s.subscriptionOffers != nil && s.subscriptionQuotes != nil && s.subscriptionOrders != nil && s.subscriptions != nil && s.subscriptionAuthorizer != nil && s.reader != nil && s.wallet != nil
}

func activationRequestForOrder(order Order) SubscriptionActivationRequest {
	return SubscriptionActivationRequest{OperationID: "subscription-activate:" + order.OrderID, OrganizationID: order.OrganizationID, ActorID: order.ActorID, CommercialOrderID: order.OrderID, PlanCode: order.PlanCode, PlanFingerprint: order.PlanFingerprint, TermMonths: order.TermMonths}
}

func activationRequestFingerprint(request SubscriptionActivationRequest) string {
	encoded, _ := json.Marshal(struct {
		Version         string `json:"version"`
		OperationID     string `json:"operation_id"`
		OrganizationID  string `json:"organization_id"`
		ActorID         string `json:"actor_id"`
		SourceType      string `json:"source_type"`
		SourceID        string `json:"source_id"`
		PlanCode        string `json:"plan_code"`
		PlanFingerprint string `json:"plan_fingerprint"`
		TermMonths      int    `json:"term_months"`
	}{"subscription-activation-v1", request.OperationID, request.OrganizationID, request.ActorID, "commercial_subscription_order", request.CommercialOrderID, request.PlanCode, request.PlanFingerprint, request.TermMonths})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func matchesActivationResult(order Order, result SubscriptionActivationResult) bool {
	request := activationRequestForOrder(order)
	if result.OperationID != request.OperationID || result.OrganizationID != order.OrganizationID || result.CommercialOrderID != order.OrderID || result.PlanCode != order.PlanCode || result.PlanFingerprint != order.PlanFingerprint || result.ActivationRequestFingerprint != activationRequestFingerprint(request) || result.DecidedAt.IsZero() {
		return false
	}
	switch result.Outcome {
	case SubscriptionActivationActivated:
		return result.FailureCode == "" && result.SubscriptionID != "" && result.StartsAt != nil && result.ExpiresAt != nil && result.StartsAt.Before(*result.ExpiresAt) && result.EntitlementSetFingerprint != ""
	case SubscriptionActivationRejected:
		return (result.FailureCode == SubscriptionActivationActiveSubscriptionExists || result.FailureCode == SubscriptionActivationPlanChanged) && result.SubscriptionID == "" && result.StartsAt == nil && result.ExpiresAt == nil && result.EntitlementSetFingerprint == ""
	default:
		return false
	}
}

func copyActivationProof(order *Order, result SubscriptionActivationResult) {
	order.ActivationOperationID = result.OperationID
	order.ActivationRequestFingerprint = result.ActivationRequestFingerprint
	order.ActivationOutcome = result.Outcome
	order.ActivationFailureCode = result.FailureCode
	order.ActivationSubscriptionID = result.SubscriptionID
	order.ActivationStartsAt = result.StartsAt
	order.ActivationExpiresAt = result.ExpiresAt
	order.ActivationEntitlementSetFingerprint = result.EntitlementSetFingerprint
	decidedAt := result.DecidedAt.UTC()
	order.ActivationDecidedAt = &decidedAt
}

func activationResultFromOrder(order Order) SubscriptionActivationResult {
	result := SubscriptionActivationResult{OperationID: order.ActivationOperationID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, PlanCode: order.PlanCode, PlanFingerprint: order.PlanFingerprint, ActivationRequestFingerprint: order.ActivationRequestFingerprint, Outcome: order.ActivationOutcome, FailureCode: order.ActivationFailureCode, SubscriptionID: order.ActivationSubscriptionID, StartsAt: order.ActivationStartsAt, ExpiresAt: order.ActivationExpiresAt, EntitlementSetFingerprint: order.ActivationEntitlementSetFingerprint}
	if order.ActivationDecidedAt != nil {
		result.DecidedAt = order.ActivationDecidedAt.UTC()
	}
	return result
}

func matchesReserveDecision(order Order, decision money.WalletReserveDecision) bool {
	return decision.OperationID == order.OrderID && decision.OrganizationID == order.OrganizationID && decision.CommercialOrderID == order.OrderID && decision.Currency == order.Currency && decision.AmountMinor == order.AmountMinor && decision.Validate() == nil
}

func matchesReservation(order Order, reservationID string, reservation money.WalletReservation) bool {
	return reservation.ReservationID == reservationID && matchesReservationIdentity(order, reservation)
}

func matchesReservationIdentity(order Order, reservation money.WalletReservation) bool {
	return reservation.OrganizationID == order.OrganizationID && reservation.CommercialOrderID == order.OrderID && reservation.OperationID == order.OrderID && reservation.Currency == order.Currency && reservation.AmountMinor == order.AmountMinor
}

func orderFailureForActivation(code SubscriptionActivationFailureCode) OrderFailureCode {
	if code == SubscriptionActivationPlanChanged {
		return OrderFailurePlanChanged
	}
	return OrderFailureActiveSubscriptionExists
}

func subscriptionActivationError(code SubscriptionActivationFailureCode) error {
	if code == SubscriptionActivationPlanChanged {
		return ErrPlanChanged
	}
	return ErrActiveSubscriptionExists
}

func subscriptionCancellationError(code OrderFailureCode) error {
	switch code {
	case OrderFailureInsufficientFunds:
		return ErrInsufficientFunds
	case OrderFailureActiveSubscriptionExists:
		return ErrActiveSubscriptionExists
	case OrderFailurePlanChanged:
		return ErrPlanChanged
	case OrderFailureAuthorizationRevoked:
		return ErrAuthorizationRevoked
	default:
		return ErrOrderCancelled
	}
}
