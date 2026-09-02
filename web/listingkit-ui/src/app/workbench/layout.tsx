import type { Metadata } from "next";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  title: "工作台 | 硕米智能引擎",
  description: "硕米智能引擎企业工作台",
};

export default function WorkbenchLayout({ children }: { children: ReactNode }) {
  return children;
}
