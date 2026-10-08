package preparation

import (
	"context"
	"errors"
	"sort"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/collection"
)

const (
	OperationAdapt     = "adapt"
	OperationUpload    = "upload"
	OperationOptimize  = "optimize"
	OperationPending   = "pending"
	OperationRunning   = "running"
	OperationCompleted = "completed"
	OperationCancelled = "cancelled"
	ItemPending        = "pending"
	ItemRunning        = "running"
	ItemSucceeded      = "succeeded"
	ItemMissing        = "missing"
	ItemReview         = "review"
	ItemUnknown        = "unknown"
	ItemDenied         = "denied"
	ItemFailed         = "failed"
	ItemCancelled      = "cancelled"
)

type OperationInput struct {
	PreparationID    string   `json:"preparationId"`
	ExpectedRevision int64    `json:"expectedRevision"`
	Action           string   `json:"action"`
	StoreID          string   `json:"storeId"`
	SourceIDs        []string `json:"sourceIds,omitempty"`
	CategoryID       int64    `json:"categoryId,omitempty"`
	TitleTemplateID  string   `json:"titleTemplateId,omitempty"`
	ImageTemplateID  string   `json:"imageTemplateId,omitempty"`
}

func (i OperationInput) Validate() error {
	if !collection.ValidID(i.PreparationID) || !collection.ValidID(i.StoreID) || i.ExpectedRevision < 1 || i.CategoryID < 0 || len(i.SourceIDs) > 1000 {
		return ErrInvalid
	}
	if i.Action != OperationAdapt && i.Action != OperationUpload && i.Action != OperationOptimize {
		return ErrInvalid
	}
	if i.Action != OperationOptimize && (i.TitleTemplateID != "" || i.ImageTemplateID != "") {
		return ErrInvalid
	}
	if i.Action == OperationUpload && i.CategoryID != 0 {
		return ErrInvalid
	}
	if i.Action == OperationOptimize && i.TitleTemplateID == "" && i.ImageTemplateID == "" {
		return ErrInvalid
	}
	for _, v := range []string{i.TitleTemplateID, i.ImageTemplateID} {
		if v != "" && !authidentity.IsBoundedIdentifier(v) {
			return ErrInvalid
		}
	}
	seen := map[string]bool{}
	for _, id := range i.SourceIDs {
		if !collection.ValidID(id) || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	return nil
}

type Operation struct {
	ID        string         `json:"id"`
	Owner     Scope          `json:"-"`
	Input     OperationInput `json:"input"`
	Count     int64          `json:"count"`
	Completed int64          `json:"completed"`
	Status    string         `json:"status"`
	Execution string         `json:"execution"`
	CreatedAt time.Time      `json:"createdAt"`
}
type OperationReceipt struct {
	Operation Operation `json:"operation"`
	Replayed  bool      `json:"replayed"`
}
type OperationItem struct {
	SourceID        string `json:"sourceId"`
	RecordID        string `json:"recordId,omitempty"`
	RecordRevision  int64  `json:"recordRevision,omitempty"`
	Status          string `json:"status"`
	ResultReference string `json:"resultReference,omitempty"`
	Note            string `json:"note,omitempty"`
}

func ValidItemStatus(status string) bool {
	switch status {
	case ItemPending, ItemRunning, ItemSucceeded, ItemMissing, ItemReview, ItemUnknown, ItemDenied, ItemFailed, ItemCancelled:
		return true
	}
	return false
}
func ItemTerminal(status string) bool {
	return ValidItemStatus(status) && status != ItemPending && status != ItemRunning
}
func OperationCommandID(scope Scope, key string) string {
	return collection.StableID(scope.OrganizationID, scope.ActorID, "supply-operation", key)
}
func ItemCommandID(operationID, sourceID, action string) string {
	return collection.StableID(operationID, sourceID, action)
}
func WorkflowID(org, id string) string { return "supply/" + org + "/" + id }

type OperationCommit struct {
	service *OperationService
	owner   collection.AuthorizedOwner
	scope   Scope
	key     string
	input   OperationInput
}

func (p OperationCommit) Read(ctx context.Context) (Scope, string, OperationInput, error) {
	if p.service == nil || !collection.ValidID(p.key) || p.input.Validate() != nil {
		return Scope{}, "", OperationInput{}, ErrForbidden
	}
	scope, err := p.service.authorize(ctx, p.input.Action)
	if err != nil || scope != p.scope {
		return Scope{}, "", OperationInput{}, ErrForbidden
	}
	owner, err := p.owner.Scope(ctx)
	if err != nil || owner != scope {
		return Scope{}, "", OperationInput{}, ErrForbidden
	}
	input := p.input
	input.SourceIDs = append([]string(nil), input.SourceIDs...)
	return scope, p.key, input, nil
}

type OperationRepository interface {
	FindOperation(context.Context, Scope, string, string) (OperationReceipt, error)
	PrepareOperation(context.Context, OperationCommit) (OperationReceipt, error)
	ReadOperation(context.Context, Scope, string) (Operation, error)
	ListOperationItems(context.Context, Scope, string, Query) (collection.Page[OperationItem], error)
	ReadExecutionOperation(context.Context, string, string) (Operation, error)
	CancelOperation(context.Context, OperationAccess) (Operation, error)
	MarkExecution(context.Context, OperationAccess, string) error
	BeginOperationItem(context.Context, OperationAccess, string) (OperationItem, error)
	FinishOperationItem(context.Context, OperationAccess, OperationItem) error
}

type OperationService struct {
	base       *Service
	owners     CollectionOwnerAuthority
	repository OperationRepository
	execution  collection.ExecutionAuthorizer
}

func NewOperationService(base *Service, owners CollectionOwnerAuthority, repo OperationRepository) (*OperationService, error) {
	if base == nil || owners == nil || repo == nil {
		return nil, ErrUnavailable
	}
	return &OperationService{base: base, owners: owners, repository: repo}, nil
}
func (s *OperationService) WithExecution(auth collection.ExecutionAuthorizer) (*OperationService, error) {
	if s == nil || auth == nil {
		return nil, ErrUnavailable
	}
	s.execution = auth
	return s, nil
}
func (s *OperationService) authorize(ctx context.Context, action string) (Scope, error) {
	scope, err := s.base.authorize(ctx, PermissionManage)
	if err != nil {
		return Scope{}, err
	}
	if action == OperationUpload {
		other, err := s.base.authorize(ctx, PermissionSubmit)
		if err != nil || scope != other {
			return Scope{}, ErrForbidden
		}
	}
	return scope, nil
}
func (s *OperationService) Create(ctx context.Context, key string, input OperationInput) (OperationReceipt, error) {
	if ctx == nil {
		return OperationReceipt{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, input.Action)
	if err != nil {
		return OperationReceipt{}, err
	}
	if !collection.ValidID(key) || input.Validate() != nil {
		return OperationReceipt{}, ErrInvalid
	}
	input.SourceIDs = append([]string(nil), input.SourceIDs...)
	sort.Strings(input.SourceIDs)
	prior, err := s.repository.FindOperation(ctx, scope, key, collection.Digest(input))
	if err == nil {
		return prior, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return OperationReceipt{}, err
	}
	owner, err := s.owners.AuthorizeOwner(ctx)
	if err != nil {
		return OperationReceipt{}, err
	}
	proof := OperationCommit{service: s, owner: owner, scope: scope, key: key, input: input}
	if _, _, _, err = proof.Read(ctx); err != nil {
		return OperationReceipt{}, err
	}
	return s.repository.PrepareOperation(ctx, proof)
}
func (s *OperationService) Read(ctx context.Context, id string) (Operation, error) {
	if ctx == nil || !collection.ValidID(id) {
		return Operation{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.base.authorize(ctx, PermissionRead)
	if err != nil {
		return Operation{}, err
	}
	return s.repository.ReadOperation(ctx, scope, id)
}
func (s *OperationService) ListItems(ctx context.Context, id string, q Query) (collection.Page[OperationItem], error) {
	if ctx == nil || !collection.ValidID(id) || q.Validate() != nil || q.Keyword != "" {
		return collection.Page[OperationItem]{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.base.authorize(ctx, PermissionRead)
	if err != nil {
		return collection.Page[OperationItem]{}, err
	}
	return s.repository.ListOperationItems(ctx, scope, id, q)
}

// Access is request/activity-local authority. Original identity is read from
// the durable operation; it is never reconstructed as a JWT for the worker.
type OperationAccess struct {
	service   *OperationService
	operation Operation
	worker    bool
	expiresAt time.Time
}

func (p OperationAccess) Read(ctx context.Context) (Operation, error) {
	if ctx == nil || ctx.Err() != nil || p.service == nil || !time.Now().Before(p.expiresAt) {
		return Operation{}, ErrForbidden
	}
	if _, ok := ctx.Deadline(); !ok {
		return Operation{}, ErrForbidden
	}
	if p.worker {
		if p.service.execution == nil {
			return Operation{}, ErrForbidden
		}
		for _, purpose := range []string{collection.PermissionRead, PermissionRead, PermissionManage} {
			if err := p.service.execution.AuthorizeExecution(ctx, p.operation.Owner, purpose); err != nil {
				return Operation{}, ErrForbidden
			}
		}
		if p.operation.Input.Action == OperationUpload {
			if err := p.service.execution.AuthorizeExecution(ctx, p.operation.Owner, PermissionSubmit); err != nil {
				return Operation{}, ErrForbidden
			}
		}
	} else {
		scope, err := p.service.authorize(ctx, p.operation.Input.Action)
		if err != nil || scope != p.operation.Owner {
			return Operation{}, ErrForbidden
		}
	}
	result := p.operation
	result.Input.SourceIDs = append([]string(nil), result.Input.SourceIDs...)
	return result, nil
}
func (s *OperationService) AuthorizeExecution(ctx context.Context, org, id string) (OperationAccess, error) {
	if ctx == nil || s.execution == nil || !authidentity.IsBoundedIdentifier(org) || !collection.ValidID(id) {
		return OperationAccess{}, ErrForbidden
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return OperationAccess{}, ErrForbidden
	}
	op, err := s.repository.ReadExecutionOperation(ctx, org, id)
	if err != nil {
		return OperationAccess{}, err
	}
	if op.Owner.OrganizationID != org || op.ID != id || op.Owner.Validate() != nil || op.Input.Validate() != nil {
		return OperationAccess{}, ErrUnavailable
	}
	expires := time.Now().Add(5 * time.Second)
	if deadline.Before(expires) {
		expires = deadline
	}
	proof := OperationAccess{service: s, operation: op, worker: true, expiresAt: expires}
	if _, err = proof.Read(ctx); err != nil {
		return OperationAccess{}, err
	}
	return proof, nil
}
func (s *OperationService) RequestAccess(ctx context.Context, id string) (OperationAccess, error) {
	op, err := s.Read(ctx, id)
	if err != nil {
		return OperationAccess{}, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return OperationAccess{}, ErrForbidden
	}
	expires := time.Now().Add(5 * time.Second)
	if deadline.Before(expires) {
		expires = deadline
	}
	proof := OperationAccess{service: s, operation: op, expiresAt: expires}
	if _, err = proof.Read(ctx); err != nil {
		return OperationAccess{}, err
	}
	return proof, nil
}
func (s *OperationService) Cancel(ctx context.Context, id string) (Operation, error) {
	if ctx == nil {
		return Operation{}, ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	proof, err := s.RequestAccess(ctx, id)
	if err != nil {
		return Operation{}, err
	}
	return s.repository.CancelOperation(ctx, proof)
}
