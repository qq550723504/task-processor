# 项目中心首版交接

执行 Issue：[#624](https://github.com/qq550723504/task-processor/issues/624)。设计依据：[项目中心架构](../architecture/ai-workbench-project-center-v1.md)。

当前交付供独立验证和用户试用。开发检查不代表产品验收；真实企业/真实资源操作、部署、现存数据库 DDL 和付费 AI 调用未执行。

## 正常启动

项目中心沿 `cmd/current-application` 和 `web/listingkit-ui` 的正常应用入口运行。配置新增可选字段，未配置时页面显示未启用；项目和模板不依赖 AIWorkbench / ProductAgent / 模型配置。

在获准的新安装数据库内由管理员预建 `ai_projects_runtime` 登录角色（NOSUPERUSER、NOCREATEDB、NOCREATEROLE、NOREPLICATION、NOBYPASSRLS，无角色继承或其他业务表权限）。管理员设置私密 `PROJECT_CENTER_SCHEMA_DSN` 环境变量，运行：

```powershell
go run ./cmd/project-center-schema-init --runtime-role ai_projects_runtime
```

初始化只安装项目 owner schema 与最小权限，不创建项目、模板、身份或其他业务数据。应用启动不执行 DDL。现存实例变更需要对应环境 owner 的单独授权。

私密 current-application manifest 增加以下字段，密码从该环境已有私密交接渠道提供；不要提交真实凭据：

```json
{
  "projectCenter": {
    "database": {
      "host": "127.0.0.1",
      "port": 5432,
      "user": "ai_projects_runtime",
      "password": "<private-runtime-password>",
      "database": "ai_projects",
      "maxConnections": 4
    }
  }
}
```

数据库需为当前安装的独立 owner pool；可与其他 owner 使用同一 PostgreSQL 服务，但 runtime 不能获得其他 schema 权限。

```powershell
go run ./cmd/current-application --config <absolute-private-manifest-path>
cd web/listingkit-ui
pnpm.cmd install --frozen-lockfile
pnpm.cmd dev --hostname 127.0.0.1 --port 3210
```

前端沿现有登录配置，`LISTINGKIT_SERVICE_API_BASE` 指向后端 `/api/v1`，`LISTINGKIT_PUBLIC_BASE_URL` 为前端可信 origin。登录后选择企业，打开 `/workbench/ai/projects`。角色须有 `workbench.project.read`；创建、编辑、关联、归档和模板管理还须有 `workbench.project.manage`。原资源的权限仍须分别授予。

## 操作路径

1. 进行中项目 → 新建项目：填写名称、目标、类型；可填截止日期和当前应用店铺详情链接。保存后打开详情。
2. 在原会话、任务、采集结果或知识库详情复制当前应用链接，回到项目详情添加关联。粘贴知识库链接后，“选择知识资料”可读取并关联其中的有效资料。添加不会运行任务或 AI。
3. 详情中的任务、资料、项目知识和成果均读取原 owner；无权限或未装配来源时显示不可访问。任务状态不可用时不展示假进度。成果链接回原任务供检查和审核。
4. 项目保存为个人模板，只复制目标、建议名称和类型。“使用模板”打开可编辑创建表单，需要确认保存；不复制店铺、日期、关联、任务、结果或执行身份。
5. 归档保留项目和关联，禁止编辑与新增/移除关联；恢复后继续整理。归档不会暂停或取消已有任务。
6. 响应丢失时保留当前浏览器会话，使用“重试原操作”读取同一操作收据。会话存储按创建者和企业隔离，不是项目事实源；服务端事实保存在 PostgreSQL。成功后重新读取当前状态。

## 数据与已知范围

项目、关联、个人模板、访问时间、操作收据与审计保存在 `ai_workbench_projects`；关联目标仍在原 owner 保存。沿当前安装的 PostgreSQL 备份与恢复方式保留数据。重启不清除项目，禁止以 `down -v` 代替正常停止。

个人项目和模板仅创建者在当前企业内可读写，不含共享、平台模板、自动拆解、自动 AI 执行或项目拥有的报告事实。列表每页 20 项，项目最多 100 个有效关联。来源读取失败只影响可读投影，不删除关联；原关联可在恢复权限后读取或由创建者移除。

本 PR 提供正常启动方法和页面入口；未对共享实例执行部署或 DDL，未宣称用户验收通过。
