import { connection } from "next/server";
import { SheinRecordsEntry } from "@/components/workbench/shein-records/entry-card";
import { isSheinRecordsAvailable } from "@/lib/server/shein-records-availability";

export default async function WorkbenchPage() {
  await connection();
  return (
    <section className="mx-auto w-full max-w-6xl px-4 py-10 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">工作台</h1>
      <p className="mt-2 text-sm text-muted-foreground">
        从这里进入当前企业已开放的业务能力。
      </p>
      <SheinRecordsEntry available={isSheinRecordsAvailable()} />
    </section>
  );
}
