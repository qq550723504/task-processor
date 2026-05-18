package updater

import (
	"errors"
	"testing"
)

func TestUpdateManagerCheckAndUpdateUsesAdapterFlow(t *testing.T) {
	t.Parallel()

	latest := &VersionInfo{
		Version:     "1.2.0",
		DownloadURL: "https://example.com/task-processor.exe",
		SHA256:      "abc123",
	}
	adapter := &stubAutoUpdateAdapter{
		latestVersion:   latest,
		updateAvailable: true,
		alreadyUpdated:  false,
		recentlyUpdated: false,
	}

	manager := &UpdateManager{
		currentVersion: "1.0.0",
		adapter:        adapter,
		restartDelay:   0,
	}

	manager.CheckAndUpdate()

	if adapter.fetchCalls != 1 {
		t.Fatalf("expected FetchLatestVersion to be called once, got %d", adapter.fetchCalls)
	}
	if len(adapter.updateChecks) != 1 || adapter.updateChecks[0] != latest {
		t.Fatalf("expected IsUpdateAvailable to be called with latest version")
	}
	if len(adapter.alreadyUpdatedChecks) != 1 || adapter.alreadyUpdatedChecks[0] != latest.Version {
		t.Fatalf("expected IsAlreadyUpdated to be checked for version %q", latest.Version)
	}
	if len(adapter.downloads) != 1 || adapter.downloads[0] != latest {
		t.Fatalf("expected DownloadAndStage to be called with latest version")
	}
	if len(adapter.markedApplied) != 1 || adapter.markedApplied[0] != latest.Version {
		t.Fatalf("expected MarkApplied to be called for version %q", latest.Version)
	}
	if !adapter.restartCalled {
		t.Fatal("expected Restart to be called")
	}
	if got := adapter.events; len(got) < 2 || got[len(got)-2] != "mark:1.2.0" || got[len(got)-1] != "restart" {
		t.Fatalf("expected MarkApplied before Restart, got events %v", got)
	}
	if len(adapter.savedErrors) != 0 {
		t.Fatalf("expected no update errors to be saved, got %d", len(adapter.savedErrors))
	}
}

func TestUpdateManagerCheckAndUpdateDoesNotMarkOrRestartWhenDownloadAndStageFails(t *testing.T) {
	t.Parallel()

	latest := &VersionInfo{
		Version:     "1.2.0",
		DownloadURL: "https://example.com/task-processor.exe",
		SHA256:      "abc123",
	}
	downloadErr := errors.New("checksum mismatch")
	adapter := &stubAutoUpdateAdapter{
		latestVersion:   latest,
		updateAvailable: true,
		downloadErr:     downloadErr,
	}

	manager := &UpdateManager{
		currentVersion: "1.0.0",
		adapter:        adapter,
		restartDelay:   0,
	}

	manager.CheckAndUpdate()

	if len(adapter.downloads) != 1 || adapter.downloads[0] != latest {
		t.Fatalf("expected DownloadAndStage to be called with latest version")
	}
	if len(adapter.markedApplied) != 0 {
		t.Fatalf("expected MarkApplied not to be called, got %v", adapter.markedApplied)
	}
	if adapter.restartCalled {
		t.Fatal("expected Restart not to be called on download/stage failure")
	}
	if len(adapter.savedErrors) != 1 {
		t.Fatalf("expected one saved error, got %d", len(adapter.savedErrors))
	}
	if adapter.savedErrors[0].currentVersion != "1.0.0" {
		t.Fatalf("expected current version to be recorded, got %q", adapter.savedErrors[0].currentVersion)
	}
	if adapter.savedErrors[0].context != "更新到版本 1.2.0 失败" {
		t.Fatalf("expected update failure context, got %q", adapter.savedErrors[0].context)
	}
	if !errors.Is(adapter.savedErrors[0].err, downloadErr) {
		t.Fatal("expected download/stage error to be preserved")
	}
}

