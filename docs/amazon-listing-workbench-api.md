# Amazon Listing Workbench API

## Task Queue

`GET /api/v1/amazon/listings/tasks`

Use this endpoint to build an operator queue instead of loading one task at a time.

Supported query params:

- `status=needs_review,failed`
- `action=fill_brand`
- `field=brand`
- `severity=warning`
- `source=llm`
- `child_status=failed`
- `needs_human=true`
- `limit=20`

Example response:

```json
{
  "count": 1,
  "query": {
    "status": ["needs_review"],
    "action": "fill_brand",
    "needs_human": true,
    "limit": 20
  },
  "items": [
    {
      "task_id": "listing-queue-1",
      "status": "needs_review",
      "needs_review": true,
      "top_action": "fill_brand",
      "total_items": 3,
      "review_summary": {
        "total_count": 3,
        "blocking_count": 0,
        "needs_human_count": 3,
        "by_action": {
          "fill_brand": 1
        }
      }
    }
  ]
}
```

## Workbench Response

`GET /api/v1/amazon/listings/tasks/:task_id/workbench`

This endpoint returns both child-task progress and structured review items for operator follow-up.

Example response:

```json
{
  "task_id": "listing-123",
  "status": "needs_review",
  "ready": false,
  "needs_review": true,
  "child_tasks": [
    {
      "kind": "product_enrich",
      "task_id": "product-task-1",
      "status": "completed"
    },
    {
      "kind": "product_image",
      "task_id": "image-task-2",
      "status": "failed",
      "error": "image processing failed: timeout"
    }
  ],
  "review_items": [
    {
      "field": "brand",
      "action": "fill_brand",
      "severity": "warning",
      "reason": "missing brand",
      "source": "llm,user_text",
      "confidence": 0.58,
      "is_inferred": true,
      "needs_human": true,
      "recommended_fix": "confirm or fill the selling brand",
      "evidence": [
        {
          "type": "user_text",
          "detail": "user input: \"portable blender bottle for smoothies\""
        },
        {
          "type": "llm",
          "detail": "LLM-generated product normalization"
        },
        {
          "type": "field_value",
          "detail": "brand = \"Generic\""
        }
      ]
    },
    {
      "field": "title",
      "action": "edit_title",
      "severity": "warning",
      "reason": "title may be too short for Amazon listing quality",
      "needs_human": true,
      "current_value": "Ceramic Mug",
      "recommended_fix": "expand the title with concrete product facts",
      "confidence": 0.62,
      "is_inferred": true,
      "evidence": [
        {
          "type": "scraped_data",
          "detail": "scraped title: \"Ceramic Mug\""
        },
        {
          "type": "field_value",
          "detail": "title = \"Ceramic Mug\""
        }
      ]
    }
  ],
  "top_action": "fill_brand",
  "action_buckets": [
    {
      "action": "fill_brand",
      "label": "待补品牌",
      "count": 1,
      "blocking_count": 0,
      "priority": 7,
      "rank": 1,
      "items": [
        {
          "message": "missing brand",
          "severity": "warning",
          "target": "brand",
          "operator_action": "fill_brand",
          "operator_advice": "confirm or fill the selling brand"
        }
      ]
    }
  ]
}
```

## Apply Field Edits

`POST /api/v1/amazon/listings/tasks/:task_id/review`

Use `action=apply_edits` to write operator fixes back into the draft, rebuild export payloads, and re-run validation.

Example request:

```json
{
  "action": "apply_edits",
  "edits": [
    {
      "field": "brand",
      "string_value": "Acme"
    },
    {
      "field": "title",
      "string_value": "High Quality Ceramic Coffee Mug for Home Kitchen Use"
    },
    {
      "field": "category_path",
      "string_list": ["Home & Kitchen", "Drinkware"]
    },
    {
      "field": "bullet_points",
      "string_list": [
        "Durable ceramic material",
        "Suitable for coffee and tea",
        "Comfortable daily-use mug"
      ]
    },
    {
      "field": "pricing.suggested_price",
      "number_value": 19.99
    }
  ]
}
```

Supported edit fields:

- `title`
- `brand`
- `description`
- `category_path`
- `bullet_points`
- `search_terms`
- `images.main_image`
- `images.white_bg_image`
- `images.gallery`
- `pricing.currency`
- `pricing.suggested_price`
- `pricing.min_price`
- `pricing.source_cost`

Review item evidence fields:

- `confidence`: numeric confidence score from canonical trace
- `is_inferred`: whether the field is primarily inferred/generated
- `evidence[]`: source details plus field snippets for operator review
- `evidence[].type`: source type such as `user_text`, `user_image`, `product_url`, `scraped_data`, `llm`, `field_value`
- `evidence[].detail`: human-readable evidence text, for example scraped title/spec fragments or the current field snapshot

Behavior after `apply_edits`:

- Matching `review_items` are removed.
- Listing export payloads are rebuilt.
- Validator runs again immediately.
- Task status becomes `completed` when no blocking issues or review items remain.
- Otherwise task remains `needs_review` with refreshed `review_items`.
