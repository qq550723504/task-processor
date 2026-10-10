package goods

import (
	"errors"
	"fmt"
	model "task-processor/internal/marketplace/shein/model"
)

const OfficialImageConstraintVersion = "shein-images-v1"

var errImageRequirements = errors.New("official image requirements are unavailable")

type OfficialImageTypeRequirement struct {
	Type    int `json:"type"`
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}

type OfficialImageGroupRequirement struct {
	Group       string `json:"group"`
	SKC, SKU    int
	MainWhenAny bool                           `json:"main_when_any"`
	Types       []OfficialImageTypeRequirement `json:"types"`
}

type OfficialImageRequirements struct {
	Version   string                          `json:"version"`
	NewScheme bool                            `json:"new_scheme"`
	Groups    []OfficialImageGroupRequirement `json:"groups"`
}

type OfficialImageDimensions struct {
	AssetID       string
	Width, Height int
}

func (r OfficialImageRequirements) AllowsSlot(slot OfficialImageSlot) bool {
	if r.Version != OfficialImageConstraintVersion || slot.Sort < 1 || slot.Sort > 100 || slot.Type == 1 && slot.Sort != 1 {
		return false
	}
	for _, group := range r.Groups {
		if group.Group != slot.Group || group.SKC != slot.SKC || group.SKU != slot.SKU {
			continue
		}
		if slot.Group == "sku" && slot.Sort != 1 {
			return false
		}
		for _, imageType := range group.Types {
			if imageType.Type == slot.Type && imageType.Maximum > 0 {
				return true
			}
		}
	}
	return false
}
func (r OfficialImageRequirements) ValidateSelection(slots []OfficialImageSlot, dimensions []OfficialImageDimensions) []OfficialIssue {
	return r.validateSelection(slots, dimensions, true)
}
func (r OfficialImageRequirements) validateSelection(slots []OfficialImageSlot, dimensions []OfficialImageDimensions, complete bool) []OfficialIssue {
	issues := []OfficialIssue{}
	issue := func(group OfficialImageSlot, code, message string) {
		issues = append(issues, OfficialIssue{Field: officialImageGroupPath(group.Group, group.SKC, group.SKU), Code: code, Message: message})
	}
	if r.Version != OfficialImageConstraintVersion || len(slots) > 10000 {
		return []OfficialIssue{{Field: "image_info", Code: "rule_unavailable", Message: "当前图片规则不可用"}}
	}
	observed := make(map[string]OfficialImageDimensions, len(dimensions))
	for _, dimension := range dimensions {
		if _, duplicate := observed[dimension.AssetID]; duplicate {
			return []OfficialIssue{{Field: "image_info", Code: "invalid", Message: "图片身份重复"}}
		}
		observed[dimension.AssetID] = dimension
	}
	positions := map[string]bool{}
	counts := map[string]map[int]int{}
	for _, slot := range slots {
		if !r.AllowsSlot(slot) {
			issue(slot, "invalid", "图片位置、类型或顺序不符合当前规范")
			continue
		}
		key := officialImageGroupPath(slot.Group, slot.SKC, slot.SKU)
		position := fmt.Sprintf("%s/%d", key, slot.Sort)
		if positions[position] {
			issue(slot, "invalid", "同一图片组的顺序不得重复")
		}
		positions[position] = true
		dimension, ok := observed[slot.AssetID]
		if !ok || !OfficialImageSizeAllowed(slot.Group, slot.Type, dimension.Width, dimension.Height) {
			issue(slot, "image_dimensions", "图片真实尺寸不符合所选位置规范")
		}
		if counts[key] == nil {
			counts[key] = map[int]int{}
		}
		counts[key][slot.Type]++
	}
	for _, group := range r.Groups {
		key := officialImageGroupPath(group.Group, group.SKC, group.SKU)
		actual := counts[key]
		total := 0
		for _, count := range actual {
			total += count
		}
		invalid := complete && group.MainWhenAny && total > 0 && actual[1] != 1
		for _, imageType := range group.Types {
			if complete && actual[imageType.Type] < imageType.Minimum || actual[imageType.Type] > imageType.Maximum {
				invalid = true
			}
		}
		if invalid {
			issue(OfficialImageSlot{Group: group.Group, SKC: group.SKC, SKU: group.SKU}, "missing_images", "按当前类目和变体规范补齐图片，并移除不允许的类型")
		}
	}
	return issues
}

