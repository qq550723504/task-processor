package agentcustomization

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const QualityDefinition = "product.quality.check"
const QualityVersion = "2.0.0"

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
type DraftSelection struct {
	RecordID         string `json:"recordId"`
	ExpectedRevision int64  `json:"expectedRevision"`
}
type DraftBinding struct {
	OfflineTrial   bool      `json:"offlineTrial,omitempty"`
	RecordID       string    `json:"recordId"`
	Revision       int64     `json:"revision"`
	SourceID       string    `json:"sourceId"`
	PreparationID  string    `json:"preparationId"`
	StoreID        string    `json:"storeId"`
	Platform       string    `json:"platform"`
	Site           string    `json:"site"`
	ProductKey     string    `json:"productKey"`
	ProductVersion string    `json:"productVersion"`
	Title          string    `json:"title"`
	RecordHash     string    `json:"recordHash"`
	ProductHash    string    `json:"productHash"`
	RulesHash      string    `json:"rulesHash"`
	InventoryHash  string    `json:"inventoryHash"`
	SavedAt        time.Time `json:"savedAt"`
}
type DraftIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type DraftSnapshot struct {
	DraftBinding
	Issues         []DraftIssue `json:"issues"`
	ReadyForUpload bool         `json:"readyForUpload"`
}
type DraftInspector interface {
	Inspect(context.Context, Scope, DraftSelection, bool) (DraftSnapshot, error)
}
type DraftInspection func(context.Context, bool) (DraftSnapshot, error)

