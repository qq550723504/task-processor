package shein

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit/tenantctx"
	common "task-processor/internal/publishing/common"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
)

type stubChatCompleter struct {
	response *openaiclient.ChatCompletionResponse
	err      error
	lastReq  *openaiclient.ChatCompletionRequest
}

func (s stubChatCompleter) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	return s.response, nil
}

func (s stubChatCompleter) Generate(ctx context.Context, prompt string) (string, error) {
	return "", errors.New("not implemented")
}

func (s stubChatCompleter) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	return "", errors.New("not implemented")
}

func (s stubChatCompleter) GetDefaultModel() string {
	return "test-model"
}

type recordingChatCompleter struct {
	response *openaiclient.ChatCompletionResponse
	lastReq  *openaiclient.ChatCompletionRequest
}

func (s *recordingChatCompleter) CreateChatCompletion(ctx context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	s.lastReq = req
	return s.response, nil
}

func (s *recordingChatCompleter) Generate(context.Context, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *recordingChatCompleter) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *recordingChatCompleter) GetDefaultModel() string {
	return "test-model"
}

type stubTranslateAPI struct{}

func (stubTranslateAPI) Translate(text string, from, to string) (string, error) {
	return "Spanish " + text, nil
}

type stubSensitiveWordRepository struct {
	pages   map[int64][]listingadmin.SensitiveWord
	created []listingadmin.SensitiveWord
	updated []listingadmin.SensitiveWord
}

func (s *stubSensitiveWordRepository) ListSensitiveWords(ctx context.Context, query listingadmin.SensitiveWordQuery) (*listingadmin.SensitiveWordPage, error) {
	items := append([]listingadmin.SensitiveWord(nil), s.pages[query.TenantID]...)
	if query.Word != "" {
		filtered := items[:0]
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Word), strings.ToLower(query.Word)) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if query.Status != nil {
		filtered := items[:0]
		for _, item := range items {
			if item.Status == *query.Status {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
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
	return &listingadmin.SensitiveWordPage{
		Items:    append([]listingadmin.SensitiveWord(nil), items[start:end]...),
		Total:    int64(len(items)),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *stubSensitiveWordRepository) GetSensitiveWord(context.Context, int64, int64) (*listingadmin.SensitiveWord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSensitiveWordRepository) CreateSensitiveWord(_ context.Context, word *listingadmin.SensitiveWord) (*listingadmin.SensitiveWord, error) {
	if word == nil {
		return nil, errors.New("word is nil")
	}
	created := *word
	created.ID = int64(len(s.created) + len(s.pages[created.TenantID]) + 1)
	s.created = append(s.created, created)
	s.pages[created.TenantID] = append(s.pages[created.TenantID], created)
	return &created, nil
}

func (s *stubSensitiveWordRepository) UpdateSensitiveWord(_ context.Context, word *listingadmin.SensitiveWord) (*listingadmin.SensitiveWord, error) {
	if word == nil {
		return nil, errors.New("word is nil")
	}
	updated := *word
	s.updated = append(s.updated, updated)
	items := s.pages[updated.TenantID]
	for i := range items {
		if items[i].ID == updated.ID {
			items[i] = updated
			s.pages[updated.TenantID] = items
			return &updated, nil
		}
	}
	s.pages[updated.TenantID] = append(s.pages[updated.TenantID], updated)
	return &updated, nil
}

func (s *stubSensitiveWordRepository) UpdateSensitiveWordStatus(context.Context, int64, int64, int16, string) (*listingadmin.SensitiveWord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubSensitiveWordRepository) DeleteSensitiveWord(context.Context, int64, int64) error {
	return errors.New("not implemented")
}

func TestPrepareSubmitProductContent_FallsBackWhenAIUnavailable(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     " Door curtain for home decor ",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     " A soft curtain for bedrooms and living rooms. ",
		}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}

	err := PrepareSubmitProductContent(context.Background(), product, "US", stubChatCompleter{err: errors.New("ai down")}, nil)
	if err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}

	if got := findLocalizedText(product.MultiLanguageNameList, "en"); got != "Door curtain for home decor" {
		t.Fatalf("english title = %q, want %q", got, "Door curtain for home decor")
	}
	if got := findLocalizedText(product.MultiLanguageDescList, "en"); got != "A soft curtain for bedrooms and living rooms." {
		t.Fatalf("english description = %q, want %q", got, "A soft curtain for bedrooms and living rooms.")
	}
	if got := findLocalizedText(product.MultiLanguageNameList, "es"); got != "Door curtain for home decor" {
		t.Fatalf("spanish title fallback = %q, want %q", got, "Door curtain for home decor")
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en"); !strings.EqualFold(got, "door curtain for home decor white") {
		t.Fatalf("english skc fallback = %q, want case-insensitive match for %q", got, "door curtain for home decor white")
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "es"); !strings.EqualFold(got, "door curtain for home decor white") {
		t.Fatalf("spanish skc fallback = %q, want case-insensitive match for %q", got, "door curtain for home decor white")
	}
}

