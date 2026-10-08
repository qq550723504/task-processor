package billing

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"task-processor/internal/ledger/money"
	"time"
)

func (s *ServicePurchases) original(ctx context.Context, c ServicePurchaseCommand) (ServicePurchaseOrder, error) {
	if c.Validate() != nil {
		return ServicePurchaseOrder{}, ErrInvalid
	}
	original, err := s.source.OriginalServicePurchase(ctx, c.OrderID)
	if err != nil {
		return ServicePurchaseOrder{}, err
	}
	if original.Validate() != nil || original.Kind != "CREATE_PURCHASE" || original.OrderID != c.OrderID || original.RequestID != c.RequestID || original.BuyerOrganizationID != c.BuyerOrganizationID || original.ProviderOrganizationID != c.ProviderOrganizationID || original.ProviderMerchantID != c.ProviderMerchantID || original.PolicyVersion != c.PolicyVersion || original.QuoteVersion != c.QuoteVersion || original.DeliveryDays != c.DeliveryDays || c.Kind != "REFUND" && c.AmountMinor != original.AmountMinor || c.Kind == "REFUND" && c.AmountMinor > original.AmountMinor {
		return ServicePurchaseOrder{}, ErrConflict
	}
	if c.Kind == "CREATE_PURCHASE" && original.Fingerprint() != c.Fingerprint() {
		return ServicePurchaseOrder{}, ErrConflict
	}
	o, err := s.store.ReadServicePurchase(ctx, c.OrderID)
	if errors.Is(err, ErrNotFound) {
		o, err = s.store.CreateServicePurchase(ctx, original, s.provider.Profile())
	}
	if err != nil {
		return o, err
	}
	if o.Source.Fingerprint() != original.Fingerprint() || o.Profile != s.provider.Profile() {
		return o, ErrFeatureUnavailable
	}
	return o, nil
}
func (s *ServicePurchases) save(ctx context.Context, o *ServicePurchaseOrder) error {
	next, err := s.store.SaveServicePurchase(ctx, *o)
	if err == nil {
		*o = next
	}
	return err
}
func serviceResult(o ServicePurchaseOrder) ServicePurchaseResult {
	return ServicePurchaseResult{OrderID: o.Source.OrderID, PaymentReceiptID: o.PaymentReceiptID, State: o.State, Reason: o.Reason, FundsExpireAt: o.FundsExpireAt, Revision: o.Version}
}
func (s *ServicePurchases) complete(ctx context.Context, o *ServicePurchaseOrder, c ServicePurchaseCommand, state, receipt string, full bool) (ServicePurchaseResult, error) {
	o.State = state
	o.Reason = ""
	r := serviceResult(*o)
	r.ReceiptID = receipt
	r.FullRefund = full
	r.Revision = o.Version + 1
	if o.CompletedCommands == nil {
		o.CompletedCommands = map[string]ServicePurchaseResult{}
	}
	o.CompletedCommands[c.ID] = r
	o.ActiveCommand = nil
	o.Operation = nil
	if err := s.save(ctx, o); err != nil {
		return ServicePurchaseResult{}, err
	}
	return r, nil
}
func (s *ServicePurchases) acceptPayment(ctx context.Context, o *ServicePurchaseOrder) error {
	observations, err := s.store.ServicePaymentObservations(ctx, o.Source.OrderID)
	if err != nil {
		return err
	}
	for _, p := range observations {
		if !p.Matches(*o) {
			return ErrConflict
		}
		if p.State != "PAID" && p.State != "PAID_REFUND_UNKNOWN" {
			continue
		}
		if o.Payment != nil && (o.Payment.TransactionID != p.TransactionID || !o.Payment.OccurredAt.Equal(p.OccurredAt)) {
			return ErrConflict
		}
		if o.PaymentReceiptID != "" && p.State != "PAID_REFUND_UNKNOWN" {
			continue
		}
		o.Payment = &p
		in := o.MoneyInput()
		receipt, err := s.funds.ReadServicePayment(ctx, in)
		if errors.Is(err, money.ErrNotFound) {
			receipt, err = s.funds.AcceptServicePayment(ctx, in)
		}
		if err != nil {
			return err
		}
		if receipt.Validate() != nil || receipt.OrderID != o.Source.OrderID || receipt.RequestFingerprint != in.Fingerprint() {
			return ErrConflict
		}
		o.PaymentReceiptID = receipt.ReceiptID
		expiry := p.OccurredAt.AddDate(0, 0, o.Profile.FreezeDays)
		o.FundsExpireAt = &expiry
		if p.State == "PAID_REFUND_UNKNOWN" {
			if _, err := s.funds.ObserveServiceRefundUncertainty(ctx, in, p.EventID); err != nil {
				return err
			}
			o.State = "RECONCILIATION_REQUIRED"
			o.Reason = "CHANNEL_REFUND_REQUIRES_RECONCILIATION"
		} else if !o.CancelRequested && o.ActiveCommand == nil {
			o.State = "PAID"
			o.Reason = ""
		}
		if err := s.save(ctx, o); err != nil {
			return err
		}
	}
	return nil
}
func (s *ServicePurchases) refreshPayment(ctx context.Context, o *ServicePurchaseOrder) error {
	if err := s.acceptPayment(ctx, o); err != nil {
		return err
	}
	if o.PaymentReceiptID != "" {
		return nil
	}
	p, err := s.provider.QueryServicePayment(ctx, *o)
	if err != nil {
		return err
	}
	if err := s.store.RecordServicePaymentObservation(ctx, *o, p); err != nil {
		return err
	}
	return s.acceptPayment(ctx, o)
}
func (s *ServicePurchases) Execute(ctx context.Context, c ServicePurchaseCommand) (ServicePurchaseResult, error) {
	if err := s.source.VerifyServiceCommand(ctx, c); err != nil {
		return ServicePurchaseResult{}, err
	}
	if _, err := s.original(ctx, c); err != nil {
		return ServicePurchaseResult{}, err
	}
	token := uuid.NewString()
	o, err := s.store.ClaimServicePurchase(ctx, c.OrderID, token, s.now().Add(time.Minute))
	if err != nil {
		return ServicePurchaseResult{}, err
	}
	defer s.store.ReleaseServicePurchase(context.WithoutCancel(ctx), c.OrderID, token)
	if err := s.acceptPayment(ctx, &o); err != nil {
		return serviceResult(o), err
	}
	recoverOriginal := o.ActiveCommand != nil && o.ActiveCommand.ID == c.ID && o.Operation != nil && o.Operation.Dispatched
	if o.State == "RECONCILIATION_REQUIRED" && !recoverOriginal {
		return serviceResult(o), nil
	}
	if r, ok := o.CompletedCommands[c.ID]; ok {
		if c.Kind == "CREATE_PURCHASE" {
			if o.Operation != nil && o.Operation.Dispatched {
				return serviceResult(o), nil
			}
			confirmed, e := s.observeExpiredFunds(ctx, &o)
			if e != nil || !confirmed {
				return serviceResult(o), e
			}
		}
		return r, nil
	}
	if c.Kind == "CREATE_PURCHASE" {
		if err := s.refreshPayment(ctx, &o); err != nil {
			return serviceResult(o), err
		}
		if o.CancelRequested {
			return serviceResult(o), nil
		}
		if o.PaymentReceiptID != "" {
			return s.complete(ctx, &o, c, o.State, o.PaymentReceiptID, false)
		}
		return serviceResult(o), nil
	}
	if o.ActiveCommand != nil && o.ActiveCommand.ID != c.ID {
		return serviceResult(o), ErrConflict
	}
	if o.ActiveCommand != nil && o.ActiveCommand.Fingerprint() != c.Fingerprint() {
		return serviceResult(o), ErrConflict
	}
	if c.Kind == "CANCEL" && !o.CancelRequested {
		o.CancelRequested = true
		o.State = "CANCELLATION_PENDING"
		if err := s.save(ctx, &o); err != nil {
			return ServicePurchaseResult{}, err
		}
	}
	if c.Kind == "CANCEL" && !o.PaymentDispatched && o.PaymentReceiptID == "" {
		// No original payment request can have escaped this order's dispatch fence.
		if err := s.source.AdmitServiceCommand(ctx, c, "close:"+c.ID); err != nil {
			return serviceResult(o), err
		}
		return s.complete(ctx, &o, c, "CLOSED_UNPAID", "never-dispatched:"+c.ID, false)
	}
	if err := s.refreshPayment(ctx, &o); err != nil {
		return serviceResult(o), err
	}
	if c.Kind == "CANCEL" && o.PaymentReceiptID == "" {
		if err := s.source.AdmitServiceCommand(ctx, c, "close:"+c.ID); err != nil {
			return serviceResult(o), err
		}
		p, err := s.provider.CloseServicePayment(ctx, o)
		if err != nil {
			return serviceResult(o), err
		}
		if err := s.store.RecordServicePaymentObservation(ctx, o, p); err != nil {
			return serviceResult(o), err
		}
		if err := s.acceptPayment(ctx, &o); err != nil {
			return serviceResult(o), err
		}
		if p.State == "CLOSED" && o.PaymentReceiptID == "" {
			return s.complete(ctx, &o, c, "CLOSED_UNPAID", "channel-closed:"+p.EventID, false)
		}
	}
	if o.PaymentReceiptID == "" {
		return serviceResult(o), ErrConflict
	}
	f, err := s.funds.ReadServiceFunds(ctx, c.OrderID)
	if err != nil {
		return serviceResult(o), err
	}
	if f.ReconciliationReason != "" && !recoverOriginal {
		o.State = "RECONCILIATION_REQUIRED"
		o.Reason = f.ReconciliationReason
		err = s.save(ctx, &o)
		return serviceResult(o), err
	}
	if o.ActiveCommand == nil {
		o.ActiveCommand = &c
		if err := s.save(ctx, &o); err != nil {
			return serviceResult(o), err
		}
	}
	for step := 0; step < 4; step++ {
		if o.Operation != nil {
			if err := s.advanceOperation(ctx, &o, c); err != nil {
				return serviceResult(o), err
			}
			if o.Operation != nil {
				return serviceResult(o), nil
			}
			if result, ok := o.CompletedCommands[c.ID]; ok {
				return result, nil
			}
		}
		f, err := s.funds.ReadServiceFunds(ctx, c.OrderID)
		if err != nil {
			return serviceResult(o), err
		}
		if f.ReconciliationReason != "" {
			o.State = "RECONCILIATION_REQUIRED"
			o.Reason = f.ReconciliationReason
			err = s.save(ctx, &o)
			return serviceResult(o), err
		}
		op, err := nextServiceOperation(o, c, f)
		if err != nil {
			return serviceResult(o), err
		}
		if op == nil {
			state := "REFUNDED"
			kind := money.ServiceRefund
			if c.Kind == "SETTLE" {
				state = "SETTLED"
				kind = money.ServiceFinish
			}
			receipt := o.Effects["service-operation:"+serviceOperationRequestID(o, c, kind)]
			if receipt.ReceiptID == "" && c.Kind == "SETTLE" {
				for _, proof := range o.Effects {
					if proof.Kind == money.ServiceFinish || proof.Kind == money.ServiceRefundRelease {
						receipt = proof
						break
					}
				}
			}
			if receipt.ReceiptID == "" {
				return serviceResult(o), ErrConflict
			}
			return s.complete(ctx, &o, c, state, receipt.ReceiptID, f.RefundedMinor+f.ChargedBackMinor == f.GrossMinor)
		}
		if err := s.source.CanDispatchServiceCommand(ctx, c); err != nil {
			o.State = "SOURCE_DENIED"
			o.Reason = "ORIGINAL_SOURCE_DISPATCH_DENIED"
			o.ActiveCommand = nil
			if saveErr := s.save(ctx, &o); saveErr != nil {
				return serviceResult(o), saveErr
			}
			return serviceResult(o), err
		}
		o.Operation = op
		if o.Operations == nil {
			o.Operations = map[string]ServiceFinancialOperation{}
		}
		o.Operations[op.Reservation.OperationID] = *op
		o.State = "SETTLEMENT_PENDING"
		if c.Kind != "SETTLE" {
			o.State = "REFUND_PENDING"
		}
		if err := s.save(ctx, &o); err != nil {
			return serviceResult(o), err
		}
	}
	return serviceResult(o), nil
}
func (s *ServicePurchases) rememberEffect(ctx context.Context, o *ServicePurchaseOrder, op ServiceFinancialOperation, r money.ServiceReceipt) error {
	if r.Validate() != nil || r.RequestFingerprint != money.ServiceFingerprint(op.Reservation) || r.OrderID != o.Source.OrderID || r.OperationID != op.Reservation.OperationID {
		return ErrConflict
	}
	if o.Effects == nil {
		o.Effects = map[string]money.ServiceReceipt{}
	}
	o.Effects[op.Reservation.OperationID] = r
	if op.Reservation.Kind == money.ServiceShare {
		o.ShareOperationID = op.Reservation.OperationID
		o.ShareProviderRequestID = op.ProviderRequestID
	}
	o.Operation = nil
	o.Reason = ""
	return s.save(ctx, o)
}
func (s *ServicePurchases) advanceOperation(ctx context.Context, o *ServicePurchaseOrder, c ServicePurchaseCommand) error {
	op := *o.Operation
	if proof := o.DeniedOperations[op.Reservation.OperationID]; proof != "" {
		return s.releaseDeniedOperation(ctx, o, c, op, proof)
	}
	if receipt, err := s.funds.ReadServiceEffect(ctx, op.Reservation); err == nil {
		return s.rememberEffect(ctx, o, op, receipt)
	} else if !errors.Is(err, money.ErrNotFound) {
		return err
	}
	// M3 may already have committed a failed terminal receipt while B7 was
	// lost. Resolve its original durable proof before trying to prepare again.
	if op.Dispatched {
		observations, err := s.store.ServiceOperationObservations(ctx, op.Reservation.OperationID)
		if err != nil {
			return err
		}
		for _, p := range observations {
			if p.State == "SUCCESS" {
				return s.acceptOperation(ctx, o, op, p)
			}
			if p.State == "FAILED" {
				return s.acceptFailure(ctx, o, c, op, p)
			}
		}
	}
	if _, err := s.funds.PrepareServiceOperation(ctx, op.Reservation); err != nil {
		return err
	}
	if !op.Dispatched {
		if err := s.source.AdmitServiceCommand(ctx, c, op.Reservation.OperationID); err != nil {
			o.State = "SOURCE_DENIED"
			o.Reason = "ORIGINAL_SOURCE_DISPATCH_DENIED"
			if o.DeniedOperations == nil {
				o.DeniedOperations = map[string]string{}
			}
			proof := "source-denied:" + op.Reservation.OperationID
			o.DeniedOperations[op.Reservation.OperationID] = proof
			if saveErr := s.save(ctx, o); saveErr != nil {
				return saveErr
			}
			if releaseErr := s.releaseDeniedOperation(ctx, o, c, op, proof); releaseErr != nil {
				return releaseErr
			}
			return err
		}
		if op.Reservation.AmountMinor == 0 {
			receipt, err := s.funds.AcceptServiceEffect(ctx, money.ServiceEffect{Operation: op.Reservation, ProviderReference: "zero-economic-effect:" + op.Reservation.OperationID, OccurredAt: o.CreatedAt})
			if err != nil {
				return err
			}
			return s.rememberEffect(ctx, o, op, receipt)
		}
		if op.Reservation.Kind == money.ServiceShare && s.now().Before(o.Payment.OccurredAt.Add(30*time.Second)) {
			return nil
		}
		if serviceReleaseOperation(op) {
			confirmed, err := s.observeExpiredFunds(ctx, o)
			if err != nil || !confirmed {
				return err
			}
		}
		if err := s.funds.AdmitServiceOperation(ctx, op.Reservation); err != nil {
			return err
		}
		op.Dispatched = true
		o.Operation = &op
		o.Operations[op.Reservation.OperationID] = op
		if err := s.save(ctx, o); err != nil {
			return err
		}
		p, err := s.provider.DispatchServiceOperation(ctx, *o, op)
		if err != nil {
			return err
		}
		if err := s.store.RecordServiceOperationObservation(ctx, *o, op, p); err != nil {
			return err
		}
	} else {
		observations, err := s.store.ServiceOperationObservations(ctx, op.Reservation.OperationID)
		if err != nil {
			return err
		}
		for _, p := range observations {
			if p.State == "SUCCESS" {
				return s.acceptOperation(ctx, o, op, p)
			}
			if p.State == "FAILED" {
				return s.acceptFailure(ctx, o, c, op, p)
			}
		}
		p, err := s.provider.QueryServiceOperation(ctx, *o, op)
		if err != nil {
			return err
		}
		if err := s.store.RecordServiceOperationObservation(ctx, *o, op, p); err != nil {
			return err
		}
		if p.State == "REPLAY_ALLOWED" {
			// A proof of absence permits only the original identity. It cannot
			// bypass a newer canonical money uncertainty fence.
			if serviceReleaseOperation(op) {
				confirmed, err := s.observeExpiredFunds(ctx, o)
				if err != nil || !confirmed {
					return err
				}
			}
			if err := s.funds.AdmitServiceOperation(ctx, op.Reservation); err != nil {
				return err
			}
			if err := s.save(ctx, o); err != nil {
				return err
			}
			confirmed, err := s.provider.DispatchServiceOperation(ctx, *o, op)
			if err != nil {
				return err
			}
			if err := s.store.RecordServiceOperationObservation(ctx, *o, op, confirmed); err != nil {
				return err
			}
		}
	}
	observations, err := s.store.ServiceOperationObservations(ctx, op.Reservation.OperationID)
	if err != nil {
		return err
	}
	for _, p := range observations {
		if p.State == "SUCCESS" {
			return s.acceptOperation(ctx, o, op, p)
		}
		if p.State == "FAILED" {
			return s.acceptFailure(ctx, o, c, op, p)
		}
	}
	if len(observations) > 0 {
		p := observations[len(observations)-1]
		o.State = p.State
		o.Reason = p.Reason
		if p.State == "FAILED" {
			o.State = "RECONCILIATION_REQUIRED"
			o.Reason = "ORIGINAL_CHANNEL_OPERATION_FAILED"
		}
		return s.save(ctx, o)
	}
	return ErrReconciliationRequired
}
func (s *ServicePurchases) acceptFailure(ctx context.Context, o *ServicePurchaseOrder, c ServicePurchaseCommand, op ServiceFinancialOperation, p ServiceOperationObservation) error {
	if !p.Matches(*o, op) || p.State != "FAILED" || p.ProviderReference == "" || p.Reason == "" || p.OccurredAt.IsZero() {
		return ErrReconciliationRequired
	}
	receipt, err := s.funds.ResolveFailedServiceOperation(ctx, money.ServiceOperationFailure{Operation: op.Reservation, ProofID: p.EventID, ProviderReference: p.ProviderReference, Reason: p.Reason, OccurredAt: p.OccurredAt})
	if err != nil {
		return err
	}
	if receipt.Validate() != nil || receipt.OrderID != c.OrderID || receipt.RequestFingerprint != money.ServiceFingerprint(op.Reservation) {
		return ErrConflict
	}
	o.State = "CHANNEL_OPERATION_FAILED"
	o.Reason = p.Reason
	o.Operation = nil
	o.ActiveCommand = nil
	result := serviceResult(*o)
	result.ReceiptID = receipt.ReceiptID
	result.Revision = o.Version + 1
	if o.CompletedCommands == nil {
		o.CompletedCommands = map[string]ServicePurchaseResult{}
	}
	o.CompletedCommands[c.ID] = result
	return s.save(ctx, o)
}
func (s *ServicePurchases) acceptOperation(ctx context.Context, o *ServicePurchaseOrder, op ServiceFinancialOperation, p ServiceOperationObservation) error {
	if !p.Matches(*o, op) || p.State != "SUCCESS" {
		return ErrConflict
	}
	receipt, err := s.funds.AcceptServiceEffect(ctx, money.ServiceEffect{Operation: op.Reservation, ProviderReference: p.ProviderReference, OccurredAt: p.OccurredAt})
	if err != nil {
		return err
	}
	return s.rememberEffect(ctx, o, op, receipt)
}
func (s *ServicePurchases) releaseDeniedOperation(ctx context.Context, o *ServicePurchaseOrder, c ServicePurchaseCommand, op ServiceFinancialOperation, proof string) error {
	if err := s.funds.AbandonUndispatchedServiceOperation(ctx, op.Reservation, proof); err != nil {
		return err
	}
	if o.AttemptGenerations == nil {
		o.AttemptGenerations = map[string]int{}
	}
	o.AttemptGenerations[c.ID+":"+string(op.Reservation.Kind)]++
	o.Operation = nil
	o.ActiveCommand = nil
	return s.save(ctx, o)
}
func (s *ServicePurchases) Checkout(ctx context.Context, org, actor, order string) (string, error) {
	if err := s.source.AuthorizeServiceCheckout(ctx, org, actor, order); err != nil {
		return "", err
	}
	original, err := s.source.OriginalServicePurchase(ctx, order)
	if err != nil {
		return "", err
	}
	if original.BuyerOrganizationID != org {
		return "", ErrNotFound
	}
	if _, err := s.original(ctx, original); err != nil {
		return "", err
	}
	token := uuid.NewString()
	o, err := s.store.ClaimServicePurchase(ctx, order, token, s.now().Add(time.Minute))
	if err != nil {
		return "", err
	}
	defer s.store.ReleaseServicePurchase(context.WithoutCancel(ctx), order, token)
	if o.CancelRequested || o.PaymentReceiptID != "" || !s.provider.NewPaymentsEnabled() || !s.now().Before(o.ExpiresAt) {
		return "", ErrOrderCancelled
	}
	if err := s.source.AuthorizeServiceCheckout(ctx, org, actor, order); err != nil {
		return "", err
	}
	if err := s.source.AdmitServiceCommand(ctx, original, "checkout:"+original.ID); err != nil {
		return "", err
	}
	if len(o.CheckoutCiphertext) > 0 {
		return s.protection.Open(serviceCheckoutBinding(o), o.CheckoutCiphertext)
	}
	if o.PaymentDispatched {
		return "", ErrReconciliationRequired
	}
	o.PaymentDispatched = true
	if err := s.save(ctx, &o); err != nil {
		return "", err
	}
	qr, err := s.provider.CreateServiceCheckout(ctx, o)
	if err != nil {
		return "", err
	}
	if !validServiceQR(qr) {
		return "", ErrReconciliationRequired
	}
	ciphertext, err := s.protection.Seal(serviceCheckoutBinding(o), qr)
	if err != nil {
		return "", err
	}
	o.CheckoutCiphertext = ciphertext
	if err := s.save(ctx, &o); err != nil {
		return "", err
	}
	if err := s.source.AuthorizeServiceCheckout(ctx, org, actor, order); err != nil {
		return "", err
	}
	// A cancellation arriving during the channel request fences disclosure too.
	if err := s.source.AdmitServiceCommand(ctx, original, "checkout:"+original.ID); err != nil {
		return "", err
	}
	return qr, nil
}
