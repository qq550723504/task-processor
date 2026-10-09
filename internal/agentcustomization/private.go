package agentcustomization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"task-processor/internal/product/quality"
	"time"
)

const QualityDefinition = "product.quality.check"
const QualityVersion = "1.0.0"

type Delivery struct {
	ID             string    `json:"id"`
	RequestID      string    `json:"requestId"`
	OrganizationID string    `json:"organizationId"`
	Definition     string    `json:"definition"`
	Version        string    `json:"version"`
	Name           string    `json:"name"`
	CreatedBy      string    `json:"createdBy"`
	CreatedAt      time.Time `json:"createdAt"`
}
type DeliveryPage struct {
	Items      []Delivery `json:"items"`
	NextCursor string     `json:"nextCursor"`
}
type QualityRun struct {
	ID             string         `json:"id"`
	DeliveryID     string         `json:"deliveryId"`
	OrganizationID string         `json:"organizationId"`
	ActorID        string         `json:"actorId"`
	Key            string         `json:"key"`
	Definition     string         `json:"definition"`
	Version        string         `json:"version"`
	Input          quality.Input  `json:"input"`
	Report         quality.Report `json:"report"`
	CreatedAt      time.Time      `json:"createdAt"`
}
type QualityRunPage struct {
	Items      []QualityRun `json:"items"`
	NextCursor string       `json:"nextCursor"`
}
type RunCommand struct {
	Scope                        Scope
	Key, DeliveryID, Fingerprint string
	Input                        quality.Input
}
type PrivateRepository interface {
	Deliveries(context.Context, Scope, string) (DeliveryPage, error)
	Delivery(context.Context, Scope, string) (Delivery, error)
	RunQuality(context.Context, RunCommand) (QualityRun, error)
	QualityRuns(context.Context, Scope, string, string) (QualityRunPage, error)
}

func privateScope(s Scope) bool { return ValidScope(s) && !s.Platform }
func (s *Service) private() (PrivateRepository, error) {
	r, ok := s.repo.(PrivateRepository)
	if !ok {
		return nil, ErrUnavailable
	}
	return r, nil
}
func (s *Service) Deliveries(ctx context.Context, scope Scope, cursor string) (DeliveryPage, error) {
	if !privateScope(scope) {
		return DeliveryPage{}, ErrForbidden
	}
	if cursor != "" && !UUID(cursor) {
		return DeliveryPage{}, ErrInvalid
	}
	r, e := s.private()
	if e != nil {
		return DeliveryPage{}, e
	}
	return r.Deliveries(ctx, scope, cursor)
}
func (s *Service) Delivery(ctx context.Context, scope Scope, id string) (Delivery, error) {
	if !privateScope(scope) {
		return Delivery{}, ErrForbidden
	}
	if !UUID(id) {
		return Delivery{}, ErrInvalid
	}
	r, e := s.private()
	if e != nil {
		return Delivery{}, e
	}
	return r.Delivery(ctx, scope, id)
}
func (s *Service) RunQuality(ctx context.Context, c RunCommand) (QualityRun, error) {
	if !privateScope(c.Scope) {
		return QualityRun{}, ErrForbidden
	}
	if !UUID(c.Key) || !UUID(c.DeliveryID) {
		return QualityRun{}, ErrInvalid
	}
	if c.Input.Specifications == nil {
		c.Input.Specifications = []quality.Specification{}
	}
	if _, e := quality.Check(c.Input); e != nil {
		return QualityRun{}, ErrInvalid
	}
	c.Fingerprint = ""
	raw, e := json.Marshal(struct {
		Command             RunCommand
		Definition, Version string
	}{c, QualityDefinition, QualityVersion})
	if e != nil {
		return QualityRun{}, ErrInvalid
	}
	digest := sha256.Sum256(raw)
	c.Fingerprint = hex.EncodeToString(digest[:])
	r, e := s.private()
	if e != nil {
		return QualityRun{}, e
	}
	return r.RunQuality(ctx, c)
}
func (s *Service) QualityRuns(ctx context.Context, scope Scope, id, cursor string) (QualityRunPage, error) {
	if !privateScope(scope) {
		return QualityRunPage{}, ErrForbidden
	}
	if !UUID(id) || (cursor != "" && !UUID(cursor)) {
		return QualityRunPage{}, ErrInvalid
	}
	r, e := s.private()
	if e != nil {
		return QualityRunPage{}, e
	}
	return r.QualityRuns(ctx, scope, id, cursor)
}
