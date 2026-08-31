import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  WorkbenchAPIError,
  createWorkbenchStore as createWorkbenchStoreWithExpectedOrganization,
  deleteWorkbenchStore as deleteWorkbenchStoreWithExpectedOrganization,
  disableWorkbenchStore as disableWorkbenchStoreWithExpectedOrganization,
  enableWorkbenchStore as enableWorkbenchStoreWithExpectedOrganization,
  getWorkbenchStore as getWorkbenchStoreWithExpectedOrganization,
  listWorkbenchStores as listWorkbenchStoresWithExpectedOrganization,
  updateWorkbenchStore as updateWorkbenchStoreWithExpectedOrganization,
} from "@/lib/api/workbench-stores";

const STORE_ID = "11111111-1111-4111-8111-111111111111";
const CREATE_KEY = "2222222a-2222-4222-8222-222222222222";
const DELETE_KEY = "33333333-3333-4333-8333-333333333333";
const ORGANIZATION_ID = "org-a";
const listWorkbenchStores = (filters: Parameters<typeof listWorkbenchStoresWithExpectedOrganization>[0]) =>
  listWorkbenchStoresWithExpectedOrganization(filters, ORGANIZATION_ID);
const getWorkbenchStore = (storeId: string) =>
  getWorkbenchStoreWithExpectedOrganization(storeId, ORGANIZATION_ID);
const createWorkbenchStore = (
  input: Parameters<typeof createWorkbenchStoreWithExpectedOrganization>[0],
  key: string,
) => createWorkbenchStoreWithExpectedOrganization(input, key, ORGANIZATION_ID);
const deleteWorkbenchStore = (
  id: string,
  version: number,
  key: string,
) => deleteWorkbenchStoreWithExpectedOrganization(id, version, key, ORGANIZATION_ID);
const updateWorkbenchStore = (
  id: string,
  input: Parameters<typeof updateWorkbenchStoreWithExpectedOrganization>[1],
  version: number,
) => updateWorkbenchStoreWithExpectedOrganization(id, input, version, ORGANIZATION_ID);
const enableWorkbenchStore = (id: string, version: number) =>
  enableWorkbenchStoreWithExpectedOrganization(id, version, ORGANIZATION_ID);
const disableWorkbenchStore = (id: string, version: number) =>
  disableWorkbenchStoreWithExpectedOrganization(id, version, ORGANIZATION_ID);
const store = {
  id: STORE_ID,
  name: "North Shop",
  platform: "shein",
  region: "SG",
  externalStoreId: "external-1",
  lifecycleStatus: "active",
  connectionStatus: "disconnected",
  version: 2,
  createdAt: "2026-08-30T01:02:03Z",
  updatedAt: "2026-08-30T02:03:04Z",
};
const list = {
  items: [store],
  quota: {
    used: 1,
    reserved: 0,
    limit: 5,
    allowed: true,
    reason: "",
  },
  pagination: { page: 2, pageSize: 20, total: 21 },
};

const fetchMock = vi.fn<typeof fetch>();

