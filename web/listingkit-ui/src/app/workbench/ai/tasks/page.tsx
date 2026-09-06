import { connection } from "next/server";
import { CompletedWorkPageContent } from "@/components/workbench/task-center/completed-work-page";
import { isSheinRecordsAvailable } from "@/lib/server/shein-records-availability";

export default async function TaskCenterPage() { await connection(); return <CompletedWorkPageContent available={isSheinRecordsAvailable()} />; }