func TestPrepareSubmitProductContent_UsesTranslateAPIForMissingTargets(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Door curtain for home decor",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "A soft curtain for bedrooms and living rooms.",
		}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}

	err := PrepareSubmitProductContent(context.Background(), product, "US", nil, stubTranslateAPI{})
	if err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}

	if got := findLocalizedText(product.MultiLanguageNameList, "es"); got != "Spanish Door curtain for home decor" {
		t.Fatalf("spanish title = %q, want %q", got, "Spanish Door curtain for home decor")
	}
	if got := findLocalizedText(product.MultiLanguageDescList, "es"); got != "Spanish A soft curtain for bedrooms and living rooms." {
		t.Fatalf("spanish description = %q, want %q", got, "Spanish A soft curtain for bedrooms and living rooms.")
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "es"); !strings.EqualFold(got, "Spanish door curtain for home decor white") {
		t.Fatalf("spanish skc = %q, want case-insensitive match for %q", got, "Spanish door curtain for home decor white")
	}
	if got := product.SKCList[0].MultiLanguageName; got.Language != "en" || !strings.EqualFold(got.Name, "door curtain for home decor white") {
		t.Fatalf("primary skc name = %+v, want english primary name", got)
	}
}

func TestOptimizeSubmitContentWithAI_SendsMainImageToAI(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Door curtain",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Soft decorative curtain for bedrooms.",
		}},
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: "https://example.com/main.jpg"},
				{ImageURL: "https://example.com/gallery.jpg"},
			},
		},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}
	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Elegant Door Curtain for Bedroom Privacy and Home Decor Styling","description":"A soft decorative door curtain designed to add privacy, texture, and a warm finished look to bedrooms and living spaces."}`,
				},
			}},
		},
	}

	title, description, err := optimizeSubmitContentWithAI(
		context.Background(),
		ai,
		findLocalizedText(product.MultiLanguageNameList, "en"),
		findLocalizedText(product.MultiLanguageDescList, "en"),
		buildSubmitContentFeatures(product),
		collectSubmitContentImageURLs(product),
	)
	if err != nil {
		t.Fatalf("optimizeSubmitContentWithAI returned error: %v", err)
	}
	if title == "" || description == "" {
		t.Fatalf("optimized content = %q / %q, want non-empty", title, description)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) < 2 {
		t.Fatalf("ai request = %+v, want multimodal user message", ai.lastReq)
	}
	parts := ai.lastReq.Messages[1].MultiContent
	if len(parts) != 2 {
		t.Fatalf("user multi-content parts = %+v, want text + main image", parts)
	}
	if parts[0].Type != "text" {
		t.Fatalf("first part type = %q, want text", parts[0].Type)
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "https://example.com/main.jpg" {
		t.Fatalf("image part = %+v, want main image only", parts[1])
	}
}

