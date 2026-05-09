import { SheinStudioPageShell } from "@/components/listingkit/shein-studio/shein-studio-page-shell";

export const dynamic = "force-static";

export default function ListingKitSDSPage() {
  return (
    <SheinStudioPageShell
      description="选择 SDS 商品、生成图片、审核款式，然后创建平台资料确认任务。"
      eyebrow="SDS 源"
      layout="compact"
      title="从 SDS 商品生成上架资料"
    />
  );
}
