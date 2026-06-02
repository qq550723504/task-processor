package listingkit

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/listingadmin"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
)

func findSheinLanguageContent(items []sheinproduct.LanguageContent, language string) string {
	return submitprep.FindLanguageContent(items, language)
}

func localizedSubmitSnapshotText(items []sheinpub.LocalizedText, language string) string {
	for _, item := range items {
		if submitprep.NormalizeLanguage(item.Language) == submitprep.NormalizeLanguage(language) {
			return item.Name
		}
	}
	return ""
}

func overrideSensitiveWordsConfigForTest(t *testing.T) func() {
	t.Helper()
	return func() {}
}

type stubListingkitSensitiveWordRepository struct {
	pages map[int64][]listingadmin.SensitiveWord
}

func (s *stubListingkitSensitiveWordRepository) ListSensitiveWords(_ context.Context, query listingadmin.SensitiveWordQuery) (*listingadmin.SensitiveWordPage, error) {
	items := append([]listingadmin.SensitiveWord(nil), s.pages[query.TenantID]...)
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = len(items)
		if pageSize == 0 {
			pageSize = 1
		}
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return &listingadmin.SensitiveWordPage{Items: nil, Total: int64(len(items)), Page: page, PageSize: pageSize}, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	if query.Status != nil {
		filtered := make([]listingadmin.SensitiveWord, 0, end-start)
		for _, item := range items[start:end] {
			if item.Status == *query.Status {
				filtered = append(filtered, item)
			}
		}
		return &listingadmin.SensitiveWordPage{Items: filtered, Total: int64(len(filtered)), Page: page, PageSize: pageSize}, nil
	}
	return &listingadmin.SensitiveWordPage{
		Items:    append([]listingadmin.SensitiveWord(nil), items[start:end]...),
		Total:    int64(len(items)),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *stubListingkitSensitiveWordRepository) GetSensitiveWord(context.Context, int64, int64) (*listingadmin.SensitiveWord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubListingkitSensitiveWordRepository) CreateSensitiveWord(context.Context, *listingadmin.SensitiveWord) (*listingadmin.SensitiveWord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubListingkitSensitiveWordRepository) UpdateSensitiveWord(context.Context, *listingadmin.SensitiveWord) (*listingadmin.SensitiveWord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubListingkitSensitiveWordRepository) UpdateSensitiveWordStatus(context.Context, int64, int64, int16, string) (*listingadmin.SensitiveWord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubListingkitSensitiveWordRepository) DeleteSensitiveWord(context.Context, int64, int64) error {
	return errors.New("not implemented")
}
