# sds

`sds` 负责对接 SDS 网站的登录态请求链路，不依赖浏览器自动化。

当前目标：

- 复用 `req/v3` 构建稳定的登录态 HTTP 客户端
- 支持从本地文件导入/持久化 Cookie
- 提供模板查询、设计上传、预览生成的请求骨架
- 避免把 SDS 特定逻辑散落到现有 TEMU / SHEIN / Amazon 模块

当前目录：

- `client/`
  - HTTP 客户端、Cookie 持久化、请求入口
- `template/`
  - 模板列表、模板详情的通用请求封装
- `design/`
  - 图案上传、预览生成、商品草稿相关的通用请求封装
- `workflow/`
  - 把远程图片或 `productimage` 产出的资产转换为 SDS 设计保存请求
- `adapter/`
  - 把 `productimage.Service` 与 SDS workflow 串起来
- `usecase/`
  - 对上层业务暴露稳定的 SDS 设计同步入口

说明：

- SDS 暂无正式开放 API，因此本模块按“人工登录一次 -> 导入 Cookie -> 复用登录态请求”的方式设计。
- 真实接口路径、签名参数、响应结构需要基于后续抓包逐步补充。

## 当前真实联调结论

截至 2026-04-23，这条链已经用真实 SDS 登录态跑通过：

- 本地文件上传到 OSS
- `POST /materials/one`
- `GET /ps/design/products/{variantId}`
- `POST /ps/design/syncDesign`

实测样例：

- `variantId = 89764`
- `prototypeGroupId = 14555`
- `layer_id = 698744758333792256`

真实联调过程中修正了 4 个代码问题：

- 不能手工设置 `Accept-Encoding`
  - 否则 gzip 响应不会自动解压，JSON 解析会直接失败
- 设计页路径模板必须使用 `%d`
  - 不能把 `int64` 按 `%s` 拼 URL
- `POST /ps/design/syncDesign` 的真实返回是 `200 + 空响应体`
  - 不能强制按 JSON 解析
- `POST /materials/one` 返回结构是 `{ret,msg,data:[...]}`
  - 不是直接返回单个 `Material`

## 已确认可用接口

当前已从前端 bundle 和实测请求确认：

- `GET /products/page`
  - 产品分页列表
- `GET /products/pageOptionGroup`
  - 搜索页筛选项
- `GET /products/{id}`
  - 产品详情
- `GET /products/{id}/recommend`
  - 推荐产品
- `GET /products/{id}/cycle`
  - 生产周期
- `GET /ps/image/get_post_signature_to_image_for_oss_upload`
  - OSS 上传签名
- `POST /materials/one`
  - 上传后登记素材库记录
- `GET /materials/findByIds`
  - 按素材 ID 回查素材信息
- `POST /ps/design/syncDesign`
  - 当前设计页真实保存接口
其中：

- 列表、筛选、详情、推荐、周期接口当前可匿名访问
- 设计相关接口和素材库动作需要登录态 `access-token`

## 自动获取登录态

当前 SDS 集成链路会按下面顺序自动引导登录态：

1. 本地文件
   - `data/sds/auth_state.json`
   - `data/sds/session_cookies.json`
2. 应用配置注入的静态认证信息
   - `TASK_PROCESSOR_SDS_ACCESS_TOKEN`
   - `TASK_PROCESSOR_SDS_OUT_ACCESS_TOKEN`
   - `TASK_PROCESSOR_SDS_MERCHANT_ID`
   - `TASK_PROCESSOR_SDS_COOKIE`
3. SDS login-service 登录态
   - `TASK_PROCESSOR_SDS_LOGIN_BASE_URL`
   - `TASK_PROCESSOR_SDS_LOGIN_TENANT_ID`
   - `TASK_PROCESSOR_SDS_LOGIN_IDENTIFIER`
   - 可选：`TASK_PROCESSOR_SDS_LOGIN_SHARED_KEY`
