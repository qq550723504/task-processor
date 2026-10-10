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
  "/referrals/register",
]);

export function isPublicRoute(pathname: string | null): boolean {
  return pathname !== null && publicRoutes.has(pathname);
}

export function isWorkbenchRoute(pathname: string | null): boolean {
  return (
    pathname !== null &&
    (pathname === "/capture/1688" || pathname === "/workbench" || pathname.startsWith("/workbench/"))
  );
}

export function ApplicationFrame({ children, toolMarketAvailable = false, productAcquisitionAvailable = false, productCollectionsAvailable = false, supplyChainAvailable = false, knowledgeAvailable = false, productReviewAvailable = false, ecoservicesAvailable = false, sheinRecordsAvailable = false, notificationCenterAvailable = false }: Readonly<{ children: React.ReactNode; toolMarketAvailable?: boolean; productAcquisitionAvailable?: boolean; productCollectionsAvailable?: boolean; supplyChainAvailable?: boolean; knowledgeAvailable?: boolean; productReviewAvailable?: boolean; ecoservicesAvailable?: boolean; sheinRecordsAvailable?: boolean; notificationCenterAvailable?: boolean }>) {
  const pathname = usePathname();

  // Public marketing, legal, and login routes must not initialize the authenticated
  // workspace shell (which reads client-side navigation state).
  if (isPublicRoute(pathname)) {
    return <>{children}</>;
  }
  if (pathname?.startsWith("/invitations/")) {
    return <ThemeProvider defaultTheme="dark"><div className="console-theme"><QueryProvider>{children}</QueryProvider></div></ThemeProvider>;
  }
  if(pathname === "/workbench/stores/shein/callback") {
    return <ThemeProvider defaultTheme="dark"><div className="console-theme"><QueryProvider><WorkbenchContextProvider>{children}</WorkbenchContextProvider></QueryProvider></div></ThemeProvider>;
  }

  if (isWorkbenchRoute(pathname)) {
    return (
      <ThemeProvider defaultTheme="dark">
        <div className="console-theme">
          <QueryProvider>
            <ToastProvider>
              <WorkbenchContextProvider>
                <WorkspaceAppShell toolMarketAvailable={toolMarketAvailable} productCollectionsAvailable={productCollectionsAvailable} supplyChainAvailable={supplyChainAvailable} productAcquisitionAvailable={productAcquisitionAvailable} knowledgeAvailable={knowledgeAvailable} productReviewAvailable={productReviewAvailable} ecoservicesAvailable={ecoservicesAvailable} sheinRecordsAvailable={sheinRecordsAvailable} notificationCenterAvailable={notificationCenterAvailable}>{children}</WorkspaceAppShell>
              </WorkbenchContextProvider>
            </ToastProvider>
          </QueryProvider>
        </div>
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
