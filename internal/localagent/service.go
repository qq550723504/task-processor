package localagent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	coreLogger "task-processor/internal/core/logger"
	alibaba1688 "task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/product/sourcing"

	"github.com/sirupsen/logrus"
)

const (
	// Browser provisioning may download a managed Chrome for up to ten
	// minutes, so the lease must outlive that operation.
	claimTTL           = 15 * time.Minute
	jobTTL             = 20 * time.Minute
	maxSnapshotBytes   = 1 << 20
	maxDiagnosticBytes = 512
	terminalRetention  = 10 * time.Minute
	maxStoredJobs      = 1024
	maxJobsPerTenant   = 256
	maxClaimAttempts   = 3
	// Keep every retained summary field bounded. Together these caps keep the
	// JSON representation well below 2 KiB even when a crawler returns hostile
	// strings, while preserving useful terminal evidence for the CLI.
	maxEnvelopeSummarySourceKeyBytes = 256
	maxEnvelopeSummarySourceURLBytes = 512
	maxEnvelopeSummaryProductIDBytes = 128
	maxEnvelopeSummaryTitleBytes     = 256
	maxEnvelopeSummarySupplierBytes  = 256
	maxEnvelopeSummaryPriceBytes     = 128
)

var (
	ErrIdentityRequired = errors.New("tenant and user identity are required")
	ErrInvalidURL       = errors.New("invalid public 1688 offer URL")
	ErrClaimExpired     = errors.New("local-agent claim expired")
	ErrInvalidClaim     = errors.New("invalid local-agent claim")
	ErrTerminalJob      = errors.New("local-agent job is already terminal")
	ErrSnapshotTooLarge = errors.New("1688 snapshot exceeds size limit")
	ErrSnapshotInvalid  = errors.New("1688 product snapshot failed server validation")
	ErrFailureInvalid   = errors.New("invalid local-agent failure diagnostic")
	ErrCapacity         = errors.New("local-agent job capacity reached")
	ErrSecureRandom     = errors.New("secure random generation failed")
)

type Service struct {
	mu   sync.Mutex
	now  func() time.Time
	jobs map[string]*record
}

type record struct {
	job            Job
	executionToken string
	retainedUntil  time.Time
	claimAttempts  int
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
	id, err := newID()
	if err != nil {
		return Job{}, err
	}
	now := s.now().UTC()
	job := Job{ID: id, TenantID: strings.TrimSpace(actor.TenantID), URL: cleanURL, State: JobPending, ExpiresAt: now.Add(jobTTL)}
	s.mu.Lock()
	s.cleanupLocked(now)
	if len(s.jobs) >= maxStoredJobs || s.tenantJobCountLocked(job.TenantID) >= maxJobsPerTenant {
		s.mu.Unlock()
		return Job{}, ErrCapacity
	}
	s.jobs[job.ID] = &record{job: job}
	s.mu.Unlock()
	return job, nil
}

func (s *Service) tenantJobCountLocked(tenantID string) int {
	count := 0
	for _, rec := range s.jobs {
		if rec.job.TenantID == tenantID {
			count++
		}
	}
	return count
}

func (s *Service) Claim(actor Actor) (*Claim, error) {
	return s.claimMatching(actor, "")
}

// ClaimJob claims a specific pending job for the actor's tenant. It is used by
// the local client when a job was just created, so another pending job cannot
// be selected accidentally.
func (s *Service) ClaimJob(actor Actor, jobID string) (*Claim, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, ErrInvalidClaim
	}
	return s.claimMatching(actor, jobID)
}

