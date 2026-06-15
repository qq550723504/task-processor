package listingkit

import "testing"

func TestTaskStudioSessionServiceSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "task_studio_session_service.go")
	assertSourceContainsAll(t, rootSource, []string{
		"type taskStudioSessionServiceConfig struct {",
		"type taskStudioSessionService struct {",
		"func newTaskStudioSessionService(config taskStudioSessionServiceConfig) *taskStudioSessionService {",
		"func (s *taskStudioSessionService) EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error) {",
		"func (s *taskStudioSessionService) GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error) {",
		"func (s *taskStudioSessionService) UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error) {",
		"func (s *taskStudioSessionService) SyncStudioDesignAsyncJob(",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func validateStudioSessionExpectedUpdatedAt(current time.Time, expected *string) error {",
		"func studioSessionDetailWithoutDesigns(session *SheinStudioSession) *SheinStudioSessionDetail {",
		"func isStudioSessionGenerationMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {",
		"func isStudioSessionReviewTaskMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {",
		"func (s *taskStudioSessionService) ensureRunner() {",
		"func studioSessionLogFields(ctx context.Context, fields logrus.Fields) logrus.Fields {",
	})

	supportSource := readTaskGenerationSourceFile(t, "task_studio_session_service_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func validateStudioSessionExpectedUpdatedAt(current time.Time, expected *string) error {",
		"func studioSessionDetailWithoutDesigns(session *SheinStudioSession) *SheinStudioSessionDetail {",
		"func isStudioSessionGenerationMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {",
		"func isStudioSessionReviewTaskMetadataOnlyUpdate(req *UpdateStudioSessionRequest) bool {",
		"func (s *taskStudioSessionService) ensureRunner() {",
		"func studioSessionLogFields(ctx context.Context, fields logrus.Fields) logrus.Fields {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func newTaskStudioSessionService(config taskStudioSessionServiceConfig) *taskStudioSessionService {",
		"func (s *taskStudioSessionService) EnsureStudioSession(ctx context.Context, req *EnsureStudioSessionRequest) (*SheinStudioSessionDetail, error) {",
		"func (s *taskStudioSessionService) GetStudioSession(ctx context.Context, sessionID string) (*SheinStudioSessionDetail, error) {",
		"func (s *taskStudioSessionService) UpdateStudioSession(ctx context.Context, sessionID string, req *UpdateStudioSessionRequest) (*SheinStudioSessionDetail, error) {",
		"func (s *taskStudioSessionService) SyncStudioDesignAsyncJob(",
	})
}
