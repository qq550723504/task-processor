// Package a1688 adapts the legacy 1688 crawler behind a small integration
// boundary.
package a1688

import (
	"context"
	"errors"

	"task-processor/internal/core/config"
	legacy "task-processor/internal/crawler/alibaba1688"
	"task-processor/internal/crawler/alibaba1688/model"
)

// ErrSourceUnavailable means no 1688 crawler source has been configured.
var ErrSourceUnavailable = errors.New("1688 crawler source unavailable")

// Source is the legacy 1688 crawler surface used by this integration adapter.
type Source interface {
	Process(url string) (*model.Product1688, error)
}

// Processor exposes 1688 crawling through a stable integration boundary.
type Processor struct {
	source Source
}

// NewProcessor wraps a 1688 crawler source.
func NewProcessor(source Source) *Processor {
	return &Processor{source: source}
}

// NewLegacyProcessor builds an adapter backed by the legacy 1688 crawler.
func NewLegacyProcessor(cfg *config.Config) *Processor {
	return NewProcessor(legacy.NewAlibaba1688Processor(cfg))
}

// Process crawls a 1688 product URL.
func (p *Processor) Process(ctx context.Context, url string) (*model.Product1688, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil || p.source == nil {
		return nil, ErrSourceUnavailable
	}
	return p.source.Process(url)
}
