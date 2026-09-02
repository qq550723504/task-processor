import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { useEffect } from "react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  useWorkbenchContext,
  WorkbenchContextProvider,
} from "@/components/providers/workbench-context-provider";
import { WORKBENCH_CONTEXT_QUERY_KEY } from "@/lib/api/workbench-context";

const ORG_A_CONTEXT = {
  user: { id: "user-1" },
  homeOrganizationId: "org-a",
  effectiveOrganizationId: "org-a",
  selectionRequired: false,
  organizations: [
    { id: "org-a", name: "硕米科技", roles: ["org-a-admin"] },
    { id: "org-b", name: "星海贸易", roles: ["org-b-viewer"] },
  ],
};

const ORG_B_CONTEXT = {
  ...ORG_A_CONTEXT,
  effectiveOrganizationId: "org-b",
};

function ContextProbe() {
  const context = useWorkbenchContext();

  if (context.isLoading) return <p>loading</p>;
  if (context.blockingError) {
    return <p role="alert">blocked:{context.blockingError.code}</p>;
  }
  if (context.error) return <p role="alert">error:{context.error.code}</p>;

  return (
    <div>
      <p>user:{context.user?.id ?? "none"}</p>
      <p>home:{context.homeOrganizationId ?? "none"}</p>
      <p>organization:{context.effectiveOrganization?.name ?? "none"}</p>
      <p>roles:{context.roles.join(",") || "none"}</p>
      <p>selection:{String(context.selectionRequired)}</p>
      <p>switching:{String(context.isSwitching)}</p>
      <button onClick={() => context.switchOrganization("org-b")}>switch</button>
      <button onClick={context.retry}>retry</button>
    </div>
  );
}

function GuardedProbe({
  guard,
}: {
  guard: (target: { id: string; name: string; roles: string[] }) => boolean | Promise<boolean>;
}) {
  const context = useWorkbenchContext();
  useEffect(
    () => context.registerOrganizationSwitchGuard(guard),
    [context, guard],
  );
  return <button onClick={() => context.switchOrganization("org-b")}>guarded switch</button>;
}

function GuardLifecycleProbe({
  active,
  guard,
}: {
  active: boolean;
  guard: (target: { id: string; name: string; roles: string[] }) => boolean;
}) {
  const context = useWorkbenchContext();
  useEffect(() => active ? context.registerOrganizationSwitchGuard(guard) : undefined, [active, context, guard]);
  return <button onClick={() => context.switchOrganization("org-b")}>lifecycle switch</button>;
}

function SnapshotGuardProbe({ lateGuard }: { lateGuard: () => boolean }) {
  const context = useWorkbenchContext();
  useEffect(() => context.registerOrganizationSwitchGuard(() => {
    context.registerOrganizationSwitchGuard(lateGuard);
    return true;
  }), [context, lateGuard]);
  return <button onClick={() => context.switchOrganization("org-b")}>snapshot switch</button>;
}

