import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { WorkbenchContextProvider } from "@/components/providers/workbench-context-provider";
import { OrganizationSwitcher } from "@/components/workbench/organization-switcher";

const MULTI_ORG_CONTEXT = {
  user: { id: "user-1" },
  homeOrganizationId: "org-a",
  effectiveOrganizationId: "org-a",
  selectionRequired: false,
  organizations: [
    { id: "org-a", name: "硕米科技", roles: ["role-a"] },
    { id: "org-b", name: "星海贸易", roles: ["role-b"] },
  ],
};

function renderSwitcher() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <WorkbenchContextProvider>
        <OrganizationSwitcher />
      </WorkbenchContextProvider>
    </QueryClientProvider>,
  );
}

describe("OrganizationSwitcher", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders one accessible Organization as a non-interactive global label", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json({
          ...MULTI_ORG_CONTEXT,
          organizations: [MULTI_ORG_CONTEXT.organizations[0]],
        }),
      ),
    );

    renderSwitcher();

    expect(await screen.findByText("硕米科技")).toBeInTheDocument();
    expect(screen.getByText("当前企业")).toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("uses an accessible Organization-ID select and disables it while switching", async () => {
    let resolveSwitch!: (response: Response) => void;
    const switchResponse = new Promise<Response>((resolve) => {
      resolveSwitch = resolve;
    });
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(MULTI_ORG_CONTEXT))
      .mockReturnValueOnce(switchResponse);
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();

    renderSwitcher();
    const select = await screen.findByRole("combobox", { name: "当前企业" });
    expect(select).toHaveValue("org-a");

    await user.selectOptions(select, "org-b");

    expect(select).toBeDisabled();
    resolveSwitch(
      Response.json({ ...MULTI_ORG_CONTEXT, effectiveOrganizationId: "org-b" }),
    );
    expect(await screen.findByRole("combobox", { name: "当前企业" })).toHaveValue(
      "org-b",
    );
  });

  it("shows a non-selected prompt when Organization selection is required", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json({
          ...MULTI_ORG_CONTEXT,
          effectiveOrganizationId: null,
          selectionRequired: true,
        }),
      ),
    );

    renderSwitcher();

    const select = await screen.findByRole("combobox", { name: "当前企业" });
    expect(select).toHaveValue("");
    expect(screen.getByRole("option", { name: "请选择企业" })).toBeDisabled();
  });

  it("announces a stable-code-based access error after a rejected switch", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockResolvedValueOnce(Response.json(MULTI_ORG_CONTEXT))
        .mockResolvedValueOnce(
          Response.json(
            {
              code: "ORGANIZATION_ACCESS_DENIED",
              message: "raw backend text must not be required",
              requestId: "req-denied",
              fieldErrors: [],
            },
            { status: 403 },
          ),
        ),
    );

    renderSwitcher();
    await userEvent.selectOptions(
      await screen.findByRole("combobox", { name: "当前企业" }),
      "org-b",
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "你没有访问所选企业的权限",
    );
    expect(screen.queryByText(/raw backend text/)).not.toBeInTheDocument();
  });
});
