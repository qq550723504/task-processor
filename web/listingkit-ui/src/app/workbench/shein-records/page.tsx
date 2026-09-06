import { connection } from "next/server";
import { SheinRecordListPage } from "@/components/workbench/shein-records/record-list-page";
import { isSheinRecordsAvailable } from "@/lib/server/shein-records-availability";

export default async function SheinRecordsPage() {
  await connection();
  return <SheinRecordListPage available={isSheinRecordsAvailable()} />;
}