function renderProvider(queryClient = createQueryClient()) {
  render(
    <QueryClientProvider client={queryClient}>
      <WorkbenchContextProvider>
        <ContextProbe />
      </WorkbenchContextProvider>
    </QueryClientProvider>,
  );
  return queryClient;
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

describe("WorkbenchContextProvider", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("derives only the effective Organization roles from validated context", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(Response.json(ORG_A_CONTEXT)),
    );

    renderProvider();

    expect(screen.getByText("loading")).toBeInTheDocument();
    expect(
      await screen.findByText("organization:硕米科技"),
    ).toBeInTheDocument();
    expect(screen.getByText("home:org-a")).toBeInTheDocument();
    expect(screen.getByText("roles:org-a-admin")).toBeInTheDocument();
    expect(screen.queryByText(/org-b-viewer/)).not.toBeInTheDocument();
  });

  it("clears old Organization queries before re-seeding the returned switch context", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    queryClient.setQueryData(["workbench", "org-a", "sentinel"], "old-data");
    const sentinelStateWhenNextContextWasInstalled: unknown[] = [];
    const unsubscribe = queryClient.getQueryCache().subscribe((event) => {
      if (
        event.query.queryHash === JSON.stringify(WORKBENCH_CONTEXT_QUERY_KEY) &&
        (event.query.state.data as typeof ORG_B_CONTEXT | undefined)
          ?.effectiveOrganizationId === "org-b"
      ) {
        sentinelStateWhenNextContextWasInstalled.push(
          queryClient.getQueryData(["workbench", "org-a", "sentinel"]),
        );
      }
    });

    renderProvider(queryClient);
    await screen.findByText("organization:硕米科技");
    await userEvent.click(screen.getByRole("button", { name: "switch" }));

    expect(
      await screen.findByText("organization:星海贸易"),
    ).toBeInTheDocument();
    expect(screen.getByText("home:org-a")).toBeInTheDocument();
    expect(screen.getByText("roles:org-b-viewer")).toBeInTheDocument();
    expect(
      queryClient.getQueryData(["workbench", "org-a", "sentinel"]),
    ).toBeUndefined();
    expect(queryClient.getQueryData(WORKBENCH_CONTEXT_QUERY_KEY)).toEqual(
      ORG_B_CONTEXT,
    );
    expect(sentinelStateWhenNextContextWasInstalled.length).toBeGreaterThan(0);
    expect(sentinelStateWhenNextContextWasInstalled).not.toContain("old-data");
    unsubscribe();
  });

  it("cancels an in-flight context refresh before switching and ignores its late response", async () => {
    let resolveRefresh!: (response: Response) => void;
    let refreshSignal: AbortSignal | undefined;
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockImplementationOnce((_input, init) => {
        refreshSignal = init?.signal ?? undefined;
        return new Promise<Response>((resolve) => {
          resolveRefresh = resolve;
        });
      })
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    const user = userEvent.setup();

    renderProvider(queryClient);
    await screen.findByText("organization:硕米科技");
    void queryClient.invalidateQueries({ queryKey: WORKBENCH_CONTEXT_QUERY_KEY });
    await waitFor(() => expect(refreshSignal).toBeDefined());

    await user.click(screen.getByRole("button", { name: "switch" }));
    expect(refreshSignal?.aborted).toBe(true);
    expect(await screen.findByText("organization:星海贸易")).toBeInTheDocument();

    resolveRefresh(Response.json(ORG_A_CONTEXT));
    await waitFor(() =>
      expect(screen.getByText("organization:星海贸易")).toBeInTheDocument(),
    );
  });

  it("pauses interval and focus refreshes for the entire switch request", async () => {
    let resolveSwitch!: (response: Response) => void;
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((input) => {
      if (input === "/api/workbench/context/effective-organization") {
        return new Promise<Response>((resolve) => {
          resolveSwitch = resolve;
        });
      }
      return Promise.resolve(Response.json(ORG_A_CONTEXT));
    });
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();

    renderProvider(queryClient);
    await screen.findByText("organization:硕米科技");
    await userEvent.click(screen.getByRole("button", { name: "switch" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(screen.getByText("switching:true")).toBeInTheDocument();

    void queryClient.invalidateQueries({ queryKey: WORKBENCH_CONTEXT_QUERY_KEY });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fetchMock).toHaveBeenCalledTimes(2);

    resolveSwitch(Response.json(ORG_B_CONTEXT));
    expect(await screen.findByText("organization:星海贸易")).toBeInTheDocument();
    expect(screen.getByText("switching:false")).toBeInTheDocument();
  });

  it("makes a successful retry authoritative after an explicit switch", async () => {
    const refreshed = {
      ...ORG_A_CONTEXT,
      effectiveOrganizationId: null,
      selectionRequired: true,
      organizations: ORG_A_CONTEXT.organizations,
    };
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT))
      .mockResolvedValueOnce(Response.json(refreshed));
    vi.stubGlobal("fetch", fetchMock);

    renderProvider();
    await screen.findByText("organization:硕米科技");
    await userEvent.click(screen.getByRole("button", { name: "switch" }));
    await screen.findByText("organization:星海贸易");
    await userEvent.click(screen.getByRole("button", { name: "retry" }));

    expect(await screen.findByText("organization:none")).toBeInTheDocument();
    expect(screen.getByText("roles:none")).toBeInTheDocument();
    expect(screen.getByText("selection:true")).toBeInTheDocument();
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
  });

  it("refreshes context after a switch when the query is invalidated", async () => {
    const refreshed = {
      ...ORG_A_CONTEXT,
      effectiveOrganizationId: "org-b",
      organizations: [
        { id: "org-a", name: "硕米科技", roles: [] },
        { id: "org-b", name: "星海贸易", roles: ["org-b-admin"] },
      ],
    };
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT))
      .mockResolvedValueOnce(Response.json(refreshed));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();

    renderProvider(queryClient);
    await screen.findByText("organization:硕米科技");
    await userEvent.click(screen.getByRole("button", { name: "switch" }));
    await screen.findByText("organization:星海贸易");

    await queryClient.invalidateQueries({
      queryKey: WORKBENCH_CONTEXT_QUERY_KEY,
    });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expect(await screen.findByText("roles:org-b-admin")).toBeInTheDocument();
  });

  it("clears scoped state and exposes a blocking error after an explicit switch rejection", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(
        Response.json(
          {
            code: "ORGANIZATION_ACCESS_REVOKED",
            message: "ignored",
            requestId: "req-revoked",
            fieldErrors: [],
          },
          { status: 403 },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    queryClient.setQueryData(["workbench", "org-a", "sentinel"], "old-data");

    renderProvider(queryClient);
    await screen.findByText("organization:硕米科技");
    await userEvent.click(screen.getByRole("button", { name: "switch" }));

    expect(
      await screen.findByRole("alert", {
        name: "",
      }),
    ).toHaveTextContent("blocked:ORGANIZATION_ACCESS_REVOKED");
    expect(
      queryClient.getQueryData(["workbench", "org-a", "sentinel"]),
    ).toBeUndefined();
    expect(
      queryClient.getQueryData(WORKBENCH_CONTEXT_QUERY_KEY),
    ).toBeUndefined();
    expect(screen.queryByText("roles:org-a-admin")).not.toBeInTheDocument();
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });

  it("keeps the full Organization list but no scoped roles when selection is required", async () => {
    const selectionContext = {
      ...ORG_A_CONTEXT,
      effectiveOrganizationId: null,
      selectionRequired: true,
    };
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(Response.json(selectionContext)),
    );

    renderProvider();

    expect(await screen.findByText("organization:none")).toBeInTheDocument();
    expect(screen.getByText("roles:none")).toBeInTheDocument();
    expect(screen.getByText("selection:true")).toBeInTheDocument();
  });

  it("cancels a denied switch guard without sending a switch request or clearing cache", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(Response.json(ORG_A_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    queryClient.setQueryData(["workbench", "org-a", "sentinel"], "old-data");
    render(
      <QueryClientProvider client={queryClient}>
        <WorkbenchContextProvider><GuardedProbe guard={() => false} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole("button", { name: "guarded switch" }));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryData(["workbench", "org-a", "sentinel"])).toBe("old-data");
  });

  it("passes the resolved target to every allowing guard before switching", async () => {
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT));
    const guard = vi.fn(() => true);
    vi.stubGlobal("fetch", fetchMock);
    render(
      <QueryClientProvider client={createQueryClient()}>
        <WorkbenchContextProvider><GuardedProbe guard={guard} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole("button", { name: "guarded switch" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(guard).toHaveBeenCalledWith(ORG_A_CONTEXT.organizations[1]);
  });

  it("cleans up an unmounted switch guard", async () => {
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT));
    const guard = vi.fn(() => false);
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    const view = render(
      <QueryClientProvider client={queryClient}>
        <WorkbenchContextProvider><GuardLifecycleProbe active guard={guard} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <WorkbenchContextProvider><GuardLifecycleProbe active={false} guard={guard} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await userEvent.click(screen.getByRole("button", { name: "lifecycle switch" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(guard).not.toHaveBeenCalled();
  });

  it("runs a snapshot of guards so registrations during a guard do not affect that switch", async () => {
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT));
    const lateGuard = vi.fn(() => false);
    vi.stubGlobal("fetch", fetchMock);
    render(
      <QueryClientProvider client={createQueryClient()}>
        <WorkbenchContextProvider><SnapshotGuardProbe lateGuard={lateGuard} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole("button", { name: "snapshot switch" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(lateGuard).not.toHaveBeenCalled();
  });

  it.each([
    ["throws", () => { throw new Error("no"); }],
    ["rejects", () => Promise.reject(new Error("no"))],
  ])("cancels a guard that %s without cache mutation", async (_name, guard) => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(Response.json(ORG_A_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    queryClient.setQueryData(["workbench", "org-a", "sentinel"], "old-data");
    render(
      <QueryClientProvider client={queryClient}>
        <WorkbenchContextProvider><GuardedProbe guard={guard} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole("button", { name: "guarded switch" }));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(queryClient.getQueryData(["workbench", "org-a", "sentinel"])).toBe("old-data");
  });

  it("refuses a pending Store mutation before invoking a guard", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(Response.json(ORG_A_CONTEXT));
    const guard = vi.fn(() => true);
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    render(
      <QueryClientProvider client={queryClient}>
        <WorkbenchContextProvider><GuardedProbe guard={guard} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const pending = queryClient.getMutationCache().build(queryClient, {
      mutationKey: ["workbench", "org-a", "stores", "mutation", "create"],
      mutationFn: () => new Promise(() => undefined),
    });
    void pending.execute(undefined);
    await waitFor(() => expect(queryClient.isMutating()).toBe(1));
    await userEvent.click(screen.getByRole("button", { name: "guarded switch" }));
    expect(guard).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("allows unrelated pending mutations to switch Organizations", async () => {
    const fetchMock = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ORG_A_CONTEXT))
      .mockResolvedValueOnce(Response.json(ORG_B_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    renderProvider(queryClient);
    await screen.findByText("organization:硕米科技");
    const pending = queryClient.getMutationCache().build(queryClient, {
      mutationKey: ["workbench", "org-a", "other", "mutation", "create"],
      mutationFn: () => new Promise(() => undefined),
    });
    void pending.execute(undefined);
    await userEvent.click(screen.getByRole("button", { name: "switch" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });

  it("rechecks pending Store mutations after an asynchronous guard", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(Response.json(ORG_A_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createQueryClient();
    let allowGuard!: () => void;
    const guard = () => new Promise<boolean>((resolve) => { allowGuard = () => resolve(true); });
    render(
      <QueryClientProvider client={queryClient}>
        <WorkbenchContextProvider><GuardedProbe guard={guard} /></WorkbenchContextProvider>
      </QueryClientProvider>,
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole("button", { name: "guarded switch" }));
    const pending = queryClient.getMutationCache().build(queryClient, {
      mutationKey: ["workbench", "org-a", "stores", "mutation", "create"],
      mutationFn: () => new Promise(() => undefined),
    });
    void pending.execute(undefined);
    await waitFor(() => expect(queryClient.isMutating()).toBe(1));
    allowGuard();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