func TestUpdateManagerCheckAndUpdateSavesFetchErrorViaAdapter(t *testing.T) {
	t.Parallel()

	fetchErr := errors.New("version endpoint unavailable")
	adapter := &stubAutoUpdateAdapter{
		fetchErr: fetchErr,
	}

	manager := &UpdateManager{
		currentVersion: "1.0.0",
		adapter:        adapter,
		restartDelay:   0,
	}

	manager.CheckAndUpdate()

	if len(adapter.savedErrors) != 1 {
		t.Fatalf("expected one saved error, got %d", len(adapter.savedErrors))
	}
	if adapter.savedErrors[0].currentVersion != "1.0.0" {
		t.Fatalf("expected current version to be recorded, got %q", adapter.savedErrors[0].currentVersion)
	}
	if adapter.savedErrors[0].context != "获取版本信息失败" {
		t.Fatalf("expected fetch error context, got %q", adapter.savedErrors[0].context)
	}
	if !errors.Is(adapter.savedErrors[0].err, fetchErr) {
		t.Fatalf("expected wrapped fetch error to be preserved")
	}
	if adapter.restartCalled {
		t.Fatal("expected Restart not to be called on fetch failure")
	}
}

func TestDefaultAutoUpdateAdapterDelegatesToExistingComponents(t *testing.T) {
	t.Parallel()

	latest := &VersionInfo{
		Version:     "2.0.0",
		DownloadURL: "https://example.com/task-processor.exe",
		SHA256:      "deadbeef",
	}

	vm := &stubVersionManager{
		latestVersion:   latest,
		updateAvailable: true,
	}
	fd := &stubFileDownloader{
		tempFilePath: "C:/temp/task-processor-new.exe",
	}
	fm := &stubFileManager{
		alreadyUpdated:  true,
		recentlyUpdated: true,
	}

	adapter := NewDefaultAutoUpdateAdapter(vm, fd, fm)

	fetched, err := adapter.FetchLatestVersion()
	if err != nil {
		t.Fatalf("FetchLatestVersion returned error: %v", err)
	}
	if fetched != latest {
		t.Fatal("expected FetchLatestVersion to return the version manager result")
	}
	if !adapter.IsUpdateAvailable(latest) {
		t.Fatal("expected IsUpdateAvailable to delegate to version manager")
	}
	if !adapter.IsAlreadyUpdated(latest.Version) {
		t.Fatal("expected IsAlreadyUpdated to delegate to file manager")
	}
	if !adapter.IsRecentlyUpdated() {
		t.Fatal("expected IsRecentlyUpdated to delegate to file manager")
	}
	if err := adapter.DownloadAndStage(latest); err != nil {
		t.Fatalf("DownloadAndStage returned error: %v", err)
	}

	savedErr := errors.New("download failed")
	adapter.SaveUpdateError("1.0.0", "更新失败", savedErr)
	adapter.MarkApplied(latest.Version)
	adapter.Restart()

	if vm.fetchCalls != 1 {
		t.Fatalf("expected version manager fetch to be called once, got %d", vm.fetchCalls)
	}
	if len(vm.updateChecks) != 1 || vm.updateChecks[0] != latest {
		t.Fatal("expected IsUpdateAvailable to forward version info to version manager")
	}
	if len(fd.downloads) != 1 {
		t.Fatalf("expected one download request, got %d", len(fd.downloads))
	}
	download := fd.downloads[0]
	if download.url != latest.DownloadURL || download.destPath != fd.tempFilePath || download.expectedHash != latest.SHA256 || download.maxRetries != 3 {
		t.Fatalf("unexpected download request: %+v", download)
	}
	if len(fm.replacements) != 1 {
		t.Fatalf("expected one replacement request, got %d", len(fm.replacements))
	}
	replacement := fm.replacements[0]
	if replacement.tmpFile != fd.tempFilePath || replacement.version != latest.Version {
		t.Fatalf("unexpected replacement request: %+v", replacement)
	}
	if len(fm.savedErrors) != 1 {
		t.Fatalf("expected one saved error, got %d", len(fm.savedErrors))
	}
	if len(fm.markedVersions) != 1 || fm.markedVersions[0] != latest.Version {
		t.Fatalf("expected MarkApplied to delegate to file manager")
	}
	if !fm.restartCalled {
		t.Fatal("expected Restart to delegate to file manager")
	}
}

