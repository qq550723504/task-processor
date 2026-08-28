import type { Metadata } from "next";

import { MarketingHomepage } from "@/components/marketing/marketing-homepage";

export const metadata: Metadata = {
  title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
  description: "连接AI智能体、全球商品数据、供应链资源与专业服务，让个人和组织拥有一支智能电商团队。",
  openGraph: {
    title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
    description: "让每个人，都拥有一支智能电商团队。",
    images: ["/sumi/fd824975-1e65-4585-9ebf-212d68cb1507.png"],
  },
  twitter: {
    card: "summary_large_image",
    title: "硕米智能引擎 | 新一代 AI 电商智能操作系统",
    description: "让每个人，都拥有一支智能电商团队。",
    images: ["/sumi/fd824975-1e65-4585-9ebf-212d68cb1507.png"],
  },
};

export default function Home() {
  return <MarketingHomepage />;
}
