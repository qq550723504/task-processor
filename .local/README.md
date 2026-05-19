# Local Runtime Artifacts

`.local/` 只用于本地开发期产物，不存放仓库源码或需要提交的配置。

建议默认放入：

- 日志与诊断输出
- 浏览器 profile / 会话状态
- 临时文件与缓存
- 本地编译出的调试二进制

推荐子目录：

- `.local/logs/`
- `.local/tmp/`
- `.local/chrome/`
- `.local/bin/`
- `.local/dev-logs/`
- `.local/playwright-cli/`
