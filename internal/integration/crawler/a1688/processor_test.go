package a1688

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/crawler/alibaba1688/model"
)

type stubSource1688 struct {
	product *model.Product1688
	err     error
	url     string
}

func (s *stubSource1688) Process(url string) (*model.Product1688, error) {
	s.url = url
	return s.product, s.err
}

func TestProcessorProcessDelegatesToSource(t *testing.T) {
	source := &stubSource1688{product: &model.Product1688{Title: "sample"}}
	processor := NewProcessor(source)

	product, err := processor.Process(context.Background(), "https://detail.1688.com/offer/1.html")
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if product.Title != "sample" {
		t.Fatalf("Process title = %q, want sample", product.Title)
	}
	if source.url != "https://detail.1688.com/offer/1.html" {
		t.Fatalf("source url = %q", source.url)
	}
}

func TestProcessorProcessReturnsSourceError(t *testing.T) {
	wantErr := errors.New("crawl failed")
	processor := NewProcessor(&stubSource1688{err: wantErr})

	_, err := processor.Process(context.Background(), "https://detail.1688.com/offer/1.html")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Process error = %v, want %v", err, wantErr)
	}
}

func TestProcessorProcessHonorsCanceledContext(t *testing.T) {
	source := &stubSource1688{product: &model.Product1688{Title: "sample"}}
	processor := NewProcessor(source)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := processor.Process(ctx, "https://detail.1688.com/offer/1.html")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Process error = %v, want context.Canceled", err)
	}
	if source.url != "" {
		t.Fatalf("source was called with url %q", source.url)
	}
}

func TestProcessorProcessReturnsUnavailableForNilSource(t *testing.T) {
	processor := NewProcessor(nil)

	_, err := processor.Process(context.Background(), "https://detail.1688.com/offer/1.html")
	if !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("Process error = %v, want ErrSourceUnavailable", err)
	}
}
