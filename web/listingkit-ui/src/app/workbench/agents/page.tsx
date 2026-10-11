import { redirect } from "next/navigation";
import { firstConnectedConsoleEntry } from "@/lib/workbench/console-primary-entry";

export default function Page() {
  redirect(firstConnectedConsoleEntry("/workbench/agents", {})!);
}
