package observations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/storecenter"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalid     = errors.New("invalid store observation request")
	ErrForbidden   = errors.New("store observation access denied")
	ErrUnavailable = errors.New("store observations unavailable")
	ErrConflict    = errors.New("store observation conflict")
	ErrNotFound    = errors.New("store observation not found")
	ErrUnsupported = errors.New("consumer orders unsupported for this application")
	ErrWindowLimit = errors.New("platform order query limit")
)

type Kind string

const (
	Products Kind = "products"
	Orders   Kind = "orders"
)

func (k Kind) Valid() bool { return k == Products || k == Orders }
func ReadPermission(k Kind) string {
	if k == Products {
		return "workbench.store.products.read"
	}
	return "workbench.store.orders.read"
}
func SyncPermission(k Kind) string {
	if k == Products {
		return "workbench.store.products.sync"
	}
	return "workbench.store.orders.sync"
}

type Scope struct{ OrganizationID, ActorID, MemberID string }

func (s Scope) Valid() bool {
	return authidentity.IsBoundedIdentifier(s.OrganizationID) && authidentity.IsBoundedIdentifier(s.ActorID) && authidentity.IsBoundedIdentifier(s.MemberID)
}
func ValidID(s string) bool {
	u, e := uuid.Parse(s)
	return e == nil && u.String() == s && u != uuid.Nil
}
func Text(s string, max int) bool {
	return utf8.ValidString(s) && s == strings.TrimSpace(s) && utf8.RuneCountInString(s) <= max && !strings.ContainsFunc(s, unicode.IsControl)
}
func Identity(s string) bool { return s != "" && Text(s, 128) }

type Binding = storecenter.ProductMerchantBinding

type Price struct {
	Currency string `json:"currency"`
	Value    string `json:"value"`
	Special  string `json:"special"`
}
type Inventory struct {
	WarehouseID string `json:"warehouseId"`
	Quantity    *int64 `json:"quantity"`
}
type SKU struct {
	ID        string      `json:"id"`
	SellerSKU string      `json:"sellerSku"`
	Prices    []Price     `json:"prices"`
	Costs     []Price     `json:"costs"`
	Inventory []Inventory `json:"inventory"`
}
type SKC struct {
	ID         string `json:"id"`
	SellerCode string `json:"sellerCode"`
	Title      string `json:"title"`
	ImageURL   string `json:"imageUrl"`
	Site       string `json:"site"`
	SiteStatus *int   `json:"siteStatus"`
	SKUs       []SKU  `json:"skus"`
}
type Product struct {
	ID   string `json:"id"`
	SKCs []SKC  `json:"skcs"`
}
type ProductPage struct {
	Total int
	Items []Product
}
type OrderRef struct {
	ID        string
	Status    int
	CreatedAt time.Time
	UpdatedAt time.Time
}
type OrderPage struct {
	ReportedCount *int
	Items         []OrderRef
}
type OrderItem struct {
	ID          string `json:"id"`
	SKU         string `json:"sku"`
	SellerSKU   string `json:"sellerSku"`
	Title       string `json:"title"`
	ImageURL    string `json:"imageUrl"`
	Status      *int   `json:"status"`
	ExchangeTag *int   `json:"exchangeTag"`
}
type Package struct {
	ID      string `json:"id"`
	Waybill string `json:"waybill"`
	Carrier string `json:"carrier"`
	Label   string `json:"label"`
}
type Order struct {
	ID                string      `json:"id"`
	Site              string      `json:"site"`
	Status            *int        `json:"status"`
	StockMode         *int        `json:"stockMode"`
	Type              *int        `json:"type"`
	Tag               *int        `json:"tag"`
	Reasons           []int       `json:"reasons"`
	Items             []OrderItem `json:"items"`
	Packages          []Package   `json:"packages"`
	Amount            *Price      `json:"amount"`
	SupplyCost        *Price      `json:"supplyCost"`
	CreatedAt         string      `json:"createdAt"`
	UpdatedAt         string      `json:"updatedAt"`
	IssuedAt          string      `json:"issuedAt"`
	NeedDeliveryAt    string      `json:"needDeliveryAt"`
	HandoverAt        string      `json:"handoverAt"`
	ExpectedCollectAt string      `json:"expectedCollectAt"`
}

func (o Order) Exceptional() bool {
	if len(o.Reasons) > 0 || o.Tag != nil && *o.Tag == 1 || o.Status != nil && (*o.Status == 8 || *o.Status == 9) {
		return true
	}
	for _, p := range o.Packages {
		if p.Label == "1" || p.Label == "2" {
			return true
		}
	}
	return false
}

