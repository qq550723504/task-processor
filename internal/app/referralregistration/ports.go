// Package referralregistration coordinates durable registration with the identity provider.
package referralregistration

import (
	"context"
	"sync"
	"task-processor/internal/referral"
	"time"
)

type Request struct {
	Key, Code, Email, GivenName, FamilyName string
	// ClientIP must be supplied by the trusted proxy boundary, never the JSON body.
	ClientIP string `json:"-"`
}
type Admission struct {
	IntentID, Subject, ResumeSecret      string
	CreateExpiresAt, CompletionExpiresAt time.Time
}
type Creation struct{ Subject, Organization, Email, GivenName, FamilyName, Proof string }
type User struct {
	Subject, Organization, Email, Proof string
	Verified                            bool
}
type Provider interface {
	Create(context.Context, Creation) error
	Read(context.Context, string) (User, error)
}
type Keys struct {
	Active            string
	Encryption, Proof map[string][]byte
	Lookup            []byte
}
type Service struct {
	gateOnce                       sync.Once
	slots                          chan struct{}
	Store                          referral.Store
	Provider                       Provider
	Keys                           Keys
	Issuer, Instance, Organization string
	Now                            func() time.Time
}

func (s *Service) enter() bool {
	s.gateOnce.Do(func() { s.slots = make(chan struct{}, 8) })
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}
func (s *Service) leave() { <-s.slots }
