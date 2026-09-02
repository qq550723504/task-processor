"use client";

import { usePathname } from "next/navigation";

import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";
import { QueryProvider } from "@/components/providers/query-provider";
import { ThemeProvider } from "@/components/providers/theme-provider";
import { ToastProvider } from "@/components/providers/toast-provider";
import { WorkbenchContextProvider } from "@/components/providers/workbench-context-provider";
import { ZitadelAuthGate } from "@/components/providers/zitadel-auth-gate";
import { WorkspaceAppShell } from "@/components/workbench/workspace-app-shell";

const publicRoutes = new Set([
  "/",
  "/login",
  "/unauthorized",
  "/privacy-policy",
  "/user-agreement",
  "/ai-compute-billing",
  "/service-agreement",
]);

export function isPublicRoute(pathname: string | null): boolean {
  return pathname !== null && publicRoutes.has(pathname);
}

export function isWorkbenchRoute(pathname: string | null): boolean {
  return (
    pathname !== null &&
    (pathname === "/workbench" || pathname.startsWith("/workbench/"))
  );
}

export function ApplicationFrame({ children }: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();

  // Public marketing, legal, and login routes must not initialize the authenticated
  // workspace shell (which reads client-side navigation state).
  if (isPublicRoute(pathname)) {
    return <>{children}</>;
  }

  if (isWorkbenchRoute(pathname)) {
    return (
      <ThemeProvider>
        <QueryProvider>
          <ToastProvider>
            <WorkbenchContextProvider>
              <WorkspaceAppShell>{children}</WorkspaceAppShell>
            </WorkbenchContextProvider>
          </ToastProvider>
        </QueryProvider>
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider>
      <QueryProvider>
        <ToastProvider>
          <ZitadelAuthGate>
            <ListingKitAppShell>{children}</ListingKitAppShell>
          </ZitadelAuthGate>
        </ToastProvider>
      </QueryProvider>
    </ThemeProvider>
  );
}
