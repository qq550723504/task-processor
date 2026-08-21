package localagent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"task-processor/internal/product/sourcing"
)

const (
	jobTTL             = 5 * time.Minute
	claimTTL           = 3 * time.Minute
	maxSnapshotBytes   = 1 << 20
	maxDiagnosticBytes = 512
)

var (
	ErrIdentityRequired = errors.New("tenant and user identity are required")
	ErrInvalidURL       = errors.New("invalid public 1688 offer URL")
	ErrClaimExpired     = errors.New("local-agent claim expired")
	ErrInvalidClaim     = errors.New("invalid local-agent claim")
	ErrTerminalJob      = errors.New("local-agent job is already terminal")
	ErrSnapshotTooLarge = errors.New("1688 snapshot exceeds size limit")
	ErrFailureInvalid   = errors.New("invalid local-agent failure diagnostic")
)

type Service struct {
	mu   sync.Mutex
	now  func() time.Time
	jobs map[string]*record
}

type record struct {
	job            Job
	executionToken string
}

func NewService(now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{now: now, jobs: make(map[string]*record)}
}

func (s *Service) Create(actor Actor, rawURL string) (Job, error) {
	if err := validateActor(actor); err != nil {
		return Job{}, err
	}
	cleanURL, err := validateOfferURL(rawURL)
	if err != nil {
		return Job{}, err
	}
	now := s.now().UTC()
	job := Job{ID: newID(), TenantID: strings.TrimSpace(actor.TenantID), URL: cleanURL, State: JobPending, ExpiresAt: now.Add(jobTTL)}
	s.mu.Lock()
	s.jobs[job.ID] = &record{job: job}
	s.mu.Unlock()
	return job, nil
}

func (s *Service) Claim(actor Actor) (*Claim, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	var selected *record
	for _, candidate := range s.jobs {
		if candidate.job.TenantID != strings.TrimSpace(actor.TenantID) {
			continue
		}
		if candidate.job.State == JobClaimed && !now.Before(candidate.job.LeaseExpiresAt) {
			if now.Before(candidate.job.ExpiresAt) {
				candidate.job.State = JobPending
				candidate.job.LeaseExpiresAt = time.Time{}
				candidate.executionToken = ""
			} else {
				candidate.job.State = JobFailed
				candidate.job.Failure = &Failure{Kind: FailureUnknown, Message: "job expired before local agent completed"}
			}
		}
		if candidate.job.State != JobPending || !now.Before(candidate.job.ExpiresAt) {
			continue
		}
		if selected == nil || candidate.job.ExpiresAt.Before(selected.job.ExpiresAt) || (candidate.job.ExpiresAt.Equal(selected.job.ExpiresAt) && candidate.job.ID < selected.job.ID) {
			selected = candidate
		}
	}
	if selected == nil {
		return nil, nil
	}
	selected.job.State = JobClaimed
	selected.job.LeaseExpiresAt = now.Add(claimTTL)
	selected.executionToken = newID()
	return &Claim{Job: selected.job, ExecutionToken: selected.executionToken}, nil
}

func (s *Service) SubmitSuccess(actor Actor, jobID, token string, product *sourcing.Alibaba1688ProductSnapshot) (Job, error) {
	if err := validateActor(actor); err != nil {
		return Job{}, err
	}
	if product == nil {
		return Job{}, fmt.Errorf("%w: product is required", ErrInvalidClaim)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.claimRecordLocked(actor, jobID, token)
	if err != nil {
		return Job{}, err
	}
	if size, err := json.Marshal(product); err != nil || len(size) > maxSnapshotBytes {
		return Job{}, ErrSnapshotTooLarge
	}
	productURL := strings.TrimSpace(product.URL)
	if productURL != "" {
		if _, err := validateOfferURL(productURL); err != nil {
			return Job{}, err
		}
	}
	if strings.TrimSpace(product.ID) == "" && sourcing.ExtractAlibaba1688ProductID(rec.job.URL) == "" {
		return Job{}, fmt.Errorf("%w: product id is required", ErrInvalidClaim)
	}
	envelope := sourcing.Alibaba1688SourceEnvelope(sourcing.Alibaba1688SourceEnvelopeInput{
		Request:     sourcing.Alibaba1688CrawlRequestInput{URL: rec.job.URL},
		Product:     product,
		SourceRunID: rec.job.ID,
	})
	rec.job.Envelope = &envelope
	rec.job.State = JobSucceeded
	rec.job.LeaseExpiresAt = time.Time{}
	rec.executionToken = ""
	return rec.job, nil
}

func (s *Service) SubmitFailure(actor Actor, jobID, token string, failure Failure) (Job, error) {
	if err := validateActor(actor); err != nil {
		return Job{}, err
	}
	if !validFailureKind(failure.Kind) || strings.TrimSpace(failure.Message) == "" || len([]byte(failure.Message)) > maxDiagnosticBytes {
		return Job{}, ErrFailureInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.claimRecordLocked(actor, jobID, token)
	if err != nil {
		return Job{}, err
	}
	failure.Message = strings.TrimSpace(failure.Message)
	rec.job.Failure = &failure
	rec.job.State = JobFailed
	rec.job.LeaseExpiresAt = time.Time{}
	rec.executionToken = ""
	return rec.job, nil
}

func (s *Service) claimRecordLocked(actor Actor, jobID, token string) (*record, error) {
	rec, ok := s.jobs[strings.TrimSpace(jobID)]
	if !ok || rec.job.TenantID != strings.TrimSpace(actor.TenantID) || token == "" || token != rec.executionToken {
		return nil, ErrInvalidClaim
	}
	now := s.now().UTC()
	if rec.job.State != JobClaimed {
		if rec.job.State == JobSucceeded || rec.job.State == JobFailed {
			return nil, ErrTerminalJob
		}
		return nil, ErrInvalidClaim
	}
	if !now.Before(rec.job.LeaseExpiresAt) || !now.Before(rec.job.ExpiresAt) {
		return nil, ErrClaimExpired
	}
	return rec, nil
}

func validateActor(actor Actor) error {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.UserID) == "" {
		return ErrIdentityRequired
	}
	return nil
}

func validateOfferURL(raw string) (string, error) {
	clean := sourcing.NormalizeAlibaba1688URL(raw)
	parsed, err := url.Parse(clean)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "detail.1688.com" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidURL
	}
	if !strings.HasPrefix(parsed.Path, "/offer/") || sourcing.ExtractAlibaba1688ProductID(parsed.Path) == "" {
		return "", ErrInvalidURL
	}
	return clean, nil
}

func validFailureKind(kind FailureKind) bool {
	switch kind {
	case FailureBrowser, FailureNavigation, FailureChallenge, FailureExtraction, FailureUnknown:
		return true
	default:
		return false
	}
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("local-agent-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
