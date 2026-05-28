import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Select } from "@/components/ui/select";
import {
  buildImageRoleOverrides,
  moveItem,
  roleLabel,
  type ImageRole,
} from "@/components/listingkit/shein/shein-data-image-gallery-model";
import { toThumbnailPreviewUrl } from "@/lib/utils/imgproxy-url";

export function FinalImageGrid({
  defaultsOrder,
  hasImageControls,
  mainUrl,
  onSelect,
  onSetActiveImage,
  onSetDeletedUrls,
  onSetOrderOverride,
  onSetRegenerationPrompt,
  roleByUrl,
  selectedUrl,
  updateRole,
  visibleImages,
}: {
  defaultsOrder: string[];
  hasImageControls?: boolean;
  mainUrl?: string;
  onSelect: (image: SheinPreviewImage) => void;
  onSetActiveImage: (image: SheinPreviewImage | null) => void;
  onSetDeletedUrls: (
    updater: (current: string[]) => string[],
  ) => void;
  onSetOrderOverride: (
    updater: (current?: string[]) => string[],
  ) => void;
  onSetRegenerationPrompt: (value: string) => void;
  roleByUrl: Record<string, ImageRole>;
  selectedUrl?: string;
  updateRole: (url: string, role: ImageRole) => void;
  visibleImages: SheinPreviewImage[];
}) {
  if (!visibleImages.length) {
    return null;
  }

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {visibleImages.map((image, index) => {
        const active = image.url === selectedUrl;
        const role = roleByUrl[image.url] ?? "gallery";
        return (
          <div
            className={[
              "group min-w-0 rounded-2xl border bg-white p-2 text-left transition",
              active
                ? "border-zinc-950 ring-2 ring-zinc-950/10"
                : "border-zinc-200 hover:border-zinc-400",
            ].join(" ")}
            key={`${image.id}-${image.url}`}
          >
            <Button
              className="block h-auto w-full justify-start p-0 text-left hover:bg-transparent"
              onClick={() => {
                onSelect(image);
                onSetActiveImage(image);
                onSetRegenerationPrompt("");
              }}
              type="button"
              variant="ghost"
            >
              <div className="relative aspect-square overflow-hidden rounded-xl bg-zinc-50">
                {/* The URLs come from SHEIN/SDS payloads and may not be known to Next image config. */}
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  alt={image.label}
                  className="h-full w-full object-contain transition group-hover:scale-[1.02]"
                  src={toThumbnailPreviewUrl(image.url, { width: 480, height: 480 })}
                />
                <span className="absolute left-2 top-2 rounded-full bg-white/90 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-zinc-800">
                  {image.url === mainUrl ? "主图" : roleLabel(role)}
                </span>
              </div>
              <div className="mt-2 min-w-0">
                <p className="truncate text-xs font-semibold text-zinc-900">
                  {image.label}
                </p>
                <p className="mt-1 truncate text-[11px] text-zinc-500">
                  {image.url}
                </p>
              </div>
            </Button>
            {hasImageControls ? (
              <div className="mt-3 grid grid-cols-2 gap-2">
                <Button
                  className="h-auto rounded-xl px-2 py-1 text-xs"
                  disabled={index === 0}
                  onClick={() =>
                    onSetOrderOverride((current) =>
                      moveItem(current ?? defaultsOrder, image.url, -1),
                    )
                  }
                  type="button"
                  variant="outline"
                >
                  上移
                </Button>
                <Button
                  className="h-auto rounded-xl px-2 py-1 text-xs"
                  disabled={index === visibleImages.length - 1}
                  onClick={() =>
                    onSetOrderOverride((current) =>
                      moveItem(current ?? defaultsOrder, image.url, 1),
                    )
                  }
                  type="button"
                  variant="outline"
                >
                  下移
                </Button>
                <Button
                  className="h-auto rounded-xl px-2 py-1 text-xs"
                  onClick={() => updateRole(image.url, "main")}
                  type="button"
                  variant="outline"
                >
                  设为主图
                </Button>
                <Select
                  className="h-auto rounded-xl px-2 py-1 text-xs font-medium"
                  value={role}
                  onChange={(event) =>
                    updateRole(image.url, event.target.value as ImageRole)
                  }
                >
                  <option value="main">主图</option>
                  <option value="gallery">图库</option>
                  <option value="swatch">色块图</option>
                  <option value="size_map">尺寸图</option>
                  <option value="skc">SKC 图</option>
                </Select>
                <Button
                  className="col-span-2 h-auto rounded-xl px-2 py-1 text-xs"
                  onClick={() => onSetDeletedUrls((current) => [...current, image.url])}
                  type="button"
                  variant="destructive"
                >
                  从提交中移除
                </Button>
              </div>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

export function MockupReferenceGrid({
  mockupImages,
  onSelect,
  onSetActiveImage,
  onSetRegenerationPrompt,
}: {
  mockupImages: SheinPreviewImage[];
  onSelect: (image: SheinPreviewImage) => void;
  onSetActiveImage: (image: SheinPreviewImage | null) => void;
  onSetRegenerationPrompt: (value: string) => void;
}) {
  if (!mockupImages.length) {
    return null;
  }

  return (
    <div className="space-y-4">
      <Separator />
      <div className="space-y-3">
      <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            SDS Mockup 渲染参考
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-600">
            这里展示 SDS 官方渲染返回的 mockup。它们只用于对照成品效果，不参与 SHEIN 最终提交图片排序。
          </p>
        </div>
        <span className="text-xs font-medium text-zinc-500">
          参考图 {mockupImages.length} 张
        </span>
      </div>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {mockupImages.map((image) => (
          <Button
            className="group h-auto min-w-0 justify-start rounded-2xl border-dashed bg-zinc-50 p-2 text-left hover:border-zinc-400"
            key={`${image.id}-${image.url}`}
            onClick={() => {
              onSelect(image);
              onSetActiveImage(image);
              onSetRegenerationPrompt("");
            }}
            type="button"
            variant="outline"
          >
            <div className="relative aspect-square overflow-hidden rounded-xl bg-white">
              {/* The URLs come from SDS payloads and may not be known to Next image config. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                alt={image.label}
                className="h-full w-full object-contain transition group-hover:scale-[1.02]"
                src={toThumbnailPreviewUrl(image.url, { width: 480, height: 480 })}
              />
              <span className="absolute left-2 top-2 rounded-full bg-white/90 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-zinc-800">
                Mockup
              </span>
            </div>
            <div className="mt-2 min-w-0">
              <p className="truncate text-xs font-semibold text-zinc-900">
                {image.label}
              </p>
              <p className="mt-1 truncate text-[11px] text-zinc-500">
                {image.url}
              </p>
            </div>
          </Button>
        ))}
      </div>
      </div>
    </div>
  );
}

export function ImageControlActions({
  canSaveImageControls,
  deletedUrls,
  isSavingControls,
  mainUrl,
  onApplySuggestedRoles,
  onSaveImageControls,
  onSetDeletedUrls,
  roleByUrl,
  saveDisabledReason,
  visibleImages,
}: {
  canSaveImageControls: boolean;
  deletedUrls: string[];
  isSavingControls?: boolean;
  mainUrl?: string;
  onApplySuggestedRoles: () => void;
  onSaveImageControls?: (payload: {
    main_image_url?: string;
    final_image_order?: string[];
    deleted_image_urls?: string[];
    image_role_overrides?: Record<string, ImageRole>;
  }) => void;
  onSetDeletedUrls: (value: string[]) => void;
  roleByUrl: Record<string, ImageRole>;
  saveDisabledReason?: string | null;
  visibleImages: SheinPreviewImage[];
}) {
  if (!onSaveImageControls) {
    return null;
  }

  return (
    <div className="space-y-4">
      <Separator />
      <div className="flex flex-wrap justify-end gap-2">
      {deletedUrls.length ? (
        <Button
          className="h-auto rounded-xl px-3 py-2 text-xs"
          onClick={() => onSetDeletedUrls([])}
          type="button"
          variant="outline"
        >
          恢复已移除图片
        </Button>
      ) : null}
      <Button
        className="h-auto rounded-xl px-3 py-2 text-xs"
        onClick={onApplySuggestedRoles}
        type="button"
        variant="outline"
      >
        自动建议图片角色
      </Button>
      <Button
        className="h-auto rounded-xl px-4 py-2 text-xs"
        disabled={!canSaveImageControls}
        onClick={() =>
          onSaveImageControls({
            main_image_url: mainUrl,
            final_image_order: visibleImages.map((image) => image.url),
            deleted_image_urls: deletedUrls,
            image_role_overrides: buildImageRoleOverrides(
              visibleImages.map((image) => image.url),
              roleByUrl,
              mainUrl,
            ),
          })
        }
        type="button"
      >
        {isSavingControls ? "保存中..." : "保存图片设置"}
      </Button>
      {saveDisabledReason ? (
        <p className="basis-full text-right text-xs text-amber-700">
          {saveDisabledReason}
        </p>
      ) : null}
      </div>
    </div>
  );
}
