import type { Metadata } from "next";
import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";
import { QueryProvider } from "@/components/providers/query-provider";
import { ThemeProvider } from "@/components/providers/theme-provider";
import { ToastProvider } from "@/components/providers/toast-provider";
import { ZitadelAuthGate } from "@/components/providers/zitadel-auth-gate";
import "./globals.css";

export const metadata: Metadata = {
  title: "ListingKit",
  description: "ListingKit 上架任务工作台",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN" className="h-full antialiased" suppressHydrationWarning>
      <body className="min-h-full bg-background text-foreground">
        <ThemeProvider>
          <QueryProvider>
            <ToastProvider>
              <ZitadelAuthGate>
                <ListingKitAppShell>{children}</ListingKitAppShell>
              </ZitadelAuthGate>
            </ToastProvider>
          </QueryProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
