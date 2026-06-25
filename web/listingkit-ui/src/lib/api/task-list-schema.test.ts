import { describe, expect, it } from "vitest";

import { parseTaskListResponse } from "@/lib/api/task-list-schema";

describe("task list schema", () => {
  it("parses task list items with explicit pod execution fields", () => {
    expect(
      parseTaskListResponse({
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            task_id: "task-1",
            pod_execution: {
              provider: "sds",
              dependency_mode: "required",
              status: "failed_blocking",
              failure_reason: "mockup sync timeout",
              history: [
                {
                  kind: "status_transition",
                  from_status: "processing",
                  to_status: "failed_blocking",
                  occurred_at: "2026-05-28T08:00:00Z",
                },
              ],
            },
          },
        ],
      }),
    ).toMatchObject({
      items: [
        {
          task_id: "task-1",
          pod_execution: {
            provider: "sds",
            dependency_mode: "required",
            status: "failed_blocking",
          },
        },
      ],
    });
  });

  it("parses SDS source metadata fields", () => {
    expect(
      parseTaskListResponse({
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            task_id: "task-sds",
            product_name: "SDS 方形挂钟",
            source_product_sku: "MG8006905",
            source_variant_sku: "MG8006905001",
            source_variant_price: "16.6",
            source_variants: [
              {
                variant_sku: "XB0610007001",
                price: "34.5",
                color: "white",
                size: "16x23cm",
              },
            ],
            variant_label: "白色 25x25cm MG8006905001",
          },
        ],
      }),
    ).toMatchObject({
      items: [
        {
          task_id: "task-sds",
          product_name: "SDS 方形挂钟",
          source_product_sku: "MG8006905",
          source_variant_sku: "MG8006905001",
          source_variant_price: 16.6,
          source_variants: [
            {
              variant_sku: "XB0610007001",
              price: 34.5,
              color: "white",
              size: "16x23cm",
            },
          ],
          variant_label: "白色 25x25cm MG8006905001",
        },
      ],
    });
  });
});
