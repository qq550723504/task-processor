import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getListingScheduledTaskConfigs,
  parseScheduledTaskConfigPageResponse,
  updateListingScheduledTaskConfigStatus,
  upsertListingScheduledTaskConfig,
} from "@/lib/api/admin-scheduled-task-configs";

describe("parseScheduledTaskConfigPageResponse", () => {
  it("accepts the ListingKit scheduled task config page shape", () => {
    expect(
      parseScheduledTaskConfigPageResponse({
        items: [
          {
            id: 1,
            tenantId: 246,
            storeId: 962,
            platform: "shein",
            taskType: "inventory",
            enabled: true,
            intervalSeconds: 3600,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ storeId: 962, taskType: "inventory", enabled: true }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseScheduledTaskConfigPageResponse({ items: [{}], total: "1" }),
    ).toThrow(
      "ListingKit API returned an unexpected scheduled task config page response",
    );
  });
});

describe("admin scheduled task config API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests scheduled task configs through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingScheduledTaskConfigs({
        page: 2,
        page_size: 10,
        platform: "shein",
        taskType: "inventory",
        enabled: true,
      }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/scheduled-task-configs?enabled=true&page=2&page_size=10&platform=shein&taskType=inventory",
    );
  });

  it("upserts scheduled task configs", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 246,
          storeId: 962,
          platform: "shein",
          taskType: "inventory",
          enabled: true,
          intervalSeconds: 3600,
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      upsertListingScheduledTaskConfig({
        storeId: 962,
        platform: "shein",
        taskType: "inventory",
        enabled: true,
        intervalSeconds: 3600,
      }),
    ).resolves.toMatchObject({ id: 1, storeId: 962, enabled: true });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/scheduled-task-configs",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });

  it("updates scheduled task config status", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          storeId: 962,
          platform: "shein",
          taskType: "inventory",
          enabled: false,
          intervalSeconds: 3600,
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      updateListingScheduledTaskConfigStatus(1, false, "pause"),
    ).resolves.toMatchObject({ id: 1, enabled: false });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/scheduled-task-configs/1/status",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PATCH");
  });
});
