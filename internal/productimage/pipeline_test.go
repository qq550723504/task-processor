package productimage_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	productimage "task-processor/internal/productimage"
	"task-processor/internal/productimage/store"
)

type stubAssetPublisher struct{}
type recordingSubmitter struct {
	submitted []string
}

type failingWhiteBackgroundRenderer struct {
	err error
}

func (r *failingWhiteBackgroundRenderer) Render(_ context.Context, _ *productimage.ImageAsset, _ *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return nil, r.err
}

type failingSceneRenderer struct {
	err error
}

func (r *failingSceneRenderer) Render(_ context.Context, _ *productimage.ImageAsset, _ *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	return nil, r.err
}

type failingSubjectExtractor struct {
	err error
}

func (e *failingSubjectExtractor) Extract(_ context.Context, _ string, _ *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return nil, e.err
}

func (s *recordingSubmitter) Submit(taskID string) error {
	s.submitted = append(s.submitted, taskID)
	return nil
}

func (p *stubAssetPublisher) Publish(_ context.Context, _ *productimage.ImageProcessRequest, result *productimage.ImageProcessResult) error {
	if result.MainImage != nil {
		if result.MainImage.Metadata == nil {
			result.MainImage.Metadata = map[string]string{}
		}
		result.MainImage.Metadata["uploaded_url"] = "https://cdn.example.com/main.jpg"
		result.MainImage.URL = result.MainImage.Metadata["uploaded_url"]
	}
	if result.WhiteBgImage != nil {
		if result.WhiteBgImage.Metadata == nil {
			result.WhiteBgImage.Metadata = map[string]string{}
		}
		result.WhiteBgImage.Metadata["uploaded_url"] = "https://cdn.example.com/white.jpg"
		result.WhiteBgImage.URL = result.WhiteBgImage.Metadata["uploaded_url"]
	}
	if result.SubjectCutout != nil {
		if result.SubjectCutout.Metadata == nil {
			result.SubjectCutout.Metadata = map[string]string{}
		}
		result.SubjectCutout.Metadata["uploaded_url"] = "https://cdn.example.com/subject.jpg"
		result.SubjectCutout.URL = result.SubjectCutout.Metadata["uploaded_url"]
	}
	for i := range result.GalleryImages {
		result.GalleryImages[i].URL = fmt.Sprintf("https://cdn.example.com/gallery-%d.jpg", i+1)
	}
	return nil
}

