package dataservice

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"time"
	"unicode/utf8"
)

type CustomInput struct {
	Name      string                `json:"name"`
	Query     dataacquisition.Query `json:"query"`
	Purpose   string                `json:"purpose"`
	TimeRange string                `json:"timeRange,omitempty"`
	Format    string                `json:"format"`
	Notes     string                `json:"notes,omitempty"`
}
type CustomSpec struct {
	Description      string `json:"description"`
	QuoteNote        string `json:"quoteNote"`
	ConfirmationNote string `json:"confirmationNote"`
	Format           string `json:"format"`
	MaximumRows      int    `json:"maximumRows"`
}
type CustomEvent struct {
	Revision   int64     `json:"revision"`
	State      string    `json:"state"`
	OperatorID string    `json:"operatorId"`
	Note       string    `json:"note"`
	At         time.Time `json:"at"`
}
type CustomRequest struct {
	ID              string           `json:"id"`
	Scope           collection.Scope `json:"-"`
	Input           CustomInput      `json:"input"`
	State           string           `json:"state"`
	Revision        int64            `json:"revision"`
	Spec            *CustomSpec      `json:"spec,omitempty"`
	SpecRevision    int64            `json:"specRevision"`
	BatchID         string           `json:"batchId,omitempty"`
	DeliveredRows   int              `json:"deliveredRows"`
	CreatedAt       time.Time        `json:"createdAt"`
	Events          []CustomEvent    `json:"events"`
	NextEventBefore int64            `json:"nextEventBefore,omitempty"`
}
type CustomPatch struct {
	State string      `json:"state"`
	Note  string      `json:"note"`
	Spec  *CustomSpec `json:"spec,omitempty"`
}
type CustomPage struct {
	Items      []CustomRequest `json:"items"`
	NextCursor string          `json:"nextCursor,omitempty"`
}
type Operator struct{ ID string }
type SpecialistAccess interface {
	Specialist(context.Context) (Operator, error)
	CheckApplicant(context.Context, collection.Scope) error
}

// DeliveryAuthority is minted only after current platform authority and the
// stored applicant/spec have been checked. Its scope cannot be JSON-supplied.
type DeliveryAuthority struct {
	requestID              string
	scope                  collection.Scope
	operator               Operator
	revision, specRevision int64
	site                   string
}

func (a DeliveryAuthority) Valid() bool {
	_, err := dataacquisition.ResolveSite(a.site)
	return err == nil && collection.ValidID(a.requestID) && a.scope.Validate() == nil && authidentity.IsBoundedIdentifier(a.operator.ID) && a.revision > 0 && a.specRevision > 0
}
func (a DeliveryAuthority) RequestID() string            { return a.requestID }
func (a DeliveryAuthority) Scope() collection.Scope      { return a.scope }
func (a DeliveryAuthority) Operator() Operator           { return a.operator }
func (a DeliveryAuthority) Site() string                 { return a.site }
func (a DeliveryAuthority) SpecificationRevision() int64 { return a.specRevision }
func (a DeliveryAuthority) Fingerprint() string {
	return collection.Digest(struct {
		Request                string
		Scope                  collection.Scope
		Operator               Operator
		Revision, SpecRevision int64
	}{a.requestID, a.scope, a.operator, a.revision, a.specRevision})
}
func (a DeliveryAuthority) Matches(r CustomRequest) bool {
	return a.Valid() && a.site == r.Input.Query.Site && a.requestID == r.ID && a.scope == r.Scope && a.revision == r.Revision && a.specRevision == r.SpecRevision && r.State == "PREPARING" && r.Spec != nil
}

type CustomRepository interface {
	Submit(context.Context, collection.Scope, string, CustomInput) (CustomRequest, error)
	Read(context.Context, collection.Scope, string) (CustomRequest, error)
	List(context.Context, collection.Scope, int) ([]CustomRequest, error)
	AdminRead(context.Context, Operator, string) (CustomRequest, error)
	AdminList(context.Context, Operator, string, string, int) (CustomPage, error)
	Command(context.Context, Operator, string) (CustomRequest, error)
	Change(context.Context, Operator, string, string, int64, CustomPatch) (CustomRequest, error)
	Deliver(context.Context, DeliveryAuthority, string, []collection.OwnProduct) (CustomRequest, error)
}

