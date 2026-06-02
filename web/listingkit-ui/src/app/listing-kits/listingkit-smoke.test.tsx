import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const push = vi.fn();

vi.mock("@/auth", () => ({
  auth: vi.fn(async () => null),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
  }),
}));

import { selectListingKitMockPayload } from "@/app/api/listing-kits/proxy-mock";
import ListingKitSDSPage from "@/app/listing-kits/sds/page";
import ListingKitStyleGalleryRoute from "@/app/listing-kits/style-gallery/page";

vi.mock("@/lib/server/style-gallery", () => ({
  buildStyleGallery: vi.fn(async () => ({
    items: [],
    summary: {
      publishedInputs: 0,
      studioLegacy: 0,
      studioSaved: 0,
      taskLinked: 0,
    },
  })),
}));

describe("ListingKit lightweight smoke", () => {
  it("keeps the local mock selector wired for ListingKit task routes", () => {
    const payload = selectListingKitMockPayload({
      method: "GET",
      path: ["tasks", "task-1", "preview"],
      bundle: {
        action: { action_key: "noop" },
        createTask: { task_id: "task-1" },
        dispatch: { dispatch_kind: "review_session" },
        preview: { task_id: "task-1", status: "completed" },
        queue: { task_id: "task-1", page: 1, page_size: 20, total: 0 },
        reviewPreview: { task_id: "task-1" },
        reviewSession: { task_id: "task-1" },
        taskResult: { task_id: "task-1", status: "completed" },
      },
    });

    expect(payload).toEqual({ task_id: "task-1", status: "completed" });
  });

  it("mounts the SDS route entry", () => {
    render(<ListingKitSDSPage />);

    expect(screen.getByText("从 POD 商品生成上架资料")).toBeInTheDocument();
  });

  it("builds and renders the style gallery route", async () => {
    render(await ListingKitStyleGalleryRoute());

    expect(screen.getByText("ListingKit 款式图库")).toBeInTheDocument();
  });
});