func TestOptimizeSubmitContentWithAI_IncludesTenantGenerationPolicyText(t *testing.T) {
	restoreRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: []string{"children", "rock", "baby"},
		},
	})
	defer restoreRepo()

	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Door Curtain","description":"A door curtain."}`,
				},
			}},
		},
	}

	_, _, err := optimizeSubmitContentWithAI(
		tenantctx.WithTenantID(context.Background(), "101"),
		ai,
		"Door curtain",
		"Soft curtain",
		"Category id: 1",
		nil,
	)
	if err != nil {
		t.Fatalf("optimizeSubmitContentWithAI returned error: %v", err)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) == 0 {
		t.Fatalf("ai request = %+v, want system prompt", ai.lastReq)
	}
	systemPrompt := ai.lastReq.Messages[0].Content
	if !strings.Contains(systemPrompt, "Additional tenant content restrictions:") {
		t.Fatalf("system prompt = %q, want tenant policy header", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("system prompt = %q, want children directive", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Do not mention babies, newborns, or infant-specific usage.") {
		t.Fatalf("system prompt = %q, want baby directive", systemPrompt)
	}
	if strings.Contains(systemPrompt, "rock") {
		t.Fatalf("system prompt = %q, want unknown topic keys omitted", systemPrompt)
	}
}

func TestExtractListingTitleAdditionWithLLM_IncludesTenantGenerationPolicyText(t *testing.T) {
	restoreRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: []string{"children", "rock", "baby"},
		},
	})
	defer restoreRepo()

	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"addition":"Rock Typography Graphic Print"}`,
				},
			}},
		},
	}

	addition := extractListingTitleAdditionWithLLM(
		tenantctx.WithTenantID(context.Background(), "101"),
		"Door curtain",
		&canonical.Product{
			Attributes: map[string]canonical.Attribute{
				"ai_style":        {Value: "Please design a rock style door curtain"},
				"picture_request": {Value: "Please design a rock print"},
			},
		},
		"Door curtain",
		ai,
	)
	if addition != "Rock Typography Graphic Print" {
		t.Fatalf("addition = %q, want extracted addition", addition)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) == 0 {
		t.Fatalf("ai request = %+v, want system prompt", ai.lastReq)
	}
	systemPrompt := ai.lastReq.Messages[0].Content
	if !strings.Contains(systemPrompt, "Additional tenant content restrictions:") {
		t.Fatalf("system prompt = %q, want tenant policy header", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("system prompt = %q, want children directive", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Do not mention babies, newborns, or infant-specific usage.") {
		t.Fatalf("system prompt = %q, want baby directive", systemPrompt)
	}
	if strings.Contains(systemPrompt, "rock") {
		t.Fatalf("system prompt = %q, want unknown topic keys omitted", systemPrompt)
	}
}

func TestExtractPromptTitleWithLLM_IncludesTenantGenerationPolicyText(t *testing.T) {
	restoreRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: []string{"children", "rock", "baby"},
		},
	})
	defer restoreRepo()

	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Floral Door Curtain"}`,
				},
			}},
		},
	}

	title := extractPromptTitleWithLLM(
		tenantctx.WithTenantID(context.Background(), "101"),
		"Please design a floral door curtain print with dramatic text and graphics, 3000px",
		nil,
		"Door curtain",
		ai,
	)
	if title != "Floral Door Curtain" {
		t.Fatalf("title = %q, want extracted title", title)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) == 0 {
		t.Fatalf("ai request = %+v, want system prompt", ai.lastReq)
	}
	systemPrompt := ai.lastReq.Messages[0].Content
	if !strings.Contains(systemPrompt, "Additional tenant content restrictions:") {
		t.Fatalf("system prompt = %q, want tenant policy header", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Do not mention children, babies, or age-specific users.") {
		t.Fatalf("system prompt = %q, want children directive", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Do not mention babies, newborns, or infant-specific usage.") {
		t.Fatalf("system prompt = %q, want baby directive", systemPrompt)
	}
	if strings.Contains(systemPrompt, "rock") {
		t.Fatalf("system prompt = %q, want unknown topic keys omitted", systemPrompt)
	}
}

func TestPromptEntryPointsOmitTenantPolicyWithoutTenantContext(t *testing.T) {
	restoreRepo := SetGenerationTopicPolicyRepository(&stubGenerationTopicPolicyRepository{
		keys: map[int64][]string{
			101: []string{"children"},
		},
	})
	defer restoreRepo()

	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Door Curtain","description":"A door curtain."}`,
				},
			}},
		},
	}

	if _, _, err := optimizeSubmitContentWithAI(context.Background(), ai, "Door curtain", "Soft curtain", "", nil); err != nil {
		t.Fatalf("optimizeSubmitContentWithAI returned error: %v", err)
	}
	if ai.lastReq == nil || len(ai.lastReq.Messages) == 0 {
		t.Fatalf("ai request = %+v, want system prompt", ai.lastReq)
	}
	if strings.Contains(ai.lastReq.Messages[0].Content, "Additional tenant content restrictions:") {
		t.Fatalf("system prompt = %q, want no tenant policy block without tenant context", ai.lastReq.Messages[0].Content)
	}
}

