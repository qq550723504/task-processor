// Package agentconfigpersistence implements the configuration owner in RunDB.
package agentconfigpersistence

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"strconv"
	"strings"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"time"
)

//go:embed schema.sql
var schemaSQL string

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) (*Store, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, agentconfig.ErrUnavailable
	}
	return &Store{db: db}, nil
}

// InstallSchema is an explicit fresh-install operation, never a constructor or
// serving side effect. Schema changes and runtime credentials stay separated.
func InstallSchema(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return agentconfig.ErrUnavailable
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// Only this explicit installation path provisions the frozen non-login
		// owner. It refuses ownership conversion of an existing foreign schema.
		if e := tx.Exec(`DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='agent_configuration_owner') THEN CREATE ROLE agent_configuration_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS; END IF; END $$;`).Error; e != nil {
			return e
		}
		var unsafe bool
		if e := tx.Raw(`SELECT rolcanlogin OR rolsuper OR rolcreaterole OR rolcreatedb OR rolbypassrls FROM pg_roles WHERE rolname='agent_configuration_owner'`).Scan(&unsafe).Error; e != nil {
			return e
		}
		if unsafe {
			return agentconfig.ErrInvalid
		}
		if e := tx.Exec(`CREATE SCHEMA IF NOT EXISTS agent_configuration AUTHORIZATION agent_configuration_owner`).Error; e != nil {
			return e
		}
		if e := tx.Raw(`SELECT nspowner <> (SELECT oid FROM pg_roles WHERE rolname='agent_configuration_owner') FROM pg_namespace WHERE nspname='agent_configuration'`).Scan(&unsafe).Error; e != nil {
			return e
		}
		if unsafe {
			return agentconfig.ErrInvalid
		}
		if e := tx.Exec("SET LOCAL ROLE agent_configuration_owner").Error; e != nil {
			return e
		}
		if e := tx.Exec(schemaSQL).Error; e != nil {
			return e
		}
		return tx.Exec("RESET ROLE").Error
	})
}
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return agentconfig.ErrUnavailable
	}
	for _, name := range []string{"organization_agents", "templates", "template_revisions", "start_snapshots", "commands"} {
		if err := db.WithContext(ctx).Table("agent_configuration." + name).Select("1").Limit(0).Find(&[]int{}).Error; err != nil {
			return agentconfig.ErrUnavailable
		}
	}
	return nil
}

type agentRow struct {
	OrganizationID, AgentID, Activation string
	ActivationEpoch, Revision           int64
	DefaultTemplateID                   *string
	DefaultTemplateRevision             *int64
	CreatedBy, UpdatedBy                string
	CreatedAt, UpdatedAt                time.Time
}

func (agentRow) TableName() string { return "agent_configuration.organization_agents" }

type templateRow struct {
	OrganizationID, AgentID, TemplateID, Lifecycle string
	HeadRevision, Revision                         int64
	CreatedBy, UpdatedBy                           string
	CreatedAt, UpdatedAt                           time.Time
}

func (templateRow) TableName() string { return "agent_configuration.templates" }

type revisionRow struct {
	OrganizationID, AgentID, TemplateID string
	Version                             int64
	Name, SchemaVersion, TargetPlatform string
	DefaultKnowledgeBaseID              *string
	CreatedBy                           string
	CreatedAt                           time.Time
}

func (revisionRow) TableName() string { return "agent_configuration.template_revisions" }

type commandRow struct {
	OrganizationID, ActorID, IdempotencyKey, CommandID, Operation, AgentID string
	TemplateID                                                             *string
	Fingerprint                                                            string
	BeforeRevision, AfterRevision                                          int64
	Receipt                                                                []byte
	CommittedAt                                                            time.Time
}

func (commandRow) TableName() string { return "agent_configuration.commands" }

type snapshotRow struct {
	ID, OrganizationID, ActorID, ContextKind, ContextID, RequestKey, AgentID, AgentVersion, Fingerprint, Digest string
	Payload                                                                                                     []byte
	ActivationEpoch, AgentRevision                                                                              int64
	TemplateID                                                                                                  *string
	TemplateRevision                                                                                            *int64
	CreatedAt                                                                                                   time.Time
}

