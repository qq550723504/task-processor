package agentconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	ImageAgentID               = "product.image.agent"
	ImageAgentVersion          = "v1.0.0"
	ImageConfigurationSchema   = "image-config-v1"
	MaxSetTasks                = 32
	MaxSetTemplateBytes        = 64 << 10
	MaxSetBackgroundBytes      = 1024
	MaxSetBriefBytes           = 2048
	MaxSetSourceReferences     = 8
	MaxSetSourceAggregateBytes = 16 << 20
)

// SetTemplate describes content preferences. Platform requirements are read
// from their marketplace owner for the actual product, never from this template.
type SetTemplate struct {
	Schema         string        `json:"schema"`
	Mode           string        `json:"mode"`
	ShareOriginals bool          `json:"shareOriginals"`
	Background     string        `json:"background"`
	Language       string        `json:"language"`
	Carousel       []ContentTask `json:"carousel"`
	Detail         []ContentTask `json:"detail"`
}

type ContentTask struct {
	ID      string `json:"id"`
	Purpose string `json:"purpose"`
	Brief   string `json:"brief,omitempty"`
}

type ContentTaskDefinition struct {
	Purpose  string `json:"purpose"`
	Label    string `json:"label"`
	Evidence string `json:"evidence,omitempty"`
}

// Return new slices: callers cannot modify the code-owned content library.
func CarouselContentTasks() []ContentTaskDefinition {
	return []ContentTaskDefinition{
		{"product_identity", "商品识别主图", ""},
		{"purchase_reason", "核心购买理由图", ""},
		{"need_solution", "核心需求解决图", ""},
		{"structure", "产品结构确认图", ""},
		{"detail_evidence", "核心细节证据图", ""},
		{"usage_scene", "核心使用场景图", ""},
		{"choice_reason", "卖点选择理由图", ""},
		{"purchase_specs", "规格购买确认图", "specifications"},
	}
}

func DetailContentTasks() []ContentTaskDefinition {
	return []ContentTaskDefinition{
		{"product_overview", "商品全貌", ""},
		{"key_benefits", "核心卖点", ""},
		{"use_scenario", "使用场景", ""},
		{"structure_functions", "结构功能", ""},
		{"detail_closeup", "细节特写", ""},
		{"specification_dimensions", "规格尺寸", "specifications"},
		{"usage_steps", "使用步骤", "instructions"},
		{"packaging_accessories", "包装配件", "accessories"},
	}
}

var contentTaskID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

func ValidSetText(value string, maximum int) bool {
	return len(value) <= maximum && utf8.ValidString(value) && strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) && r != '\n' && r != '\t'
	}) < 0
}

func (t SetTemplate) Validate() error {
	if t.Schema != ImageConfigurationSchema || t.Mode != "standard" && t.Mode != "custom" || !ValidSetText(t.Background, MaxSetBackgroundBytes) || strings.TrimSpace(t.Background) == "" {
		return fmt.Errorf("%w: invalid image template schema, mode or background", ErrInvalid)
	}
	switch t.Language {
	case "en", "zh", "es", "fr", "de", "ja":
	default:
		return fmt.Errorf("%w: unsupported image text language", ErrInvalid)
	}
	count := len(t.Carousel) + len(t.Detail)
	if count < 1 || count > MaxSetTasks {
		return fmt.Errorf("%w: choose between 1 and %d image tasks", ErrInvalid, MaxSetTasks)
	}
	seen := make(map[string]bool, count)
	for index, group := range [][]ContentTask{t.Carousel, t.Detail} {
		definitions := CarouselContentTasks()
		if index == 1 {
			definitions = DetailContentTasks()
		}
		known := make(map[string]bool, len(definitions))
		for _, definition := range definitions {
			known[definition.Purpose] = true
		}
		purposes := make(map[string]bool)
		for _, task := range group {
			if !contentTaskID.MatchString(task.ID) || seen[task.ID] || !ValidSetText(task.Brief, MaxSetBriefBytes) {
				return fmt.Errorf("%w: invalid, duplicate or oversized image task", ErrInvalid)
			}
			seen[task.ID] = true
			if t.Mode == "custom" {
				if task.Purpose != "custom" || strings.TrimSpace(task.Brief) == "" {
					return fmt.Errorf("%w: custom image task requires an instruction", ErrInvalid)
				}
			} else {
				if !known[task.Purpose] || purposes[task.Purpose] {
					return fmt.Errorf("%w: invalid or repeated content task", ErrInvalid)
				}
				purposes[task.Purpose] = true
			}
		}
	}
	raw, err := json.Marshal(t)
	if err != nil || len(raw) > MaxSetTemplateBytes {
		return fmt.Errorf("%w: image template exceeds the byte limit", ErrInvalid)
	}
	return nil
}

func CloneSetTemplate(t SetTemplate) SetTemplate {
	t.Carousel = append([]ContentTask(nil), t.Carousel...)
	t.Detail = append([]ContentTask(nil), t.Detail...)
	return t
}

func RequiredTaskEvidence(purpose string) string {
	for _, group := range [][]ContentTaskDefinition{CarouselContentTasks(), DetailContentTasks()} {
		for _, task := range group {
			if task.Purpose == purpose {
				return task.Evidence
			}
		}
	}
	return ""
}