type stubAutoUpdateAdapter struct {
	latestVersion        *VersionInfo
	fetchErr             error
	updateAvailable      bool
	alreadyUpdated       bool
	recentlyUpdated      bool
	downloadErr          error
	fetchCalls           int
	updateChecks         []*VersionInfo
	alreadyUpdatedChecks []string
	downloads            []*VersionInfo
	savedErrors          []savedUpdateError
	markedApplied        []string
	restartCalled        bool
	events               []string
}

func (s *stubAutoUpdateAdapter) FetchLatestVersion() (*VersionInfo, error) {
	s.fetchCalls++
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	return s.latestVersion, nil
}

func (s *stubAutoUpdateAdapter) IsUpdateAvailable(version *VersionInfo) bool {
	s.updateChecks = append(s.updateChecks, version)
	return s.updateAvailable
}

func (s *stubAutoUpdateAdapter) IsAlreadyUpdated(version string) bool {
	s.alreadyUpdatedChecks = append(s.alreadyUpdatedChecks, version)
	return s.alreadyUpdated
}

func (s *stubAutoUpdateAdapter) IsRecentlyUpdated() bool {
	return s.recentlyUpdated
}

func (s *stubAutoUpdateAdapter) DownloadAndStage(version *VersionInfo) error {
	s.downloads = append(s.downloads, version)
	return s.downloadErr
}

func (s *stubAutoUpdateAdapter) SaveUpdateError(currentVersion, context string, err error) {
	s.savedErrors = append(s.savedErrors, savedUpdateError{
		currentVersion: currentVersion,
		context:        context,
		err:            err,
	})
}

func (s *stubAutoUpdateAdapter) MarkApplied(version string) {
	s.markedApplied = append(s.markedApplied, version)
	s.events = append(s.events, "mark:"+version)
}

func (s *stubAutoUpdateAdapter) Restart() {
	s.restartCalled = true
	s.events = append(s.events, "restart")
}

type stubVersionManager struct {
	latestVersion   *VersionInfo
	fetchErr        error
	updateAvailable bool
	fetchCalls      int
	updateChecks    []*VersionInfo
}

func (s *stubVersionManager) FetchLatestVersion() (*VersionInfo, error) {
	s.fetchCalls++
	if s.fetchErr != nil {
		return nil, s.fetchErr
	}
	return s.latestVersion, nil
}

func (s *stubVersionManager) IsUpdateAvailable(version *VersionInfo) bool {
	s.updateChecks = append(s.updateChecks, version)
	return s.updateAvailable
}

type stubFileDownloader struct {
	tempFilePath string
	downloads    []downloadRequest
	downloadErr  error
}

func (s *stubFileDownloader) GetTempFilePath() string {
	return s.tempFilePath
}

func (s *stubFileDownloader) DownloadWithRetry(url, destPath, expectedHash string, maxRetries int) error {
	s.downloads = append(s.downloads, downloadRequest{
		url:          url,
		destPath:     destPath,
		expectedHash: expectedHash,
		maxRetries:   maxRetries,
	})
	return s.downloadErr
}

type stubFileManager struct {
	alreadyUpdated  bool
	recentlyUpdated bool
	replaceErr      error
	replacements    []replacementRequest
	savedErrors     []savedUpdateError
	markedVersions  []string
	restartCalled   bool
}

func (s *stubFileManager) ReplaceExecutable(tmpFile string, version string) error {
	s.replacements = append(s.replacements, replacementRequest{
		tmpFile: tmpFile,
		version: version,
	})
	return s.replaceErr
}

func (s *stubFileManager) SaveUpdateError(currentVersion, context string, err error) {
	s.savedErrors = append(s.savedErrors, savedUpdateError{
		currentVersion: currentVersion,
		context:        context,
		err:            err,
	})
}

func (s *stubFileManager) IsAlreadyUpdated(version string) bool {
	return s.alreadyUpdated
}

func (s *stubFileManager) IsRecentlyUpdated() bool {
	return s.recentlyUpdated
}

func (s *stubFileManager) MarkAsUpdated(version string) {
	s.markedVersions = append(s.markedVersions, version)
}

func (s *stubFileManager) RestartProgram() {
	s.restartCalled = true
}

type downloadRequest struct {
	url          string
	destPath     string
	expectedHash string
	maxRetries   int
}

type replacementRequest struct {
	tmpFile string
	version string
}

type savedUpdateError struct {
	currentVersion string
	context        string
	err            error
}