// Approval stores selected material. Listing checks whether its explicit
// references fill every required publish position, including shared assets.
func (r OfficialImageRequirements) ValidateMaterialSelection(slots []OfficialImageSlot, dimensions []OfficialImageDimensions) []OfficialIssue {
	return r.validateSelection(slots, dimensions, false)
}

func officialImageGroupPath(group string, skc, sku int) string {
	switch group {
	case "skc":
		return fmt.Sprintf("skc_list.%d.image_info", skc)
	case "sku":
		return fmt.Sprintf("skc_list.%d.sku_list.%d.image_info", skc, sku)
	case "detail":
		return fmt.Sprintf("skc_list.%d.site_detail_image_info_list", skc)
	}
	return "image_info"
}

func ResolveOfficialImageRequirements(product model.PublishProduct, fill model.FillStandards, selected []OfficialImageSlot) (OfficialImageRequirements, error) {
	flags, err := officialPictureRules(fill)
	if err != nil || len(product.SKCs) == 0 || len(product.SKCs) > 1000 || len(selected) > 10000 {
		return OfficialImageRequirements{}, errImageRequirements
	}
	_, newScheme := flags["spu_image_detail_show"]
	result := OfficialImageRequirements{Version: OfficialImageConstraintVersion, NewScheme: newScheme}
	if newScheme {
		result.Groups = append(result.Groups, officialGroupRequirement("spu", 0, 0, spuImagePolicy(flags), true))
	}
	for i, skc := range product.SKCs {
		if len(skc.SKUs) == 0 || len(skc.SKUs) > 1000 {
			return OfficialImageRequirements{}, errImageRequirements
		}
		result.Groups = append(result.Groups, officialGroupRequirement("skc", i, 0, skcImagePolicy(flags, newScheme, len(product.SKCs) > 1), false))
		allSKURequired := officialSKUImagesRequired(flags, skc, i, selected)
		for j := range skc.SKUs {
			result.Groups = append(result.Groups, officialGroupRequirement("sku", i, j, imageGroupPolicy{mainRequired: allSKURequired, single: true}, false))
		}
		result.Groups = append(result.Groups, OfficialImageGroupRequirement{Group: "detail", SKC: i, Types: []OfficialImageTypeRequirement{{Type: 7, Maximum: 10}}})
	}
	for _, group := range result.Groups {
		for _, imageType := range group.Types {
			if imageType.Minimum > imageType.Maximum {
				return OfficialImageRequirements{}, errImageRequirements
			}
		}
	}
	return result, nil
}

// This is the publishing contract, shared by plan compatibility checks and the
// final Marketplace builder. It does not infer any unsupported provider resize.
func OfficialImageSizeAllowed(group string, imageType, width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	switch imageType {
	case 1, 2:
		return width == 1340 && height == 1785 || width == height && width >= 900 && width <= 2200
	case 5:
		if group == "spu" {
			return width == 1200 && height == 1200
		}
		return width == height && width >= 900 && width <= 2200
	case 6:
		return width == 80 && height == 80
	case 7:
		return width > 900 && height > 900 && width%3 == 0 && height%4 == 0 && width/3 == height/4
	}
	return false
}

