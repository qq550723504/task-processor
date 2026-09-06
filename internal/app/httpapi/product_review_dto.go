package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"task-processor/internal/product/review"
	"time"

	"github.com/google/uuid"
	sigjson "sigs.k8s.io/json"
)

const productReviewSchemaVersion = 1
const productReviewCoverage = "product-title-proposals-only"
const maxProductReviewQueryBytes = 1024
const maxProductReviewResponseBytes = 128 << 10

var productReviewLimitPattern = regexp.MustCompile(`^[1-9][0-9]{0,2}$`)

type productReviewInputDTO struct {
	ProductKey  string `json:"product_key"`
	BaseVersion string `json:"base_version"`
}

type productReviewQualityDTO struct {
	Overall               float64 `json:"overall"`
	EvidenceCoverage      float64 `json:"evidence_coverage"`
	RequiredFieldCoverage float64 `json:"required_field_coverage"`
}

type productReviewDecisionDTO struct {
	Action   string `json:"action"`
	Actor    string `json:"actor"`
	Revision string `json:"revision"`
	Before   string `json:"before"`
	After    string `json:"after"`
	At       string `json:"at"`
}

type productReviewReceiptDTO struct {
	ProposalID     string `json:"proposal_id"`
	Revision       string `json:"revision"`
	ProductVersion string `json:"product_version"`
	PublicationID  string `json:"publication_id"`
	Actor          string `json:"actor"`
	At             string `json:"at"`
}

type productReviewViewDTO struct {
	SchemaVersion int                        `json:"schema_version"`
	Coverage      string                     `json:"coverage"`
	ProposalID    string                     `json:"proposal_id"`
	Owner         string                     `json:"owner"`
	Input         productReviewInputDTO      `json:"input"`
	Before        string                     `json:"before"`
	After         string                     `json:"after"`
	OriginalTitle string                     `json:"original_title"`
	Policy        string                     `json:"policy"`
	State         string                     `json:"state"`
	Revision      string                     `json:"revision"`
	Evidence      []review.Evidence          `json:"evidence"`
	Quality       productReviewQualityDTO    `json:"quality"`
	Unresolved    []string                   `json:"unresolved"`
	Decisions     []productReviewDecisionDTO `json:"decisions"`
	ApplyReceipt  *productReviewReceiptDTO   `json:"apply_receipt,omitempty"`
}

func marshalProductReviewView(view review.View) ([]byte, error) {
	if err := review.ValidateView(view); err != nil {
		return nil, err
	}
	decisions := make([]productReviewDecisionDTO, len(view.History))
	for index, decision := range view.History {
		if decision.At.IsZero() {
			return nil, review.ErrUnavailable
		}
		decisions[index] = productReviewDecisionDTO{Action: decision.Action, Actor: decision.Actor, Revision: strconv.FormatUint(decision.Revision, 10), Before: decision.Before, After: decision.After, At: decision.At.UTC().Format(time.RFC3339Nano)}
	}
	evidence := append([]review.Evidence{}, view.Evidence...)
	unresolved := append([]string{}, view.Unresolved...)
	dto := productReviewViewDTO{
		SchemaVersion: productReviewSchemaVersion,
		Coverage:      productReviewCoverage,
		ProposalID:    view.ID,
		Owner:         view.Owner,
		Input:         productReviewInputDTO{ProductKey: view.Input.ProductKey, BaseVersion: strconv.FormatUint(view.Input.BaseVersion, 10)},
		Before:        view.Before,
		After:         view.Title,
		OriginalTitle: view.OriginalTitle,
		Policy:        view.Policy,
		State:         view.State,
		Revision:      strconv.FormatUint(view.Revision, 10),
		Evidence:      evidence,
		Quality:       productReviewQualityDTO{Overall: view.Quality.Overall, EvidenceCoverage: view.Quality.EvidenceCoverage, RequiredFieldCoverage: view.Quality.RequiredFieldCoverage},
		Unresolved:    unresolved,
		Decisions:     decisions,
	}
	if view.Receipt != nil {
		if view.Receipt.At.IsZero() {
			return nil, review.ErrUnavailable
		}
		dto.ApplyReceipt = &productReviewReceiptDTO{ProposalID: view.Receipt.ProposalID, Revision: strconv.FormatUint(view.Receipt.Revision, 10), ProductVersion: strconv.FormatUint(view.Receipt.ProductVersion, 10), PublicationID: view.Receipt.PublicationID, Actor: view.Receipt.Actor, At: view.Receipt.At.UTC().Format(time.RFC3339Nano)}
	}
	return json.Marshal(dto)
}

