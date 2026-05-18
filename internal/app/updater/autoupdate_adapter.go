package updater

// AutoUpdateAdapter 将更新流程与底层实现隔离开，便于后续验证第三方自更新库。
type AutoUpdateAdapter interface {
	FetchLatestVersion() (*VersionInfo, error)
	IsUpdateAvailable(*VersionInfo) bool
	IsAlreadyUpdated(version string) bool
	IsRecentlyUpdated() bool
	DownloadAndStage(*VersionInfo) error
	SaveUpdateError(currentVersion, context string, err error)
	MarkApplied(version string)
	Restart()
}

type versionInfoSource interface {
	FetchLatestVersion() (*VersionInfo, error)
	IsUpdateAvailable(*VersionInfo) bool
}

type updateFileDownloader interface {
	GetTempFilePath() string
	DownloadWithRetry(url, destPath, expectedHash string, maxRetries int) error
}

type updateFileManager interface {
	ReplaceExecutable(tmpFile string, newVersion string) error
	SaveUpdateError(currentVersion, context string, err error)
	IsAlreadyUpdated(version string) bool
	IsRecentlyUpdated() bool
	MarkAsUpdated(version string)
	RestartProgram()
}

type defaultAutoUpdateAdapter struct {
	versionManager versionInfoSource
	fileDownloader updateFileDownloader
	fileManager    updateFileManager
}

// NewDefaultAutoUpdateAdapter 使用当前 updater 组件创建默认适配器。
func NewDefaultAutoUpdateAdapter(
	vm versionInfoSource,
	fd updateFileDownloader,
	fm updateFileManager,
) AutoUpdateAdapter {
	return &defaultAutoUpdateAdapter{
		versionManager: vm,
		fileDownloader: fd,
		fileManager:    fm,
	}
}

func (a *defaultAutoUpdateAdapter) FetchLatestVersion() (*VersionInfo, error) {
	return a.versionManager.FetchLatestVersion()
}

func (a *defaultAutoUpdateAdapter) IsUpdateAvailable(version *VersionInfo) bool {
	return a.versionManager.IsUpdateAvailable(version)
}

func (a *defaultAutoUpdateAdapter) IsAlreadyUpdated(version string) bool {
	return a.fileManager.IsAlreadyUpdated(version)
}

func (a *defaultAutoUpdateAdapter) IsRecentlyUpdated() bool {
	return a.fileManager.IsRecentlyUpdated()
}

func (a *defaultAutoUpdateAdapter) DownloadAndStage(version *VersionInfo) error {
	tmpFile := a.fileDownloader.GetTempFilePath()
	if err := a.fileDownloader.DownloadWithRetry(version.DownloadURL, tmpFile, version.SHA256, 3); err != nil {
		return err
	}

	return a.fileManager.ReplaceExecutable(tmpFile, version.Version)
}

func (a *defaultAutoUpdateAdapter) SaveUpdateError(currentVersion, context string, err error) {
	a.fileManager.SaveUpdateError(currentVersion, context, err)
}

func (a *defaultAutoUpdateAdapter) MarkApplied(version string) {
	a.fileManager.MarkAsUpdated(version)
}

func (a *defaultAutoUpdateAdapter) Restart() {
	a.fileManager.RestartProgram()
}
