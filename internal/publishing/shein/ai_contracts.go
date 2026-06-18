package shein

import "context"

type TextGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}
