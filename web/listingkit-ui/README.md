# ListingKit UI

Desktop-first internal UI for ListingKit review and generation operations.

产品文档：
- [ListingKit 产品总览](../../docs/product/listingkit-product-overview.md)
- [ListingKit 操作指南](../../docs/product/listingkit-operating-guide.md)

Current scope:
- review workspace: `preview`, `generation-review-session`, `generation-review-preview`, `dispatch`, `action`
- queue console: `generation-queue`, `resolved_action_summary`, `recovery_summary`

## Setup

1. Install dependencies:

```bash
npm install
```

2. Copy the environment template:

```bash
cp .env.example .env.local
```

3. Point `LISTINGKIT_API_BASE` at the existing Go API.

Default:

```bash
LISTINGKIT_API_BASE=http://localhost:8080/api/v1/listing-kits
```

When embedding ListingKit in yudao Vben, configure the yudao token verifier:

```bash
YUDAO_CHECK_TOKEN_URL=http://127.0.0.1:48081/admin-api/system/oauth2/check-token
YUDAO_OAUTH_CLIENT_ID=default
YUDAO_OAUTH_CLIENT_SECRET=your_oauth_client_secret
NEXT_PUBLIC_YUDAO_PARENT_ORIGINS=http://127.0.0.1:5666,http://localhost:5666
```

If `YUDAO_CHECK_TOKEN_URL`, `YUDAO_OAUTH_CLIENT_ID`, and
`YUDAO_OAUTH_CLIENT_SECRET` are all set, the Next.js proxy verifies the browser
Bearer token with yudao before forwarding requests to the ListingKit Go API.
The verified `user_id` and `tenant_id` override browser-supplied tenant headers.

如果只是本地联调，想暂时跳过页面授权门禁，可以额外设置：

```bash
LISTINGKIT_UI_BYPASS_AUTH_GATE=1
```

这个开关只在非生产环境生效，只跳过前端 `YudaoAuthGate`，不会修改后端 API 的实际鉴权逻辑。

如果要使用 ListingKit 里的 AI 能力，需要先在 ListingKit 设置页为当前租户或用户保存 AI 配置。
后端不再回退到仓库级别的默认 OpenAI / 图片环境变量。

下面这些旧环境变量说明可以删除，不再作为 ListingKit 的兜底来源：

```bash
OPENAI_API_KEY=your_api_key
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_IMAGE_MODEL=gpt-image-1
OPENAI_API_STYLE=openai
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY=...
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL=...
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE=nanobanana
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL=nano-banana-fast
```

可选的本地演示模式：

```bash
LISTINGKIT_UI_USE_MOCK=1
```

4. Start the app:

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

## Routes

- `/` task launcher
- `/listing-kits` task list
- `/listing-kits/new` generic ListingKit task creation
- `/listing-kits/shein` SHEIN Studio workflow
- `/listing-kits/sds` SDS product browser
- `/listing-kits/settings` ListingKit settings
- `/listing-kits/[taskId]/status`
- `/listing-kits/[taskId]/workspace`
- `/listing-kits/[taskId]/queue`
- `/listing-kits/canonical-products`
- `/listing-kits/style-gallery`
- `/listing-kits/shein/gallery`
- `/listing-kits/shein/batches/[batchId]`

## Commands

```bash
npm run dev
npm run lint
npm test
npm run build
```

## Notes

- The UI treats the existing listingkit JSON contracts as the source of truth.
- The browser talks to the Next.js proxy at `/api/listing-kits`; the proxy forwards to `LISTINGKIT_API_BASE`.
- The proxy serves built-in mock responses when `LISTINGKIT_UI_USE_MOCK=1`, the task id is `demo-task`, or the task id starts with `mock-`.
- Conditional reads reuse `delta_token` / `ETag` through the shared API client.
- Queue row actions currently resolve to `Review`, `Retry`, or `Inspect` from row state.
- Local file upload now supports both:
  - `provider=local`: backend writes to local disk
  - `provider=s3`: backend writes to S3-compatible object storage
- Object storage setup reference:
  [D:\code\task-processor\docs\development\listingkit-object-storage.md](D:/code/task-processor/docs/development/listingkit-object-storage.md)
