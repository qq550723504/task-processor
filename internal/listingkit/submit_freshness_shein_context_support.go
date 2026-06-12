package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

func joinReviewNotes(notes []string) string {
	return strings.Join(uniqueNonEmptyStrings(notes), "；")
}

func sheinFreshnessCanonicalProduct(task *Task) *canonical.Product {
	if task == nil || task.Result == nil {
		return nil
	}
	if task.Result.CanonicalProduct != nil {
		return task.Result.CanonicalProduct
	}
	if task.Result.StandardProductSnapshot != nil {
		return task.Result.StandardProductSnapshot.CanonicalProduct
	}
	return nil
}

func sheinFreshnessImageAssets(task *Task) *productimage.ImageProcessResult {
	if task == nil || task.Result == nil {
		return nil
	}
	if task.Result.ImageAssets != nil {
		return task.Result.ImageAssets
	}
	if task.Result.StandardProductSnapshot != nil {
		return task.Result.StandardProductSnapshot.ImageAssets
	}
	return nil
}

func cloneSheinPackageForFreshness(pkg *SheinPackage) (*SheinPackage, error) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil, nil
	}
	raw, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	var cloned SheinPackage
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return sheinpub.NormalizePackageSemanticFields(&cloned), nil
}

func (s *service) loadSheinAttributeTemplatesForFreshness(
	ctx context.Context,
	task *Task,
	categoryID int,
) (*sheinattribute.AttributeTemplateInfo, error) {
	if categoryID <= 0 {
		return nil, fmt.Errorf("missing SHEIN category_id")
	}
	api, err := s.buildSheinAttributeAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	return api.GetAttributeTemplates(categoryID)
}

func (s *service) loadSheinCategoryInfoForFreshness(
	ctx context.Context,
	task *Task,
	categoryID int,
) (*sheincategory.CategoryInfo, error) {
	if categoryID <= 0 {
		return nil, fmt.Errorf("missing SHEIN category_id")
	}
	api, err := s.buildSheinCategoryAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	return api.GetCategory(categoryID)
}

func sheinParseCascadeValueIDs(raw *string) map[int]struct{} {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return nil
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == ' ' || r == '\n' || r == '\t'
	})
	if len(fields) == 0 {
		return nil
	}
	result := make(map[int]struct{}, len(fields))
	for _, field := range fields {
		id, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || id <= 0 {
			continue
		}
		result[id] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

type sheinFreshnessRuntimeClientFactory struct {
	svc  *service
	task *Task
}

func (f sheinFreshnessRuntimeClientFactory) NewAPIClient(ctx context.Context, storeID int64) *SheinRuntimeAPIClient {
	if f.svc == nil || f.task == nil {
		return nil
	}
	client, resolvedStoreID, err := f.svc.newSheinAPIClient(ctx, f.task)
	if err != nil {
		return nil
	}
	if storeID > 0 && resolvedStoreID > 0 && resolvedStoreID != storeID {
		return nil
	}
	return client
}
