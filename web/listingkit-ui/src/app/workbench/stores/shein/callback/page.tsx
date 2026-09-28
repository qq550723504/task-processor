import type { Metadata } from "next";
import { SheinAuthorizationCallback } from "@/components/workbench/stores/shein-authorization-callback";
export const dynamic = "force-dynamic";
export const metadata: Metadata = {
  title: "SHEIN 官方授权返回",
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};
export default function Page() {
  return <SheinAuthorizationCallback />;
}
