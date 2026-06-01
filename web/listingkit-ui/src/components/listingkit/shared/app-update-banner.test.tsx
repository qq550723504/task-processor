import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AppUpdateBanner } from "@/components/listingkit/shared/app-update-banner";
import { resolveAppUpdatePollIntervalMs } from "@/components/listingkit/shared/app-update-banner";

describe("AppUpdateBanner", () => {
  async function flushAsyncWork() {
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
  }

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("uses a positive environment-derived poll interval when provided", () => {
    expect(resolveAppUpdatePollIntervalMs("15000")).toBe(15_000);
  });

  it("falls back to the default poll interval for invalid values", () => {
    expect(resolveAppUpdatePollIntervalMs("0")).toBe(60_000);
    expect(resolveAppUpdatePollIntervalMs("-1")).toBe(60_000);
    expect(resolveAppUpdatePollIntervalMs("abc")).toBe(60_000);
  });

  it("stays hidden when the detected build does not change", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json({
        appVersion: "0.1.0",
        buildId: "build-a",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    vi.useFakeTimers();

    render(<AppUpdateBanner pollIntervalMs={1_000} />);
    await flushAsyncWork();

    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(1_000);
    });
    await flushAsyncWork();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(screen.queryByText("发现新版本，刷新后即可使用。")).not.toBeInTheDocument();
  });

  it("shows a refresh banner after a new build is detected", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        Response.json({
          appVersion: "0.1.0",
          buildId: "build-a",
        }),
      )
      .mockResolvedValueOnce(
        Response.json({
          appVersion: "0.1.0",
          buildId: "build-b",
        }),
      );
    vi.stubGlobal("fetch", fetchMock);
    vi.useFakeTimers();

    render(<AppUpdateBanner pollIntervalMs={1_000} />);
    await flushAsyncWork();

    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(1_000);
    });
    await flushAsyncWork();

    expect(screen.getByText("发现新版本，刷新后即可使用。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "立即刷新" })).toBeInTheDocument();
  });

  it("reloads the page when the refresh action is clicked", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        Response.json({
          appVersion: "0.1.0",
          buildId: "build-a",
        }),
      )
      .mockResolvedValueOnce(
        Response.json({
          appVersion: "0.1.0",
          buildId: "build-b",
        }),
      );
    vi.stubGlobal("fetch", fetchMock);
    vi.useFakeTimers();

    const reload = vi.fn();

    render(<AppUpdateBanner onRefresh={reload} pollIntervalMs={1_000} />);
    await flushAsyncWork();

    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(1_000);
    });
    await flushAsyncWork();

    fireEvent.click(screen.getByRole("button", { name: "立即刷新" }));

    expect(reload).toHaveBeenCalledTimes(1);
  });
});