// Immutable local trial history only; there is no manual executor.
type RecordedInput struct {
	Name           string `json:"name"`
	Material       string `json:"material"`
	Dimensions     string `json:"dimensions"`
	Description    string `json:"description"`
	Specifications []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"specifications"`
}
type RecordedReport struct {
	RuleVersion string `json:"ruleVersion"`
	Summary     string `json:"summary"`
	Findings    []struct {
		Code       string `json:"code"`
		Field      string `json:"field"`
		Message    string `json:"message"`
		Suggestion string `json:"suggestion"`
	} `json:"findings"`
}
type QualityRun struct {
	ID             string          `json:"id"`
	DeliveryID     string          `json:"deliveryId"`
	OrganizationID string          `json:"organizationId"`
	ActorID        string          `json:"actorId"`
	Key            string          `json:"key"`
	Definition     string          `json:"definition"`
	Version        string          `json:"version"`
	Input          *RecordedInput  `json:"input,omitempty"`
	Report         *RecordedReport `json:"report,omitempty"`
	Draft          *DraftSnapshot  `json:"draft,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}
type QualitySummary struct {
	ID             string        `json:"id"`
	DeliveryID     string        `json:"deliveryId"`
	OrganizationID string        `json:"organizationId"`
	ActorID        string        `json:"actorId"`
	Version        string        `json:"version"`
	Title          string        `json:"title"`
	Draft          *DraftBinding `json:"draft,omitempty"`
	FindingCount   int           `json:"findingCount"`
	CreatedAt      time.Time     `json:"createdAt"`
}
type QualityRunPage struct {
	Items      []QualitySummary `json:"items"`
	NextCursor string           `json:"nextCursor"`
}
type SavedQualityPage struct {
	Items      []QualityRun
	NextCursor string
}
type RunCommand struct {
	Scope                        Scope
	Key, DeliveryID, Fingerprint string
	Input                        DraftSelection
}
type PrivateRepository interface {
	Deliveries(context.Context, Scope, string) (DeliveryPage, error)
	Delivery(context.Context, Scope, string) (Delivery, error)
	RunQuality(context.Context, RunCommand, DraftInspection) (QualityRun, error)
	QualityRuns(context.Context, Scope, string, string) (SavedQualityPage, error)
	QualityRun(context.Context, Scope, string, string) (QualityRun, error)
}

func (s *Service) WithDrafts(reader DraftInspector) *Service { s.drafts = reader; return s }
func privateScope(s Scope) bool                              { return ValidScope(s) && !s.Platform }
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
	if !UUID(c.Key) || !UUID(c.DeliveryID) || !UUID(c.Input.RecordID) || c.Input.ExpectedRevision < 1 || c.Input.ExpectedRevision > 9007199254740991 {
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
	return r.RunQuality(ctx, c, func(ctx context.Context, head bool) (DraftSnapshot, error) {
		if s.drafts == nil {
			return DraftSnapshot{}, ErrUnavailable
		}
		v, e := s.drafts.Inspect(ctx, c.Scope, c.Input, head)
		if e == nil && (!ValidDraft(v) || v.RecordID != c.Input.RecordID || v.Revision != c.Input.ExpectedRevision) {
			return DraftSnapshot{}, ErrUnavailable
		}
		return v, e
	})
}
func ValidDraft(v DraftSnapshot) bool {
	if !UUID(v.RecordID) || !UUID(v.SourceID) || !UUID(v.PreparationID) || !UUID(v.StoreID) || v.Revision < 1 || v.Revision > 9007199254740991 || v.Platform != "shein" || v.Site != "shein-us" || v.ProductKey == "" || len(v.ProductKey) > 128 || !version(v.ProductVersion) || len([]rune(v.Title)) > 200 || v.SavedAt.IsZero() || len(v.Issues) > 4096 || v.Issues == nil || v.ReadyForUpload && len(v.Issues) != 0 {
		return false
	}
	for _, h := range []string{v.RecordHash, v.ProductHash, v.RulesHash, v.InventoryHash} {
		if len(h) != 64 {
			return false
		}
		if _, e := hex.DecodeString(h); e != nil {
			return false
		}
	}
	for _, i := range v.Issues {
		if i.Code == "" || len(i.Code) > 128 || len(i.Field) > 8192 || i.Message == "" || len(i.Message) > 8192 {
			return false
		}
	}
	raw, e := json.Marshal(v)
	return e == nil && len(raw) <= 2<<20
}
func version(v string) bool {
	if v == "" || v[0] == '0' || len(v) > 19 {
		return false
	}
	for _, c := range v {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(v) < 19 || v <= "9223372036854775807"
}
func (s *Service) authorizeRun(ctx context.Context, scope Scope, run QualityRun) error {
	if run.OrganizationID != scope.OrganizationID || run.ActorID != scope.ActorID {
		return ErrForbidden
	}
	if run.Version == "1.0.0" && run.Input != nil && run.Report != nil && run.Draft == nil {
		return nil
	}
	if run.Version != QualityVersion || run.Draft == nil || !ValidDraft(*run.Draft) || s.drafts == nil {
		return ErrUnavailable
	}
	v, e := s.drafts.Inspect(ctx, scope, DraftSelection{run.Draft.RecordID, run.Draft.Revision}, false)
	if e != nil {
		return e
	}
	if !ValidDraft(v) || v.DraftBinding != run.Draft.DraftBinding {
		return ErrUnavailable
	}
	return nil
}
func (s *Service) QualityRun(ctx context.Context, scope Scope, delivery, id string) (QualityRun, error) {
	if !privateScope(scope) {
		return QualityRun{}, ErrForbidden
	}
	if !UUID(delivery) || !UUID(id) {
		return QualityRun{}, ErrInvalid
	}
	r, e := s.private()
	if e != nil {
		return QualityRun{}, e
	}
	v, e := r.QualityRun(ctx, scope, delivery, id)
	if e != nil {
		return QualityRun{}, e
	}
	if e = s.authorizeRun(ctx, scope, v); e != nil {
		return QualityRun{}, e
	}
	return v, nil
}
func (s *Service) QualityRuns(ctx context.Context, scope Scope, id, cursor string) (QualityRunPage, error) {
	out := QualityRunPage{Items: []QualitySummary{}}
	if !privateScope(scope) {
		return out, ErrForbidden
	}
	if !UUID(id) || (cursor != "" && !UUID(cursor)) {
		return out, ErrInvalid
	}
	r, e := s.private()
	if e != nil {
		return out, e
	}
	page, e := r.QualityRuns(ctx, scope, id, cursor)
	if e != nil {
		return out, e
	}
	for _, run := range page.Items {
		if e = s.authorizeRun(ctx, scope, run); e != nil {
			return QualityRunPage{}, e
		}
		v := QualitySummary{ID: run.ID, DeliveryID: run.DeliveryID, OrganizationID: run.OrganizationID, ActorID: run.ActorID, Version: run.Version, CreatedAt: run.CreatedAt}
		if run.Draft != nil {
			v.Title = run.Draft.Title
			binding := run.Draft.DraftBinding
			v.Draft = &binding
			v.FindingCount = len(run.Draft.Issues)
		} else {
			v.Title = run.Input.Name
			v.FindingCount = len(run.Report.Findings)
		}
		out.Items = append(out.Items, v)
	}
	out.NextCursor = page.NextCursor
	return out, nil
}
