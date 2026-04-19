package generation

import (
	"context"
	"database/sql/driver"
	"encoding/json"

	"task-processor/internal/asset"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog"
)

type Request struct {
	TaskID    string
	Product   *catalog.Product
	Inventory *asset.Inventory
	Recipes   []assetrecipe.AssetRecipe
}

type DispatchRequest struct {
	TaskID    string
	Product   *catalog.Product
	Inventory *asset.Inventory
	Tasks     []Task
}

type DeferredRenderRequest struct {
	TaskID    string
	Product   *catalog.Product
	Task      Task
	BaseAsset asset.AssetRecord
}

type Task struct {
	TaskID           string            `json:"task_id,omitempty"`
	ID               string            `json:"id,omitempty"`
	Platform         string            `json:"platform,omitempty"`
	RecipeID         string            `json:"recipe_id,omitempty"`
	AssetKind        asset.Kind        `json:"asset_kind,omitempty"`
	Slot             string            `json:"slot,omitempty"`
	Purpose          string            `json:"purpose,omitempty"`
	TemplateLabel    string            `json:"template_label,omitempty"`
	RenderProfile    string            `json:"render_profile,omitempty"`
	Status           string            `json:"status,omitempty"`
	ExecutionStatus  string            `json:"execution_status,omitempty"`
	ExecutionMode    string            `json:"execution_mode,omitempty"`
	CanExecute       bool              `json:"can_execute,omitempty"`
	SatisfiedBy      string            `json:"satisfied_by,omitempty"`
	FallbackFrom     string            `json:"fallback_from,omitempty"`
	Lineage          []string          `json:"lineage,omitempty"`
	SourceAssetIDs   []string          `json:"source_asset_ids,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ReviewConfidence float64           `json:"review_confidence,omitempty"`
}

type Result struct {
	Tasks  []Task               `json:"tasks,omitempty"`
	Assets []asset.AssetRecord  `json:"assets,omitempty"`
	Review *asset.ReviewSummary `json:"review,omitempty"`
	Error  string               `json:"error,omitempty"`
}

type DeferredRenderer interface {
	Render(ctx context.Context, req DeferredRenderRequest) (*asset.AssetRecord, error)
}

type Service interface {
	Plan(ctx context.Context, req Request) (*Result, error)
	Execute(ctx context.Context, req Request) (*Result, error)
	Dispatch(ctx context.Context, req DispatchRequest) (*Result, error)
}

func (t Task) Value() (driver.Value, error) {
	return json.Marshal(t)
}

func (t *Task) Scan(value any) error {
	if value == nil {
		*t = Task{}
		return nil
	}
	switch typed := value.(type) {
	case []byte:
		return json.Unmarshal(typed, t)
	case string:
		return json.Unmarshal([]byte(typed), t)
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, t)
	}
}
