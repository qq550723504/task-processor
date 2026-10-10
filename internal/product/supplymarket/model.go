// Package supplymarket owns explicit market disclosure and manual supply
// applications. It never widens the member-private Collection reader.
package supplymarket

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

const (
	PermissionRead   = "workbench.supply-market.read"
	PermissionSelect = "workbench.supply-market.select"
	PermissionApply  = "workbench.supply-market.apply"
	PermissionDesign = "workbench.supply-market.design"
	Timeout          = 10 * time.Second
)

var (
	ErrInvalid     = errors.New("invalid supply market request")
	ErrForbidden   = errors.New("supply market permission denied")
	ErrNotFound    = errors.New("supply market resource not found")
	ErrConflict    = errors.New("supply market revision or intent conflict")
	ErrUnknown     = errors.New("supply market commit outcome unknown")
	ErrUnavailable = errors.New("supply market dependency unavailable")
)

type Stage string

const (
	Draft              Stage = "DRAFT"
	Submitted          Stage = "SUBMITTED"
	Evaluating         Stage = "EVALUATING"
	SupplementRequired Stage = "SUPPLEMENT_REQUIRED"
	Approved           Stage = "APPROVED"
	Rejected           Stage = "REJECTED"
	PlanConfirmed      Stage = "PLAN_CONFIRMED"
	Closed             Stage = "CLOSED"
)

type Evaluation struct {
	Action               string `json:"action"`
	Note                 string `json:"note"`
	CooperationConfirmed bool   `json:"cooperationConfirmed,omitempty"`
}

func NextStage(kind string, stage Stage, event Evaluation) (Stage, error) {
	if (kind != "selected" && kind != "connection") || !text(event.Note, 1, 2000) {
		return "", ErrInvalid
	}
	if stage == Approved || stage == Rejected || stage == PlanConfirmed || stage == Closed {
		return "", ErrConflict
	}
	switch event.Action {
	case "evaluate":
		if stage == Submitted {
			return Evaluating, nil
		}
	case "request_supplement":
		if kind != "selected" {
			return "", ErrInvalid
		}
		if stage == Evaluating {
			return SupplementRequired, nil
		}
	case "supplement":
		if kind != "selected" {
			return "", ErrInvalid
		}
		if stage == SupplementRequired {
			return Evaluating, nil
		}
	case "approve":
		if kind != "selected" || !event.CooperationConfirmed {
			return "", ErrInvalid
		}
		if stage == Evaluating {
			return Approved, nil
		}
	case "reject":
		if kind != "selected" {
			return "", ErrInvalid
		}
		if stage == Evaluating {
			return Rejected, nil
		}
	case "confirm_plan":
		if kind != "connection" {
			return "", ErrInvalid
		}
		if stage == Evaluating {
			return PlanConfirmed, nil
		}
	case "close":
		if kind != "connection" {
			return "", ErrInvalid
		}
		if stage == Evaluating {
			return Closed, nil
		}
	default:
		return "", ErrInvalid
	}
	return "", ErrConflict
}

// SupplyDeclaration records a declaration, not inventory or payment facts.
type SupplyDeclaration struct {
	Stock           int    `json:"stock"`
	Capacity        string `json:"capacity,omitempty"`
	MinimumQuantity int    `json:"minimumQuantity"`
	LeadDays        int    `json:"leadDays"`
	Province        string `json:"province"`
	City            string `json:"city"`
	PriceNote       string `json:"priceNote,omitempty"`
	AfterSaleNote   string `json:"afterSaleNote,omitempty"`
}

func (s SupplyDeclaration) Validate() error {
	if s.Stock < 0 || s.Stock > 1_000_000_000 || s.Stock == 0 && !text(s.Capacity, 1, 4000) || !text(s.Capacity, 0, 4000) || s.MinimumQuantity < 1 || s.MinimumQuantity > 1_000_000_000 || s.LeadDays < 0 || s.LeadDays > 3650 || !text(s.Province, 1, 100) || !text(s.City, 1, 100) || !text(s.PriceNote, 0, 4000) || !text(s.AfterSaleNote, 0, 4000) {
		return ErrInvalid
	}
	return nil
}

