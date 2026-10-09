# 我的供应链可选装配

本目录维护 `supply-asset-init` 的运维归属。它为当前 Asset owner 的全新空数据库安装来源图片批准 schema，并给预先建立的 `supply_asset_runtime` 角色授予必要权限。应用启动不执行 DDL。

完整配置、Product 空库初始化和用户操作见 [运行说明](../../../docs/operations/my-supply-chain-shein-v1.md)。此功能不在默认 Account Compose 中自动启用，不能仅打开前端开关便宣称可用。

维护者显式准备独立 Asset 数据库与窄运行角色，将 schema owner DSN 保存到受保护的绝对路径文件。命令核对精确数据库名、仅接受本机连接，拒绝把非空业务数据库作为全新安装目标。从仓库根目录执行：

```powershell
go run ./cmd/supply-asset-init -dsn-file C:\private\supply-asset-owner.dsn -confirm-database assets -install-empty-schema
```

已有合格 schema 仅补授权时，省略 `-install-empty-schema`。执行者必须持有该目标数据库的初始化授权；这些说明不构成共享或生产实例的数据操作授权。

随后按运行说明启动当前 application，注入独立 Product/Store/Asset runtime 配置、官方应用 registry、Temporal 与可选来源文件存储。普通图片上传消费 `sourceMedia`；可选标题 Agent 使用另行准入的当前 Agent 装配。

停止进程和删除数据是独立操作。重启沿用同一数据库和存储配置，禁止用 `down -v` 代替停止。真实 SHEIN 上传、付费 Agent 调用及用户试用分别验收。