func (snapshotRow) TableName() string { return "agent_configuration.start_snapshots" }
func decimal(v int64) string          { return strconv.FormatInt(v, 10) }
func version(v string) (int64, error) {
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n <= 0 || decimal(n) != v {
		return 0, agentconfig.ErrInvalid
	}
	return n, nil
}
func scopeOK(s agent.Scope) bool { return agent.ValidID(s.OrganizationID) && agent.ValidID(s.ActorID) }
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func hash(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", agentconfig.ErrInvalid
	}
	return digest(b), nil
}
func agentView(r agentRow) agentconfig.OrganizationAgent {
	v := agentconfig.OrganizationAgent{AgentID: r.AgentID, Activation: r.Activation, Revision: decimal(r.Revision), ActivationEpoch: decimal(r.ActivationEpoch), UpdatedAt: r.UpdatedAt}
	if r.DefaultTemplateID != nil && r.DefaultTemplateRevision != nil {
		v.DefaultTemplate = &agentconfig.TemplateRef{TemplateID: *r.DefaultTemplateID, Revision: decimal(*r.DefaultTemplateRevision)}
	}
	return v
}
func findAgent(db *gorm.DB, org, id string, lock bool) (agentRow, error) {
	var r agentRow
	q := db.Where("organization_id=? AND agent_id=?", org, id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	e := q.Take(&r).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = agentconfig.ErrNotFound
	}
	return r, e
}
func findTemplate(db *gorm.DB, org, agentID, id string, lock bool) (templateRow, error) {
	var r templateRow
	q := db.Where("organization_id=? AND agent_id=? AND template_id=?", org, agentID, id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	e := q.Take(&r).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = agentconfig.ErrNotFound
	}
	return r, e
}
func templateView(db *gorm.DB, r templateRow, v int64) (agentconfig.Template, error) {
	if v == 0 {
		v = r.HeadRevision
	}
	var p revisionRow
	e := db.Where("organization_id=? AND agent_id=? AND template_id=? AND version=?", r.OrganizationID, r.AgentID, r.TemplateID, v).Take(&p).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = agentconfig.ErrNotFound
	}
	return agentconfig.Template{TemplateID: r.TemplateID, AgentID: r.AgentID, Lifecycle: r.Lifecycle, Revision: decimal(r.Revision), Version: decimal(p.Version), SchemaVersion: p.SchemaVersion, TemplateInput: agentconfig.TemplateInput{Name: p.Name, TargetPlatform: p.TargetPlatform, DefaultKnowledgeBaseID: text(p.DefaultKnowledgeBaseID)}, CreatedAt: r.CreatedAt}, e
}
func (s *Store) ReadAgent(ctx context.Context, scope agent.Scope, id string) (agentconfig.OrganizationAgent, error) {
	if !scopeOK(scope) || !agent.ValidID(id) {
		return agentconfig.OrganizationAgent{}, agentconfig.ErrInvalid
	}
	r, e := findAgent(s.db.WithContext(ctx), scope.OrganizationID, id, false)
	return agentView(r), e
}
func (s *Store) ReadTemplate(ctx context.Context, scope agent.Scope, agentID, id string, v uint64) (agentconfig.Template, error) {
	if !scopeOK(scope) || !agentconfig.UUID(id) || v > math.MaxInt64 {
		return agentconfig.Template{}, agentconfig.ErrInvalid
	}
	r, e := findTemplate(s.db.WithContext(ctx), scope.OrganizationID, agentID, id, false)
	if e != nil {
		return agentconfig.Template{}, e
	}
	return templateView(s.db.WithContext(ctx), r, int64(v))
}
func (s *Store) ListAgents(ctx context.Context, scope agent.Scope, cursor, activation string, size int) ([]agentconfig.OrganizationAgent, string, error) {
	if !scopeOK(scope) || size < 1 || size > 100 || (cursor != "" && !agent.ValidID(cursor)) || (activation != "" && activation != "ENABLED" && activation != "DISABLED") {
		return nil, "", agentconfig.ErrInvalid
	}
	q := s.db.WithContext(ctx).Where("organization_id=? AND agent_id>?", scope.OrganizationID, cursor)
	if activation != "" {
		q = q.Where("activation=?", activation)
	}
	var rows []agentRow
	if e := q.Order("agent_id").Limit(size + 1).Find(&rows).Error; e != nil {
		return nil, "", e
	}
	next := ""
	if len(rows) > size {
		rows = rows[:size]
		next = rows[size-1].AgentID
	}
	out := []agentconfig.OrganizationAgent{}
	for _, r := range rows {
		out = append(out, agentView(r))
	}
	return out, next, nil
}
func (s *Store) Templates(ctx context.Context, scope agent.Scope, id, cursor, lifecycle string, size int) ([]agentconfig.Template, string, error) {
	if !scopeOK(scope) || size < 1 || size > 100 || (cursor != "" && !agentconfig.UUID(cursor)) || (lifecycle != "" && lifecycle != "ACTIVE" && lifecycle != "ARCHIVED") {
		return nil, "", agentconfig.ErrInvalid
	}
	q := s.db.WithContext(ctx).Where("organization_id=? AND agent_id=?", scope.OrganizationID, id)
	if cursor != "" {
		q = q.Where("template_id>?", cursor)
	}
	if lifecycle != "" {
		q = q.Where("lifecycle=?", lifecycle)
	}
	var rows []templateRow
	if e := q.Order("template_id").Limit(size + 1).Find(&rows).Error; e != nil {
		return nil, "", e
	}
	next := ""
	if len(rows) > size {
		rows = rows[:size]
		next = rows[size-1].TemplateID
	}
	out := []agentconfig.Template{}
	for _, r := range rows {
		v, e := templateView(s.db.WithContext(ctx), r, 0)
		if e != nil {
			return nil, "", e
		}
		out = append(out, v)
	}
	return out, next, nil
}

