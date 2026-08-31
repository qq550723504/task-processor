import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  WorkbenchAPIError,
  type WorkbenchStore,
} from "@/lib/api/workbench-stores";
import {
  useCreateWorkbenchStore,
  useDeleteWorkbenchStore,
  useDisableWorkbenchStore,
  useEnableWorkbenchStore,
  useUpdateWorkbenchStore,
  useWorkbenchStore,
  useWorkbenchStores,
  workbenchStoreKeys,
} from "@/lib/query/use-workbench-stores";

const STORE_ID = "11111111-1111-4111-8111-111111111111";
const CREATE_KEY_1 = "22222222-2222-4222-8222-222222222221";
const CREATE_KEY_2 = "22222222-2222-4222-8222-222222222222";
const DELETE_KEY = "33333333-3333-4333-8333-333333333333";
const store = {
  id: STORE_ID,
  name: "North Shop",
  platform: "shein" as const,
  region: "SG",
  externalStoreId: "",
  lifecycleStatus: "active" as const,
  connectionStatus: "disconnected" as const,
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
    reason: "" as const,
  },
  pagination: { page: 1, pageSize: 20, total: 1 },
};

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  enable: vi.fn(),
  disable: vi.fn(),
  remove: vi.fn(),
  context: {
    effectiveOrganization: null as { id: string; name: string; roles: string[] } | null,
  },
}));

vi.mock("@/lib/api/workbench-stores", async (importOriginal) => {
  const actual = await importOriginal<
    typeof import("@/lib/api/workbench-stores")
  >();
  return {
    ...actual,
    listWorkbenchStores: (...args: unknown[]) => mocks.list(...args),
    getWorkbenchStore: (...args: unknown[]) => mocks.get(...args),
    createWorkbenchStore: (...args: unknown[]) => mocks.create(...args),
    updateWorkbenchStore: (...args: unknown[]) => mocks.update(...args),
    enableWorkbenchStore: (...args: unknown[]) => mocks.enable(...args),
    disableWorkbenchStore: (...args: unknown[]) => mocks.disable(...args),
    deleteWorkbenchStore: (...args: unknown[]) => mocks.remove(...args),
  };
});

vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => ({
    effectiveOrganization: mocks.context.effectiveOrganization,
  }),
}));

function createHarness() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retryDelay: 0 },
    },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

function selectOrganization(id: string | null) {
  mocks.context.effectiveOrganization = id
    ? { id, name: `Organization ${id}`, roles: [] }
    : null;
}

