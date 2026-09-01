import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

const navigation = vi.hoisted(() => ({
  pathname: "/workbench",
  replace: vi.fn(),
}));

const injectedWorkbenchContext = vi.hoisted(() => ({ value: null as unknown }));

vi.mock("next/navigation", () => ({
  usePathname: () => navigation.pathname,
  useRouter: () => ({ replace: navigation.replace }),
}));

vi.mock(
  "@/components/providers/workbench-context-provider",
  async (importOriginal) => {
    const actual =
      await importOriginal<
        typeof import("@/components/providers/workbench-context-provider")
      >();
    return {
      ...actual,
      useWorkbenchContext: () =>
        injectedWorkbenchContext.value ?? actual.useWorkbenchContext(),
    };
  },
);

import { WorkbenchContextProvider } from "@/components/providers/workbench-context-provider";
import { WorkspaceAppShell } from "@/components/workbench/workspace-app-shell";

const ACTIVE_CONTEXT = {
  user: { id: "user-1" },
  homeOrganizationId: "org-a",
  effectiveOrganizationId: "org-a",
  selectionRequired: false,
  organizations: [
    { id: "org-a", name: "硕米科技", roles: ["role-a"] },
    { id: "org-b", name: "星海贸易", roles: ["role-b"] },
  ],
};

function renderShell(child = <p>organization child</p>) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <WorkbenchContextProvider>
        <WorkspaceAppShell>{child}</WorkspaceAppShell>
      </WorkbenchContextProvider>
    </QueryClientProvider>,
  );
}

