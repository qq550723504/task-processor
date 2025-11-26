# 自动更新功能

## 功能说明

自动更新器会定期检查服务器上的版本信息，如果发现新版本，会自动下载并更新程序。

## 配置说明

在配置文件中添加以下配置：

```yaml
updater:
  enabled: true  # 是否启用自动更新
  updateURL: "https://auto-update-1303159911.cos.ap-shanghai.myqcloud.com/task-processor/version.json"
  checkInterval: 300  # 检查间隔（秒），默认5分钟
  insecureSkipVerify: false  # 是否跳过TLS证书验证
```

## 配置项说明

- `enabled`: 是否启用自动更新功能
  - `true`: 启用（生产环境推荐）
  - `false`: 禁用（开发环境推荐）

- `updateURL`: 版本检查地址
  - 指向包含版本信息的JSON文件
  - 默认使用腾讯云COS存储

- `checkInterval`: 检查间隔（秒）
  - 默认300秒（5分钟）
  - 建议不要设置太短，避免频繁请求

- `insecureSkipVerify`: 跳过TLS证书验证
  - `true`: 跳过验证（不安全，仅在证书问题时临时使用）
  - `false`: 验证证书（推荐）

## 版本信息格式

服务器上的 `version.json` 文件格式：

```json
{
  "version": "1.0.1",
  "release_date": "2024-01-15T10:00:00Z",
  "download_url": "https://example.com/task-processor-1.0.1.exe",
  "sha256": "abc123...",
  "changelog": "修复了若干bug，优化了性能",
  "force_update": false
}
```

## 工作流程

1. **定期检查**: 按配置的间隔定期检查版本
2. **版本比较**: 比较当前版本和服务器版本
3. **下载更新**: 如果有新版本，下载到临时目录
4. **校验文件**: 使用SHA256校验文件完整性
5. **替换程序**: 备份当前版本，替换为新版本
6. **自动重启**: 启动新版本程序，退出当前进程

## 安全特性

- **SHA256校验**: 确保下载的文件完整性
- **备份机制**: 更新前自动备份当前版本为 `.old` 文件
- **重试机制**: 下载失败时自动重试3次
- **错误日志**: 更新失败时保存详细错误日志到 `update-error.log`

## 故障排除

### 证书错误

如果遇到TLS证书错误，可以临时启用 `insecureSkipVerify`:

```yaml
updater:
  insecureSkipVerify: true
```

**注意**: 这会降低安全性，仅在必要时使用。

### 网络问题

如果需要代理，设置环境变量：

```bash
# Windows
set HTTP_PROXY=http://proxy:port
set HTTPS_PROXY=http://proxy:port

# Linux/Mac
export HTTP_PROXY=http://proxy:port
export HTTPS_PROXY=http://proxy:port
```

### 更新失败

查看 `update-error.log` 文件了解详细错误信息。

## 回滚

如果新版本有问题，可以手动回滚：

1. 删除当前的 `task-processor.exe`
2. 将 `task-processor.exe.old` 重命名为 `task-processor.exe`
3. 重新启动程序

## 开发环境

开发环境建议禁用自动更新：

```yaml
updater:
  enabled: false
```

## 生产环境

生产环境建议启用自动更新：

```yaml
updater:
  enabled: true
  checkInterval: 300  # 5分钟检查一次
```