describe("Organization-scoped workbench Store queries", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    selectOrganization("org-a");
    mocks.list.mockResolvedValue(list);
    mocks.get.mockResolvedValue(store);
    mocks.create.mockResolvedValue(store);
    mocks.update.mockResolvedValue(store);
    mocks.enable.mockResolvedValue(store);
    mocks.disable.mockResolvedValue(store);
    mocks.remove.mockResolvedValue({ id: STORE_ID, deleted: true, version: 3 });
    vi.stubGlobal("crypto", {
      randomUUID: vi
        .fn()
        .mockReturnValueOnce(CREATE_KEY_1)
        .mockReturnValueOnce(CREATE_KEY_2)
        .mockReturnValueOnce(DELETE_KEY),
    });
  });

  it("partitions root, list, and item keys by effective Organization", () => {
    const filters = { page: 1, pageSize: 20 } as const;
    expect(workbenchStoreKeys.root("org-a")).toEqual([
      "workbench",
      "org-a",
      "stores",
    ]);
    expect(workbenchStoreKeys.list("org-a", filters)).not.toEqual(
      workbenchStoreKeys.list("org-b", filters),
    );
    expect(workbenchStoreKeys.item("org-a", STORE_ID)).not.toEqual(
      workbenchStoreKeys.item("org-b", STORE_ID),
    );
  });

  it("disables list and item requests without an effective Organization", async () => {
    selectOrganization(null);
    const { wrapper } = createHarness();
    const listHook = renderHook(
      () => useWorkbenchStores({ page: 1, pageSize: 20 }),
      { wrapper },
    );
    const itemHook = renderHook(() => useWorkbenchStore(STORE_ID), { wrapper });

    expect(listHook.result.current.fetchStatus).toBe("idle");
    expect(itemHook.result.current.fetchStatus).toBe("idle");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mocks.list).not.toHaveBeenCalled();
    expect(mocks.get).not.toHaveBeenCalled();
  });

  it("fails mutations locally when no effective Organization was captured", async () => {
    selectOrganization(null);
    const { client, wrapper } = createHarness();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useCreateWorkbenchStore(), { wrapper });

    await act(async () => {
      await expect(
        result.current.mutateAsync({
          name: "Shop",
          platform: "shein",
          region: "SG",
        }),
      ).rejects.toMatchObject({
        status: 409,
        code: "ORGANIZATION_SELECTION_REQUIRED",
      });
    });
    expect(mocks.create).not.toHaveBeenCalled();
    expect(invalidate).not.toHaveBeenCalled();
  });

  it("uses Organization only as a cache partition and never as HTTP input", async () => {
    const { wrapper } = createHarness();
    const { result } = renderHook(
      () => useWorkbenchStores({ page: 1, pageSize: 20, status: "active" }),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mocks.list).toHaveBeenCalledWith({
      page: 1,
      pageSize: 20,
      status: "active",
    });
  });

  it("reuses one create key across eligible retries and creates a new key for the next submission", async () => {
    mocks.create
      .mockRejectedValueOnce(
        new WorkbenchAPIError(503, "DEPENDENCY_UNAVAILABLE", "", "", []),
      )
      .mockResolvedValue(store);
    const { wrapper } = createHarness();
    const { result } = renderHook(() => useCreateWorkbenchStore(), { wrapper });
    const input = { name: "Shop", platform: "shein" as const, region: "SG" };

    await act(async () => {
      await result.current.mutateAsync(input);
    });
    expect(mocks.create).toHaveBeenNthCalledWith(1, input, CREATE_KEY_1);
    expect(mocks.create).toHaveBeenNthCalledWith(2, input, CREATE_KEY_1);

    await act(async () => {
      await result.current.mutateAsync(input);
    });
    expect(mocks.create).toHaveBeenNthCalledWith(3, input, CREATE_KEY_2);
  });

  it("reuses one delete key across retries and keeps it distinct from create", async () => {
    mocks.remove
      .mockRejectedValueOnce(
        new WorkbenchAPIError(0, "WORKBENCH_REQUEST_FAILED", "", "", []),
      )
      .mockResolvedValue({ id: STORE_ID, deleted: true, version: 3 });
    const { wrapper } = createHarness();
    const { result } = renderHook(
      () => ({
        create: useCreateWorkbenchStore(),
        remove: useDeleteWorkbenchStore(),
      }),
      { wrapper },
    );

    await act(async () => {
      await result.current.create.mutateAsync({
        name: "Shop",
        platform: "shein",
        region: "SG",
      });
      await result.current.remove.mutateAsync({ id: STORE_ID, version: 2 });
    });

    expect(mocks.create).toHaveBeenCalledWith(expect.anything(), CREATE_KEY_1);
    expect(mocks.remove).toHaveBeenNthCalledWith(1, STORE_ID, 2, CREATE_KEY_2);
    expect(mocks.remove).toHaveBeenNthCalledWith(2, STORE_ID, 2, CREATE_KEY_2);
    expect(CREATE_KEY_1).not.toBe(CREATE_KEY_2);
  });

  it("does not retry semantic 4xx failures", async () => {
    mocks.create.mockRejectedValue(
      new WorkbenchAPIError(409, "STORE_ALREADY_EXISTS", "", "", []),
    );
    const { wrapper } = createHarness();
    const { result } = renderHook(() => useCreateWorkbenchStore(), { wrapper });

    await act(async () => {
      await expect(
        result.current.mutateAsync({
          name: "Shop",
          platform: "shein",
          region: "SG",
        }),
      ).rejects.toMatchObject({ code: "STORE_ALREADY_EXISTS" });
    });
    expect(mocks.create).toHaveBeenCalledTimes(1);
  });

  it("preserves per-call mutation callbacks with the caller input", async () => {
    const { wrapper } = createHarness();
    const { result } = renderHook(() => useCreateWorkbenchStore(), { wrapper });
    const input = { name: "Shop", platform: "shein" as const, region: "SG" };
    const onSuccess = vi.fn();

    await act(async () => {
      await result.current.mutateAsync(input, { onSuccess });
    });

    expect(onSuccess).toHaveBeenCalledWith(
      store,
      input,
      undefined,
      expect.objectContaining({ client: expect.any(QueryClient) }),
    );
  });

  it("exposes only public variables for keyed and unkeyed mutations", async () => {
    const { wrapper } = createHarness();
    const { result } = renderHook(
      () => ({
        create: useCreateWorkbenchStore(),
        remove: useDeleteWorkbenchStore(),
        update: useUpdateWorkbenchStore(),
        enable: useEnableWorkbenchStore(),
      }),
      { wrapper },
    );
    const createInput = {
      name: "Shop",
      platform: "shein" as const,
      region: "SG",
    };
    const updateInput = {
      id: STORE_ID,
      version: 2,
      input: { name: "Renamed", region: "MY" },
    };
    const versionInput = { id: STORE_ID, version: 2 };

    await act(async () => {
      await result.current.create.mutateAsync(createInput);
      await result.current.remove.mutateAsync(versionInput);
      await result.current.update.mutateAsync(updateInput);
      await result.current.enable.mutateAsync(versionInput);
    });
    await waitFor(() => {
      expect(result.current.create.variables).toBeDefined();
      expect(result.current.remove.variables).toBeDefined();
      expect(result.current.update.variables).toBeDefined();
      expect(result.current.enable.variables).toBeDefined();
    });

    expect(result.current.create.variables).toEqual(createInput);
    expect(result.current.create.variables).not.toHaveProperty("organizationId");
    expect(result.current.create.variables).not.toHaveProperty("operationKey");
    expect(result.current.remove.variables).toEqual(versionInput);
    expect(result.current.remove.variables).not.toHaveProperty("organizationId");
    expect(result.current.remove.variables).not.toHaveProperty("operationKey");
    expect(result.current.update.variables).toEqual(updateInput);
    expect(result.current.update.variables).not.toHaveProperty("organizationId");
    expect(result.current.update.variables).not.toHaveProperty("operationKey");
    expect(result.current.enable.variables).toEqual(versionInput);
    expect(result.current.enable.variables).not.toHaveProperty("organizationId");
    expect(result.current.enable.variables).not.toHaveProperty("operationKey");
  });

  it("passes the last projection version to update, enable, and disable", async () => {
    const { wrapper } = createHarness();
    const { result } = renderHook(
      () => ({
        update: useUpdateWorkbenchStore(),
        enable: useEnableWorkbenchStore(),
        disable: useDisableWorkbenchStore(),
      }),
      { wrapper },
    );

    await act(async () => {
      await result.current.update.mutateAsync({
        id: STORE_ID,
        version: 7,
        input: { name: "Renamed", region: "MY" },
      });
      await result.current.enable.mutateAsync({ id: STORE_ID, version: 8 });
      await result.current.disable.mutateAsync({ id: STORE_ID, version: 9 });
    });

    expect(mocks.update).toHaveBeenCalledWith(
      STORE_ID,
      { name: "Renamed", region: "MY" },
      7,
    );
    expect(mocks.enable).toHaveBeenCalledWith(STORE_ID, 8);
    expect(mocks.disable).toHaveBeenCalledWith(STORE_ID, 9);
  });

  it("invalidates only each successful operation's captured Organization root", async () => {
    const { client, wrapper } = createHarness();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const hooks = renderHook(
      () => ({
        update: useUpdateWorkbenchStore(),
        enable: useEnableWorkbenchStore(),
        disable: useDisableWorkbenchStore(),
        remove: useDeleteWorkbenchStore(),
      }),
      { wrapper },
    );

    await act(async () => {
      await hooks.result.current.update.mutateAsync({
        id: STORE_ID,
        version: 2,
        input: { name: "Renamed", region: "MY" },
      });
      await hooks.result.current.enable.mutateAsync({ id: STORE_ID, version: 3 });
      await hooks.result.current.disable.mutateAsync({ id: STORE_ID, version: 4 });
      await hooks.result.current.remove.mutateAsync({ id: STORE_ID, version: 5 });
    });

    expect(invalidate).toHaveBeenCalledTimes(4);
    for (const [filters] of invalidate.mock.calls) {
      expect(filters).toEqual({
        queryKey: ["workbench", "org-a", "stores"],
      });
    }
  });

  it("does not let an in-flight success invalidate a newly selected Organization", async () => {
    let resolveCreate!: (value: typeof store) => void;
    mocks.create.mockReturnValue(
      new Promise<typeof store>((resolve) => {
        resolveCreate = resolve;
      }),
    );
    const { client, wrapper } = createHarness();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const { result, rerender } = renderHook(() => useCreateWorkbenchStore(), {
      wrapper,
    });

    let submission!: Promise<WorkbenchStore>;
    act(() => {
      submission = result.current.mutateAsync({
        name: "Shop",
        platform: "shein",
        region: "SG",
      });
    });
    await waitFor(() => expect(mocks.create).toHaveBeenCalled());
    selectOrganization("org-b");
    rerender();
    await act(async () => {
      resolveCreate(store);
      await submission;
    });

    expect(invalidate).toHaveBeenCalledWith({
      queryKey: ["workbench", "org-a", "stores"],
    });
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: ["workbench", "org-b", "stores"],
    });
  });
});
