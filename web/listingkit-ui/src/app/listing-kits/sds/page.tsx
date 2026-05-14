import { SheinStudioPageShell } from "@/components/listingkit/shein-studio/shein-studio-page-shell";

export const dynamic = "force-static";

export default function ListingKitSDSPage() {
  return (
    <SheinStudioPageShell
      description="选择 POD 商品、生成图片、审核款式，然后创建平台资料确认任务。"
      eyebrow="POD"
      layout="compact"
      title="从 POD 商品生成上架资料"
    />
  );
}
