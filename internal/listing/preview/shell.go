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

type TaskShellInput struct {
	TaskID           string
	Status           string
	SelectedPlatform string
	ResultPlatforms  []string
	RequestPlatforms []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func BuildTaskShell(input TaskShellInput) *Preview {
	var completedAt *time.Time
	switch input.Status {
	case "completed", "needs_review", "failed":
		value := input.UpdatedAt
		completedAt = &value
	}
	return BuildProjection(ProjectionInput{
		Shell: ShellInput{
			TaskID:           input.TaskID,
			Status:           input.Status,
			SelectedPlatform: input.SelectedPlatform,
			Platforms:        ResolvePlatforms(input.ResultPlatforms, input.RequestPlatforms),
			CreatedAt:        input.CreatedAt,
			CompletedAt:      completedAt,
		},
	})
}