func (s *Service) claimMatching(actor Actor, jobID string) (*Claim, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	if jobID != "" {
		selected, ok := s.jobs[jobID]
		if !ok || selected.job.TenantID != strings.TrimSpace(actor.TenantID) {
			return nil, ErrInvalidClaim
		}
		if selected.job.State != JobPending || !now.Before(selected.job.ExpiresAt) {
			return nil, ErrInvalidClaim
		}
		return s.claimRecordForPendingLocked(selected, now)
	}
	var selected *record
	for _, candidate := range s.jobs {
		if candidate.job.TenantID != strings.TrimSpace(actor.TenantID) {
			continue
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
	return s.claimRecordForPendingLocked(selected, now)
}

func (s *Service) claimRecordForPendingLocked(selected *record, now time.Time) (*Claim, error) {
	token, err := newID()
	if err != nil {
		return nil, err
	}
	selected.job.State = JobClaimed
	selected.job.LeaseExpiresAt = now.Add(claimTTL)
	selected.job.ExpiresAt = now.Add(jobTTL)
	selected.executionToken = token
	selected.claimAttempts++
	return &Claim{Job: selected.job, ExecutionToken: selected.executionToken}, nil
}

func (s *Service) SubmitSuccess(actor Actor, jobID, token string, product *sourcing.Alibaba1688ProductSnapshot) (Job, error) {
	if err := validateActor(actor); err != nil {
		return Job{}, err
	}
	if product == nil {
		return Job{}, fmt.Errorf("%w: %w: product is required", ErrSnapshotInvalid, ErrInvalidClaim)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.claimRecordLocked(actor, jobID, token)
	if err != nil {
		return Job{}, err
	}
	size, err := json.Marshal(product)
	if err != nil {
		return Job{}, ErrSnapshotInvalid
	}
	if len(size) > maxSnapshotBytes {
		return Job{}, ErrSnapshotTooLarge
	}
	productURL, err := validateOfferURL(product.URL)
	if err != nil || productURL != rec.job.URL {
		return Job{}, fmt.Errorf("%w: %w: product URL does not match claimed offer", ErrSnapshotInvalid, ErrInvalidURL)
	}
	if strings.TrimSpace(product.ID) == "" || product.ID != sourcing.ExtractAlibaba1688ProductID(rec.job.URL) {
		return Job{}, fmt.Errorf("%w: %w: product id does not match claimed offer", ErrSnapshotInvalid, ErrInvalidClaim)
	}
	if err := validateCrawlerSnapshot(product); err != nil {
		return Job{}, ErrSnapshotInvalid
	}
	envelope := sourcing.Alibaba1688SourceEnvelope(sourcing.Alibaba1688SourceEnvelopeInput{
		Request:     sourcing.Alibaba1688CrawlRequestInput{URL: rec.job.URL},
		Product:     product,
		SourceRunID: rec.job.ID,
	})
	summary := EnvelopeSummary{
		SourceKey:    envelope.Identity.SourceKey(),
		SourceURL:    envelope.Identity.SourceURL,
		ProductID:    envelope.Identity.ProductID,
		Title:        envelope.ProductCandidate.Title,
		AssetCount:   len(envelope.AssetCandidates),
		VariantCount: len(envelope.ProductCandidate.Variants),
		SupplierName: envelope.SupplierOrCostFacts.SupplierName,
		Price:        envelope.SupplierOrCostFacts.Price,
	}
	summary = boundEnvelopeSummary(summary)
	// Terminal records are retained only for lifecycle/idempotency checks. The
	// current API has no terminal read route, so keep the reconstructed envelope
	// on the immediate return value without retaining its potentially large
	// payload in the in-memory job record.
	rec.job.Envelope = nil
	rec.job.EnvelopeSummary = &summary
	rec.job.State = JobSucceeded
	rec.job.LeaseExpiresAt = time.Time{}
	rec.executionToken = ""
	rec.retainedUntil = s.now().UTC().Add(terminalRetention)
	logTransition(rec.job, "succeeded", "")
	completed := rec.job
	completed.Envelope = &envelope
	return completed, nil
}

func validateCrawlerSnapshot(product *sourcing.Alibaba1688ProductSnapshot) error {
	videos := make([]model.Video, 0, len(product.Videos))
	for _, video := range product.Videos {
		videos = append(videos, model.Video{VideoURL: video.VideoURL, CoverURL: video.CoverURL})
	}
	details := make([]model.ProductDetail, 0, len(product.ProductDetails))
	for _, detail := range product.ProductDetails {
		details = append(details, model.ProductDetail{Content: detail.Content, Images: detail.Images})
	}
	variants := make([]model.Variant, 0, len(product.Variants))
	for _, variant := range product.Variants {
		variants = append(variants, model.Variant{Attributes: variant.Attributes, Name: variant.Name, Image: variant.Image, Stock: variant.Stock, Price: variant.Price})
	}
	var packInfo *model.PackInfo
	if product.PackInfo != nil {
		packInfo = &model.PackInfo{PackageType: product.PackInfo.PackageType, Weight: product.PackInfo.Weight, PackageImages: product.PackInfo.PackageImages, Instructions: product.PackInfo.Instructions}
	}
	return alibaba1688.NewProductChecker().ValidateProduct(&model.Product1688{
		Title:            product.Title,
		URL:              product.URL,
		Images:           product.Images,
		MainImage:        product.MainImage,
		Videos:           videos,
		MinPrice:         product.MinPrice,
		MaxPrice:         product.MaxPrice,
		MinOrderQuantity: product.MinOrderQuantity,
		SalesVolume:      product.SalesVolume,
		ReviewCount:      product.ReviewCount,
		Rating:           product.Rating,
		Supplier: model.SupplierInfo{
			ID:              product.Supplier.ID,
			Name:            product.Supplier.Name,
			CompanyName:     product.Supplier.CompanyName,
			Location:        product.Supplier.Location,
			ShopURL:         product.Supplier.ShopURL,
			CardType:        product.Supplier.CardType,
			YearsInBusiness: product.Supplier.YearsInBusiness,
			Rating:          product.Supplier.Rating,
			ResponseRate:    product.Supplier.ResponseRate,
			IsGoldSupplier:  product.Supplier.IsGoldSupplier,
			IsVerified:      product.Supplier.IsVerified,
		},
		ProductDetails: details,
		PackInfo:       packInfo,
		Variants:       variants,
	})
}

func boundEnvelopeSummary(summary EnvelopeSummary) EnvelopeSummary {
	summary.SourceKey = truncateUTF8Bytes(summary.SourceKey, maxEnvelopeSummarySourceKeyBytes)
	summary.SourceURL = truncateUTF8Bytes(summary.SourceURL, maxEnvelopeSummarySourceURLBytes)
	summary.ProductID = truncateUTF8Bytes(summary.ProductID, maxEnvelopeSummaryProductIDBytes)
	summary.Title = truncateUTF8Bytes(summary.Title, maxEnvelopeSummaryTitleBytes)
	summary.SupplierName = truncateUTF8Bytes(summary.SupplierName, maxEnvelopeSummarySupplierBytes)
	summary.Price = truncateUTF8Bytes(summary.Price, maxEnvelopeSummaryPriceBytes)
	return summary
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
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
	rec.retainedUntil = s.now().UTC().Add(terminalRetention)
	logTransition(rec.job, "failed", failure.Kind)
	return rec.job, nil
}

func (s *Service) cleanupLocked(now time.Time) {
	for id, rec := range s.jobs {
		switch rec.job.State {
		case JobPending:
			if !now.Before(rec.job.ExpiresAt) {
				delete(s.jobs, id)
			}
		case JobClaimed:
			if !now.Before(rec.job.ExpiresAt) {
				delete(s.jobs, id)
				continue
			}
			if !now.Before(rec.job.LeaseExpiresAt) {
				if rec.claimAttempts >= maxClaimAttempts {
					delete(s.jobs, id)
					continue
				}
				rec.job.State = JobPending
				rec.job.LeaseExpiresAt = time.Time{}
				rec.executionToken = ""
			}
		case JobSucceeded, JobFailed:
			if !now.Before(rec.retainedUntil) {
				delete(s.jobs, id)
			}
		}
	}
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
	original := strings.TrimSpace(raw)
	originalParsed, err := url.Parse(original)
	if err != nil || originalParsed.RawQuery != "" || originalParsed.ForceQuery || originalParsed.Fragment != "" {
		return "", ErrInvalidURL
	}
	clean := sourcing.NormalizeAlibaba1688URL(original)
	parsed, err := url.Parse(clean)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "detail.1688.com") || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawPath != "" {
		return "", ErrInvalidURL
	}
	if !offerPathPattern.MatchString(parsed.Path) {
		return "", ErrInvalidURL
	}
	return clean, nil
}

var offerPathPattern = regexp.MustCompile(`^/offer/[0-9]+\.html$`)

func validFailureKind(kind FailureKind) bool {
	switch kind {
	case FailureBrowser, FailureNavigation, FailureChallenge, FailureExtraction, FailureUnknown:
		return true
	default:
		return false
	}
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", ErrSecureRandom
	}
	return hex.EncodeToString(b), nil
}

func logTransition(job Job, state string, failureKind FailureKind) {
	fields := logrus.Fields{
		"job_id":    job.ID,
		"tenant_id": job.TenantID,
		"state":     state,
	}
	if failureKind != "" {
		fields["failure_kind"] = failureKind
	}
	coreLogger.GetGlobalLogger("local-agent").WithFields(fields).Info("local-agent job transition")
}