4. 应用配置注入的账号密码自动登录
   - `TASK_PROCESSOR_SDS_USERNAME`
   - `TASK_PROCESSOR_SDS_PASSWORD`
   - 可选：
     - `TASK_PROCESSOR_SDS_MERCHANT_NAME`
     - `TASK_PROCESSOR_SDS_DOMAIN_NAME`
     - `TASK_PROCESSOR_SDS_VERIFY_CAPTCHA_PARAM`
     - `TASK_PROCESSOR_SDS_EXTRA_INFO`

说明：

- `internal/app/httpapi` 会通过 `internal/core/config` 把上述环境变量注入到 SDS 客户端配置。
- 直接调用 `sds/client.DefaultConfig()` 只会返回静态默认值，不再隐式读取环境变量。

当 SDS 接口返回以下任一鉴权失效信号时，客户端会自动重拉登录态并重试一次：

- `ret=20001`
- HTTP `401/403`
- HTTP `400` 且响应体明确包含 `用户未登录`、`auth required` 或 `login required`

可以直接用 CLI 单独验证 `req` 登录：

```bash
cd tools/debug && go run ./test-sds -mode login \
  -username 你的账号 \
  -password 你的密码 \
  -domain-name www.sdsdiy.com
```

如果 SDS 当前登录链路要求风控字段，再补：

```bash
-verify-captcha-param '...'
-extra-info '...'
```

## 最小示例

```go
package main

import (
	"context"
	"fmt"
	"log"

	"task-processor/internal/sds/client"
	"task-processor/internal/sds/template"
)

func main() {
	ctx := context.Background()

	c, err := client.New(nil)
	if err != nil {
		log.Fatal(err)
	}

	svc := template.NewService(c)
	list, err := svc.ListProducts(ctx, template.ListParams{
		Page:         1,
		Size:         20,
		SideActiveID: "overseas",
		IsOverseas:   "overseas",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("total:", list.TotalCount)
	if len(list.Items) == 0 {
		return
	}

	detail, err := svc.GetProduct(ctx, fmt.Sprintf("%d", list.Items[0].ID))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("first product:", detail.Name, detail.SKU)
}
```

## 后续待补

- `syncDesign` 的响应体建模
- 设计完成后生成 SDS 商品 / end product 的真实请求体
- 预览生成链路和设计结果回查

## 当前已具备的设计链路

- `GET /ps/design/products/{variantId}`
  - 获取设计页初始化数据、图层、PSD、模板组
- `GET /merchant_product_parents/{parentId}/prototypeGroup`
  - 获取父商品可用模板组
- `GET /merchant/product/resultGroup/select`
  - 获取结果分组选项
- `GET /cut/filecode/content`
  - 获取 PSD 智能对象和切图信息
- `POST /materials/one`
  - 登记素材
- `POST /ps/design/syncDesign`
  - 保存设计

`internal/sds/design` 目前已支持：

- 上传图片并登记素材
- 获取设计页初始化数据
- 构造默认 `syncDesign` 请求
- 一次性完成 `PrepareAndSyncDesign`

`internal/sds/workflow` 目前已支持：

- 从远程图片 URL 构造 `design.UploadRequest`
- 从 `productimage.ImageAsset` 构造 `design.UploadRequest`
- 一次性完成“图片源 -> 上传素材 -> 保存 SDS 设计”

`internal/sds/adapter` 目前已支持：

- 创建 `productimage` 任务并同步执行
- 读取已有 `productimage` 任务结果
- 把 `productimage.ImageProcessResult` 直接同步到 SDS

`internal/sds/usecase` 目前已支持：

- 从远程图片同步 SDS
- 从本地文件同步 SDS
- 从 `productimage` 结果同步 SDS
- 从 `productimage` 请求直接产图并同步 SDS

## 联调建议

- 优先在 `tools/debug` 目录下执行 `go run ./test-sds -mode sync-file`
  - 这条链路最短，最适合确认 SDS 登录态、素材上传和设计保存是否正常
- 如果本地已有 `data/sds/auth_state.json`
  - 可以不再显式传 `-token`
- 如果没有本地登录态
  - 先从浏览器已登录会话导入当前 `access-token`
  - 再执行 `sync-file` 或 `sync-url`
