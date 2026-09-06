import { ArrowRight, FileText } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export function SheinRecordsEntry({ available }: { available: boolean }) {
  return (
    <Card className="mt-6 max-w-xl p-5 sm:p-6">
      <FileText aria-hidden="true" className="mb-4 size-6 text-primary" />
      <h2 className="text-lg font-semibold">SHEIN 本地资料</h2>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">查看本地保存的 SHEIN 资料及离线诊断。这里的资料不是平台已上架商品。</p>
      {available ? <Button asChild className="mt-5"><Link href="/workbench/shein-records" prefetch={false}>查看本地资料<ArrowRight aria-hidden="true" /></Link></Button> : <p className="mt-5 text-sm font-medium">暂未启用</p>}
    </Card>
  );
}
