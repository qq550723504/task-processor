package aicapability

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrAsyncJobBindingInvalid  = errors.New("async job binding is invalid")
	ErrAsyncJobBindingNotFound = errors.New("async job binding not found")
	ErrAsyncJobBindingConflict = errors.New("async job binding conflicts with existing route")
)

// AsyncJobBinding stores only the routing and lifecycle metadata required to
// query an asynchronous Provider job after the submit request has completed.
type AsyncJobBinding struct {
	JobID                string
	TenantID             string
	UserID               string
	BusinessTaskID       string
	TraceID              string
	Capability           Capability
	Operation            Operation
	ProviderID           string
	ModelID              string
	RoutingKey           string
	CredentialReference  string
	PolicyVersion        string
	ConfigurationVersion string
	SubmittedAt          time.Time
	UpdatedAt            time.Time
	ExpiresAt            time.Time
	Status               string
	LastErrorCategory    ErrorCategory
}

// AsyncJobBindingStore persists Provider routing metadata for async jobs.
type AsyncJobBindingStore interface {
	PutAsyncJobBinding(context.Context, AsyncJobBinding) error
	GetAsyncJobBinding(context.Context, string) (AsyncJobBinding, error)
	UpdateAsyncJobBindingStatus(context.Context, string, string, ErrorCategory) error
}

func ValidateAsyncJobBinding(binding AsyncJobBinding) error {
	if strings.TrimSpace(binding.JobID) == "" {
		return ErrAsyncJobBindingInvalid
	}
	return nil
}
