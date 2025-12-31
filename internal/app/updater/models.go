// Package updater 提供自动更新器的数据结构定义
package updater

import "time"

// VersionInfo 版本信息
type VersionInfo struct {
	Version     string    `json:"version"`      // 版本号
	ReleaseDate time.Time `json:"release_date"` // 发布时间
	DownloadURL string    `json:"download_url"` // 下载地址
	SHA256      string    `json:"sha256"`       // 文件哈希
	Changelog   string    `json:"changelog"`    // 更新日志
	ForceUpdate bool      `json:"force_update"` // 是否强制更新
}
