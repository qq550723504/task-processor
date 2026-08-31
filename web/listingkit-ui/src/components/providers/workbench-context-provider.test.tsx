import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
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
      <button onClick={() => context.switchOrganization("org-b")}>
        switch
      </button>
    </div>
  );
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
});
