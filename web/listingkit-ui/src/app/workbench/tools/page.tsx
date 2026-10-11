import { redirect } from "next/navigation";
import { ConsoleUnavailable } from "@/components/workbench/console/console-overview";

export default function Page() {
  if (process.env.LISTINGKIT_TOOL_MARKET_ENABLED === "true") {
    redirect("/workbench/tools/official");
  }
  return <ConsoleUnavailable pathname="/workbench/tools" />;
}
