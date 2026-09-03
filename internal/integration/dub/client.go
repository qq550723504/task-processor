package dub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.dub.co"

const maxResponseBytes = 1 << 20

var (
	ErrInvalidConfig = errors.New("dub integration configuration is invalid")
	ErrInvalidInput  = errors.New("dub integration input is invalid")
)

// Config configures the server-to-server Dub REST adapter.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// Client is the narrow Dub boundary used for partner attribution. It uses the
// REST API directly so the application does not depend on Dub's generated Go SDK.
type Client struct {
	baseURL *url.URL
	apiKey  string
	http    *http.Client
}

// APIError is a redacted Dub API failure. It intentionally excludes request
// headers and request bodies so credentials and customer data are not copied
// into ordinary error messages.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return "dub api request failed"
	}
	if e.Code != "" {
		return fmt.Sprintf("dub api request failed: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("dub api request failed: status=%d message=%s", e.StatusCode, e.Message)
}

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
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Country   string `json:"country"`
	GroupID   string `json:"groupId"`
	BannedAt  string `json:"bannedAt"`
	ProgramID string `json:"programId"`
}

// PartnerLinkInput creates a conversion-tracked link for an existing Dub
// partner. Exactly one stable local ExternalPartnerID is required; Dub resolves
// it through tenantId so callers do not need to persist Dub partner IDs.
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

// SaleInput records one attributable paid event. InvoiceID is required by this
// adapter even though Dub marks it optional because Dub documents invoiceId as
// the sale idempotency key. Requiring it prevents retry-driven double counting.
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

// SaleResult is the subset of Dub's sale response used for reconciliation.
type SaleResult struct {
	EventName string `json:"eventName"`
	Customer  struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		ExternalID string `json:"externalId"`
	} `json:"customer"`
	Sale struct {
		Amount           int64          `json:"amount"`
		Currency         string         `json:"currency"`
		PaymentProcessor string         `json:"paymentProcessor"`
		InvoiceID        string         `json:"invoiceId"`
		Metadata         map[string]any `json:"metadata"`
	} `json:"sale"`
}

// NewClient creates a Dub REST adapter. The API key is required and must remain
// server-side. A caller-owned HTTP client may be supplied for transport policy,
// tracing, tests, proxies, or mTLS.
func NewClient(cfg Config) (*Client, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%w: api key is required", ErrInvalidConfig)
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%w: base url is invalid", ErrInvalidConfig)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{baseURL: parsed, apiKey: apiKey, http: httpClient}, nil
}

// UpsertPartner creates or updates one Dub partner using the stable local
// partner identity as tenantId.
func (c *Client) UpsertPartner(ctx context.Context, input PartnerInput) (Partner, error) {
	input = normalizePartnerInput(input)
	if err := validatePartnerInput(input); err != nil {
		return Partner{}, err
	}
	payload := struct {
		Email    string `json:"email"`
		Name     string `json:"name,omitempty"`
		Username string `json:"username,omitempty"`
		TenantID string `json:"tenantId"`
		Country  string `json:"country,omitempty"`
	}{Email: input.Email, Name: input.Name, Username: input.Username, TenantID: input.ExternalID, Country: input.Country}

	var result Partner
	if err := c.postJSON(ctx, "/partners", payload, &result); err != nil {
		return Partner{}, err
	}
	return result, nil
}

// CreatePartnerLink creates a conversion-tracked referral link for a partner.
func (c *Client) CreatePartnerLink(ctx context.Context, input PartnerLinkInput) (Link, error) {
	input = normalizePartnerLinkInput(input)
	if err := validatePartnerLinkInput(input); err != nil {
		return Link{}, err
	}
	payload := struct {
		TenantID  string `json:"tenantId"`
		URL       string `json:"url,omitempty"`
		Key       string `json:"key,omitempty"`
		Comments  string `json:"comments,omitempty"`
		LinkProps struct {
			ExternalID      string `json:"externalId,omitempty"`
			TenantID        string `json:"tenantId,omitempty"`
			TrackConversion bool   `json:"trackConversion"`
		} `json:"linkProps"`
	}{TenantID: input.ExternalPartnerID, URL: input.DestinationURL, Key: input.Key, Comments: input.Comments}
	payload.LinkProps.ExternalID = input.ExternalLinkID
	payload.LinkProps.TenantID = input.ExternalPartnerID
	payload.LinkProps.TrackConversion = true

	var result Link
	if err := c.postJSON(ctx, "/partners/links", payload, &result); err != nil {
		return Link{}, err
	}
	return result, nil
}

// TrackLead records a signup or other lead conversion.
func (c *Client) TrackLead(ctx context.Context, input LeadInput) (*LeadResult, error) {
	input = normalizeLeadInput(input)
	if err := validateLeadInput(input); err != nil {
		return nil, err
	}
	payload := struct {
		ClickID            string         `json:"clickId"`
		EventName          string         `json:"eventName"`
		CustomerExternalID string         `json:"customerExternalId"`
		CustomerName       string         `json:"customerName,omitempty"`
		CustomerEmail      string         `json:"customerEmail,omitempty"`
		Metadata           map[string]any `json:"metadata,omitempty"`
	}{
		ClickID: input.ClickID, EventName: input.EventName, CustomerExternalID: input.CustomerExternalID,
		CustomerName: input.CustomerName, CustomerEmail: input.CustomerEmail, Metadata: input.Metadata,
	}

	var raw json.RawMessage
	if err := c.postJSON(ctx, "/track/lead", payload, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var result LeadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode dub lead response: %w", err)
	}
	return &result, nil
}