func officialPictureRules(fill model.FillStandards) (map[string]bool, error) {
	flags := make(map[string]bool, len(fill.Pictures))
	for _, rule := range fill.Pictures {
		if _, duplicate := flags[rule.Field]; duplicate || rule.Enabled == nil || rule.Field == "" {
			return nil, errImageRequirements
		}
		flags[rule.Field] = *rule.Enabled
	}
	if _, ok := flags["sku_image_required"]; !ok {
		return nil, errImageRequirements
	}
	if _, newScheme := flags["spu_image_detail_show"]; newScheme {
		for _, field := range []string{"spu_image_detail_required", "spu_image_detail_single", "spu_image_square_show", "spu_image_square_required", "skc_image_detail_show", "skc_image_detail_required", "skc_image_detail_single", "skc_image_square_show", "skc_image_square_required"} {
			if _, ok := flags[field]; !ok {
				return nil, errImageRequirements
			}
		}
	}
	return flags, nil
}

type imageGroupPolicy struct{ mainRequired, detailRequired, detailAllowed, squareRequired, squareAllowed, pieceRequired, single bool }

func spuImagePolicy(flags map[string]bool) imageGroupPolicy {
	return imageGroupPolicy{mainRequired: flags["spu_image_detail_required"], detailRequired: flags["spu_image_detail_required"] && !flags["spu_image_detail_single"], detailAllowed: flags["spu_image_detail_show"], squareRequired: flags["spu_image_square_required"], squareAllowed: flags["spu_image_square_show"], single: flags["spu_image_detail_single"]}
}

func skcImagePolicy(flags map[string]bool, newScheme, pieceRequired bool) imageGroupPolicy {
	if !newScheme {
		return imageGroupPolicy{true, true, true, true, true, pieceRequired, false}
	}
	return imageGroupPolicy{true, flags["skc_image_detail_required"] && !flags["skc_image_detail_single"], flags["skc_image_detail_show"], flags["skc_image_square_required"], flags["skc_image_square_show"], pieceRequired, flags["skc_image_detail_single"]}
}

func officialSKUImagesRequired(flags map[string]bool, skc model.ProductSKC, index int, selected []OfficialImageSlot) bool {
	if flags["sku_image_required"] {
		return true
	}
	for _, sku := range skc.SKUs {
		if sku.Quantity != nil && sku.Quantity.Quantity >= 2 {
			return true
		}
	}
	for _, slot := range selected {
		if slot.Group == "sku" && slot.SKC == index {
			return true
		}
	}
	return false
}

func officialGroupRequirement(group string, skc, sku int, policy imageGroupPolicy, mainWhenAny bool) OfficialImageGroupRequirement {
	minimum := func(required bool) int {
		if required {
			return 1
		}
		return 0
	}
	maximum := func(allowed bool, value int) int {
		if allowed {
			return value
		}
		return 0
	}
	types := []OfficialImageTypeRequirement{{1, minimum(policy.mainRequired), 1}}
	if group != "sku" {
		types = append(types, OfficialImageTypeRequirement{2, minimum(policy.detailRequired), maximum(policy.detailAllowed && !policy.single, 10)}, OfficialImageTypeRequirement{5, minimum(policy.squareRequired), maximum(policy.squareAllowed, 1)})
		if group == "skc" {
			types = append(types, OfficialImageTypeRequirement{6, minimum(policy.pieceRequired), 1})
		}
	}
	return OfficialImageGroupRequirement{Group: group, SKC: skc, SKU: sku, MainWhenAny: mainWhenAny, Types: types}
}

func imageGroupViolation(counts map[int]int, policy imageGroupPolicy) bool {
	return counts[1] > 1 || policy.mainRequired && counts[1] != 1 || counts[2] > 10 || policy.detailRequired && counts[2] < 1 || !policy.detailAllowed && counts[2] > 0 || counts[5] > 1 || policy.squareRequired && counts[5] != 1 || !policy.squareAllowed && counts[5] > 0 || counts[6] > 1 || policy.pieceRequired && counts[6] != 1 || policy.single && counts[2] > 0
}
