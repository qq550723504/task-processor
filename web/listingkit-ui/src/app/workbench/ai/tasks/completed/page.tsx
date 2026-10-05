import { BusinessTaskPage } from "@/components/workbench/ai-workbench/task-page";
import { isSheinRecordsAvailable } from "@/lib/server/shein-records-availability";
import { connection } from "next/server";

export default async function Page() {
  await connection();
  return <BusinessTaskPage mode="completed" sheinRecordsAvailable={isSheinRecordsAvailable()} />;
}