func textValid(s string, max int, required bool) bool {
	return (!required || strings.TrimSpace(s) != "") && len(s) <= max && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func formatValid(s string) bool { return s == "csv" || s == "json" || s == "xlsx" }
func ValidateCustomProducts(rows []collection.OwnProduct) error {
	if len(rows) < 1 || len(rows) > 200 {
		return ErrInvalid
	}
	raw, err := json.Marshal(rows)
	if err != nil || len(raw) > collection.MaxPayloadBytes {
		return ErrInvalid
	}
	for i, row := range rows {
		for _, v := range row.Variants {
			if v.Price < 0 || math.IsNaN(v.Price) || math.IsInf(v.Price, 0) || v.Stock < 0 || (v.SourceID == "" && v.SKU == "") || len(v.Currency) != 3 {
				return ErrInvalid
			}
			for _, r := range v.Currency {
				if r < 'A' || r > 'Z' {
					return ErrInvalid
				}
			}
		}
		if _, err = collection.OwnEnvelope(collection.StableID("custom-row-validation", strconv.Itoa(i)), row); err != nil {
			return ErrInvalid
		}
	}
	return nil
}
func NormalizeCustomInput(input CustomInput) (CustomInput, error) {
	q, err := dataacquisition.NormalizeQuery(input.Query)
	if err != nil || !textValid(input.Name, 200, true) || !textValid(input.Purpose, 1000, true) || !textValid(input.TimeRange, 1000, false) || !textValid(input.Notes, 4000, false) || !formatValid(input.Format) {
		return CustomInput{}, ErrInvalid
	}
	input.Query = q
	input.Name = strings.TrimSpace(input.Name)
	return input, nil
}
func ValidateCustomChange(request CustomRequest, patch CustomPatch) error {
	if request.State == "CLOSED" || request.State == "DELIVERED" {
		return ErrConflict
	}
	if !textValid(patch.Note, 2000, true) {
		return ErrInvalid
	}
	if patch.State != "SPEC_CONFIRMED" && patch.Spec != nil {
		return ErrInvalid
	}
	switch patch.State {
	case "EVALUATING":
		if request.State != "SUBMITTED" && request.State != "EVALUATING" {
			return ErrConflict
		}
	case "SPEC_CONFIRMED":
		if request.State != "EVALUATING" && request.State != "SPEC_CONFIRMED" {
			return ErrConflict
		}
		if patch.Spec == nil || !textValid(patch.Spec.Description, 4000, true) || !textValid(patch.Spec.QuoteNote, 2000, true) || !textValid(patch.Spec.ConfirmationNote, 2000, true) || !formatValid(patch.Spec.Format) || patch.Spec.MaximumRows < 1 || patch.Spec.MaximumRows > 200 {
			return ErrInvalid
		}
	case "PREPARING":
		if (request.State != "SPEC_CONFIRMED" && request.State != "PREPARING") || request.Spec == nil || request.SpecRevision < 1 {
			return ErrConflict
		}
	case "CLOSED":
	default:
		return ErrConflict
	}
	return nil
}

type CustomService struct {
	repo    CustomRepository
	access  Access
	special SpecialistAccess
}

func NewCustomService(repo CustomRepository, access Access, special SpecialistAccess) (*CustomService, error) {
	if repo == nil || access == nil || special == nil {
		return nil, ErrUnavailable
	}
	return &CustomService{repo, access, special}, nil
}
func (s *CustomService) Submit(ctx context.Context, command string, input CustomInput) (CustomRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, err := s.access.Resolve(ctx, PermissionMarket)
	if err != nil {
		return CustomRequest{}, err
	}
	if !collection.ValidID(command) {
		return CustomRequest{}, ErrInvalid
	}
	input, err = NormalizeCustomInput(input)
	if err != nil {
		return CustomRequest{}, err
	}
	return s.repo.Submit(ctx, scope, command, input)
}
func (s *CustomService) Read(ctx context.Context, id string) (CustomRequest, error) {
	scope, err := s.access.Resolve(ctx, PermissionMarket)
	if err != nil {
		return CustomRequest{}, err
	}
	return s.repo.Read(ctx, scope, id)
}
func (s *CustomService) List(ctx context.Context) ([]CustomRequest, error) {
	scope, err := s.access.Resolve(ctx, PermissionMarket)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, scope, 100)
}
func (s *CustomService) AdminRead(ctx context.Context, id string) (CustomRequest, error) {
	op, err := s.special.Specialist(ctx)
	if err != nil {
		return CustomRequest{}, err
	}
	return s.repo.AdminRead(ctx, op, id)
}
func (s *CustomService) Command(ctx context.Context, command string) (CustomRequest, error) {
	op, err := s.special.Specialist(ctx)
	if err != nil {
		return CustomRequest{}, err
	}
	return s.repo.Command(ctx, op, command)
}
func (s *CustomService) AdminList(ctx context.Context, state, cursor string, limit int) (CustomPage, error) {
	op, err := s.special.Specialist(ctx)
	if err != nil {
		return CustomPage{}, err
	}
	return s.repo.AdminList(ctx, op, state, cursor, limit)
}
func (s *CustomService) Change(ctx context.Context, id, command string, revision int64, patch CustomPatch) (CustomRequest, error) {
	op, err := s.special.Specialist(ctx)
	if err != nil {
		return CustomRequest{}, err
	}
	if !collection.ValidID(id) || !collection.ValidID(command) || revision < 1 {
		return CustomRequest{}, ErrInvalid
	}
	return s.repo.Change(ctx, op, id, command, revision, patch)
}
func (s *CustomService) Deliver(ctx context.Context, id, command string, revision, specRevision int64, rows []collection.OwnProduct) (CustomRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	op, err := s.special.Specialist(ctx)
	if err != nil {
		return CustomRequest{}, err
	}
	if !collection.ValidID(id) || !collection.ValidID(command) || revision < 1 || specRevision < 1 || len(rows) < 1 || len(rows) > 200 {
		return CustomRequest{}, ErrInvalid
	}
	request, err := s.repo.AdminRead(ctx, op, id)
	if err != nil {
		return CustomRequest{}, err
	}
	// Replay is verified by the immutable delivery command in persistence; it
	// does not mint new Product facts or replace an already delivered dataset.
	if request.State != "DELIVERED" && (request.State != "PREPARING" || request.Revision != revision || request.SpecRevision != specRevision) {
		return CustomRequest{}, ErrConflict
	}
	if err = s.special.CheckApplicant(ctx, request.Scope); err != nil {
		return CustomRequest{}, err
	}
	if err := ValidateCustomProducts(rows); err != nil {
		return CustomRequest{}, err
	}
	authority := DeliveryAuthority{requestID: id, scope: request.Scope, operator: op, revision: revision, specRevision: specRevision, site: request.Input.Query.Site}
	return s.repo.Deliver(ctx, authority, command, rows)
}
