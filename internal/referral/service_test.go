package referral

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
)

type selfStore struct {
	Store
	created               int
	issuer, subject, code string
}

func (s *selfStore) Read(_ context.Context, issuer, subject string) (Projection, error) {
	s.issuer = issuer
	s.subject = subject
	return Projection{Code: s.code, Count: 4}, nil
}
func (s *selfStore) CreateCode(_ context.Context, issuer, subject, code string) (string, error) {
	s.created++
	s.issuer = issuer
	s.subject = subject
	if s.code == "" {
		s.code = code
	}
	return s.code, nil
}
func TestReadDoesNotCreateAndEarningsUnavailable(t *testing.T) {
	st := &selfStore{}
	got, err := (Service{st}).Read(context.Background(), "issuer", "person")
	if err != nil || st.created != 0 || got.Code != "" || got.Count != 4 || got.EarningsAvailability != "unavailable" || st.subject != "person" {
		t.Fatalf("projection=%+v err=%v", got, err)
	}
}
func TestExplicitCodeIsRandomAndStable(t *testing.T) {
	st := &selfStore{}
	s := Service{st}
	a, err := s.CreateCode(context.Background(), "issuer", "person")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateCode(context.Background(), "issuer", "person")
	decoded, e := base64.RawURLEncoding.DecodeString(a)
	if err != nil || e != nil || len(decoded) != 32 || a != b {
		t.Fatal("code must be random and stable")
	}
	if _, err = s.CreateCode(context.Background(), "issuer", ""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("empty identity=%v", err)
	}
}
