# ListingKit UI

Desktop-first internal UI for ListingKit review and generation operations.

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

For `SHEIN Studio` image generation, also configure:

```bash
OPENAI_API_KEY=your_api_key
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_IMAGE_MODEL=gpt-image-1
OPENAI_API_STYLE=openai
```

`SHEIN Studio` now also falls back to the repo-level backend image settings when present, especially:

```bash
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY=...
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL=...
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE=nanobanana
TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL=nano-banana-fast
```

This is useful when the backend is already using a provider-specific image client such as `nanobanana`.

Optional local demo mode:

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
- `/listing-kits/[taskId]/workspace`
- `/listing-kits/[taskId]/queue`

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
