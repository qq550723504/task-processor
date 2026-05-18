import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/auth", () => ({
  auth: vi.fn(async () => null),
}));

import { shouldBypassListingKitProxyAuth } from "@/app/api/listing-kits/proxy-auth";
import { selectListingKitMockPayload } from "@/app/api/listing-kits/proxy-mock";
import ListingKitSheinStudioPage from "@/app/listing-kits/shein/page";
import ListingKitStyleGalleryRoute from "@/app/listing-kits/style-gallery/page";

vi.mock("@/components/listingkit/shein-studio/shein-studio-page-shell", () => ({
  SheinStudioPageShell: () => <div>SHEIN Studio shell mounted</div>,
}));

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
  it("allows local ZITADEL auth bypass only with the explicit bypass flag", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("LISTINGKIT_UI_BYPASS_AUTH_GATE", "1");

    expect(shouldBypassListingKitProxyAuth()).toBe(true);

    vi.stubEnv("NODE_ENV", "production");
    expect(shouldBypassListingKitProxyAuth()).toBe(false);
  });

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

  it("mounts the SHEIN Studio route shell", () => {
    render(<ListingKitSheinStudioPage />);

    expect(screen.getByText("SHEIN Studio shell mounted")).toBeInTheDocument();
  });

  it("builds and renders the style gallery route", async () => {
    render(await ListingKitStyleGalleryRoute());

    expect(screen.getByText("ListingKit 款式图库")).toBeInTheDocument();
  });
});
