import { describe, expect, it } from "vitest";

import { createAmazonWorkspaceAdapter } from "@/components/listingkit/workspace/amazon-workspace-adapter";
import { createSheinWorkspaceAdapter } from "@/components/listingkit/workspace/shein-workspace-adapter";
import {
  createGenericWorkspaceProjection,
  resolveWorkspacePlatformAdapter,
} from "@/components/listingkit/workspace/workspace-platform-adapter";

describe("resolveWorkspacePlatformAdapter", () => {
  it("uses the selected marketplace adapter instead of a workspace-level platform switch", () => {
    const projection = resolveWorkspacePlatformAdapter(
      "amazon",
      [
        createAmazonWorkspaceAdapter({
          amazon: {
            title: "Amazon desk lamp",
            brand: "Acme",
            product_type: "Lighting",
          },
        }),
      ],
      () => createGenericWorkspaceProjection("amazon"),
    );

    expect(projection).toEqual({
      kind: "amazon",
      platform: "amazon",
      title: "Amazon desk lamp",
      subtitle: "Acme · Lighting",
    });
  });

  it("keeps platforms without an adapter on the generic workspace path", () => {
    const projection = resolveWorkspacePlatformAdapter(
      "temu",
      [createAmazonWorkspaceAdapter()],
      () => createGenericWorkspaceProjection("temu"),
    );

    expect(projection).toEqual({
      kind: "generic",
      platform: "temu",
    });
  });
});

describe("createSheinWorkspaceAdapter", () => {
  it("projects the SHEIN review flow inside the SHEIN adapter", () => {
    const projection = createSheinWorkspaceAdapter({
      taskId: "task-123",
      isFinalReviewMode: false,
      shein: {
        submit_readiness: {
          status: "blocked",
          blocking_items: [
            {
              key: "category_unresolved",
              message: "Choose a category",
            },
          ],
        },
      },
    }).project();

    expect(projection.kind).toBe("shein");
    if (projection.kind !== "shein") {
      return;
    }

    expect(projection.projection.flowSteps).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "category",
          state: "blocked",
        }),
        expect.objectContaining({
          key: "submit",
          state: "blocked",
        }),
      ]),
    );
  });
});
