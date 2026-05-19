import type { Metadata } from "next";
import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";
import { QueryProvider } from "@/components/providers/query-provider";
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
    <html lang="zh-CN" className="h-full antialiased">
      <body className="min-h-full bg-zinc-100 text-zinc-950">
        <QueryProvider>
          <ZitadelAuthGate>
            <ListingKitAppShell>{children}</ListingKitAppShell>
          </ZitadelAuthGate>
        </QueryProvider>
      </body>
    </html>
  );
}