type productReviewCursorDTO struct {
	Version    int    `json:"v"`
	ProposalID string `json:"proposal_id"`
}

func encodeProductReviewCursor(cursor review.PageCursor) (string, error) {
	if err := (review.PageRequest{Limit: 1, Cursor: &cursor}).Validate(); err != nil {
		return "", err
	}
	wire, err := json.Marshal(productReviewCursorDTO{Version: 1, ProposalID: cursor.ID})
	if err != nil {
		return "", review.ErrInvalid
	}
	return base64.RawURLEncoding.EncodeToString(wire), nil
}

func decodeProductReviewCursor(value string) (review.PageCursor, error) {
	if value == "" || len(value) > 256 {
		return review.PageCursor{}, review.ErrInvalid
	}
	wire, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return review.PageCursor{}, review.ErrInvalid
	}
	var dto productReviewCursorDTO
	violations, err := sigjson.UnmarshalStrict(wire, &dto)
	parsed, idErr := uuid.Parse(dto.ProposalID)
	if err != nil || len(violations) != 0 || dto.Version != 1 || idErr != nil || parsed == uuid.Nil || parsed.String() != dto.ProposalID {
		return review.PageCursor{}, review.ErrInvalid
	}
	cursor := review.PageCursor{ID: dto.ProposalID}
	canonical, err := encodeProductReviewCursor(cursor)
	if err != nil || canonical != value {
		return review.PageCursor{}, review.ErrInvalid
	}
	return cursor, nil
}

func parseProductReviewCollectionQuery(query url.Values) (review.PageRequest, error) {
	for key, values := range query {
		if (key != "view" && key != "limit" && key != "cursor") || len(values) != 1 {
			return review.PageRequest{}, review.ErrInvalid
		}
	}
	view, present := query["view"]
	if !present || len(view) != 1 || view[0] != "actionable" {
		return review.PageRequest{}, review.ErrInvalid
	}
	request := review.PageRequest{Limit: 20}
	if raw, ok := query["limit"]; ok {
		if !productReviewLimitPattern.MatchString(raw[0]) {
			return review.PageRequest{}, review.ErrInvalid
		}
		limit, err := strconv.Atoi(raw[0])
		if err != nil || limit > review.MaxPageSize {
			return review.PageRequest{}, review.ErrInvalid
		}
		request.Limit = limit
	}
	if raw, ok := query["cursor"]; ok {
		cursor, err := decodeProductReviewCursor(raw[0])
		if err != nil {
			return review.PageRequest{}, err
		}
		request.Cursor = &cursor
	}
	return request, request.Validate()
}

type productReviewCollectionItemDTO struct {
	ProposalID       string `json:"proposal_id"`
	ProductKey       string `json:"product_key"`
	BaseVersion      string `json:"base_version"`
	ProposalRevision string `json:"proposal_revision"`
	State            string `json:"state"`
}

type productReviewPageDTO struct {
	SchemaVersion int                              `json:"schema_version"`
	Coverage      string                           `json:"coverage"`
	Items         []productReviewCollectionItemDTO `json:"items"`
	NextCursor    *string                          `json:"next_cursor"`
}

func marshalProductReviewPage(page review.Page) ([]byte, error) {
	items := make([]productReviewCollectionItemDTO, len(page.Items))
	for index, item := range page.Items {
		if err := review.ValidateCollectionItem(item); err != nil {
			return nil, err
		}
		items[index] = productReviewCollectionItemDTO{ProposalID: item.ID, ProductKey: item.ProductKey, BaseVersion: strconv.FormatUint(item.BaseVersion, 10), ProposalRevision: strconv.FormatUint(item.Revision, 10), State: item.State}
	}
	if page.NextCursor != nil && len(page.Items) == 0 {
		return nil, review.ErrUnavailable
	}
	var next *string
	if page.NextCursor != nil {
		encoded, err := encodeProductReviewCursor(*page.NextCursor)
		if err != nil {
			return nil, err
		}
		next = &encoded
	}
	return json.Marshal(productReviewPageDTO{SchemaVersion: productReviewSchemaVersion, Coverage: productReviewCoverage, Items: items, NextCursor: next})
}
