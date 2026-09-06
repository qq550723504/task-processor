import { notFound } from "next/navigation";
import { ConsoleUnavailable } from "@/components/workbench/console/console-overview";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";

export default async function UnavailableConsolePage({ params }: { params: Promise<{ section: string[] }> }) {
  const { section } = await params;
  const pathname = `/workbench/${section.join("/")}`;
  const route = findConsoleRoute(pathname);
  if (!route || route.node.availability !== "unavailable") notFound();
  return <ConsoleUnavailable pathname={pathname} />;
}
