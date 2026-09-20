// Package membership projects the current identity provider's organization
// project assignments. It does not persist an independent member directory.
package membership

import (
	"context"
	"errors"
)

var (
	ErrAuthentication  = errors.New("membership authentication required")
	ErrPermission      = errors.New("membership permission denied")
	ErrInvalidRequest  = errors.New("invalid membership request")
	ErrUnavailable     = errors.New("membership dependency unavailable")
	ErrInvalidResponse = errors.New("invalid membership provider response")
	ErrNotFound        = errors.New("member not found")
)

type Member struct {
	ID              string   `json:"id"`
	UserID          string   `json:"userId"`
	OrganizationID  string   `json:"organizationId"`
	ProjectID       string   `json:"projectId"`
	DisplayName     string   `json:"displayName"`
	LoginName       string   `json:"loginName"`
	Roles           []string `json:"roles"`
	State           string   `json:"state"`
	CreatedAt       string   `json:"createdAt"`
	ChangedAt       string   `json:"changedAt"`
	ObservedVersion string   `json:"observedVersion"`
	CanChangeRole   bool     `json:"canChangeRole"`
	CanRemove       bool     `json:"canRemove"`
}

type PageRequest struct{ Limit, Offset int }
type Page struct {
	Items []Member
	Total int
}

type Directory interface {
	List(context.Context, string, PageRequest) (Page, error)
	Read(context.Context, string, string) (Member, error)
}
type Authorizer interface {
	Authorize(string, []string, string) bool
}

type Result struct {
	SchemaVersion   string   `json:"schemaVersion"`
	UserID          string   `json:"userId"`
	OrganizationID  string   `json:"organizationId"`
	Items           []Member `json:"items"`
	Total           int      `json:"total"`
	CanManage       bool     `json:"canManage"`
	AssignableRoles []string `json:"assignableRoles"`
}