func TestPrepareSubmitProductContent_PreservesExistingContentWithoutAIRewrite(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Envelope style pillow cover",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Envelope style pillow cover designed for everyday home decor.",
		}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "beige"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "beige",
			}},
		}},
	}
	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Unexpected rewrite","description":"Unexpected rewrite"}`,
				},
			}},
		},
	}

	if err := PrepareSubmitProductContent(context.Background(), product, "US", ai, nil); err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}
	if ai.lastReq != nil {
		t.Fatalf("ai request = %+v, want submit content to skip AI rewrite", ai.lastReq)
	}
	if got := findLocalizedText(product.MultiLanguageNameList, "en"); got != "Envelope style pillow cover" {
		t.Fatalf("english title = %q, want original reviewed content", got)
	}
	if got := findLocalizedText(product.MultiLanguageDescList, "en"); got != "Envelope style pillow cover designed for everyday home decor." {
		t.Fatalf("english description = %q, want original reviewed content", got)
	}
	if got := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en"); !strings.EqualFold(got, "envelope style pillow cover beige") {
		t.Fatalf("english skc = %q, want case-insensitive match for %q", got, "envelope style pillow cover beige")
	}
}

func TestApplySubmitContent_TruncatesTitleToSheinLimit(t *testing.T) {
	t.Parallel()

	title := strings.Repeat("A", sheinSubmitTitleMaxLength+20)
	description := strings.Repeat("B", sheinSubmitDescriptionMaxLength+50)
	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "white",
			}},
		}},
	}

	applySubmitContent(product, title, description)

	gotTitle := findLocalizedText(product.MultiLanguageNameList, "en")
	if len(gotTitle) != sheinSubmitTitleMaxLength {
		t.Fatalf("title length = %d, want %d", len(gotTitle), sheinSubmitTitleMaxLength)
	}
	gotDescription := findLocalizedText(product.MultiLanguageDescList, "en")
	if len(gotDescription) != sheinSubmitDescriptionMaxLength {
		t.Fatalf("description length = %d, want %d", len(gotDescription), sheinSubmitDescriptionMaxLength)
	}
	gotSKCTitle := findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en")
	if len(gotSKCTitle) > sheinSubmitTitleMaxLength {
		t.Fatalf("skc title length = %d, want <= %d", len(gotSKCTitle), sheinSubmitTitleMaxLength)
	}
}

func TestBuildSubmitSnapshot_CapturesFinalPayloadFields(t *testing.T) {
	supplierCode := "SKC-1"
	product := &sheinproduct.Product{
		SPUName:      "SPU-123",
		SupplierCode: "SUP-001",
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: "https://example.com/1.jpg"},
				{ImageURL: "https://example.com/2.jpg"},
			},
		},
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Door curtain"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Soft curtain"}},
		SKCList: []sheinproduct.SKC{{
			SupplierCode:      &supplierCode,
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{
				{Language: "en", Name: "white"},
				{Language: "es", Name: "blanco"},
			},
		}},
	}

	snapshot := BuildSubmitSnapshot(product)
	if snapshot == nil {
		t.Fatal("BuildSubmitSnapshot returned nil")
	}
	if snapshot.SPUName != "SPU-123" || snapshot.SupplierCode != "SUP-001" {
		t.Fatalf("snapshot header = %+v", snapshot)
	}
	if snapshot.ImageCount != 2 {
		t.Fatalf("image count = %d, want 2", snapshot.ImageCount)
	}
	if len(snapshot.SKCList) != 1 {
		t.Fatalf("skc snapshot count = %d, want 1", len(snapshot.SKCList))
	}
	if snapshot.SKCList[0].SupplierCode != "SKC-1" || snapshot.SKCList[0].PrimaryName != "white" {
		t.Fatalf("skc snapshot = %+v", snapshot.SKCList[0])
	}
}

func TestRetrySensitiveWordCleanup_RemovesFlaggedWord(t *testing.T) {
	restore := overrideSensitiveWordsConfigForTest(t)
	defer restore()

	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy curtain for home decor"}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName:     sheinproduct.LanguageContent{Language: "en", Name: "whimsy white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "whimsy white"}},
		}},
	}

	if !RetrySensitiveWordCleanup(context.Background(), product, []string{"敏感词：whimsy"}) {
		t.Fatal("expected sensitive-word retry cleanup to return true")
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english title still contains whimsy: %+v", product.MultiLanguageNameList)
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageDescList, "en")), "whimsy") {
		t.Fatalf("english description still contains whimsy: %+v", product.MultiLanguageDescList)
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english skc still contains whimsy: %+v", product.SKCList[0].MultiLanguageNameList)
	}
}

