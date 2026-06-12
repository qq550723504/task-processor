package preview

import "time"

type ShellInput struct {
	TaskID           string
	Status           string
	SelectedPlatform string
	Platforms        []string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

func BuildShell(input ShellInput) *Preview {
	return &Preview{
		TaskID:           input.TaskID,
		Status:           input.Status,
		SelectedPlatform: input.SelectedPlatform,
		Platforms:        append([]string(nil), input.Platforms...),
		CreatedAt:        input.CreatedAt,
		CompletedAt:      input.CompletedAt,
	}
}
