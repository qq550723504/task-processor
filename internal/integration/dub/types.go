package dub

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"unicode/utf8"
)

const DefaultBaseURL = "https://api.dub.co"

var (
	ErrInvalidConfig = errors.New("dub integration configuration is invalid")
	ErrInvalidInput  = errors.New("dub integration input is invalid")
)

// PartnerInput is the local partner identity projected into Dub. ExternalID is
// the stable partner/user ID owned by Shuomi and is sent as Dub tenantId.
type PartnerInput struct {
	ExternalID string
	Email      string
	Name       string
	Username   string
	Country    string
}

// Partner is the subset of Dub's partner representation needed by Shuomi.
type Partner struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Country  string `json:"country"`
	GroupID  string `json:"groupId"`
	BannedAt string `json:"bannedAt"`
}

// PartnerLinkInput creates a referral link for an existing Dub partner.
// ExternalPartnerID is the stable local partner ID; Dub resolves it through
// tenantId so callers do not need to persist Dub's partner ID.
type PartnerLinkInput struct {
	ExternalPartnerID string
	DestinationURL    string
	Key               string
	ExternalLinkID    string
	Comments          string
}

// Link is the subset of Dub's short-link representation needed by Shuomi.
type Link struct {
	ID              string `json:"id"`
	Domain          string `json:"domain"`
	Key             string `json:"key"`
	URL             string `json:"url"`
	ShortLink       string `json:"shortLink"`
	ExternalID      string `json:"externalId"`
	TenantID        string `json:"tenantId"`
	ProgramID       string `json:"programId"`
	PartnerID       string `json:"partnerId"`
	TrackConversion bool   `json:"trackConversion"`
}

// LeadInput records a conversion from a Dub click to a stable Shuomi customer.
// ClickID may be empty only for Dub's deferred lead-tracking path, where Dub
// resolves an existing customer by CustomerExternalID.
type LeadInput struct {
	ClickID            string
	EventName          string
	CustomerExternalID string
	CustomerName       string
	CustomerEmail      string
	Metadata           map[string]any
}

// LeadResult is intentionally small. A duplicate lead may return a nil result
// with no error because Dub deduplicates customer + event name.
type LeadResult struct {
	Click struct {
		ID string `json:"id"`
	} `json:"click"`
	Link struct {
		ID         string `json:"id"`
		PartnerID  string `json:"partnerId"`
		ProgramID  string `json:"programId"`
		TenantID   string `json:"tenantId"`
		ExternalID string `json:"externalId"`
		ShortLink  string `json:"shortLink"`
	} `json:"link"`
	Customer struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		ExternalID string `json:"externalId"`
	} `json:"customer"`
}

// SaleInput records one attributable paid event. Amount uses Dub's minor-unit
// convention: cents for two-decimal currencies, full integer units for
// zero-decimal currencies. InvoiceID is required by this adapter because Dub
// documents invoiceId as its sale idempotency key.
type SaleInput struct {
	CustomerExternalID string
	Amount              int64
	Currency            string
	EventName           string
	PaymentProcessor    string
	InvoiceID           string
	LeadEventName       string
	ClickID             string
	CustomerName        string
	CustomerEmail       string
	Metadata            map[string]any
}

// SaleCustomer is nullable in Dub's sale response schema.
type SaleCustomer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	ExternalID string `json:"externalId"`
}

// SaleRecord is nullable in Dub's sale response schema.
type SaleRecord struct {
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	PaymentProcessor string         `json:"paymentProcessor"`
	InvoiceID        string         `json:"invoiceId"`
	Metadata         map[string]any `json:"metadata"`
}

// SaleResult is the subset of Dub's sale response used for reconciliation.
// Customer and Sale are pointers because Dub's documented response permits
// either field to be null. A root-level JSON null is represented by a nil
// *SaleResult from TrackSale.
type SaleResult struct {
	EventName string        `json:"eventName"`
	Customer  *SaleCustomer `json:"customer"`
	Sale      *SaleRecord   `json:"sale"`
}