func TestRetrySensitiveWordCleanup_PersistsNewValidationWordsToTenantRepository(t *testing.T) {
	restoreConfig := writeSensitiveWordsConfigForTest(t, `{
  "static_words": {},
  "dynamic_words": {},
  "last_updated": "2026-06-02T00:00:00Z",
  "version": "1.0.0",
  "platform": "shein"
}`)
	defer restoreConfig()
	repo := &stubSensitiveWordRepository{pages: map[int64][]listingadmin.SensitiveWord{}}
	restoreRepo := submitprep.SetSensitiveWordRepository(repo)
	defer restoreRepo()

	ctx := tenantctx.WithTenantID(context.Background(), "101")
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy curtain for home decor"}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName:     sheinproduct.LanguageContent{Language: "en", Name: "whimsy white"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "whimsy white"}},
		}},
	}

	if !RetrySensitiveWordCleanup(ctx, product, []string{"敏感词：[Whimsy]"}) {
		t.Fatal("expected sensitive-word retry cleanup to return true")
	}
	if len(repo.created) != 1 {
		t.Fatalf("created sensitive words = %+v, want 1 record", repo.created)
	}
	if repo.created[0].TenantID != 101 || repo.created[0].Language != "en" || repo.created[0].Word != "whimsy" || repo.created[0].Status != 1 {
		t.Fatalf("created sensitive word = %+v", repo.created[0])
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english title still contains whimsy: %+v", product.MultiLanguageNameList)
	}
}

func TestRetrySensitiveWordCleanup_ReenablesExistingDisabledValidationWord(t *testing.T) {
	restoreConfig := writeSensitiveWordsConfigForTest(t, `{
  "static_words": {},
  "dynamic_words": {},
  "last_updated": "2026-06-02T00:00:00Z",
  "version": "1.0.0",
  "platform": "shein"
}`)
	defer restoreConfig()
	repo := &stubSensitiveWordRepository{
		pages: map[int64][]listingadmin.SensitiveWord{
			101: {{
				ID:       1,
				TenantID: 101,
				Language: "en",
				Word:     "Whimsy",
				Status:   0,
				Tags:     "manual",
			}},
		},
	}
	restoreRepo := submitprep.SetSensitiveWordRepository(repo)
	defer restoreRepo()

	ctx := tenantctx.WithTenantID(context.Background(), "101")
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}},
	}

	if !RetrySensitiveWordCleanup(ctx, product, []string{"敏感词：[Whimsy]"}) {
		t.Fatal("expected sensitive-word retry cleanup to return true")
	}
	if len(repo.created) != 0 {
		t.Fatalf("created sensitive words = %+v, want no new record", repo.created)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("updated sensitive words = %+v, want 1 updated record", repo.updated)
	}
	if repo.updated[0].Status != 1 || !strings.Contains(repo.updated[0].Tags, "validation-retry") {
		t.Fatalf("updated sensitive word = %+v", repo.updated[0])
	}
}

func TestSanitizeDraftPayloadSensitiveContent_CleansDraftTextFields(t *testing.T) {
	restore := writeSensitiveWordsConfigForTest(t, `{
  "static_words": {
    "en": ["bpa free", "amazon"]
  },
  "dynamic_words": {},
  "last_updated": "2026-06-02T00:00:00Z",
  "version": "1.0.0",
  "platform": "shein"
}`)
	defer restore()

	pkg := &Package{
		DraftPayload: &RequestDraft{
			MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Amazon BPA Free Vase"}},
			MultiLanguageDescList: []LocalizedText{{Language: "en", Name: "Amazon BPA Free vase for home decor."}},
			ProductAttributeList: []common.Attribute{
				{Name: "Material Detail", Value: "Amazon BPA Free acrylic"},
				{Name: "Length", Value: "12"},
			},
			SKCList: []SKCRequestDraft{{
				SkcName:               "Amazon BPA Free Blue",
				MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Amazon BPA Free Blue"}},
			}},
		},
	}

	changed := SanitizeDraftPayloadSensitiveContent(pkg, context.Background(), nil)
	if !changed {
		t.Fatal("changed = false, want true")
	}

	assertNoSensitivePhrase(t, firstLocalizedText(pkg.DraftPayload.MultiLanguageNameList), "draft title")
	assertNoSensitivePhrase(t, firstLocalizedText(pkg.DraftPayload.MultiLanguageDescList), "draft description")
	assertNoSensitivePhrase(t, pkg.DraftPayload.SKCList[0].SkcName, "draft skc name")
	assertNoSensitivePhrase(t, pkg.DraftPayload.SKCList[0].MultiLanguageNameList[0].Name, "draft localized skc name")
	assertNoSensitivePhrase(t, pkg.DraftPayload.ProductAttributeList[0].Value, "draft free-text attribute")
	if got := pkg.DraftPayload.ProductAttributeList[1].Value; got != "12" {
		t.Fatalf("structured attribute value = %q, want unchanged", got)
	}
}

