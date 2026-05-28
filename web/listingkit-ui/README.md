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

Configure ZITADEL for the ListingKit UI session, proxy identity, and Go API:

```bash
ZITADEL_ISSUER_URL=https://your-zitadel-instance
ZITADEL_CLIENT_ID=your_client_id
ZITADEL_CLIENT_SECRET=your_client_secret
ZITADEL_REDIRECT_URI=http://localhost:3000/api/zitadel-auth/callback
ZITADEL_POST_LOGOUT_REDIRECT_URI=http://localhost:3000
```

The Next.js UI and the Go API must point at the same `ZITADEL_ISSUER_URL`,
`ZITADEL_CLIENT_ID`, and `ZITADEL_CLIENT_SECRET`. If they differ, the UI can
finish login successfully while `/api/listing-kits/*` still fails because the
Go API introspects the forwarded bearer token against a different issuer/client
pair.

The Next.js proxy verifies the ZITADEL session or bearer token before forwarding
requests to the ListingKit Go API. The verified ZITADEL user and resource owner
are forwarded as ListingKit identity headers.

The Go API also verifies direct `/api/v1/listing-kits/*` and
`/api/v1/shein-login/*` bearer-token requests when `ZITADEL_ISSUER_URL` and
`ZITADEL_CLIENT_ID` are set. ListingKit routes now fail closed when ZITADEL is
missing or the current session is invalid.

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

SHEIN Studio 草稿、批次、UI 侧 async job 状态和大请求分片 stage 默认写入当前工作目录下的 `.data`。部署到需要保留状态的环境时，显式配置并挂载持久目录：

```bash
LISTINGKIT_UI_STORAGE_DIR=/var/lib/listingkit-ui
LISTINGKIT_UI_ASYNC_JOB_TIMEOUT_MS=3600000
```

该本地 JSON 存储可以缓解刷新和同机重启后的状态丢失；超时未完成的 UI async job 会转为失败，避免一直停在 `running`。多副本部署前仍建议替换成共享数据库或对象存储。

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