func normalizePartnerInput(input PartnerInput) PartnerInput {
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.Email = strings.TrimSpace(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	input.Username = strings.TrimSpace(input.Username)
	input.Country = strings.ToUpper(strings.TrimSpace(input.Country))
	return input
}

func validatePartnerInput(input PartnerInput) error {
	if input.ExternalID == "" || charCount(input.ExternalID) > 255 {
		return fmt.Errorf("%w: external partner id is required and must be <= 255 characters", ErrInvalidInput)
	}
	if charCount(input.Name) > 100 || charCount(input.Username) > 100 {
		return fmt.Errorf("%w: partner name and username must be <= 100 characters", ErrInvalidInput)
	}
	if charCount(input.Email) > 190 {
		return fmt.Errorf("%w: partner email is too long", ErrInvalidInput)
	}
	if !validEmail(input.Email) {
		return fmt.Errorf("%w: partner email is invalid", ErrInvalidInput)
	}
	if input.Country != "" && charCount(input.Country) != 2 {
		return fmt.Errorf("%w: partner country must be an ISO 3166-1 alpha-2 code", ErrInvalidInput)
	}
	return nil
}

func normalizePartnerLinkInput(input PartnerLinkInput) PartnerLinkInput {
	input.ExternalPartnerID = strings.TrimSpace(input.ExternalPartnerID)
	input.DestinationURL = strings.TrimSpace(input.DestinationURL)
	input.Key = strings.TrimSpace(input.Key)
	input.ExternalLinkID = strings.TrimSpace(input.ExternalLinkID)
	input.Comments = strings.TrimSpace(input.Comments)
	return input
}

func validatePartnerLinkInput(input PartnerLinkInput) error {
	if input.ExternalPartnerID == "" || charCount(input.ExternalPartnerID) > 255 {
		return fmt.Errorf("%w: external partner id is required and must be <= 255 characters", ErrInvalidInput)
	}
	if input.ExternalLinkID == "" || charCount(input.ExternalLinkID) > 255 {
		return fmt.Errorf("%w: external link id is required and must be <= 255 characters", ErrInvalidInput)
	}
	if charCount(input.Key) > 190 {
		return fmt.Errorf("%w: link key must be <= 190 characters", ErrInvalidInput)
	}
	if charCount(input.DestinationURL) > 32000 {
		return fmt.Errorf("%w: destination url must be <= 32000 characters", ErrInvalidInput)
	}
	if input.DestinationURL != "" {
		parsed, err := url.Parse(input.DestinationURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("%w: destination url is invalid", ErrInvalidInput)
		}
	}
	return nil
}

func normalizeLeadInput(input LeadInput) LeadInput {
	input.ClickID = strings.TrimSpace(input.ClickID)
	input.EventName = strings.TrimSpace(input.EventName)
	input.CustomerExternalID = strings.TrimSpace(input.CustomerExternalID)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.CustomerEmail = strings.TrimSpace(input.CustomerEmail)
	return input
}

func validateLeadInput(input LeadInput) error {
	if input.EventName == "" || charCount(input.EventName) > 255 {
		return fmt.Errorf("%w: lead event name is required and must be <= 255 characters", ErrInvalidInput)
	}
	if input.CustomerExternalID == "" || charCount(input.CustomerExternalID) > 100 {
		return fmt.Errorf("%w: customer external id is required and must be <= 100 characters", ErrInvalidInput)
	}
	if input.CustomerEmail != "" && !validEmail(input.CustomerEmail) {
		return fmt.Errorf("%w: customer email is invalid", ErrInvalidInput)
	}
	return nil
}

func normalizeSaleInput(input SaleInput) SaleInput {
	input.CustomerExternalID = strings.TrimSpace(input.CustomerExternalID)
	input.Currency = strings.ToLower(strings.TrimSpace(input.Currency))
	input.EventName = strings.TrimSpace(input.EventName)
	input.PaymentProcessor = strings.ToLower(strings.TrimSpace(input.PaymentProcessor))
	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	input.LeadEventName = strings.TrimSpace(input.LeadEventName)
	input.ClickID = strings.TrimSpace(input.ClickID)
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.CustomerEmail = strings.TrimSpace(input.CustomerEmail)
	if input.Currency == "" {
		input.Currency = "usd"
	}
	if input.EventName == "" {
		input.EventName = "Invoice paid"
	}
	if input.PaymentProcessor == "" {
		input.PaymentProcessor = "custom"
	}
	return input
}

func validateSaleInput(input SaleInput) error {
	if input.CustomerExternalID == "" || charCount(input.CustomerExternalID) > 100 {
		return fmt.Errorf("%w: customer external id is required and must be <= 100 characters", ErrInvalidInput)
	}
	if input.Amount < 0 {
		return fmt.Errorf("%w: sale amount must be >= 0", ErrInvalidInput)
	}
	if charCount(input.Currency) != 3 {
		return fmt.Errorf("%w: currency must be a three-letter ISO 4217 code", ErrInvalidInput)
	}
	if input.InvoiceID == "" {
		return fmt.Errorf("%w: invoice id is required for idempotent sale tracking", ErrInvalidInput)
	}
	if charCount(input.EventName) > 255 {
		return fmt.Errorf("%w: sale event name must be <= 255 characters", ErrInvalidInput)
	}
	if !allowedPaymentProcessor(input.PaymentProcessor) {
		return fmt.Errorf("%w: unsupported payment processor", ErrInvalidInput)
	}
	if input.CustomerEmail != "" && !validEmail(input.CustomerEmail) {
		return fmt.Errorf("%w: customer email is invalid", ErrInvalidInput)
	}
	return nil
}

func allowedPaymentProcessor(value string) bool {
	switch value {
	case "stripe", "shopify", "polar", "paddle", "apple", "revenuecat", "lemonsqueezy", "dub", "custom":
		return true
	default:
		return false
	}
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(address.Address, value)
}

func charCount(value string) int {
	return utf8.RuneCountInString(value)
}
