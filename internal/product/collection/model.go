// Package collection owns actor-private product batches and immutable source references.
package collection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

const (
	PermissionRead   = "workbench.collection.read"
	PermissionManage = "workbench.collection.manage"
	Timeout          = 10 * time.Second
	MaxPayloadBytes  = 2 << 20
)

var (
	ErrInvalid     = errors.New("invalid collection request")
	ErrForbidden   = errors.New("collection permission denied")
	ErrNotFound    = errors.New("collection resource not found")
	ErrConflict    = errors.New("collection revision or operation conflict")
	ErrUnknown     = errors.New("collection commit outcome unknown")
	ErrUnavailable = errors.New("collection dependency unavailable")
)

type Scope struct{ OrganizationID, ActorID, MemberID string }

func (s Scope) Validate() error {
	if !authidentity.IsBoundedIdentifier(s.OrganizationID) || !authidentity.IsBoundedIdentifier(s.ActorID) || !authidentity.IsBoundedIdentifier(s.MemberID) {
		return ErrForbidden
	}
	return nil
}

type Batch struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Revision   int64      `json:"revision"`
	Count      int64      `json:"count"`
	CreatedAt  time.Time  `json:"createdAt"`
	ArchivedAt *time.Time `json:"archivedAt,omitempty"`
}
type Source struct {
	ProductKey    string `json:"productKey"`
	PublicationID string `json:"publicationId"`
	Version       uint64 `json:"version,string"`
	OperationID   string `json:"operationId,omitempty"`
	Kind          string `json:"kind"`
}
type Item struct {
	ID           string     `json:"id"`
	BatchID      string     `json:"batchId"`
	Source       Source     `json:"source"`
	Revision     int64      `json:"revision"`
	CreatedAt    time.Time  `json:"createdAt"`
	ArchivedAt   *time.Time `json:"archivedAt,omitempty"`
	Title        string     `json:"title,omitempty"`
	ThumbnailURL string     `json:"thumbnailUrl,omitempty"`
}
type ItemDetail struct {
	Item    Item                    `json:"item"`
	Product catalog.ProductSnapshot `json:"product"`
}
type Query struct {
	After, Keyword string
	Limit          int
}

func (q Query) Validate() error {
	if q.Limit < 1 || q.Limit > 100 || len(q.Keyword) > 80 || !utf8.ValidString(q.Keyword) || strings.ContainsAny(q.Keyword, "\x00\r\n") || q.After != "" && !ValidID(q.After) {
		return ErrInvalid
	}
	return nil
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
	Total      int64  `json:"total"`
}
type OwnProduct struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Brand       string            `json:"brand,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Variants    []OwnVariant      `json:"variants,omitempty"`
	Images      []string          `json:"images"`
}
type OwnVariant struct {
	SourceID   string            `json:"sourceId"`
	Title      string            `json:"title"`
	SKU        string            `json:"sku"`
	Attributes map[string]string `json:"attributes"`
	Currency   string            `json:"currency"`
	Price      float64           `json:"price"`
	Stock      int               `json:"stock"`
}
type Mutation struct {
	Action            string       `json:"action"`
	BatchID           string       `json:"batchId,omitempty"`
	ItemID            string       `json:"itemId,omitempty"`
	TargetBatchID     string       `json:"targetBatchId,omitempty"`
	ExpectedRevision  int64        `json:"expectedRevision,omitempty"`
	Name              string       `json:"name,omitempty"`
	SourceOperationID string       `json:"sourceOperationId,omitempty"`
	Product           *OwnProduct  `json:"product,omitempty"`
	Products          []OwnProduct `json:"products,omitempty"`
}
type Command struct {
	Scope                       Scope
	Key, OperationID, InputHash string
	Mutation                    Mutation
	Source                      *Source
	Envelope                    *sourcing.SourceEnvelope
	Envelopes                   []sourcing.SourceEnvelope
}
type Receipt struct {
	OperationID string `json:"operationId"`
	BatchID     string `json:"batchId,omitempty"`
	ItemID      string `json:"itemId,omitempty"`
	Revision    int64  `json:"revision"`
	Replayed    bool   `json:"replayed"`
}
type Authorizer interface {
	Authorize(context.Context, string) (Scope, error)
}
type Repository interface {
	Execute(context.Context, Command) (Receipt, error)
	ReadOperation(context.Context, Scope, string) (Receipt, error)
	ListBatches(context.Context, Scope, Query) (Page[Batch], error)
	ReadBatch(context.Context, Scope, string) (Batch, error)
	ListItems(context.Context, Scope, string, Query) (Page[Item], error)
	ReadItem(context.Context, Scope, string) (Item, error)
}

func ValidID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
func ValidSourceKind(value string) bool {
	return value == "own" || value == "acquisition" || value == "market" || value == "sds_template" || value == "sds_finished"
}
func StableID(parts ...string) string {
	data, _ := json.Marshal(parts)
	return uuid.NewSHA1(uuid.NameSpaceURL, data).String()
}
func Digest(value any) string {
	data, _ := json.Marshal(value)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// OwnEnvelope records user-declared evidence. Images are candidates, never approvals.
func OwnEnvelope(id string, input OwnProduct) (sourcing.SourceEnvelope, error) {
	if !ValidID(id) || strings.TrimSpace(input.Title) == "" || len(input.Title) > 2000 || len(input.Description) > 1<<20 || len(input.Images) > 40 || len(input.Variants) > 1000 || len(input.Attributes) > 256 {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) > MaxPayloadBytes || !utf8.Valid(raw) {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	envelope := sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: "user_input", SourcePlatform: "own_product", SourceID: id}, RawReference: sourcing.RawSourceReference{ReferenceType: "user_input", ReferenceID: id, Checksum: sourcing.RawSnapshotChecksum(string(raw))}, ProductCandidate: sourcing.ProductCandidate{Title: input.Title, Description: input.Description, Brand: input.Brand, Attributes: input.Attributes}}
	for _, variant := range input.Variants {
		envelope.ProductCandidate.Variants = append(envelope.ProductCandidate.Variants, sourcing.ProductVariantCandidate{SourceID: variant.SourceID, Title: variant.Title, SKU: variant.SKU, Attributes: variant.Attributes, Currency: variant.Currency, Price: variant.Price, Stock: variant.Stock})
	}
	for index, value := range input.Images {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || len(value) > 2048 {
			return sourcing.SourceEnvelope{}, ErrInvalid
		}
		envelope.AssetCandidates = append(envelope.AssetCandidates, sourcing.AssetCandidate{SourceID: StableID(id, value), URL: value, MediaType: "image", Role: map[bool]string{true: "main", false: "detail"}[index == 0]})
	}
	return sourcing.NormalizePublicationEnvelope(envelope)
}
