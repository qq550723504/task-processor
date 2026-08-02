import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinLoginPage } from "@/components/listingkit/shein-login/shein-login-page";

const mocks = vi.hoisted(() => {
  const cancelMutation = {
    mutate: vi.fn(),
    isPending: false,
    error: null,
  };

  return {
    searchParams: new URLSearchParams(),
    cancelMutation,
    useSheinLoginAccounts: vi.fn(),
    useLoginSheinAccount: vi.fn(),
    useSubmitSheinVerifyCode: vi.fn(),
    useClearSheinCookie: vi.fn(),
    useClearSheinLastFailure: vi.fn(),
    useCancelSheinLogin: vi.fn(),
    useSheinLastFailure: vi.fn(),
  };
});

vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.searchParams,
}));

vi.mock("@/lib/query/use-shein-login", () => ({
  useSheinLoginAccounts: (...args: unknown[]) => mocks.useSheinLoginAccounts(...args),
  useLoginSheinAccount: (...args: unknown[]) => mocks.useLoginSheinAccount(...args),
  useSubmitSheinVerifyCode: (...args: unknown[]) => mocks.useSubmitSheinVerifyCode(...args),
  useClearSheinCookie: (...args: unknown[]) => mocks.useClearSheinCookie(...args),
  useClearSheinLastFailure: (...args: unknown[]) => mocks.useClearSheinLastFailure(...args),
  useCancelSheinLogin: (...args: unknown[]) => mocks.useCancelSheinLogin(...args),
  useSheinLastFailure: (...args: unknown[]) => mocks.useSheinLastFailure(...args),
}));

function idleMutation() {
  return {
    mutate: vi.fn(),
    isPending: false,
    error: null,
  };
}

describe("SheinLoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.searchParams = new URLSearchParams("tenant_id=227");
    mocks.cancelMutation.mutate.mockReset();
    mocks.cancelMutation.isPending = false;
    mocks.cancelMutation.error = null;

    mocks.useSheinLoginAccounts.mockReturnValue({
      data: [
        {
          account: {
            store_id: 870,
            tenant_id: 227,
            store_name: "SHEIN Active Store",
            username: "active-store",
          },
          has_cookie: false,
          cookie_ttl: 0,
          waiting_for_verify_code: true,
          login_in_progress: true,
          latest_attempt: {
            id: "attempt-870",
            status: "waiting_verify_code",
          },
        },
      ],
      isLoading: false,
      isError: false,
      isFetching: false,
      refetch: vi.fn(),
    });
    mocks.useLoginSheinAccount.mockReturnValue(idleMutation());
    mocks.useSubmitSheinVerifyCode.mockReturnValue(idleMutation());
    mocks.useClearSheinCookie.mockReturnValue(idleMutation());
    mocks.useClearSheinLastFailure.mockReturnValue(idleMutation());
    mocks.useCancelSheinLogin.mockReturnValue(mocks.cancelMutation);
    mocks.useSheinLastFailure.mockReturnValue({
      data: undefined,
      isLoading: false,
    });
  });

  it("shows active-row cancel controls and triggers cancellation", async () => {
    const user = userEvent.setup();
    render(<SheinLoginPage />);

    const row = (await screen.findByText("SHEIN Active Store")).closest("tr");
    expect(row).not.toBeNull();

    const cancelButton = within(row as HTMLElement).getByRole("button", {
      name: "Cancel Login for store 870",
    });
    expect(cancelButton).toBeInTheDocument();

    await user.click(cancelButton);

    expect(mocks.cancelMutation.mutate).toHaveBeenCalledWith(870, expect.any(Object));
  });

  it("closes the verify-code dialog without cancelling the login", async () => {
    const user = userEvent.setup();
    render(<SheinLoginPage />);

    const row = (await screen.findByText("SHEIN Active Store")).closest("tr");
    expect(row).not.toBeNull();

    await user.click(
      within(row as HTMLElement).getByRole("button", {
        name: "Open Verify Code for store 870",
      }),
    );

    await user.click(screen.getByRole("button", { name: "Close Verify Code Dialog" }));

    expect(mocks.cancelMutation.mutate).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Close Verify Code Dialog" })).not.toBeInTheDocument();
  });
});
