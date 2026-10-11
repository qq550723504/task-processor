import { redirect } from "next/navigation";
import { ConsoleUnavailable } from "@/components/workbench/console/console-overview";
import { firstConnectedConsoleEntry } from "@/lib/workbench/console-primary-entry";

export default function Page() {
  const destination = firstConnectedConsoleEntry("/workbench/tools", { toolMarketAvailable: process.env.LISTINGKIT_TOOL_MARKET_ENABLED === "true" });
  if (destination) redirect(destination);
  return <ConsoleUnavailable pathname="/workbench/tools" />;
}
