"use client";

import { usePathname } from "next/navigation";

import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";
import { QueryProvider } from "@/components/providers/query-provider";
import { ThemeProvider } from "@/components/providers/theme-provider";
import { ToastProvider } from "@/components/providers/toast-provider";
import { ZitadelAuthGate } from "@/components/providers/zitadel-auth-gate";

export function ApplicationFrame({ children }: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();

  // Public marketing and login routes must not initialize the authenticated
  // workspace shell (which reads client-side navigation state).
  if (
    pathname === "/" ||
    pathname === "/login" ||
    pathname === "/unauthorized"
  ) {
    return <>{children}</>;
  }

  return (
    <ThemeProvider>
      <QueryProvider>
        <ToastProvider>
          <ZitadelAuthGate><ListingKitAppShell>{children}</ListingKitAppShell></ZitadelAuthGate>
        </ToastProvider>
      </QueryProvider>
    </ThemeProvider>
  );
}