type ConnectionInput struct {
	Name       string `json:"name"`
	Website    string `json:"website"`
	Contact    string `json:"contact"`
	Telephone  string `json:"telephone"`
	Categories string `json:"categories"`
}

func (i ConnectionInput) Validate() error {
	u, err := url.Parse(i.Website)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || len(i.Website) > 2048 || !text(i.Name, 1, 200) || !text(i.Contact, 1, 100) || !text(i.Telephone, 1, 100) || !text(i.Categories, 1, 4000) {
		return ErrInvalid
	}
	return nil
}

type PublicProduct struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Brand       string            `json:"brand,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Variants    []PublicVariant   `json:"variants,omitempty"`
	Images      []string          `json:"images"`
}
type PublicVariant struct {
	SourceID   string            `json:"sourceId"`
	Title      string            `json:"title,omitempty"`
	SKU        string            `json:"sku,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Currency   string            `json:"currency,omitempty"`
	Price      float64           `json:"price,omitempty"`
	Stock      int               `json:"stock"`
	Images     []string          `json:"images,omitempty"`
}

// PublicProjection builds a concrete allowlist. Canonical sources, traces,
// review internals, cost prices and qualification objects are never serialized.
func PublicProjection(snapshot catalog.ProductSnapshot) (PublicProduct, error) {
	if !text(snapshot.Title, 1, 2000) || !text(snapshot.Description, 0, sourcing.MaxSourceEnvelopeStringBytes) || !text(snapshot.Brand, 0, 1000) || len(snapshot.Attributes) > 256 || len(snapshot.Variants) > sourcing.MaxSourceEnvelopeCollectionItems || len(snapshot.Images) < 1 || len(snapshot.Images) > 40 {
		return PublicProduct{}, ErrInvalid
	}
	p := PublicProduct{Title: snapshot.Title, Description: snapshot.Description, Brand: snapshot.Brand}
	var err error
	if p.Attributes, err = publicAttributes(snapshot.Attributes); err != nil {
		return PublicProduct{}, err
	}
	for _, image := range snapshot.Images {
		if !publicImage(image.URL) {
			return PublicProduct{}, ErrInvalid
		}
		p.Images = append(p.Images, image.URL)
	}
	for _, v := range snapshot.Variants {
		if !text(v.SourceID, 0, 128) || !text(v.Title, 0, 2000) || !text(v.SKU, 0, 1000) || v.Stock < 0 || len(v.Images) > 40 {
			return PublicProduct{}, ErrInvalid
		}
		item := PublicVariant{SourceID: v.SourceID, Title: v.Title, SKU: v.SKU, Stock: v.Stock}
		if item.Attributes, err = publicAttributes(v.Attributes); err != nil {
			return PublicProduct{}, err
		}
		if v.Price != nil {
			if !text(v.Price.Currency, 1, 16) || v.Price.Amount < 0 || math.IsNaN(v.Price.Amount) || math.IsInf(v.Price.Amount, 0) {
				return PublicProduct{}, ErrInvalid
			}
			item.Currency, item.Price = v.Price.Currency, v.Price.Amount
		}
		for _, image := range v.Images {
			if !publicImage(image.URL) {
				return PublicProduct{}, ErrInvalid
			}
			item.Images = append(item.Images, image.URL)
		}
		p.Variants = append(p.Variants, item)
	}
	// Match the current source publication budget before disclosure, so every
	// released projection can actually be selected into the recipient's Product.
	items := 8 + len(snapshot.Attributes) + len(snapshot.Variants) + len(snapshot.Images)
	for _, v := range snapshot.Variants {
		items += len(v.Attributes) + len(v.Images)
	}
	if items > sourcing.MaxSourceEnvelopeAggregateItems {
		return PublicProduct{}, ErrInvalid
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > 2<<20 {
		return PublicProduct{}, ErrInvalid
	}
	return p, nil
}
func publicAttributes(values []catalog.Attribute) (map[string]string, error) {
	if len(values) > 256 {
		return nil, ErrInvalid
	}
	r := map[string]string{}
	for _, a := range values {
		if !text(a.Name, 1, 200) || !text(a.Value, 0, 4000) {
			return nil, ErrInvalid
		}
		if _, exists := r[a.Name]; exists {
			return nil, ErrInvalid
		}
		r[a.Name] = a.Value
	}
	return r, nil
}
func publicImage(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && len(raw) <= 2048 && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == "" && u.RawQuery == "" && !strings.ContainsAny(raw, "\r\n\x00")
}
func text(s string, min, max int) bool {
	return utf8.ValidString(s) && len(s) >= min && len(s) <= max && !strings.ContainsRune(s, '\x00') && (min == 0 || strings.TrimSpace(s) != "")
}

type SourceReference struct {
	Scope                 collection.Scope `json:"-"`
	ItemID                string           `json:"itemId"`
	ItemRevision          int64            `json:"itemRevision"`
	ProductKey            string           `json:"productKey"`
	OriginalPublicationID string           `json:"originalPublicationId"`
	OriginalVersion       uint64           `json:"originalVersion,string"`
	PublicationID         string           `json:"publicationId"`
	Version               uint64           `json:"version,string"`
	ApplyID               string           `json:"applyId,omitempty"`
}
type Record struct {
	ID                   string             `json:"id"`
	Kind                 string             `json:"kind"`
	Owner                collection.Scope   `json:"-"`
	Source               *SourceReference   `json:"source,omitempty"`
	Product              *PublicProduct     `json:"product,omitempty"`
	Supply               *SupplyDeclaration `json:"supply,omitempty"`
	Connection           *ConnectionInput   `json:"connection,omitempty"`
	FileIDs              []string           `json:"fileIds,omitempty"`
	Disclosure           bool               `json:"disclosure"`
	Stage                Stage              `json:"stage"`
	Revision             int64              `json:"revision"`
	CooperationConfirmed bool               `json:"cooperationConfirmed"`
	CreatedAt            time.Time          `json:"createdAt"`
}
type Event struct {
	ID        string    `json:"id"`
	RecordID  string    `json:"recordId"`
	ActorID   string    `json:"actorId"`
	Action    string    `json:"action"`
	Note      string    `json:"note"`
	FileIDs   []string  `json:"fileIds,omitempty"`
	Stage     Stage     `json:"stage"`
	CreatedAt time.Time `json:"createdAt"`
}
type Release struct {
	ID            string            `json:"id"`
	RecordID      string            `json:"-"`
	Channel       string            `json:"channel"`
	OriginalOwner collection.Scope  `json:"-"`
	Source        SourceReference   `json:"-"`
	Product       PublicProduct     `json:"product"`
	Supply        SupplyDeclaration `json:"supply"`
	Revision      int64             `json:"revision"`
	Active        bool              `json:"active"`
	PublishedAt   time.Time         `json:"publishedAt"`
}
type Query struct {
	After, Keyword, Kind string
	Limit                int
	Ended                bool
}

func (q Query) Validate() error {
	if q.Limit < 1 || q.Limit > 50 || !text(q.Keyword, 0, 80) || q.After != "" && !collection.ValidID(q.After) || q.Kind != "" && q.Kind != "official" && q.Kind != "selected" && q.Kind != "connection" {
		return ErrInvalid
	}
	return nil
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
	Total      int64  `json:"total"`
}
type Receipt struct {
	OperationID string              `json:"operationId"`
	RecordID    string              `json:"recordId,omitempty"`
	ReleaseID   string              `json:"releaseId,omitempty"`
	Collection  *collection.Receipt `json:"collection,omitempty"`
	Revision    int64               `json:"revision"`
	Replayed    bool                `json:"replayed"`
}
