import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SdsLoginPage } from "@/components/listingkit/sds-login/sds-login-page";

const getSDSLoginStatus = vi.fn();
const getSDSLoginAuthState = vi.fn();
const triggerSDSLogin = vi.fn();
const clearSDSLoginState = vi.fn();
const manualSDSLogin = vi.fn();

vi.mock("@/lib/api/sds-login", () => ({
  getSDSLoginStatus: (...args: unknown[]) => getSDSLoginStatus(...args),
  getSDSLoginAuthState: (...args: unknown[]) => getSDSLoginAuthState(...args),
  triggerSDSLogin: (...args: unknown[]) => triggerSDSLogin(...args),
  clearSDSLoginState: (...args: unknown[]) => clearSDSLoginState(...args),
  manualSDSLogin: (...args: unknown[]) => manualSDSLogin(...args),
}));

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <SdsLoginPage />
    </QueryClientProvider>,
  );
}

describe("SdsLoginPage", () => {
  beforeEach(() => {
    getSDSLoginStatus.mockReset();
    getSDSLoginAuthState.mockReset();
    triggerSDSLogin.mockReset();
    clearSDSLoginState.mockReset();
    manualSDSLogin.mockReset();

    getSDSLoginStatus.mockResolvedValue({
      tenant_id: "1",
      identifier: "869",
      merchant_name: "",
      username: "",
      has_cookie: false,
      has_access_token: false,
      waiting_for_verify_code: false,
      login_in_progress: false,
    });
    getSDSLoginAuthState.mockResolvedValue(null);
    triggerSDSLogin.mockResolvedValue({});
    clearSDSLoginState.mockResolvedValue({});
    manualSDSLogin.mockResolvedValue({});
  });

  it("shows a manual SDS login form when credentials are missing", async () => {
    renderPage();

    expect(await screen.findByText("处理 SDS 登录状态")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByLabelText("租户")).toHaveValue("1");
      expect(screen.getByLabelText("Identifier")).toHaveValue("869");
    });
    expect(screen.getByLabelText("商家名")).toHaveValue("");
    expect(screen.getByLabelText("用户名")).toHaveValue("");
    expect(screen.getByLabelText("密码")).toHaveValue("");
    expect(
      screen.getByRole("button", { name: "提交手动登录" }),
    ).toBeInTheDocument();
  });

  it("submits manual SDS login credentials and refreshes status", async () => {
    getSDSLoginStatus
      .mockResolvedValueOnce({
        tenant_id: "1",
        identifier: "869",
        merchant_name: "",
        username: "",
        has_cookie: false,
        has_access_token: false,
        waiting_for_verify_code: false,
        login_in_progress: false,
      })
      .mockResolvedValueOnce({
        tenant_id: "1",
        identifier: "869",
        merchant_name: "pod",
        username: "zone",
        has_cookie: true,
        has_access_token: true,
        waiting_for_verify_code: false,
        login_in_progress: false,
      });

    renderPage();

    await screen.findByText("处理 SDS 登录状态");

    fireEvent.change(screen.getByLabelText("商家名"), {
      target: { value: "pod" },
    });
    fireEvent.change(screen.getByLabelText("用户名"), {
      target: { value: "zone" },
    });
    fireEvent.change(screen.getByLabelText("密码"), {
      target: { value: "secret" },
    });

    fireEvent.click(screen.getByRole("button", { name: "提交手动登录" }));

    await waitFor(() => {
      expect(manualSDSLogin).toHaveBeenCalledWith({
        tenantID: "1",
        identifier: "869",
        merchantName: "pod",
        username: "zone",
        password: "secret",
      });
    });
    expect(
      await screen.findByText("已提交 SDS 手动登录，请稍后刷新状态查看是否恢复。"),
    ).toBeInTheDocument();
    expect(await screen.findByText("pod")).toBeInTheDocument();
  });

  it("keeps manual edits while still falling back to fetched defaults", async () => {
    getSDSLoginStatus.mockResolvedValue({
      tenant_id: "tenant-from-status",
      identifier: "identifier-from-status",
      merchant_name: "merchant-from-status",
      username: "username-from-status",
      has_cookie: false,
      has_access_token: false,
      waiting_for_verify_code: false,
      login_in_progress: false,
    });
    getSDSLoginAuthState.mockResolvedValue({
      tenant_id: "tenant-from-auth",
      identifier: "identifier-from-auth",
      merchant_name: "merchant-from-auth",
      username: "username-from-auth",
      access_token: "",
      cookies: [],
      source: "manual",
      current_url: "",
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getByLabelText("租户")).toHaveValue("tenant-from-status");
      expect(screen.getByLabelText("Identifier")).toHaveValue("identifier-from-status");
      expect(screen.getByLabelText("商家名")).toHaveValue("merchant-from-status");
      expect(screen.getByLabelText("用户名")).toHaveValue("username-from-status");
    });

    fireEvent.change(screen.getByLabelText("商家名"), {
      target: { value: "custom-merchant" },
    });
    fireEvent.change(screen.getByLabelText("用户名"), {
      target: { value: "custom-user" },
    });

    fireEvent.click(screen.getByRole("button", { name: "刷新状态" }));

    await waitFor(() => {
      expect(getSDSLoginStatus).toHaveBeenCalledTimes(2);
    });
    expect(screen.getByLabelText("商家名")).toHaveValue("custom-merchant");
    expect(screen.getByLabelText("用户名")).toHaveValue("custom-user");
    expect(screen.getByLabelText("租户")).toHaveValue("tenant-from-status");
    expect(screen.getByLabelText("Identifier")).toHaveValue("identifier-from-status");
  });
});