// TrackSale records one paid conversion. The caller remains the owner of order,
// subscription, commission, refund, and payout state; Dub owns attribution.
func (c *Client) TrackSale(ctx context.Context, input SaleInput) (*SaleResult, error) {
	input = normalizeSaleInput(input)
	if err := validateSaleInput(input); err != nil {
		return nil, err
	}
	payload := struct {
		CustomerExternalID string         `json:"customerExternalId"`
		Amount              int64          `json:"amount"`
		Currency            string         `json:"currency"`
		EventName           string         `json:"eventName"`
		PaymentProcessor    string         `json:"paymentProcessor"`
		InvoiceID           string         `json:"invoiceId"`
		Metadata            map[string]any `json:"metadata,omitempty"`
		LeadEventName       string         `json:"leadEventName,omitempty"`
		ClickID             string         `json:"clickId,omitempty"`
		CustomerName        string         `json:"customerName,omitempty"`
		CustomerEmail       string         `json:"customerEmail,omitempty"`
	}{
		CustomerExternalID: input.CustomerExternalID, Amount: input.Amount, Currency: input.Currency,
		EventName: input.EventName, PaymentProcessor: input.PaymentProcessor, InvoiceID: input.InvoiceID,
		Metadata: input.Metadata, LeadEventName: input.LeadEventName, ClickID: input.ClickID,
		CustomerName: input.CustomerName, CustomerEmail: input.CustomerEmail,
	}

	var result SaleResult
	if err := c.postJSON(ctx, "/track/sale", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode dub request: %w", err)
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build dub request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send dub request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read dub response: %w", err)
	}
	if len(responseBody) > maxResponseBytes {
		return fmt.Errorf("read dub response: response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(resp.StatusCode, responseBody)
	}
	if result == nil || len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	if raw, ok := result.(*json.RawMessage); ok {
		*raw = append((*raw)[:0], responseBody...)
		return nil
	}
	if err := json.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("decode dub response: %w", err)
	}
	return nil
}

func decodeAPIError(status int, body []byte) error {
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &APIError{StatusCode: status, Message: http.StatusText(status)}
	}
	message := strings.TrimSpace(envelope.Error.Message)
	if message == "" {
		message = http.StatusText(status)
	}
	return &APIError{StatusCode: status, Code: strings.TrimSpace(envelope.Error.Code), Message: message}
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
	if input.ExternalID == "" || len(input.ExternalID) > 255 {
		return fmt.Errorf("%w: external partner id is required and must be <= 255 characters", ErrInvalidInput)
	}
	if len(input.Name) > 100 || len(input.Username) > 100 {
		return fmt.Errorf("%w: partner name and username must be <= 100 characters", ErrInvalidInput)
	}
	if len(input.Email) > 190 {
		return fmt.Errorf("%w: partner email is too long", ErrInvalidInput)
	}
	address, err := mail.ParseAddress(input.Email)
	if err != nil || !strings.EqualFold(address.Address, input.Email) {
		return fmt.Errorf("%w: partner email is invalid", ErrInvalidInput)
	}
	if input.Country != "" && len(input.Country) != 2 {
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
	if input.ExternalPartnerID == "" || len(input.ExternalPartnerID) > 255 {
		return fmt.Errorf("%w: external partner id is required and must be <= 255 characters", ErrInvalidInput)
	}
	if input.ExternalLinkID == "" || len(input.ExternalLinkID) > 255 {
		return fmt.Errorf("%w: external link id is required and must be <= 255 characters", ErrInvalidInput)
	}
	if len(input.Key) > 190 {
		return fmt.Errorf("%w: link key must be <= 190 characters", ErrInvalidInput)
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
	if input.EventName == "" || len(input.EventName) > 255 {
		return fmt.Errorf("%w: lead event name is required and must be <= 255 characters", ErrInvalidInput)
	}
	if input.CustomerExternalID == "" || len(input.CustomerExternalID) > 100 {
		return fmt.Errorf("%w: customer external id is required and must be <= 100 characters", ErrInvalidInput)
	}
	if input.CustomerEmail != "" {
		address, err := mail.ParseAddress(input.CustomerEmail)
		if err != nil || !strings.EqualFold(address.Address, input.CustomerEmail) {
			return fmt.Errorf("%w: customer email is invalid", ErrInvalidInput)
		}
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
	if input.CustomerExternalID == "" || len(input.CustomerExternalID) > 100 {
		return fmt.Errorf("%w: customer external id is required and must be <= 100 characters", ErrInvalidInput)
	}
	if input.Amount < 0 {
		return fmt.Errorf("%w: sale amount must be >= 0", ErrInvalidInput)
	}
	if len(input.Currency) != 3 {
		return fmt.Errorf("%w: currency must be a three-letter ISO 4217 code", ErrInvalidInput)
	}
	if input.InvoiceID == "" {
		return fmt.Errorf("%w: invoice id is required for idempotent sale tracking", ErrInvalidInput)
	}
	if len(input.EventName) > 255 {
		return fmt.Errorf("%w: sale event name must be <= 255 characters", ErrInvalidInput)
	}
	if !allowedPaymentProcessor(input.PaymentProcessor) {
		return fmt.Errorf("%w: unsupported payment processor", ErrInvalidInput)
	}
	if input.CustomerEmail != "" {
		address, err := mail.ParseAddress(input.CustomerEmail)
		if err != nil || !strings.EqualFold(address.Address, input.CustomerEmail) {
			return fmt.Errorf("%w: customer email is invalid", ErrInvalidInput)
		}
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