type TrackNode struct {
	Description string `json:"description"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	AtMillis    int64  `json:"atMillis"`
}
type Track struct {
	Carrier string      `json:"carrier"`
	Waybill string      `json:"waybill"`
	Nodes   []TrackNode `json:"nodes"`
}
type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func (w Window) Key() string {
	return w.Start.UTC().Format(time.RFC3339) + "/" + w.End.UTC().Format(time.RFC3339)
}

type Checkpoint struct {
	Page          int      `json:"page"`
	Windows       []Window `json:"windows"`
	ExpectedTotal *int     `json:"expectedTotal"`
	Seen          int      `json:"seen"`
	Pages         int      `json:"pages"`
	Incomplete    bool     `json:"incomplete"`
	Notes         []string `json:"notes"`
}

func (c *Checkpoint) Note(s string) {
	c.Incomplete = true
	for _, x := range c.Notes {
		if x == s {
			return
		}
	}
	if len(c.Notes) < 20 {
		c.Notes = append(c.Notes, s)
	}
}

type Sync struct {
	ID         string     `json:"id"`
	CommandID  string     `json:"commandId"`
	StoreID    string     `json:"storeId"`
	Kind       Kind       `json:"kind"`
	Owner      Scope      `json:"-"`
	Binding    Binding    `json:"-"`
	Key        string     `json:"-"`
	Revision   int64      `json:"-"`
	Generation int64      `json:"-"`
	Status     string     `json:"status"`
	Progress   Checkpoint `json:"progress"`
	Range      *Window    `json:"range"`
	CreatedAt  time.Time  `json:"createdAt"`
	ObservedAt *time.Time `json:"observedAt"`
	ErrorCode  string     `json:"errorCode"`
}

func (s Sync) Terminal() bool {
	return s.Status == "completed" || s.Status == "partial" || s.Status == "failed" || s.Status == "suspended"
}

type BeginInput struct {
	Kind   Kind       `json:"kind"`
	Stores []string   `json:"stores"`
	Start  *time.Time `json:"start"`
	End    *time.Time `json:"end"`
}
type Command struct {
	ID        string     `json:"id"`
	Owner     Scope      `json:"-"`
	Key       string     `json:"-"`
	Hash      string     `json:"-"`
	Input     BeginInput `json:"input"`
	CreatedAt time.Time  `json:"createdAt"`
	Syncs     []Sync     `json:"syncs"`
}

func ChildKey(parent, store string, kind Kind) string {
	h := sha256.Sum256([]byte(parent + "\x00" + store + "\x00" + string(kind)))
	return hex.EncodeToString(h[:])
}

type Record struct {
	StoreID    string    `json:"storeId"`
	SyncID     string    `json:"syncId"`
	ID         string    `json:"id"`
	ObservedAt time.Time `json:"observedAt"`
	Stale      bool      `json:"stale,omitempty"`
	Product    *Product  `json:"product,omitempty"`
	Order      *Order    `json:"order,omitempty"`
	WindowKey  string    `json:"-"`
}
type Query struct {
	Kind       Kind
	Stores     []string
	SyncID     string
	Keyword    string
	Status     string
	After      string
	Limit      int
	Sources    map[string]string
	TodayStart time.Time
	// RecordByteLimit is the server-calculated JSON array budget after reserving
	// batch metadata and the response envelope. Zero uses the repository default.
	RecordByteLimit int
}
type Summary struct {
	Total        int `json:"total"`
	Active       int `json:"active"`
	OffShelf     int `json:"offShelf"`
	Today        int `json:"today"`
	TodayUnknown int `json:"todayUnknown"`
	Pending      int `json:"pending"`
	Transit      int `json:"transit"`
	Exceptional  int `json:"exceptional"`
	Unknown      int `json:"unknown"`
}
type Result struct {
	Items    []Record `json:"items"`
	Next     string   `json:"next"`
	Summary  Summary  `json:"summary"`
	Syncs    []Sync   `json:"syncs"`
	Latest   []Sync   `json:"latest"`
	Complete bool     `json:"complete"`
}
type Merchant interface {
	Binding() Binding
	Check(context.Context) error
	Products(context.Context, int) (ProductPage, error)
	Orders(context.Context, Window, int) (OrderPage, error)
	OrderDetails(context.Context, []string) ([]Order, error)
	Track(context.Context, string, string) ([]Track, error)
}
type Access interface {
	Open(context.Context, Scope, string, Kind, bool, *Binding) (Merchant, error)
	Authorize(context.Context, Scope, Kind, bool) error
}
type Directory interface {
	ListStores(context.Context, Scope) ([]string, error)
}
type Repository interface {
	Begin(context.Context, Scope, string, string, BeginInput, []Sync) (Command, error)
	CommandByKey(context.Context, Scope, string) (Command, error)
	ReadSync(context.Context, string, string) (Sync, error)
	Heads(context.Context, string, Kind, []string) ([]Sync, []Sync, error)
	CommitPage(context.Context, Sync, Checkpoint, []Record, string, time.Time) (Sync, error)
	Stop(context.Context, Sync, string, string) (Sync, error)
	List(context.Context, string, Query) (Result, error)
	Record(context.Context, string, string, Kind, string, string) (Record, error)
}
type Starter interface {
	Ensure(context.Context, string, string) error
}