func TestPrepareSubmitProductContent_LoadsTenantSensitiveWordsFromRepository(t *testing.T) {
	restoreConfig := writeSensitiveWordsConfigForTest(t, `{
  "static_words": {},
  "dynamic_words": {},
  "last_updated": "2026-06-02T00:00:00Z",
  "version": "1.0.0",
  "platform": "shein"
}`)
	defer restoreConfig()
	restoreRepo := submitprep.SetSensitiveWordRepository(&stubSensitiveWordRepository{
		pages: map[int64][]listingadmin.SensitiveWord{
			101: {{
				TenantID: 101,
				Language: "en",
				Word:     "whimsy",
				Status:   1,
			}},
		},
	})
	defer restoreRepo()

	ctx := tenantctx.WithTenantID(context.Background(), "101")
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Whimsy Door Curtain",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Whimsy curtain for home decor.",
		}},
		ProductAttributeList: []sheinproduct.ProductAttribute{{
			AttributeExtraValue: "Whimsy fabric finish",
		}},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "Whimsy White"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "Whimsy White",
			}},
		}},
	}

	if err := PrepareSubmitProductContent(ctx, product, "US", nil, nil); err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}

	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english title still contains tenant sensitive word: %+v", product.MultiLanguageNameList)
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageDescList, "en")), "whimsy") {
		t.Fatalf("english description still contains tenant sensitive word: %+v", product.MultiLanguageDescList)
	}
	if strings.Contains(strings.ToLower(product.ProductAttributeList[0].AttributeExtraValue), "whimsy") {
		t.Fatalf("attribute still contains tenant sensitive word: %+v", product.ProductAttributeList)
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("skc still contains tenant sensitive word: %+v", product.SKCList[0].MultiLanguageNameList)
	}
}

func TestPrepareSubmitProductContent_CleansFreeTextAttributesAndSKCNames(t *testing.T) {
	restore := writeSensitiveWordsConfigForTest(t, `{
  "static_words": {
    "en": ["bpa free", "amazon"]
  },
  "dynamic_words": {},
  "last_updated": "2026-06-02T00:00:00Z",
  "version": "1.0.0",
  "platform": "shein"
}`)
	defer restore()

	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Amazon BPA Free Vase",
		}},
		MultiLanguageDescList: []sheinproduct.LanguageContent{{
			Language: "en",
			Name:     "Amazon BPA Free vase for home decor.",
		}},
		ProductAttributeList: []sheinproduct.ProductAttribute{
			{AttributeExtraValue: "Amazon BPA Free acrylic"},
			{AttributeExtraValue: "12"},
		},
		SKCList: []sheinproduct.SKC{{
			MultiLanguageName: sheinproduct.LanguageContent{Language: "en", Name: "Amazon BPA Free Blue"},
			MultiLanguageNameList: []sheinproduct.LanguageContent{{
				Language: "en",
				Name:     "Amazon BPA Free Blue",
			}},
		}},
	}

	if err := PrepareSubmitProductContent(context.Background(), product, "US", nil, nil); err != nil {
		t.Fatalf("PrepareSubmitProductContent returned error: %v", err)
	}

	assertNoSensitivePhrase(t, findLocalizedText(product.MultiLanguageNameList, "en"), "submit title")
	assertNoSensitivePhrase(t, findLocalizedText(product.MultiLanguageDescList, "en"), "submit description")
	assertNoSensitivePhrase(t, product.ProductAttributeList[0].AttributeExtraValue, "submit free-text attribute")
	if got := product.ProductAttributeList[1].AttributeExtraValue; got != "12" {
		t.Fatalf("structured submit attribute value = %q, want unchanged", got)
	}
	assertNoSensitivePhrase(t, product.SKCList[0].MultiLanguageName.Name, "submit skc name")
	assertNoSensitivePhrase(t, findLocalizedText(product.SKCList[0].MultiLanguageNameList, "en"), "submit localized skc name")
}

func findLocalizedText(items []sheinproduct.LanguageContent, language string) string {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Language), language) {
			return strings.TrimSpace(item.Name)
		}
	}
	return ""
}