describe("WorkspaceAppShell", () => {
  afterEach(() => {
    navigation.pathname = "/workbench";
    navigation.replace.mockReset();
    injectedWorkbenchContext.value = null;
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("shows a bounded loading shell before rendering Organization children", () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockReturnValue(new Promise(() => {})),
    );

    renderShell();

    expect(screen.getByRole("status")).toHaveTextContent("正在加载工作台");
    expect(screen.queryByText("organization child")).not.toBeInTheDocument();
  });

  it("renders the two implemented desktop navigation groups without extra menu items", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(Response.json(ACTIVE_CONTEXT)),
    );

    renderShell();

    expect(await screen.findByText("organization child")).toBeInTheDocument();
    expect(screen.getByText("硕米智能引擎")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "当前企业" })).toHaveValue(
      "org-a",
    );
    const navigationLandmark = screen.getByRole("navigation", {
      name: "工作台导航",
    });
    expect(
      within(navigationLandmark).getByRole("link", { name: "工作台" }),
    ).toHaveAttribute("href", "/workbench");
    expect(within(navigationLandmark).getAllByRole("link")).toHaveLength(2);
    expect(screen.getByText("店铺中心")).toBeInTheDocument();
    expect(
      within(navigationLandmark).getByRole("link", { name: "我的店铺" }),
    ).toHaveAttribute("href", "/workbench/stores");
    expect(screen.getAllByRole("main")).toHaveLength(1);
  });

  it("reveals a reachable mobile navigation without using the desktop sidebar trigger", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(Response.json(ACTIVE_CONTEXT)),
    );
    const user = userEvent.setup();

    renderShell();

    const mobileNavigationButton = await screen.findByRole("button", {
      name: "打开工作台导航",
    });
    expect(mobileNavigationButton).toHaveAttribute("aria-expanded", "false");
    expect(mobileNavigationButton).toHaveClass("md:hidden");
    expect(screen.getByRole("button", { name: "折叠桌面导航" })).toHaveClass(
      "hidden",
      "md:inline-flex",
    );
    expect(
      screen.queryByRole("navigation", { name: "移动工作台导航" }),
    ).not.toBeInTheDocument();

    mobileNavigationButton.focus();
    await user.keyboard("[Enter]");

    expect(mobileNavigationButton).toHaveAttribute("aria-expanded", "true");
    const mobileNavigation = screen.getByRole("navigation", {
      name: "移动工作台导航",
    });
    const mobileWorkbenchLink = within(mobileNavigation).getByRole("link", {
      name: "工作台",
    });
    expect(mobileWorkbenchLink).toHaveAttribute("href", "/workbench");
    mobileWorkbenchLink.addEventListener("click", (event) =>
      event.preventDefault(),
    );

    await user.click(mobileWorkbenchLink);

    expect(mobileNavigationButton).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByRole("navigation", { name: "移动工作台导航" }),
    ).not.toBeInTheDocument();
  });

  it("uses exact Workbench and prefix Store active matching in both navigation modes", async () => {
    navigation.pathname = "/workbench/stores/11111111-1111-4111-8111-111111111111";
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(Response.json(ACTIVE_CONTEXT)),
    );
    const user = userEvent.setup();
    renderShell();

    const desktopNavigation = await screen.findByRole("navigation", { name: "工作台导航" });
    expect(within(desktopNavigation).getByRole("link", { name: "我的店铺" })).toHaveAttribute("aria-current", "page");
    expect(within(desktopNavigation).getByRole("link", { name: "工作台" })).not.toHaveAttribute("aria-current");
    await user.click(screen.getByRole("button", { name: "打开工作台导航" }));
    expect(within(screen.getByRole("navigation", { name: "移动工作台导航" })).getByRole("link", { name: "我的店铺" })).toHaveAttribute("aria-current", "page");
  });

  it("persistently identifies delegated operation with safe Organization names", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json({
          ...ACTIVE_CONTEXT,
          effectiveOrganizationId: "org-b",
        }),
      ),
    );

    renderShell();

    expect(
      await screen.findByRole("status", { name: "企业代管状态" }),
    ).toHaveTextContent("正在代管星海贸易");
    expect(
      screen.getByRole("status", { name: "企业代管状态" }),
    ).toHaveTextContent("账号归属硕米科技");
  });

  it("uses a stable non-ID fallback for an injected unnamed delegated target", async () => {
    injectedWorkbenchContext.value = {
      user: { id: "user-1" },
      homeOrganizationId: "org-a",
      organizations: [
        { id: "org-a", name: "硕米科技", roles: ["role-a"] },
        { id: "org-b", name: "", roles: ["role-b"] },
      ],
      effectiveOrganization: { id: "org-b", name: "", roles: ["role-b"] },
      roles: ["role-b"],
      selectionRequired: false,
      isLoading: false,
      isSwitching: false,
      error: null,
      blockingError: null,
      retry: vi.fn(),
      switchOrganization: vi.fn(),
    };

    render(
      <WorkspaceAppShell>
        <p>organization child</p>
      </WorkspaceAppShell>,
    );

    const indicator = await screen.findByRole("status", {
      name: "企业代管状态",
    });
    expect(indicator).toHaveTextContent("正在代管当前企业");
    expect(indicator).not.toHaveTextContent("org-b");
  });

  it("keeps the switcher available but withholds scoped children when selection is required", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json({
          ...ACTIVE_CONTEXT,
          effectiveOrganizationId: null,
          selectionRequired: true,
        }),
      ),
    );

    renderShell();

    expect(
      await screen.findByRole("combobox", { name: "当前企业" }),
    ).toHaveValue("");
    expect(screen.getByRole("status")).toHaveTextContent("请选择企业以继续");
    expect(screen.queryByText("organization child")).not.toBeInTheDocument();
  });

  it("replaces a zero-Organization route without redirect-looping on the destination", async () => {
    const noOrganizationContext = {
      ...ACTIVE_CONTEXT,
      effectiveOrganizationId: null,
      selectionRequired: false,
      organizations: [],
    };
    vi.stubGlobal(
      "fetch",
      vi
        .fn<typeof fetch>()
        .mockImplementation(() =>
          Promise.resolve(Response.json(noOrganizationContext)),
        ),
    );

    const firstRender = renderShell();

    await waitFor(() => {
      expect(navigation.replace).toHaveBeenCalledWith(
        "/workbench/no-organization",
      );
    });
    expect(screen.queryByText("organization child")).not.toBeInTheDocument();
    firstRender.unmount();

    navigation.replace.mockReset();
    navigation.pathname = "/workbench/no-organization";
    renderShell(<p>no Organization access page</p>);

    expect(
      await screen.findByText("no Organization access page"),
    ).toBeInTheDocument();
    expect(navigation.replace).not.toHaveBeenCalled();
  });

  it("renders stable-code copy for initial errors and recovers only on explicit retry", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        Response.json(
          {
            code: "DEPENDENCY_UNAVAILABLE",
            message: "do not render this raw text",
            requestId: "req-down",
            fieldErrors: [],
          },
          { status: 503 },
        ),
      )
      .mockResolvedValueOnce(Response.json(ACTIVE_CONTEXT));
    vi.stubGlobal("fetch", fetchMock);

    renderShell();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "企业权限服务暂时不可用，请稍后重试",
    );
    expect(screen.queryByText(/do not render/)).not.toBeInTheDocument();
    expect(screen.queryByText("organization child")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "重新加载" }));

    expect(await screen.findByText("organization child")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("redirects authentication failures to login instead of retrying the stale session", async () => {
    const retry = vi.fn();
    injectedWorkbenchContext.value = {
      user: null,
      homeOrganizationId: null,
      organizations: [],
      effectiveOrganization: null,
      roles: [],
      selectionRequired: false,
      isLoading: false,
      isSwitching: false,
      error: { code: "AUTHENTICATION_REQUIRED" },
      blockingError: null,
      retry,
      switchOrganization: vi.fn(),
    };

    render(<WorkspaceAppShell><p>organization child</p></WorkspaceAppShell>);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "登录状态已失效，请重新登录",
    );
    await userEvent.click(screen.getByRole("button", { name: "重新加载" }));

    expect(navigation.replace).toHaveBeenCalledWith(
      "/login?returnTo=%2Fworkbench",
    );
    expect(retry).not.toHaveBeenCalled();
  });

  it("removes old scoped children and blocks after an explicit switch failure", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(ACTIVE_CONTEXT))
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

    renderShell();
    await userEvent.selectOptions(
      await screen.findByRole("combobox", { name: "当前企业" }),
      "org-b",
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "你对所选企业的访问权限已被撤销",
    );
    expect(screen.queryByText("organization child")).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
