package productenrich

import "context"

type InputParser interface {
	ParseInput(ctx context.Context, req *GenerateRequest) (*ParsedInput, error)
	CollectImages(ctx context.Context, urls []string) ([]string, error)
	CleanText(text string) string
	Scrape1688(ctx context.Context, url string) (*ScrapedData, error)
}

type InputParserConfig struct{}
