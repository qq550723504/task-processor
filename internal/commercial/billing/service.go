package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"task-processor/internal/ledger/money"
	"task-processor/internal/ledger/orgresource"
)

// OrderStore owns only commercial order meaning. It does not implement money
// or resource balance changes; those are called through the two owner ports.
type OrderStore interface {
	CreatePendingResourceOrder(context.Context, CreateResourceOrderRequest, Quote) (Order, error)
	UpdateOrder(context.Context, Order) error
}

type Service struct {
	offers OfferCatalog
	quotes QuoteEngine
	orders OrderStore
	reader OrderReader
	wallet WalletPort
	grants PurchasedResourceGrantPort
	now    func() time.Time
}

func NewService(offers OfferCatalog, quotes QuoteEngine, orders OrderStore, reader OrderReader, wallet WalletPort, grants PurchasedResourceGrantPort) (*Service, error) {
	if offers == nil || quotes == nil || orders == nil || reader == nil || wallet == nil {
		return nil, ErrFeatureUnavailable
	}
	return &Service{offers: offers, quotes: quotes, orders: orders, reader: reader, wallet: wallet, grants: grants, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *Service) CreateQuote(ctx context.Context, request QuoteRequest) (Quote, error) {
	if s == nil || s.quotes == nil {
		return Quote{}, ErrFeatureUnavailable
	}
	return s.quotes.CreateQuote(ctx, request)
}

func (s *Service) ReadWallet(ctx context.Context, organizationID string) (money.OrganizationWalletSnapshot, error) {
	if s == nil || s.wallet == nil {
		return money.OrganizationWalletSnapshot{}, ErrFeatureUnavailable
	}
	return s.wallet.ReadOrganizationWallet(ctx, organizationID, CurrencyCNY)
}

func (s *Service) ListWalletEntries(ctx context.Context, organizationID, cursor string, limit int) (money.WalletEntryPage, error) {
	if s == nil || s.wallet == nil {
		return money.WalletEntryPage{}, ErrFeatureUnavailable
	}
	return s.wallet.ListOrganizationWalletEntries(ctx, organizationID, CurrencyCNY, cursor, limit)
}

func (s *Service) CreateResourceOrder(ctx context.Context, request CreateResourceOrderRequest) (Order, error) {
	if s == nil || s.orders == nil || s.quotes == nil || s.wallet == nil || s.grants == nil {
		return Order{}, ErrFeatureUnavailable
	}
	request.OrganizationID = strings.TrimSpace(request.OrganizationID)
	request.QuoteID = strings.TrimSpace(request.QuoteID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.OrganizationID == "" || request.QuoteID == "" || request.IdempotencyKey == "" {
		return Order{}, ErrInvalid
	}
	quote, err := s.quotes.ReadQuote(ctx, request.OrganizationID, request.QuoteID)
	if err != nil {
		return Order{}, err
	}
	order, err := s.orders.CreatePendingResourceOrder(ctx, request, quote)
	if err != nil {
		return Order{}, err
	}
	if order.Status != OrderPending {
		return order, nil
	}
	reservation, err := s.wallet.ReserveCommercialPurchase(ctx, money.ReserveWalletFundsInput{OperationID: order.OrderID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, Currency: order.Currency, AmountMinor: order.AmountMinor})
	if err != nil {
		if errors.Is(err, money.ErrWalletInsufficientBalance) {
			order.Status = OrderCancelled
			order.UpdatedAt = s.now().UTC()
			_ = s.orders.UpdateOrder(ctx, order)
			return order, ErrInsufficientFunds
		}
		order.Status = OrderReconciliationRequired
		order.UpdatedAt = s.now().UTC()
		_ = s.orders.UpdateOrder(ctx, order)
		return order, ErrReconciliationRequired
	}
	order.WalletReservationID = reservation.ReservationID
	order.Status = OrderFundsReserved
	order.UpdatedAt = s.now().UTC()
	if err := s.orders.UpdateOrder(ctx, order); err != nil {
		return order, ErrReconciliationRequired
	}
	item := order.Items[0]
	grant, err := s.grants.GrantPurchasedResource(ctx, orgresource.PurchasedResourceGrantInput{OrganizationID: order.OrganizationID, OperationID: "grant:" + order.OrderID, CommercialOrderID: order.OrderID, CommercialOrderItemID: item.OrderItemID, ResourceType: item.ResourceType, Quantity: item.ResourceQuantity, Principal: orgresource.Principal{ID: "commercial-billing", Kind: orgresource.PrincipalTrustedCommercial}})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, orgresource.ErrConcurrencyRetry) {
			order.Status = OrderReconciliationRequired
			order.UpdatedAt = s.now().UTC()
			_ = s.orders.UpdateOrder(ctx, order)
			return order, ErrReconciliationRequired
		}
		if _, releaseErr := s.wallet.ReleaseCommercialPurchase(ctx, money.ReleaseWalletReservationInput{OperationID: "release:" + order.OrderID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, ReservationID: reservation.ReservationID, Reason: "resource_grant_failed"}); releaseErr != nil {
			order.Status = OrderReconciliationRequired
			order.UpdatedAt = s.now().UTC()
			_ = s.orders.UpdateOrder(ctx, order)
			return order, ErrReconciliationRequired
		}
		order.Status = OrderCancelled
		order.UpdatedAt = s.now().UTC()
		_ = s.orders.UpdateOrder(ctx, order)
		return order, err
	}
	if grant.Snapshot.OperationID == "" {
		order.Status = OrderReconciliationRequired
		order.UpdatedAt = s.now().UTC()
		_ = s.orders.UpdateOrder(ctx, order)
		return order, ErrReconciliationRequired
	}
	order.Status = OrderFulfilling
	order.UpdatedAt = s.now().UTC()
	if err := s.orders.UpdateOrder(ctx, order); err != nil {
		return order, ErrReconciliationRequired
	}
	if _, err := s.wallet.CommitCommercialPurchase(ctx, money.CommitWalletReservationInput{OperationID: "commit:" + order.OrderID, OrganizationID: order.OrganizationID, CommercialOrderID: order.OrderID, ReservationID: reservation.ReservationID}); err != nil {
		order.Status = OrderReconciliationRequired
		order.UpdatedAt = s.now().UTC()
		_ = s.orders.UpdateOrder(ctx, order)
		return order, ErrReconciliationRequired
	}
	order.Status = OrderFulfilled
	order.UpdatedAt = s.now().UTC()
	if err := s.orders.UpdateOrder(ctx, order); err != nil {
		return order, ErrReconciliationRequired
	}
	return order, nil
}

func (s *Service) CreateWalletTopUpOrder(context.Context, CreateWalletTopUpOrderRequest) (Order, error) {
	return Order{}, ErrFeatureUnavailable
}

func (s *Service) ReadOrder(ctx context.Context, organizationID, orderID string) (Order, error) {
	if s == nil || s.reader == nil {
		return Order{}, ErrFeatureUnavailable
	}
	return s.reader.ReadOrder(ctx, organizationID, orderID)
}

func (s *Service) ListOrders(ctx context.Context, organizationID string, filter OrderFilter) (OrderPage, error) {
	if s == nil || s.reader == nil {
		return OrderPage{}, ErrFeatureUnavailable
	}
	return s.reader.ListOrders(ctx, organizationID, filter)
}

func (s *Service) ReadOrderSummary(ctx context.Context, organizationID string, from, until time.Time) (OrderSummary, error) {
	if s == nil || s.reader == nil {
		return OrderSummary{}, ErrFeatureUnavailable
	}
	return s.reader.ReadOrderSummary(ctx, organizationID, from, until)
}
