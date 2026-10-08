// Package ecoservices owns third-party qualification, listings and fulfillment.
package ecoservices

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

var (
	ErrInvalid      = errors.New("ecoservices invalid input")
	ErrNotFound     = errors.New("ecoservices not found")
	ErrConflict     = errors.New("ecoservices version or intent conflict")
	ErrForbidden    = errors.New("ecoservices relationship denied")
	ErrUnavailable  = errors.New("ecoservices dependency unavailable")
	ErrNotQualified = errors.New("ecoservices provider not qualified")
)

const PolicyVersion = "ecoservices-v1-10-platform-fee-manual-expiry"

type Scope struct {
	OrganizationID, ActorID string
	Platform                bool
}
type Category string

const (
	CompanyRegistration   Category = "COMPANY_REGISTRATION"
	TrademarkRegistration Category = "TRADEMARK_REGISTRATION"
	StoreOpening          Category = "STORE_OPENING"
	StoreOperation        Category = "STORE_OPERATION"
)

func validCategory(c Category) bool {
	return c == CompanyRegistration || c == TrademarkRegistration || c == StoreOpening || c == StoreOperation
}
func ValidID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u != uuid.Nil && u.String() == id
}
func validText(v string, max int) bool {
	return v != "" && strings.TrimSpace(v) == v && len(v) <= max && !strings.ContainsRune(v, 0)
}

type Application struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"-"`
	CompanyName        string     `json:"companyName"`
	RegistrationNumber string     `json:"registrationNumber"`
	Categories         []Category `json:"categories"`
	Regions            []string   `json:"regions"`
	FileIDs            []string   `json:"fileIds"`
	State              string     `json:"state"`
	Version            int64      `json:"version,string"`
	AgreementVersion   string     `json:"agreementVersion"`
	AgreementAccepted  bool       `json:"agreementAccepted"`
	MerchantID         string     `json:"-"`
	OnboardingState    string     `json:"onboardingState"`
	ReviewReason       string     `json:"reviewReason"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}
