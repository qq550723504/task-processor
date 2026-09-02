import { render, screen } from "@testing-library/react";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ usePathname: vi.fn() }));
vi.mock("@/components/providers/theme-provider", () => ({
  ThemeProvider: ({ children }: { children: ReactNode }) => (
    <div data-testid="theme-provider">{children}</div>
  ),
}));
vi.mock("@/components/providers/query-provider", () => ({
  QueryProvider: ({ children }: { children: ReactNode }) => (
    <div data-testid="query-provider">{children}</div>
  ),
}));
vi.mock("@/components/providers/toast-provider", () => ({
  ToastProvider: ({ children }: { children: ReactNode }) => (
    <div data-testid="toast-provider">{children}</div>
  ),
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({
  WorkbenchContextProvider: ({ children }: { children: ReactNode }) => (
    <div data-testid="workbench-context-provider">{children}</div>
  ),
}));
vi.mock("@/components/providers/zitadel-auth-gate", () => ({
  ZitadelAuthGate: ({ children }: { children: ReactNode }) => (
    <div data-testid="legacy-auth-gate">{children}</div>
  ),
}));
vi.mock("@/components/workbench/workspace-app-shell", () => ({
  WorkspaceAppShell: ({ children }: { children: ReactNode }) => (
    <div data-testid="workspace-app-shell">{children}</div>
  ),
}));
vi.mock("@/components/listingkit/shared/listingkit-app-shell", () => ({
  ListingKitAppShell: ({ children }: { children: ReactNode }) => (
    <div data-testid="legacy-listingkit-shell">{children}</div>
  ),
}));

import {
  ApplicationFrame,
  isPublicRoute,
  isWorkbenchRoute,
} from "./application-frame";

describe("isPublicRoute", () => {
  it("keeps every published policy document outside the authenticated workspace shell", () => {
    for (const pathname of ["/privacy-policy", "/user-agreement", "/ai-compute-billing", "/service-agreement"]) {
      expect(isPublicRoute(pathname)).toBe(true);
    }

    expect(isPublicRoute("/listing-kits/home")).toBe(false);
  });
});

describe("isWorkbenchRoute", () => {
  it("matches the Workbench root and descendants without matching lookalikes", () => {
    expect(isWorkbenchRoute("/workbench")).toBe(true);
    expect(isWorkbenchRoute("/workbench/no-organization")).toBe(true);
    expect(isWorkbenchRoute("/workbenches")).toBe(false);
    expect(isWorkbenchRoute(null)).toBe(false);
  });
});

describe("ApplicationFrame", () => {
  it("routes Workbench pages through the isolated provider and shell chain", () => {
    vi.mocked(usePathname).mockReturnValue("/workbench/no-organization");

    render(
      <ApplicationFrame>
        <p>route child</p>
      </ApplicationFrame>,
    );

    expect(screen.getByTestId("theme-provider")).toContainElement(
      screen.getByTestId("query-provider"),
    );
    expect(screen.getByTestId("query-provider")).toContainElement(
      screen.getByTestId("toast-provider"),
    );
    expect(screen.getByTestId("toast-provider")).toContainElement(
      screen.getByTestId("workbench-context-provider"),
    );
    expect(screen.getByTestId("workbench-context-provider")).toContainElement(
      screen.getByTestId("workspace-app-shell"),
    );
    expect(screen.queryByTestId("legacy-auth-gate")).not.toBeInTheDocument();
    expect(screen.queryByTestId("legacy-listingkit-shell")).not.toBeInTheDocument();
  });

  it("keeps legacy authenticated pages on the legacy auth gate and shell", () => {
    vi.mocked(usePathname).mockReturnValue("/listing-kits/home");

    render(
      <ApplicationFrame>
        <p>legacy child</p>
      </ApplicationFrame>,
    );

    expect(screen.getByTestId("legacy-auth-gate")).toContainElement(
      screen.getByTestId("legacy-listingkit-shell"),
    );
    expect(
      screen.queryByTestId("workbench-context-provider"),
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId("workspace-app-shell")).not.toBeInTheDocument();
  });

  it("keeps public routes shell-free", () => {
    vi.mocked(usePathname).mockReturnValue("/privacy-policy");

    render(
      <ApplicationFrame>
        <p>public child</p>
      </ApplicationFrame>,
    );

    expect(screen.getByText("public child")).toBeInTheDocument();
    expect(screen.queryByTestId("theme-provider")).not.toBeInTheDocument();
    expect(screen.queryByTestId("legacy-auth-gate")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("workbench-context-provider"),
    ).not.toBeInTheDocument();
  });
});
