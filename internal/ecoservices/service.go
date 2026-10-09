package ecoservices

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

func Fingerprint(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append([]byte("ecoservices:v1:"), data...))
	return hex.EncodeToString(sum[:])
}
func (s *Service) Mutate(ctx context.Context, c Command) (Result, error) {
	// Internal owner evidence is never accepted from a caller.
	c.RefundReviewProof = nil
	c.RefundableAmount = nil
	if !ValidID(c.Key) || !validText(c.Scope.ActorID, 256) || !c.Scope.Platform && !validText(c.Scope.OrganizationID, 128) {
		return Result{}, ErrInvalid
	}
	if c.Kind != "application_submit" && c.Kind != "listing_create" && c.Kind != "request_create" && (!ValidID(c.ID) || c.Version < 1) {
		return Result{}, ErrInvalid
	}
	switch c.Kind {
	case "application_submit":
		if c.Scope.Platform || c.Application == nil || !validText(c.Application.CompanyName, 256) || !validText(c.Application.RegistrationNumber, 128) || len(c.Application.Categories) < 1 || len(c.Application.Categories) > 4 || len(c.Application.Regions) < 1 || len(c.Application.Regions) > 30 || len(c.Application.FileIDs) < 1 || len(c.Application.FileIDs) > 10 {
			return Result{}, ErrInvalid
		}
		seen := map[Category]bool{}
		for _, category := range c.Application.Categories {
			if !validCategory(category) || seen[category] {
				return Result{}, ErrInvalid
			}
			seen[category] = true
		}
		for _, v := range c.Application.Regions {
			if !validText(v, 128) {
				return Result{}, ErrInvalid
			}
		}
	case "application_review", "application_reject":
		if !c.Scope.Platform || !validText(c.Reason, 2000) {
			return Result{}, ErrForbidden
		}
	case "agreement_accept":
		if c.Scope.Platform || c.AgreementVersion != PolicyVersion {
			return Result{}, ErrInvalid
		}
	case "listing_create", "listing_update", "listing_publish":
		if c.Scope.Platform {
			return Result{}, ErrForbidden
		}
		if c.Kind != "listing_publish" {
			if err := ValidateListing(c.Listing, s.freezeDays); err != nil {
				return Result{}, err
			}
		}
	case "request_create":
		if c.Scope.Platform || !ValidID(c.ID) || !validText(c.Description, 10000) || len(c.FileIDs) > 10 {
			return Result{}, ErrInvalid
		}
	case "quote":
		if c.Quote == nil || c.Quote.DeliveryDays > s.freezeDays {
			return Result{}, ErrInvalid
		}
	case "confirm_quote", "start", "deliver", "accept", "reject", "cancel", "refund_propose", "refund_confirm", "refund_review", "refund_review_reject":
	default:
		return Result{}, ErrInvalid
	}
	allFiles := append([]string(nil), c.FileIDs...)
	if c.Application != nil {
		allFiles = append(allFiles, c.Application.FileIDs...)
	}
	if c.Delivery != nil {
		allFiles = append(allFiles, c.Delivery.FileIDs...)
	}
	if len(allFiles) > 10 {
		return Result{}, ErrInvalid
	}
	seenFiles := map[string]bool{}
	for _, id := range allFiles {
		if !ValidID(id) || seenFiles[id] {
			return Result{}, ErrInvalid
		}
		seenFiles[id] = true
	}
	c.Fingerprint = ""
	c.Fingerprint = Fingerprint(c)
	if c.Kind == "start" || c.Kind == "deliver" || c.Kind == "accept" || c.Kind == "reject" {
		// Replays keep their immutable result. New fulfillment must read the
		// current funds through the original purchase, outside the E lock.
		if result, found, err := s.repo.ReadMutationResult(ctx, c); err != nil || found {
			return result, err
		}
		page, err := s.repo.Read(ctx, Query{Scope: c.Scope, Kind: "requests", ID: c.ID, Page: 1, PageSize: 1})
		if err != nil {
			return Result{}, err
		}
		if len(page.Requests) != 1 {
			return Result{}, ErrNotFound
		}
		if s.trading == nil {
			return Result{}, ErrUnavailable
		}
		request := page.Requests[0]
		original, err := s.repo.OriginalFinancialCommand(ctx, request.OrderID)
		if err != nil {
			return Result{}, err
		}
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := s.trading.ExecuteServiceCommand(callCtx, original)
		cancel()
		if err != nil || result.OrderID != request.OrderID || result.PaymentReceiptID == "" || result.PaymentReceiptID != request.PaymentReceiptID {
			return Result{}, ErrUnavailable
		}
		if result.State == "RECONCILIATION_REQUIRED" {
			if err := s.repo.CompleteFinancialCommand(ctx, original, result); err != nil {
				return Result{}, err
			}
			return Result{}, ErrConflict
		}
	}
	if c.Kind == "refund_propose" || c.Kind == "refund_review" {
		// A replay returns the immutable original result before querying money.
		if result, found, err := s.repo.ReadMutationResult(ctx, c); err != nil || found {
			return result, err
		}
		page, err := s.repo.Read(ctx, Query{Scope: c.Scope, Kind: "requests", ID: c.ID, Page: 1, PageSize: 1})
		if err != nil {
			return Result{}, err
		}
		if len(page.Requests) != 1 {
			return Result{}, ErrNotFound
		}
		if s.trading == nil {
			return Result{}, ErrUnavailable
		}
		if c.Kind == "refund_review" {
			in, err := BuildRefundReviewAdmission(c, page.Requests[0])
			if err != nil {
				return Result{}, err
			}
			trading, ok := s.trading.(RefundReviewTradingPort)
			if !ok {
				return Result{}, ErrUnavailable
			}
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			proof, err := trading.AdmitServiceRefundReview(callCtx, in)
			cancel()
			if err != nil {
				return Result{}, err
			}
			if !proof.Matches(in) {
				return Result{}, ErrConflict
			}
			c.RefundReviewProof = &proof
			return s.repo.Apply(ctx, c, s.freezeDays)
		}
		remaining, err := s.trading.ReadServiceRefundableAmount(ctx, page.Requests[0].OrderID)
		if err != nil {
			return Result{}, ErrUnavailable
		}
		c.RefundableAmount = &remaining
	}
	return s.repo.Apply(ctx, c, s.freezeDays)
}
func ValidateListing(l *Listing, freezeDays int) error {
	if l == nil || !validCategory(l.Category) || !validText(l.Title, 128) || !validText(l.Description, 2000) || l.PriceMinor <= 0 || l.DeliveryDays < 1 || l.DeliveryDays > freezeDays || len(l.Items) < 1 || len(l.Items) > 10 || len(l.Regions) < 1 || len(l.Regions) > 30 || len(l.Platforms) > 20 {
		return ErrInvalid
	}
	for _, set := range [][]string{l.Items, l.Regions, l.Platforms} {
		for _, v := range set {
			if !validText(v, 512) {
				return ErrInvalid
			}
		}
	}
	return nil
}
func (s *Service) Read(ctx context.Context, q Query) (Page, error) {
	if q.Group != "" && (q.Group != "enterprise" && q.Group != "shop" || q.Kind != "catalog" && q.Kind != "provider_listings") {
		return Page{}, ErrInvalid
	}
	if q.Stage != "" && (q.Kind != "requests" || q.Stage != "pending" && q.Stage != "servicing" && q.Stage != "acceptance" && q.Stage != "completed" && q.Stage != "cancelled") {
		return Page{}, ErrInvalid
	}
	if !validText(q.Scope.ActorID, 256) || !q.Scope.Platform && !validText(q.Scope.OrganizationID, 128) || len(q.Search) > 200 || q.Page < 1 || q.PageSize < 1 || q.PageSize > 100 || q.Page > 100000 || q.Category != "" && !validCategory(q.Category) || q.ID != "" && !ValidID(q.ID) || q.Side != "" && q.Side != "buyer" && q.Side != "provider" {
		return Page{}, ErrInvalid
	}
	switch q.Kind {
	case "catalog", "applications", "provider_listings", "requests", "due_orders":
	default:
		return Page{}, ErrInvalid
	}
	if q.Kind == "due_orders" && !q.Scope.Platform {
		return Page{}, ErrForbidden
	}
	return s.repo.Read(ctx, q)
}
func (s *Service) Recover(ctx context.Context) error {
	if s.trading == nil {
		return ErrUnavailable
	}
	commands, err := s.repo.PendingFinancialCommands(ctx, 20)
	if err != nil {
		return err
	}
	for _, command := range commands {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := s.trading.ExecuteServiceCommand(callCtx, command)
		cancel()
		if err != nil {
			continue
		}
		if err := s.repo.CompleteFinancialCommand(ctx, command, result); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) AdmitFinancialCommand(ctx context.Context, c FinancialCommand) (FinancialCommand, error) {
	return s.repo.AdmitFinancialCommand(ctx, c)
}