function jsonResponse(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function requestAt(index = 0) {
  const [input, init] = fetchMock.mock.calls[index]!;
  return {
    input,
    init: init!,
    headers: new Headers(init?.headers),
  };
}

describe("workbench Store API", () => {
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  it("rebuilds the allowlisted list query and omits absent filters", async () => {
    fetchMock.mockResolvedValue(jsonResponse(list));

    await listWorkbenchStores({
      page: 2,
      pageSize: 20,
      platform: "shein",
      status: "active",
    });
    expect(requestAt().input).toBe(
      "/api/workbench/stores?page=2&pageSize=20&platform=shein&status=active",
    );
    expect(requestAt().init).toMatchObject({
      method: "GET",
      credentials: "same-origin",
    });
    expect(requestAt().headers.get("Accept")).toBe("application/json");
    expect(requestAt().headers.get("X-Expected-Organization-ID")).toBe(
      ORGANIZATION_ID,
    );
    expect(requestAt().headers.has("Content-Type")).toBe(false);
    expect(requestAt().headers.has("If-Match")).toBe(false);
    expect(requestAt().headers.has("Idempotency-Key")).toBe(false);

    fetchMock.mockClear();
    fetchMock.mockResolvedValue(jsonResponse({ ...list, items: [] }));
    await listWorkbenchStores({ page: 1, pageSize: 100 });
    expect(requestAt().input).toBe(
      "/api/workbench/stores?page=1&pageSize=100",
    );
  });

  it("rejects arbitrary and Organization filters before fetch", async () => {
    await expect(
      listWorkbenchStores({
        page: 1,
        pageSize: 20,
        organizationId: "org-a",
      } as never),
    ).rejects.toMatchObject({ code: "INVALID_REQUEST" });
    await expect(
      listWorkbenchStores({ page: 1, pageSize: 20, arbitrary: "x" } as never),
    ).rejects.toMatchObject({ code: "INVALID_REQUEST" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("sends the exact get request without mutation headers", async () => {
    fetchMock.mockResolvedValue(jsonResponse(store));
    await expect(getWorkbenchStore(STORE_ID)).resolves.toEqual(store);

    expect(requestAt().input).toBe(`/api/workbench/stores/${STORE_ID}`);
    expect(requestAt().init).toMatchObject({
      method: "GET",
      credentials: "same-origin",
    });
    expect([...requestAt().headers]).toEqual([
      ["accept", "application/json"],
      ["x-expected-organization-id", ORGANIZATION_ID],
    ]);
  });

  it("sends normalized create JSON with only its operation header", async () => {
    fetchMock.mockResolvedValue(jsonResponse(store, 201));
    await createWorkbenchStore(
      {
        name: " North Shop ",
        platform: "shein",
        region: " SG ",
        externalStoreId: "   ",
      },
      CREATE_KEY,
    );

    expect(requestAt().input).toBe("/api/workbench/stores");
    expect(requestAt().init).toMatchObject({
      method: "POST",
      credentials: "same-origin",
      body: JSON.stringify({
        name: "North Shop",
        platform: "shein",
        region: "SG",
      }),
    });
    expect(requestAt().headers.get("Content-Type")).toBe("application/json");
    expect(requestAt().headers.get("Idempotency-Key")).toBe(CREATE_KEY);
    expect(requestAt().headers.has("If-Match")).toBe(false);
  });

  it("sends exact update JSON and a quoted last version", async () => {
    fetchMock.mockResolvedValue(jsonResponse(store));
    await updateWorkbenchStore(
      STORE_ID,
      { name: " Renamed ", region: " MY " },
      2,
    );

    expect(requestAt().input).toBe(`/api/workbench/stores/${STORE_ID}`);
    expect(requestAt().init).toMatchObject({
      method: "PUT",
      credentials: "same-origin",
      body: JSON.stringify({ name: "Renamed", region: "MY" }),
    });
    expect(requestAt().headers.get("Content-Type")).toBe("application/json");
    expect(requestAt().headers.get("If-Match")).toBe('"2"');
    expect(requestAt().headers.has("Idempotency-Key")).toBe(false);
  });

  it("sends exact enable and disable requests without bodies or content type", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse(store))
      .mockResolvedValueOnce(jsonResponse(store));
    await enableWorkbenchStore(STORE_ID, 2);
    await disableWorkbenchStore(STORE_ID, 3);

    expect(requestAt(0).input).toBe(
      `/api/workbench/stores/${STORE_ID}/enable`,
    );
    expect(requestAt(1).input).toBe(
      `/api/workbench/stores/${STORE_ID}/disable`,
    );
    for (const [index, version] of [2, 3].entries()) {
      expect(requestAt(index).init.method).toBe("POST");
      expect(requestAt(index).init.body).toBeUndefined();
      expect(requestAt(index).headers.has("Content-Type")).toBe(false);
      expect(requestAt(index).headers.get("If-Match")).toBe(`"${version}"`);
      expect(requestAt(index).headers.has("Idempotency-Key")).toBe(false);
    }
  });

  it("sends exact delete headers and no body", async () => {
    const deleted = { id: STORE_ID, deleted: true, version: 3 };
    fetchMock.mockResolvedValue(jsonResponse(deleted));
    await expect(
      deleteWorkbenchStore(STORE_ID, 2, DELETE_KEY),
    ).resolves.toEqual(deleted);

    expect(requestAt().input).toBe(`/api/workbench/stores/${STORE_ID}`);
    expect(requestAt().init).toMatchObject({
      method: "DELETE",
      credentials: "same-origin",
    });
    expect(requestAt().init.body).toBeUndefined();
    expect(requestAt().headers.get("If-Match")).toBe('"2"');
    expect(requestAt().headers.get("Idempotency-Key")).toBe(DELETE_KEY);
    expect(requestAt().headers.has("Content-Type")).toBe(false);
  });

  it("rejects noncanonical resource and operation UUIDs before fetch", async () => {
    for (const call of [
      () => getWorkbenchStore("not-a-uuid"),
      () => createWorkbenchStore(
        { name: "Shop", platform: "shein", region: "SG" },
        "00000000-0000-0000-0000-000000000000",
      ),
      () => deleteWorkbenchStore(STORE_ID, 2, CREATE_KEY.toUpperCase()),
    ]) {
      await expect(call()).rejects.toMatchObject({ code: "INVALID_REQUEST" });
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects non-positive, fractional, and unsafe versions before fetch", async () => {
    for (const version of [0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1]) {
      await expect(enableWorkbenchStore(STORE_ID, version)).rejects.toMatchObject(
        { code: "INVALID_REQUEST" },
      );
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("reuses the authoritative stable Workbench error envelope", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(
        {
          code: "STORE_VERSION_CONFLICT",
          message: "Store has changed",
          requestId: "req-1",
          fieldErrors: [{ field: "If-Match", code: "conflict" }],
        },
        409,
      ),
    );

    const error = await updateWorkbenchStore(
      STORE_ID,
      { name: "Shop", region: "SG" },
      2,
    ).catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(WorkbenchAPIError);
    expect(error).toMatchObject({
      status: 409,
      code: "STORE_VERSION_CONFLICT",
      message: "Store has changed",
      requestId: "req-1",
      fieldErrors: [{ field: "If-Match", code: "conflict" }],
    });
  });

  it("maps network, non-JSON, malformed error, and malformed success bodies to bounded failures", async () => {
    const cases: Array<() => Promise<unknown>> = [];
    fetchMock.mockRejectedValueOnce(new Error("token=network-secret"));
    cases.push(() => getWorkbenchStore(STORE_ID));
    fetchMock.mockResolvedValueOnce(
      new Response("password=raw-upstream", { status: 502 }),
    );
    cases.push(() => getWorkbenchStore(STORE_ID));
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ code: "BROKEN", message: "raw", requestId: "req" }, 409),
    );
    cases.push(() => getWorkbenchStore(STORE_ID));
    fetchMock.mockResolvedValueOnce(jsonResponse({ ...store, password: "secret" }));
    cases.push(() => getWorkbenchStore(STORE_ID));

    for (const call of cases) {
      const error = await call().catch((caught: unknown) => caught);
      expect(error).toBeInstanceOf(WorkbenchAPIError);
      expect(error).toMatchObject({
        status: expect.any(Number),
        code: expect.stringMatching(
          /^(WORKBENCH_REQUEST_FAILED|INVALID_WORKBENCH_RESPONSE)$/,
        ),
        requestId: "",
        fieldErrors: [],
      });
      expect(String(error)).not.toMatch(/network-secret|raw-upstream|secret/);
    }
  });

  it("rejects Store response values that violate the Task 7A public contract", async () => {
    const invalidPayloads = [
      { ...store, name: " North Shop" },
      { ...store, region: "SG\u0000" },
      { ...store, createdAt: "2026-08-30T01:02:03+08:00" },
      { ...store, updatedAt: "2026-02-30T02:03:04Z" },
      { ...list, items: Array.from({ length: 101 }, () => store) },
      {
        ...list,
        quota: { ...list.quota, allowed: false, reason: "" },
      },
      {
        ...list,
        pagination: { ...list.pagination, pageSize: 1, total: 0 },
      },
    ];

    for (const payload of invalidPayloads) {
      fetchMock.mockResolvedValueOnce(jsonResponse(payload));
      await expect(
        "items" in payload ? listWorkbenchStores({ page: 1, pageSize: 20 }) : getWorkbenchStore(STORE_ID),
      ).rejects.toMatchObject({ code: "INVALID_WORKBENCH_RESPONSE" });
    }
  });

  it("rejects valid success payloads returned with an operation's wrong 2xx status", async () => {
    const cases = [
      {
        status: 200,
        payload: store,
        call: () =>
          createWorkbenchStore(
            { name: "Shop", platform: "shein", region: "SG" },
            CREATE_KEY,
          ),
      },
      {
        status: 201,
        payload: list,
        call: () => listWorkbenchStores({ page: 1, pageSize: 20 }),
      },
      {
        status: 201,
        payload: store,
        call: () => getWorkbenchStore(STORE_ID),
      },
      {
        status: 201,
        payload: store,
        call: () =>
          updateWorkbenchStore(
            STORE_ID,
            { name: "Shop", region: "SG" },
            2,
          ),
      },
      {
        status: 201,
        payload: store,
        call: () => enableWorkbenchStore(STORE_ID, 2),
      },
      {
        status: 201,
        payload: store,
        call: () => disableWorkbenchStore(STORE_ID, 2),
      },
      {
        status: 201,
        payload: { id: STORE_ID, deleted: true, version: 3 },
        call: () => deleteWorkbenchStore(STORE_ID, 2, DELETE_KEY),
      },
    ];
    const results: string[] = [];

    for (const testCase of cases) {
      fetchMock.mockResolvedValueOnce(
        jsonResponse(testCase.payload, testCase.status),
      );
      const result = await testCase.call().catch((error: unknown) => error);
      results.push(
        result instanceof WorkbenchAPIError ? result.code : "resolved",
      );
    }

    expect(results).toEqual(
      Array.from({ length: cases.length }, () => "INVALID_WORKBENCH_RESPONSE"),
    );
  });
});