type Listing struct {
	ID                     string   `json:"id"`
	ProviderOrganizationID string   `json:"-"`
	ProviderName           string   `json:"providerName"`
	Category               Category `json:"category"`
	Title                  string   `json:"title"`
	Description            string   `json:"description"`
	Items                  []string `json:"items"`
	Regions                []string `json:"regions"`
	Platforms              []string `json:"platforms"`
	PriceMinor             int64    `json:"priceMinor,string"`
	DeliveryDays           int      `json:"deliveryDays"`
	State                  string   `json:"state"`
	Version                int64    `json:"version,string"`
}
type Quote struct {
	AmountMinor        int64  `json:"amountMinor,string"`
	Scope              string `json:"scope"`
	AcceptanceCriteria string `json:"acceptanceCriteria"`
	DeliveryDays       int    `json:"deliveryDays"`
	Version            int64  `json:"version,string"`
}
type Delivery struct {
	Version     int64     `json:"version,string"`
	Content     string    `json:"content"`
	FileIDs     []string  `json:"fileIds"`
	SubmittedAt time.Time `json:"submittedAt"`
}
type RefundAgreement struct {
	Version           int64  `json:"version,string"`
	AmountMinor       int64  `json:"amountMinor,string"`
	Reason            string `json:"reason"`
	BuyerConfirmed    bool   `json:"buyerConfirmed"`
	ProviderConfirmed bool   `json:"providerConfirmed"`
	State             string `json:"state"`
}
type Request struct {
	ID                      string           `json:"id"`
	BuyerOrganizationID     string           `json:"-"`
	ProviderOrganizationID  string           `json:"-"`
	ListingID               string           `json:"listingId"`
	ListingVersion          int64            `json:"listingVersion,string"`
	Title                   string           `json:"title"`
	Category                Category         `json:"category"`
	Description             string           `json:"description"`
	FileIDs                 []string         `json:"fileIds"`
	State                   string           `json:"state"`
	Version                 int64            `json:"version,string"`
	Quote                   *Quote           `json:"quote,omitempty"`
	Delivery                *Delivery        `json:"delivery,omitempty"`
	Refund                  *RefundAgreement `json:"refund,omitempty"`
	OrderID                 string           `json:"orderId,omitempty"`
	PaymentReceiptID        string           `json:"-"`
	AcceptanceID            string           `json:"acceptanceId,omitempty"`
	AcceptedDeliveryVersion int64            `json:"acceptedDeliveryVersion,string"`
	FinancialFence          bool             `json:"financialHold"`
	FinancialState          string           `json:"financialState"`
	FinancialRevision       int64            `json:"-"`
	FinancialReason         string           `json:"financialReason"`
	FundsExpireAt           *time.Time       `json:"fundsExpireAt,omitempty"`
	CreatedAt               time.Time        `json:"createdAt"`
	UpdatedAt               time.Time        `json:"updatedAt"`
	Side                    string           `json:"side" gorm:"-"`
}
type Command struct {
	Scope             Scope
	Key, Kind, ID     string
	Version           int64
	Application       *Application
	Listing           *Listing
	Description       string
	FileIDs           []string
	Quote             *Quote
	Delivery          *Delivery
	RefundAmountMinor int64
	RefundVersion     int64
	DeliveryVersion   int64
	Reason            string
	AgreementVersion  string
	PolicyAccepted    string
	Fingerprint       string
}
type Result struct {
	Application *Application `json:"application,omitempty"`
	Listing     *Listing     `json:"listing,omitempty"`
	Request     *Request     `json:"request,omitempty"`
}
type FinancialCommand struct {
	RecoveryGeneration                                      int64 `json:"-"`
	ID, RequestID, OrderID, Kind, SourceProofID, ActorID    string
	BuyerOrganizationID, ProviderOrganizationID, MerchantID string
	Quote                                                   Quote
	AmountMinor                                             int64
	PolicyVersion                                           string
	State                                                   string
	DispatchAdmitted                                        bool
	DispatchOperationID                                     string
}
type Query struct {
	Group, Stage            string
	Scope                   Scope
	Kind, ID, Search, State string
	Category                Category
	Side                    string
	Page, PageSize          int
	From, To                *time.Time
}
type Page struct {
	Applications []Application    `json:"applications,omitempty"`
	Listings     []Listing        `json:"listings,omitempty"`
	Requests     []Request        `json:"requests,omitempty"`
	Total        int64            `json:"total,string"`
	Counts       map[string]int64 `json:"counts,omitempty"`
}
type Repository interface {
	Apply(context.Context, Command, int) (Result, error)
	Read(context.Context, Query) (Page, error)
	PendingFinancialCommands(context.Context, int) ([]FinancialCommand, error)
	AdmitFinancialCommand(context.Context, FinancialCommand) (FinancialCommand, error)
	CompleteFinancialCommand(context.Context, FinancialCommand, FinancialResult) error
	FinancialCommand(context.Context, string) (FinancialCommand, error)
	OriginalFinancialCommand(context.Context, string) (FinancialCommand, error)
	VerifyProviderQualification(context.Context, string, string) error
}
type FinancialResult struct {
	OrderID, PaymentReceiptID, ReceiptID, State, Reason string
	FundsExpireAt                                       *time.Time
	Revision                                            int64
	FullRefund                                          bool
}
type TradingPort interface {
	ExecuteServiceCommand(context.Context, FinancialCommand) (FinancialResult, error)
}
type Service struct {
	repo       Repository
	trading    TradingPort
	freezeDays int
}

func NewService(repo Repository, trading TradingPort, freezeDays int) (*Service, error) {
	if repo == nil || freezeDays < 1 || freezeDays > 180 {
		return nil, ErrUnavailable
	}
	return &Service{repo: repo, trading: trading, freezeDays: freezeDays}, nil
}
