import type { Metadata } from "next";
import { headers } from "next/headers";

import { ApplicationFrame } from "@/components/application-frame";
import "./globals.css";

export async function generateMetadata(): Promise<Metadata> {
  const requestHeaders = await headers();
  const host = requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "localhost:3000";
  const protocol = requestHeaders.get("x-forwarded-proto") ?? "https";
  const metadataBase = new URL(`${protocol}://${host}`);
  const image = new URL("/og-v2.png", metadataBase).toString();

  return {
    metadataBase,
    title: "ListingKit | 一份资料，多平台增长",
    description: "AI 自动完成商品资料生产与渠道适配，团队只需审核并决策。",
    openGraph: {
      title: "ListingKit | 一份资料，多平台增长",
      description: "AI 做繁琐工作，你只需审核。",
      images: [image],
    },
    twitter: {
      card: "summary_large_image",
      title: "ListingKit | 一份资料，多平台增长",
      description: "AI 做繁琐工作，你只需审核。",
      images: [image],
    },
  };
}

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="zh-CN" className="h-full antialiased" suppressHydrationWarning>
      <body className="min-h-full bg-background text-foreground">
        <ApplicationFrame>{children}</ApplicationFrame>
      </body>
    </html>
  );
}
