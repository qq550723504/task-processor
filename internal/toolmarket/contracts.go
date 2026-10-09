// Package toolmarket owns enterprise tool preferences and human customization
// requests. Acquisition, authorization, billing and Agent execution retain their owners.
package toolmarket

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"task-processor/internal/authidentity"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalid      = errors.New("INVALID_REQUEST")
	ErrForbidden    = errors.New("FORBIDDEN")
	ErrNotFound     = errors.New("NOT_FOUND")
	ErrConflict     = errors.New("IDEMPOTENCY_CONFLICT")
	ErrRevision     = errors.New("REVISION_MISMATCH")
	ErrPrecondition = errors.New("PRECONDITION_REQUIRED")
	ErrUnavailable  = errors.New("DEPENDENCY_UNAVAILABLE")
)

const AcquisitionID = "product-acquisition"

type Scope struct{ OrganizationID, ActorID string }

func (s Scope) Valid(platform bool) bool {
	return authidentity.IsBoundedIdentifier(s.ActorID) && (platform || authidentity.IsBoundedIdentifier(s.OrganizationID))
}
func UUID(s string) bool {
	id, e := uuid.Parse(s)
	return e == nil && id != uuid.Nil && id.String() == s
}
func bounded(s string, n int, multiline bool) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= n && strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\t' || r == '\r'))
	}) < 0
}

type Demand struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (d Demand) Valid() bool {
	return (d.Kind == "DATA" || d.Kind == "CONNECTION" || d.Kind == "AUTOMATION" || d.Kind == "OUTPUT") && bounded(d.Title, 120, false) && bounded(d.Description, 4000, true)
}

type Progress struct {
	Stage string `json:"stage"`
	Note  string `json:"note"`
}

func (p Progress) Valid() bool { return knownStage(p.Stage) && bounded(p.Note, 2000, true) }

var stages = []string{"SUBMITTED", "EVALUATING", "PLAN_CONFIRMED", "DEVELOPING", "DELIVERED", "CLOSED"}

func knownStage(s string) bool {
	for _, v := range stages {
		if s == v {
			return true
		}
	}
	return false
}
func Transition(from, to string) bool {
	if !knownStage(from) || !knownStage(to) || from == "DELIVERED" || from == "CLOSED" {
		return false
	}
	if from == to || to == "CLOSED" {
		return true
	}
	for i := 0; i < 4; i++ {
		if stages[i] == from && stages[i+1] == to {
			return true
		}
	}
	return false
}

type Activation struct {
	ToolID    string    `json:"toolId"`
	Enabled   bool      `json:"enabled"`
	Revision  string    `json:"revision"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Request struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Demand
	Stage     string    `json:"stage"`
	Revision  string    `json:"revision"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Event struct {
	Revision   string    `json:"revision"`
	Stage      string    `json:"stage"`
	Note       string    `json:"note"`
	OccurredAt time.Time `json:"occurredAt"`
}
type Detail struct {
	Request Request `json:"request"`
	Events  []Event `json:"events"`
}
type RequestPage struct {
	Items      []RequestSummary `json:"items"`
	NextCursor string           `json:"nextCursor"`
}
type RequestSummary struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Kind           string    `json:"kind"`
	Title          string    `json:"title"`
	Stage          string    `json:"stage"`
	Revision       string    `json:"revision"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type Command struct {
	Scope                     Scope
	Key, Operation, ID        string
	Expected                  int64
	Absent, Platform, Enabled bool
	Demand                    Demand
	Progress                  Progress
}

func (c Command) Valid() bool {
	if !c.Scope.Valid(c.Platform) || !UUID(c.Key) || c.Expected < 0 {
		return false
	}
	switch c.Operation {
	case "activation":
		return !c.Platform && c.ID == AcquisitionID && ((c.Absent && c.Expected == 0 && c.Enabled) || (!c.Absent && c.Expected > 0))
	case "create":
		return !c.Platform && c.ID == "" && c.Absent && c.Expected == 0 && c.Demand.Valid()
	case "progress":
		return c.Platform && UUID(c.ID) && !c.Absent && c.Expected > 0 && c.Progress.Valid()
	}
	return false
}

type Receipt struct {
	CommandID   string    `json:"commandId"`
	Operation   string    `json:"operation"`
	ID          string    `json:"id"`
	Revision    string    `json:"revision"`
	CommittedAt time.Time `json:"committedAt"`
}
type Repository interface {
	Activations(context.Context, Scope) ([]Activation, error)
	Requests(context.Context, Scope, bool, string, int) (RequestPage, error)
	Detail(context.Context, Scope, bool, string) (Detail, error)
	Execute(context.Context, Command, func(context.Context) error) (Receipt, error)
}
type Tool struct {
	ID            string      `json:"id"`
	Version       string      `json:"version"`
	Name          string      `json:"name"`
	Category      string      `json:"category"`
	Description   string      `json:"description"`
	Status        string      `json:"status"`
	Reason        string      `json:"reason"`
	Activation    *Activation `json:"activation"`
	LocalCapture  bool        `json:"localCapture"`
	OnlineCapture bool        `json:"onlineCapture"`
	Download      bool        `json:"download"`
}

// Readiness is supplied by the serving installation, not by enterprise choices.
type Readiness struct{ LocalCapture, OnlineCapture, Download bool }

func Catalog(r Readiness, activations []Activation, mine bool) []Tool {
	result := []Tool{}
	defs := [][4]string{{AcquisitionID, "商品采集插件", "采集", "采集1688商品标题、主图、规格与价格等基础数据"}, {"fingerprint-browser", "指纹浏览器", "环境", "多账号独立环境与指纹隔离"}, {"image-processing", "图片处理工具", "图片", "图片裁切、抠图与批量处理"}, {"data-import", "数据导入工具", "数据", "导入业务资料并整理为标准数据"}, {"automation", "自动化工具", "流程", "自动执行重复业务流程"}, {"bulk-listing", "批量刊登工具", "流程", "将商品资料批量发布到平台"}, {"order-processing", "订单处理工具", "数据", "订单汇总与处理"}, {"stock-sync", "库存同步工具", "数据", "多平台库存同步"}}
	for _, d := range defs {
		t := Tool{ID: d[0], Version: "0.1.0", Name: d[1], Category: d[2], Description: d[3], Status: "DEVELOPING", Reason: "开发中"}
		for i := range activations {
			if activations[i].ToolID == t.ID {
				a := activations[i]
				t.Activation = &a
				break
			}
		}
		if mine && (t.Activation == nil || !t.Activation.Enabled) {
			continue
		}
		if t.ID == AcquisitionID {
			t.LocalCapture, t.OnlineCapture, t.Download = r.LocalCapture, r.OnlineCapture, r.Download
			t.Status, t.Reason = "UNAVAILABLE", "当前安装尚未接入采集能力"
			if r.LocalCapture || r.OnlineCapture {
				t.Status, t.Reason = "AVAILABLE", ""
			}
		}
		result = append(result, t)
	}
	return result
}
