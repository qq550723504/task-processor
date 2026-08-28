import type { Metadata } from "next";
import { headers } from "next/headers";

import { ApplicationFrame } from "@/components/application-frame";
import "./globals.css";

export async function generateMetadata(): Promise<Metadata> {
  const requestHeaders = await headers();
  const host = requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "localhost:3000";
  const protocol = requestHeaders.get("x-forwarded-proto") ?? "https";
  const metadataBase = new URL(`${protocol}://${host}`);
  const image = new URL("/sumi/fd824975-1e65-4585-9ebf-212d68cb1507.png", metadataBase).toString();

  return {
    metadataBase,
    title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
    description: "连接AI智能体、全球商品数据、供应链资源与专业服务，让个人和组织拥有一支智能电商团队。",
    openGraph: {
      title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
      description: "让每个人，都拥有一支智能电商团队。",
      images: [image],
    },
    twitter: {
      card: "summary_large_image",
      title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
      description: "让每个人，都拥有一支智能电商团队。",
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
