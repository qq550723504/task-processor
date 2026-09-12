import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
const navigation = vi.hoisted(() => ({ replace: vi.fn() }));
vi.mock("next/navigation", () => ({ usePathname: () => "/capture/1688", useRouter: () => navigation }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => ({ isLoading: false, error: { code: "AUTHENTICATION_REQUIRED" }, blockingError: null, organizations: [], selectionRequired: false }) }));
import { WorkspaceAppShell } from "@/components/workbench/workspace-app-shell";
afterEach(() => { navigation.replace.mockReset(); window.history.replaceState(null, "", "/"); });
it("keeps the original tab and opens a key-free login in a separate tab", () => {
  const key = "22222222-2222-4222-8222-22222222222b";
  window.history.replaceState(null, "", `/capture/1688#operationKey=${key}`);
  render(<WorkspaceAppShell><p>must stay hidden</p></WorkspaceAppShell>);
  expect(screen.queryByText("must stay hidden")).not.toBeInTheDocument();
  const link = screen.getByRole("link", { name: "在新标签页重新登录" });
  expect(link).toHaveAttribute("href", "/login?returnTo=%2Fworkbench");
  expect(link).toHaveAttribute("target", "_blank");
  expect(link).toHaveAttribute("rel", "noopener noreferrer");
  expect(link).toHaveAttribute("referrerpolicy", "no-referrer");
  expect(link.getAttribute("href")).not.toContain(key);
  expect(navigation.replace).not.toHaveBeenCalled();
  expect(window.location.hash).toBe(`#operationKey=${key}`);
  expect(screen.getByRole("button", { name: "登录完成后刷新此页核实" })).toBeVisible();
});
it.each(["#operationKey=bad", "#operationKey=22222222-2222-4222-8222-22222222222b&actor=x", "#idempotencyKey=22222222-2222-4222-8222-22222222222b", "?x=1#operationKey=22222222-2222-4222-8222-22222222222b"])("does not extend the recovery exception to %s", suffix => {
  window.history.replaceState(null, "", `/capture/1688${suffix}`);
  render(<WorkspaceAppShell><p>hidden</p></WorkspaceAppShell>);
  expect(screen.queryByRole("link", { name: "在新标签页重新登录" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button"));
  expect(navigation.replace).toHaveBeenCalledTimes(1);
});
