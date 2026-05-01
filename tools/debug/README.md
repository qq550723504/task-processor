# Debug Tools

这个目录保存仓库内受管的调试可执行程序。

- 这里只放非生产发布入口
- 正式服务入口仍只在根目录 `cmd/`
- 运行调试工具前，请先准备本地配置文件或必需环境变量

示例：

```bash
cd tools/debug
go run ./test-amazon --help
go run ./test-sds --help
```