func TestService_ProcessImages_CompatPipeline(t *testing.T) {
	repo := store.NewMemTaskRepository()
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:       repo,
		AssetPublisher: &stubAssetPublisher{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg", "https://example.com/b.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}

	result, err := svc.ProcessImages(context.Background(), task)
	if err != nil {
		t.Fatalf("ProcessImages() error = %v", err)
	}
	storedTask, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if result.SubjectCutout == nil {
		t.Fatal("expected subject cutout")
	}
	if result.MainImage == nil {
		t.Fatal("expected main image")
	}
	if result.WhiteBgImage == nil {
		t.Fatal("expected white background image")
	}
	if len(result.GalleryImages) != 1 {
		t.Fatalf("gallery images = %d, want 1", len(result.GalleryImages))
	}
	if result.Compliance == nil || !result.Compliance.Passed {
		t.Fatal("expected passing compliance report")
	}
	if result.SubjectCutout.Operations[0] != "select_subject" {
		t.Fatalf("unexpected subject cutout operations: %+v", result.SubjectCutout.Operations)
	}
	if _, ok := result.MainImage.Metadata["promo_badge_removed"]; ok {
		t.Fatalf("did not expect promo badge cleanup metadata for clean image: %+v", result.MainImage.Metadata)
	}
	lastOp := result.WhiteBgImage.Operations[len(result.WhiteBgImage.Operations)-1]
	if lastOp != "render_white_bg_placeholder" {
		t.Fatalf("unexpected white background operations: %+v", result.WhiteBgImage.Operations)
	}
	if result.MainImage.URL != "https://cdn.example.com/main.jpg" {
		t.Fatalf("main image URL = %q, want published URL", result.MainImage.URL)
	}
	if result.WhiteBgImage.URL != "https://cdn.example.com/white.jpg" {
		t.Fatalf("white background image URL = %q, want published URL", result.WhiteBgImage.URL)
	}
	if len(result.StageSummaries) == 0 {
		t.Fatal("expected stage summaries")
	}
	if len(result.ImageTraces) == 0 {
		t.Fatal("expected image traces")
	}
	if result.StageSummaries[0].Stage != "parse_source" {
		t.Fatalf("first stage summary = %+v, want parse_source", result.StageSummaries[0])
	}
	foundPublish := false
	for _, trace := range result.ImageTraces {
		if trace.Stage == "publish_assets" && trace.AssetType == string(productimage.AssetTypeMainImage) && trace.Outcome == "success" {
			foundPublish = true
			break
		}
	}
	if !foundPublish {
		t.Fatalf("expected publish_assets success trace, got %+v", result.ImageTraces)
	}
	if result.Review == nil || result.Review.NeedsReview {
		t.Fatalf("expected auto-approved review decision, got %+v", result.Review)
	}
	if result.Quality == nil {
		t.Fatal("expected quality assessment")
	}
	if storedTask.Status != productimage.TaskStatusCompleted {
		t.Fatalf("stored status = %q, want completed", storedTask.Status)
	}
}

func TestService_ReviewTask_ApproveRejectRetry(t *testing.T) {
	repo := store.NewMemTaskRepository()
	submitter := &recordingSubmitter{}
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:       repo,
		TaskSubmitter:  submitter,
		AssetPublisher: &stubAssetPublisher{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg", "https://example.com/b.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}
	if _, err := svc.ProcessImages(context.Background(), task); err != nil {
		t.Fatalf("ProcessImages() error = %v", err)
	}
	if err := repo.MarkNeedsReview(context.Background(), task.ID, &productimage.ImageProcessResult{
		Review: &productimage.ReviewDecision{NeedsReview: true, Reasons: []string{"manual test gate"}},
	}, "manual test gate"); err != nil {
		t.Fatalf("MarkNeedsReview() error = %v", err)
	}

	approved, err := svc.ReviewTask(context.Background(), task.ID, &productimage.ReviewTaskRequest{Action: "approve"})
	if err != nil {
		t.Fatalf("ReviewTask approve error = %v", err)
	}
	if approved.Status != productimage.TaskStatusCompleted {
		t.Fatalf("approve status = %q, want completed", approved.Status)
	}

	reviewTask, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg", "https://example.com/b.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() second error = %v", err)
	}
	if _, err := svc.ProcessImages(context.Background(), reviewTask); err != nil {
		t.Fatalf("ProcessImages() second error = %v", err)
	}
	if err := repo.MarkNeedsReview(context.Background(), reviewTask.ID, &productimage.ImageProcessResult{
		Review: &productimage.ReviewDecision{NeedsReview: true, Reasons: []string{"manual test gate"}},
	}, "manual test gate"); err != nil {
		t.Fatalf("MarkNeedsReview() second error = %v", err)
	}

	rejected, err := svc.ReviewTask(context.Background(), reviewTask.ID, &productimage.ReviewTaskRequest{Action: "reject", Reason: "manual quality rejection"})
	if err != nil {
		t.Fatalf("ReviewTask reject error = %v", err)
	}
	if rejected.Status != productimage.TaskStatusRejected {
		t.Fatalf("reject status = %q, want rejected", rejected.Status)
	}
	if rejected.Error != "manual quality rejection" {
		t.Fatalf("reject error = %q, want manual reason", rejected.Error)
	}

	retried, err := svc.ReviewTask(context.Background(), reviewTask.ID, &productimage.ReviewTaskRequest{Action: "retry"})
	if err != nil {
		t.Fatalf("ReviewTask retry error = %v", err)
	}
	if retried.Status != productimage.TaskStatusPending {
		t.Fatalf("retry status = %q, want pending", retried.Status)
	}
	if len(submitter.submitted) < 3 {
		t.Fatalf("expected submitter to record retries, got %+v", submitter.submitted)
	}
}

func TestService_ProcessImages_ReusesExistingAssetsOnRetry(t *testing.T) {
	repo := store.NewMemTaskRepository()
	submitter := &recordingSubmitter{}
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:              repo,
		TaskSubmitter:         submitter,
		AssetPublisher:        &stubAssetPublisher{},
		CleanupTemporaryFiles: false,
		ReuseExistingAssets:   true,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg", "https://example.com/b.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}
	firstResult, err := svc.ProcessImages(context.Background(), task)
	if err != nil {
		t.Fatalf("first ProcessImages() error = %v", err)
	}

	if err := repo.MarkNeedsReview(context.Background(), task.ID, firstResult, "manual retry gate"); err != nil {
		t.Fatalf("MarkNeedsReview() retry error = %v", err)
	}
	if _, err := svc.ReviewTask(context.Background(), task.ID, &productimage.ReviewTaskRequest{Action: "retry"}); err != nil {
		t.Fatalf("ReviewTask retry error = %v", err)
	}
	retryTask, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	secondResult, err := svc.ProcessImages(context.Background(), retryTask)
	if err != nil {
		t.Fatalf("second ProcessImages() error = %v", err)
	}

	if firstResult.SubjectCutout == nil || secondResult.SubjectCutout == nil {
		t.Fatal("expected subject cutout in both runs")
	}
	if firstResult.SubjectCutout.URL != secondResult.SubjectCutout.URL {
		t.Fatalf("subject cutout URL changed across retry: %q vs %q", firstResult.SubjectCutout.URL, secondResult.SubjectCutout.URL)
	}
	foundReused := false
	for _, trace := range secondResult.ImageTraces {
		if trace.Stage == "extract_subject" && trace.Outcome == "reused" {
			foundReused = true
			break
		}
	}
	if !foundReused {
		t.Fatalf("expected reused trace, got %+v", secondResult.ImageTraces)
	}
}

func TestService_ProcessImages_FlagsImageIPRisk(t *testing.T) {
	repo := store.NewMemTaskRepository()
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:       repo,
		AssetPublisher: &stubAssetPublisher{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/nike_logo_promo.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}

	_, err = svc.ProcessImages(context.Background(), task)
	if err == nil {
		t.Fatal("expected high image IP risk to block processing")
	}
}

func TestService_ProcessImages_DowngradesWhiteBackgroundFailureToNeedsReview(t *testing.T) {
	repo := store.NewMemTaskRepository()
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:         repo,
		AssetPublisher:   &stubAssetPublisher{},
		WhiteBgRenderer:  &failingWhiteBackgroundRenderer{err: fmt.Errorf("white background provider timeout")},
		SceneRenderer:    nil,
		ReviewAssessor:   productimage.NewDefaultReviewAssessor(),
		QualityAssessor:  productimage.NewDefaultQualityAssessor(),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg", "https://example.com/b.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}

	result, err := svc.ProcessImages(context.Background(), task)
	if err != nil {
		t.Fatalf("ProcessImages() error = %v", err)
	}
	if result == nil || result.MainImage == nil {
		t.Fatalf("expected main image to survive white background failure, got %+v", result)
	}
	if result.WhiteBgImage != nil {
		t.Fatalf("expected white background image to be absent after renderer failure, got %+v", result.WhiteBgImage)
	}
	if result.Review == nil || !result.Review.NeedsReview {
		t.Fatalf("expected needs_review decision, got %+v", result.Review)
	}
	if !containsReviewReason(result.Review.Reasons, "render_white_bg") {
		t.Fatalf("expected render_white_bg failure reason, got %+v", result.Review.Reasons)
	}

	storedTask, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if storedTask.Status != productimage.TaskStatusNeedsReview {
		t.Fatalf("stored status = %q, want needs_review", storedTask.Status)
	}
}

func TestService_ProcessImages_DowngradesGalleryFailureToNeedsReview(t *testing.T) {
	repo := store.NewMemTaskRepository()
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:       repo,
		AssetPublisher: &stubAssetPublisher{},
		SceneRenderer:  &failingSceneRenderer{err: fmt.Errorf("scene generation timeout")},
		ReviewAssessor: productimage.NewDefaultReviewAssessor(),
		QualityAssessor: productimage.NewDefaultQualityAssessor(),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg", "https://example.com/b.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}

	result, err := svc.ProcessImages(context.Background(), task)
	if err != nil {
		t.Fatalf("ProcessImages() error = %v", err)
	}
	if result == nil || result.MainImage == nil || result.WhiteBgImage == nil {
		t.Fatalf("expected main and white background images to survive gallery failure, got %+v", result)
	}
	if len(result.GalleryImages) != 0 {
		t.Fatalf("expected gallery images to be absent after scene renderer failure, got %+v", result.GalleryImages)
	}
	if result.Review == nil || !result.Review.NeedsReview {
		t.Fatalf("expected needs_review decision, got %+v", result.Review)
	}
	if !containsReviewReason(result.Review.Reasons, "render_gallery") {
		t.Fatalf("expected render_gallery failure reason, got %+v", result.Review.Reasons)
	}

	storedTask, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if storedTask.Status != productimage.TaskStatusNeedsReview {
		t.Fatalf("stored status = %q, want needs_review", storedTask.Status)
	}
}

func TestService_ProcessImages_StillFailsWhenSubjectExtractionFails(t *testing.T) {
	repo := store.NewMemTaskRepository()
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:          repo,
		AssetPublisher:    &stubAssetPublisher{},
		SubjectExtractor:  &failingSubjectExtractor{err: fmt.Errorf("subject extraction boom")},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/a.jpg", "https://example.com/b.jpg"},
		Marketplace: "amazon",
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}

	result, err := svc.ProcessImages(context.Background(), task)
	if err == nil {
		t.Fatal("expected ProcessImages() to fail when subject extraction fails")
	}
	if result != nil {
		t.Fatalf("expected nil result on subject extraction failure, got %+v", result)
	}

	storedTask, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if storedTask.Status != productimage.TaskStatusFailed {
		t.Fatalf("stored status = %q, want failed", storedTask.Status)
	}
}

func containsReviewReason(reasons []string, needle string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, needle) {
			return true
		}
	}
	return false
}
