import { redirect } from "next/navigation";
import { ConsoleUnavailable } from "@/components/workbench/console/console-overview";
import { firstConnectedConsoleEntry } from "@/lib/workbench/console-primary-entry";
import { consolePrimaryDeploymentCapabilities } from "@/lib/server/console-primary-availability";

export default function Page() {
  const destination = firstConnectedConsoleEntry("/workbench/data", consolePrimaryDeploymentCapabilities());
  if (destination) redirect(destination);
  return <ConsoleUnavailable pathname="/workbench/data" />;
}
