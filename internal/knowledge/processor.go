package knowledge

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Processor struct {
	repo    Repository
	objects KnowledgeObjectStore
	parser  DocumentParser
	owner   string
}

func NewProcessor(repo Repository, objects KnowledgeObjectStore, parser DocumentParser) (*Processor, error) {
	if repo == nil || objects == nil || parser == nil {
		return nil, ErrUnavailable
	}
	return &Processor{repo: repo, objects: objects, parser: parser, owner: uuid.NewString()}, nil
}

// Run is owned by current-application and exits only after its bounded workers.
func (p *Processor) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := p.Sweep(ctx); err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (p *Processor) Sweep(ctx context.Context) error {
	// Claim only immediately runnable work: queued jobs must not consume leases.
	revisions, err := p.repo.ClaimProcessing(ctx, p.owner+":"+uuid.NewString(), 2)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, 2)
	for _, revision := range revisions {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		}
		wg.Go(func() { defer func() { <-slots }(); p.process(ctx, revision) })
	}
	wg.Wait()
	return nil
}
func (p *Processor) process(ctx context.Context, r Revision) {
	// A claimed revision has a 30s lease. I/O plus parsing fits in one 20s budget.
	jobContext, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	object := revisionObject(r)
	if r.State == Admitted {
		inspection, err := p.objects.Inspect(jobContext, object)
		if err != nil {
			return
		} // expiry retries inspection; no missing-object claim on transport failure
		if !inspection.Exists {
			_ = p.repo.Finish(ctx, r, ParseResult{Failure: "UPLOAD_INCOMPLETE"})
			return
		}
		if !matches(object, inspection) {
			_ = p.repo.Finish(ctx, r, ParseResult{Failure: "OBJECT_INTEGRITY_FAILURE"})
			return
		}
		if p.repo.ConfirmObject(ctx, r) != nil {
			return
		}
		return // next bounded sweep parses
	}
	data, err := p.objects.ReadBounded(jobContext, object, MaxUploadBytes)
	if err != nil {
		if errors.Is(err, ErrIntegrity) {
			_ = p.repo.Finish(ctx, r, ParseResult{Failure: "OBJECT_INTEGRITY_FAILURE"})
			return
		}
		_ = p.repo.Finish(ctx, r, ParseResult{Failure: "OBJECT_READ_UNAVAILABLE", Transient: true})
		return
	}
	if int64(len(data)) != r.SizeBytes || Digest(data) != r.SHA256 {
		_ = p.repo.Finish(ctx, r, ParseResult{Failure: "OBJECT_INTEGRITY_FAILURE"})
		return
	}
	var result ParseResult
	if r.ContentType == "text/plain" || r.ContentType == "text/markdown" {
		result = NormalizeText(string(data), "")
	} else {
		result = p.parser.Parse(jobContext, r.ContentType, data)
		if result.Failure == "" {
			result = NormalizeText(result.Text, result.Warning)
		}
	}
	_ = p.repo.Finish(ctx, r, result)
}