func (s *Store) Execute(ctx context.Context, c agentconfig.Command, eligibility ...func(context.Context) error) (agentconfig.Receipt, error) {
	if len(eligibility) > 1 {
		return agentconfig.Receipt{}, agentconfig.ErrInvalid
	}
	if !scopeOK(c.Scope) || !agentconfig.UUID(c.Key) || !agent.ValidID(c.AgentID) || c.Expected >= math.MaxInt64 {
		return agentconfig.Receipt{}, agentconfig.ErrInvalid
	}
	if (c.Operation == "create-template" || c.Operation == "update-template") && !c.Input.Valid() {
		return agentconfig.Receipt{}, agentconfig.ErrInvalid
	}
	if (c.Operation == "update-template" || c.Operation == "archive-template") && !agentconfig.UUID(c.TemplateID) {
		return agentconfig.Receipt{}, agentconfig.ErrInvalid
	}
	if c.Absent && (c.Operation != "enable" || c.Expected != 0) {
		return agentconfig.Receipt{}, agentconfig.ErrInvalid
	}
	if c.Operation != "create-template" && !c.Absent && c.Expected == 0 {
		return agentconfig.Receipt{}, agentconfig.ErrPrecondition
	}
	if c.Default != nil {
		if !agentconfig.UUID(c.Default.TemplateID) {
			return agentconfig.Receipt{}, agentconfig.ErrInvalid
		}
		if _, e := version(c.Default.Revision); e != nil {
			return agentconfig.Receipt{}, e
		}
	}
	c.Input.Name = strings.TrimSpace(c.Input.Name)
	fp, e := hash(c)
	if e != nil {
		return agentconfig.Receipt{}, e
	}
	var result agentconfig.Receipt
	e = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		claim := commandRow{OrganizationID: c.Scope.OrganizationID, ActorID: c.Scope.ActorID, IdempotencyKey: c.Key, CommandID: uuid.NewString(), Operation: c.Operation, AgentID: c.AgentID, TemplateID: nullable(c.TemplateID), Fingerprint: fp, Receipt: []byte("{}"), CommittedAt: now}
		write := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
		if write.Error != nil {
			return write.Error
		}
		if write.RowsAffected == 0 {
			var old commandRow
			if e := tx.Where("organization_id=? AND actor_id=? AND idempotency_key=?", c.Scope.OrganizationID, c.Scope.ActorID, c.Key).Take(&old).Error; e != nil {
				return e
			}
			if old.Fingerprint != fp {
				return agentconfig.ErrConflict
			}
			if json.Unmarshal(old.Receipt, &result) != nil || result.CommandID == "" {
				return agentconfig.ErrUnavailable
			}
			return nil
		}
		// Committed receipts precede mutable Knowledge eligibility and CAS. The
		// idempotency insert also waits for a concurrent winner's transaction.
		if len(eligibility) == 1 && eligibility[0] != nil {
			if e := eligibility[0](ctx); e != nil {
				return e
			}
		}
		a, e := findAgent(tx, c.Scope.OrganizationID, c.AgentID, true)
		if errors.Is(e, agentconfig.ErrNotFound) && c.Operation == "enable" && c.Absent {
			a = agentRow{OrganizationID: c.Scope.OrganizationID, AgentID: c.AgentID, Activation: "ENABLED", Revision: 1, ActivationEpoch: 1, CreatedBy: c.Scope.ActorID, UpdatedBy: c.Scope.ActorID, CreatedAt: now, UpdatedAt: now}
			insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&a)
			if insert.Error != nil {
				return insert.Error
			}
			if insert.RowsAffected == 0 {
				return agentconfig.ErrRevision
			}
			result = agentconfig.Receipt{CommandID: claim.CommandID, Operation: c.Operation, AgentID: c.AgentID, Revision: "1", CommittedAt: now}
		} else {
			if e != nil {
				return e
			}
			if a.Revision >= math.MaxInt64 || a.ActivationEpoch >= math.MaxInt64 {
				return agentconfig.ErrChanged
			}
			if c.Operation != "create-template" && c.Operation != "update-template" && c.Operation != "archive-template" && (c.Absent || int64(c.Expected) != a.Revision) {
				return agentconfig.ErrRevision
			}
			result = agentconfig.Receipt{CommandID: claim.CommandID, Operation: c.Operation, AgentID: c.AgentID, BeforeRevision: decimal(a.Revision), Revision: decimal(a.Revision), CommittedAt: now}
			switch c.Operation {
			case "enable", "disable":
				state := "ENABLED"
				if c.Operation == "disable" {
					state = "DISABLED"
				}
				result.Noop = a.Activation == state
				if !result.Noop {
					a.Activation = state
					a.ActivationEpoch++
					a.Revision++
				}
			case "default":
				if c.Default == nil {
					result.Noop = a.DefaultTemplateID == nil
					a.DefaultTemplateID = nil
					a.DefaultTemplateRevision = nil
				} else {
					t, e := findTemplate(tx, c.Scope.OrganizationID, c.AgentID, c.Default.TemplateID, true)
					if e != nil {
						return e
					}
					if t.Lifecycle != "ACTIVE" {
						return agentconfig.ErrArchived
					}
					v, _ := version(c.Default.Revision)
					if _, e := templateView(tx, t, v); e != nil {
						return e
					}
					result.Noop = a.DefaultTemplateID != nil && *a.DefaultTemplateID == t.TemplateID && a.DefaultTemplateRevision != nil && *a.DefaultTemplateRevision == v
					a.DefaultTemplateID = &t.TemplateID
					a.DefaultTemplateRevision = &v
				}
				if !result.Noop {
					a.Revision++
				}
			case "create-template", "update-template", "archive-template":
				t := templateRow{OrganizationID: c.Scope.OrganizationID, AgentID: c.AgentID, TemplateID: uuid.NewString(), Lifecycle: "ACTIVE", HeadRevision: 1, Revision: 1, CreatedBy: c.Scope.ActorID, UpdatedBy: c.Scope.ActorID, CreatedAt: now, UpdatedAt: now}
				if c.Operation == "create-template" {
					if e := tx.Create(&t).Error; e != nil {
						return e
					}
					result.BeforeRevision = ""
				} else {
					t, e = findTemplate(tx, c.Scope.OrganizationID, c.AgentID, c.TemplateID, true)
					if e != nil {
						return e
					}
					if int64(c.Expected) != t.Revision {
						return agentconfig.ErrRevision
					}
					if t.Revision >= math.MaxInt64 || t.HeadRevision >= math.MaxInt64 {
						return agentconfig.ErrChanged
					}
					result.BeforeRevision = decimal(t.Revision)
					if c.Operation == "archive-template" {
						if a.DefaultTemplateID != nil && *a.DefaultTemplateID == t.TemplateID {
							return agentconfig.ErrDefault
						}
						result.Noop = t.Lifecycle == "ARCHIVED"
						if !result.Noop {
							t.Lifecycle = "ARCHIVED"
							t.Revision++
						}
					} else {
						if t.Lifecycle != "ACTIVE" {
							return agentconfig.ErrArchived
						}
						t.Revision++
						t.HeadRevision++
					}
					if !result.Noop {
						t.UpdatedBy = c.Scope.ActorID
						t.UpdatedAt = now
						if e := tx.Model(&templateRow{}).Where("organization_id=? AND agent_id=? AND template_id=?", t.OrganizationID, t.AgentID, t.TemplateID).Updates(map[string]any{"lifecycle": t.Lifecycle, "revision": t.Revision, "head_revision": t.HeadRevision, "updated_by": t.UpdatedBy, "updated_at": now}).Error; e != nil {
							return e
						}
					}
				}
				if c.Operation != "archive-template" {
					r := revisionRow{OrganizationID: t.OrganizationID, AgentID: t.AgentID, TemplateID: t.TemplateID, Version: t.HeadRevision, Name: c.Input.Name, SchemaVersion: agentconfig.ParameterSchema, TargetPlatform: c.Input.TargetPlatform, DefaultKnowledgeBaseID: nullable(c.Input.DefaultKnowledgeBaseID), CreatedBy: c.Scope.ActorID, CreatedAt: now}
					if e := tx.Create(&r).Error; e != nil {
						return e
					}
				}
				result.TemplateID = t.TemplateID
				result.Revision = decimal(t.Revision)
				result.Version = decimal(t.HeadRevision)
			default:
				return agentconfig.ErrInvalid
			}
			if c.Operation == "enable" || c.Operation == "disable" || c.Operation == "default" {
				a.UpdatedBy = c.Scope.ActorID
				a.UpdatedAt = now
				if !result.Noop {
					if e := tx.Model(&agentRow{}).Where("organization_id=? AND agent_id=?", a.OrganizationID, a.AgentID).Updates(map[string]any{"activation": a.Activation, "activation_epoch": a.ActivationEpoch, "revision": a.Revision, "default_template_id": a.DefaultTemplateID, "default_template_revision": a.DefaultTemplateRevision, "updated_by": a.UpdatedBy, "updated_at": now}).Error; e != nil {
						return e
					}
				}
				result.Revision = decimal(a.Revision)
			}
		}
		raw, e := json.Marshal(result)
		if e != nil || len(raw) > 4096 {
			return agentconfig.ErrUnavailable
		}
		before, _ := strconv.ParseInt(result.BeforeRevision, 10, 64)
		after, _ := version(result.Revision)
		return tx.Model(&commandRow{}).Where("command_id=?", claim.CommandID).Updates(map[string]any{"receipt": raw, "before_revision": before, "after_revision": after, "template_id": nullable(result.TemplateID)}).Error
	})
	return result, e
}

var _ agentconfig.Repository = (*Store)(nil)
