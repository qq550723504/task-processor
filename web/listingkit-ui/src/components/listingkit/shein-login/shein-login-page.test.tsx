import * as React from "react";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinLoginPage } from "@/components/listingkit/shein-login/shein-login-page";

const mocks = vi.hoisted(() => {
  const cancelController = {
    pending: new Map<number, { fail: (message?: string) => void; succeed: () => void }>(),
    reset() {
      this.pending.clear();
    },
    fail(storeID: number, message = "Cancel login failed.") {
      const pending = this.pending.get(storeID);
      if (!pending) {
        throw new Error(`No pending cancellation for store ${storeID}`);
      }
      pending.fail(message);
    },
    succeed(storeID: number) {
      const pending = this.pending.get(storeID);
      if (!pending) {
        throw new Error(`No pending cancellation for store ${storeID}`);
      }
      pending.succeed();
    },
  };

  const cancelMutation: {
    mutateAsync: ReturnType<typeof vi.fn>;
    isPending: boolean;
    error: Error | null;
  } = {
    mutateAsync: vi.fn(),
    isPending: false,
    error: null,
  };

  return {
    searchParams: new URLSearchParams(),
    cancelController,
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
    mutateAsync: vi.fn(),
    isPending: false,
    error: null,
  };
}

describe("SheinLoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.searchParams = new URLSearchParams("tenant_id=227");
    mocks.cancelController.reset();
    mocks.cancelMutation.mutateAsync.mockReset();
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
        {
          account: {
            store_id: 871,
            tenant_id: 227,
            store_name: "SHEIN Backup Store",
            username: "backup-store",
          },
          has_cookie: false,
          cookie_ttl: 0,
          waiting_for_verify_code: true,
          login_in_progress: true,
          latest_attempt: {
            id: "attempt-871",
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

    expect(mocks.cancelMutation.mutateAsync).toHaveBeenCalledWith(870);
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

    expect(mocks.cancelMutation.mutateAsync).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Close Verify Code Dialog" })).not.toBeInTheDocument();
  });

  it("does not offer cancellation for inline login progress without an owned attempt", async () => {
    mocks.useSheinLoginAccounts.mockReturnValue({
      data: [
        {
          account: {
            store_id: 872,
            tenant_id: 227,
            store_name: "SHEIN Inline Store",
            username: "inline-store",
          },
          has_cookie: false,
          cookie_ttl: 0,
          waiting_for_verify_code: false,
          login_in_progress: true,
        },
      ],
      isLoading: false,
      isError: false,
      isFetching: false,
      refetch: vi.fn(),
    });

    render(<SheinLoginPage />);

    const row = (await screen.findByText("SHEIN Inline Store")).closest("tr");
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).queryByRole("button", { name: /Cancel Login/ })).not.toBeInTheDocument();
  });

  it("keeps cancellation pending and errors scoped to the target store", async () => {
    const user = userEvent.setup();
    mocks.useCancelSheinLogin.mockImplementation(() => {
      const [pendingStoreIDs, setPendingStoreIDs] = React.useState<number[]>([]);
      const [error, setError] = React.useState<Error | null>(null);

      return {
        mutateAsync: (storeID: number) =>
          new Promise<void>((resolve, reject) => {
            setPendingStoreIDs((previous) =>
              previous.includes(storeID) ? previous : [...previous, storeID],
            );
            mocks.cancelController.pending.set(storeID, {
              fail: (message = "Cancel login failed.") => {
                const nextError = new Error(message);
                setError(nextError);
                setPendingStoreIDs((previous) => previous.filter((value) => value !== storeID));
                mocks.cancelController.pending.delete(storeID);
                reject(nextError);
              },
              succeed: () => {
                setError(null);
                setPendingStoreIDs((previous) => previous.filter((value) => value !== storeID));
                mocks.cancelController.pending.delete(storeID);
                resolve();
              },
            });
          }),
        isPending: pendingStoreIDs.length > 0,
        error,
      };
    });

    render(<SheinLoginPage />);

    const activeRow = (await screen.findByText("SHEIN Active Store")).closest("tr");
    const backupRow = (await screen.findByText("SHEIN Backup Store")).closest("tr");
    expect(activeRow).not.toBeNull();
    expect(backupRow).not.toBeNull();

    await user.click(
      within(activeRow as HTMLElement).getByRole("button", {
        name: "Cancel Login for store 870",
      }),
    );

    expect(
      within(activeRow as HTMLElement).getByRole("button", {
        name: "Cancelling login for store 870",
      }),
    ).toHaveAttribute("aria-busy", "true");
    expect(
      within(activeRow as HTMLElement).getByRole("button", {
        name: "重新登录",
      }),
    ).toBeDisabled();
    expect(
      within(activeRow as HTMLElement).getByRole("button", {
        name: "Open Verify Code for store 870",
      }),
    ).toBeDisabled();
    expect(screen.getByRole("status")).toHaveTextContent("Cancelling the active login attempt...");

    expect(
      within(backupRow as HTMLElement).getByRole("button", {
        name: "Cancel Login for store 871",
      }),
    ).toBeEnabled();
    expect(
      within(backupRow as HTMLElement).getByRole("button", {
        name: "重新登录",
      }),
    ).toBeEnabled();
    expect(
      within(backupRow as HTMLElement).getByRole("button", {
        name: "Open Verify Code for store 871",
      }),
    ).toBeEnabled();

    await user.click(
      within(backupRow as HTMLElement).getByRole("button", {
        name: "Cancel Login for store 871",
      }),
    );
    expect(
      within(backupRow as HTMLElement).getByRole("button", {
        name: "Cancelling login for store 871",
      }),
    ).toHaveAttribute("aria-busy", "true");

    await act(async () => {
      mocks.cancelController.fail(870, "Cancel login failed for store 870.");
    });

    expect(
      within(backupRow as HTMLElement).getByRole("button", {
        name: "Cancelling login for store 871",
      }),
    ).toBeDisabled();

    await act(async () => {
      mocks.cancelController.succeed(871);
    });
    expect(screen.getByRole("status")).toHaveTextContent("Cancel login failed for store 870.");

    await user.click(
      within(backupRow as HTMLElement).getByRole("button", {
        name: "Open Verify Code for store 871",
      }),
    );

    expect(screen.getByRole("dialog", { name: "Submit Verify Code" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close Verify Code Dialog" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel Login" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Cancel Login" }));

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close Verify Code Dialog" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancelling..." })).toBeDisabled();

    await act(async () => {
      mocks.cancelController.fail(871, "Cancel login failed for store 871.");
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Cancel login failed for store 871.");
    expect(screen.getByRole("alert")).not.toHaveTextContent("Cancel login failed for store 870.");
  });
});
