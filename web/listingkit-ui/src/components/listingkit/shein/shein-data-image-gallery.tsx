import { useMemo, useState } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ImagePreviewDialog } from "@/components/listingkit/shein/shein-data-image-gallery-dialog";
import {
  FinalImageGrid,
  ImageControlActions,
  MockupReferenceGrid,
} from "@/components/listingkit/shein/shein-data-image-gallery-grid";
import { ImageRoleStatus } from "@/components/listingkit/shein/shein-data-image-gallery-sections";
import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";
import { toThumbnailPreviewUrl } from "@/lib/utils/imgproxy-url";
import {
  hasSavedImageRole,
  normalizeImageRole,
  suggestImageRoles,
  type ImageRole,
} from "@/components/listingkit/shein/shein-data-image-gallery-model";

export function SheinDataImageGallery({
  images,
  availableImages = [],
  mockupImages = [],
  selectedUrl,
  onSelect,
  finalImages,
  variantCount,
  onSaveImageControls,
  onRegenerate,
  isRegenerating = false,
  regenerationError,
  isSavingControls = false,
  saveMessage,
  saveErrorMessage,
}: {
  images: SheinPreviewImage[];
  availableImages?: SheinPreviewImage[];
  mockupImages?: SheinPreviewImage[];
  selectedUrl?: string;
  onSelect: (image: SheinPreviewImage) => void;
  finalImages?: Array<{
    url?: string;
    role?: string;
    main?: boolean;
    swatch?: boolean;
    size_map?: boolean;
  }>;
  variantCount?: number;
  onSaveImageControls?: (payload: {
    main_image_url?: string;
    final_image_order?: string[];
    deleted_image_urls?: string[];
    image_role_overrides?: Record<string, ImageRole>;
  }) => void;
  onRegenerate?: (image: SheinPreviewImage, prompt: string) => Promise<void> | void;
  isRegenerating?: boolean;
  regenerationError?: string | null;
  isSavingControls?: boolean;
  saveMessage?: string | null;
  saveErrorMessage?: string | null;
}) {
  const [activeImage, setActiveImage] = useState<SheinPreviewImage | null>(null);
  const [regenerationPrompt, setRegenerationPrompt] = useState("");
  const [orderOverride, setOrderOverride] = useState<string[]>();
  const [deletedUrls, setDeletedUrls] = useState<string[]>([]);
  const [mainOverride, setMainOverride] = useState<string>();
  const [includedAvailableUrls, setIncludedAvailableUrls] = useState<string[]>([]);
  const [roleOverrides, setRoleOverrides] = useState<
    Record<string, ImageRole>
  >({});

  const workingImages = useMemo(() => {
    const selected = new Set(images.map((image) => image.url));
    return [
      ...images,
      ...availableImages.filter(
        (image) =>
          includedAvailableUrls.includes(image.url) && !selected.has(image.url),
      ),
    ];
  }, [availableImages, images, includedAvailableUrls]);

  const defaults = useMemo(() => {
    const finalByUrl = new Map(finalImages?.map((image) => [image.url, image]));
    const nextOrder = workingImages.map((image) => image.url);
    const nextRoles: Record<string, ImageRole> = {};
    let nextMain = finalImages?.find((image) => image.main)?.url ?? workingImages[0]?.url;
    for (const image of workingImages) {
      const finalImage = finalByUrl.get(image.url);
      const finalRole = normalizeImageRole(finalImage?.role);
      if (finalRole === "main" || finalImage?.main) {
        nextRoles[image.url] = "main";
        nextMain = image.url;
      } else if (finalRole === "swatch") {
        nextRoles[image.url] = "swatch";
      } else if (finalRole === "size_map" || finalImage?.size_map) {
        nextRoles[image.url] = "size_map";
      } else if (finalRole === "skc") {
        nextRoles[image.url] = "skc";
      } else if (finalImage?.swatch) {
        nextRoles[image.url] = "swatch";
      } else {
        nextRoles[image.url] = "gallery";
      }
    }
    if (!hasSavedImageRole(finalImages)) {
      return {
        order: nextOrder,
        ...suggestImageRoles(workingImages, nextRoles, nextMain),
      };
    }
    return { order: nextOrder, mainUrl: nextMain, roles: nextRoles };
  }, [finalImages, workingImages]);

  const order = orderOverride ?? defaults.order;
  const mainUrl = mainOverride ?? defaults.mainUrl;
  const roleByUrl = { ...defaults.roles, ...roleOverrides };

  const visibleImages = useMemo(() => {
    const byUrl = new Map(workingImages.map((image) => [image.url, image]));
    return order
      .map((url) => byUrl.get(url))
      .filter((image): image is SheinPreviewImage => Boolean(image))
      .filter((image) => !deletedUrls.includes(image.url));
  }, [deletedUrls, order, workingImages]);
  const roleCounts = visibleImages.reduce(
    (counts, image) => {
      const role = image.url === mainUrl ? "main" : roleByUrl[image.url] ?? "gallery";
      counts[role] += 1;
      return counts;
    },
    {
      main: 0,
      gallery: 0,
      swatch: 0,
      size_map: 0,
      skc: 0,
    } satisfies Record<ImageRole, number>,
  );
  const singleVariantUsesMainImage =
    (variantCount ?? 0) === 1 && roleCounts.main > 0;
  const canSaveImageControls = Boolean(mainUrl) && visibleImages.length > 0 && !isSavingControls;
  const saveDisabledReason = !visibleImages.length
    ? "至少保留一张最终提交图片"
    : !mainUrl
      ? "请先设置主图"
      : null;

  const updateRole = (url: string, role: ImageRole) => {
    if (role === "main") {
      setMainOverride(url);
    } else if (url === mainUrl) {
      const fallbackMain = visibleImages.find((image) => image.url !== url)?.url;
      setMainOverride(fallbackMain);
    }
    setRoleOverrides((current) => ({ ...current, [url]: role }));
  };

  const applySuggestedRoles = () => {
    const suggested = suggestImageRoles(visibleImages, defaults.roles, mainUrl);
    setMainOverride(suggested.mainUrl);
    setRoleOverrides(suggested.roles);
  };

  const activeImageCanRegenerate = activeImage
    ? images.some((image) => image.url === activeImage.url)
    : false;
  const canRegenerate =
    Boolean(onRegenerate && activeImage && activeImageCanRegenerate) &&
    regenerationPrompt.trim().length > 0 &&
    !isRegenerating;

  const unselectedAvailableImages = availableImages.filter(
    (image) => !includedAvailableUrls.includes(image.url),
  );

  if (workingImages.length === 0 && unselectedAvailableImages.length === 0 && mockupImages.length === 0) {
    return null;
  }

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN 提交图片
            </p>
            <p className="mt-1 text-sm leading-6 text-zinc-600">
              这里展示最终会提交到 SHEIN 的图片。提交前请确认主图、色块图、尺寸图和排序。
            </p>
          </div>
          <span className="text-xs font-medium text-zinc-500">
            最终提交 {visibleImages.length} / {workingImages.length} 张
          </span>
        </div>

        {workingImages.length === 0 ? (
          <Alert variant="warning">
            <AlertDescription>
              暂未生成 SHEIN 成品图。下方 SDS mockup 仅作为渲染参考，不会自动进入最终提交图包。
            </AlertDescription>
          </Alert>
        ) : null}

        {onSaveImageControls && workingImages.length > 0 ? (
          <div className="grid gap-2 rounded-2xl border border-zinc-200 bg-zinc-50 p-3 text-xs sm:grid-cols-5">
            <ImageRoleStatus
              count={roleCounts.main}
              label="主图"
              required
            />
            <ImageRoleStatus
              count={roleCounts.swatch}
              fallbackReady={singleVariantUsesMainImage}
              label="色块图"
            />
            <ImageRoleStatus
              count={roleCounts.size_map}
              label="尺寸图"
            />
            <ImageRoleStatus
              count={roleCounts.skc}
              fallbackReady={singleVariantUsesMainImage}
              label="SKC 图"
            />
            <ImageRoleStatus
              count={roleCounts.gallery}
              label="图库"
            />
          </div>
        ) : null}

        {visibleImages.length > 0 ? (
          <FinalImageGrid
            defaultsOrder={defaults.order}
            hasImageControls={Boolean(onSaveImageControls)}
            mainUrl={mainUrl}
            onSelect={onSelect}
            onSetActiveImage={setActiveImage}
            onSetDeletedUrls={setDeletedUrls}
            onRemoveImage={(url) =>
              setIncludedAvailableUrls((current) =>
                current.filter((includedUrl) => includedUrl !== url),
              )
            }
            onSetOrderOverride={setOrderOverride}
            onSetRegenerationPrompt={setRegenerationPrompt}
            roleByUrl={roleByUrl}
            selectedUrl={selectedUrl}
            updateRole={updateRole}
            visibleImages={visibleImages}
          />
        ) : null}

        {unselectedAvailableImages.length > 0 ? (
          <div className="space-y-3 rounded-2xl border border-amber-200 bg-amber-50/60 p-4">
            <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-amber-800">
                  可用来源图
                </p>
                <p className="mt-1 text-sm leading-6 text-amber-900/80">
                  来源图默认不提交；加入提交后会带 IP 风险标记，请确认来源授权。
                </p>
              </div>
              <span className="text-xs font-medium text-amber-800">
                可选 {unselectedAvailableImages.length} 张
              </span>
            </div>
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              {unselectedAvailableImages.map((image) => (
                <div
                  className="min-w-0 rounded-2xl border border-amber-200 bg-white p-2"
                  key={`${image.id}-${image.url}`}
                >
                  <div className="relative aspect-square overflow-hidden rounded-xl bg-zinc-50">
                    {/* Source URLs are external and may not be known to Next image config. */}
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      alt={image.label}
                      className="h-full w-full object-contain"
                      src={toThumbnailPreviewUrl(image.url, { width: 480, height: 480 })}
                    />
                    <span className="absolute left-2 top-2 rounded-full bg-amber-100/95 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-amber-900">
                      来源图
                    </span>
                  </div>
                  <p className="mt-2 truncate text-xs font-semibold text-zinc-900">
                    {image.label}
                  </p>
                  <Button
                    className="mt-3 w-full rounded-xl px-2 py-1 text-xs"
                    onClick={() => {
                      setIncludedAvailableUrls((current) =>
                        current.includes(image.url) ? current : [...current, image.url],
                      );
                      setDeletedUrls((current) =>
                        current.filter((url) => url !== image.url),
                      );
                      setOrderOverride((current) =>
                        current && !current.includes(image.url)
                          ? [...current, image.url]
                          : current,
                      );
                    }}
                    type="button"
                    variant="outline"
                  >
                    加入提交图片
                  </Button>
                </div>
              ))}
            </div>
          </div>
        ) : null}

        <MockupReferenceGrid
          mockupImages={mockupImages}
          onSelect={onSelect}
          onSetActiveImage={setActiveImage}
          onSetRegenerationPrompt={setRegenerationPrompt}
        />
        {workingImages.length > 0 ? (
          <ImageControlActions
            canSaveImageControls={canSaveImageControls}
            deletedUrls={deletedUrls}
            isSavingControls={isSavingControls}
            mainUrl={mainUrl}
            onApplySuggestedRoles={applySuggestedRoles}
            onSaveImageControls={onSaveImageControls}
            onSetDeletedUrls={setDeletedUrls}
            roleByUrl={roleByUrl}
            saveDisabledReason={saveDisabledReason}
            visibleImages={visibleImages}
          />
        ) : null}
        {saveMessage ? (
          <Alert variant="success">
            <AlertDescription>{saveMessage}</AlertDescription>
          </Alert>
        ) : null}
        {saveErrorMessage ? (
          <Alert variant="destructive">
            <AlertDescription>{saveErrorMessage}</AlertDescription>
          </Alert>
        ) : null}
      </div>

      <ImagePreviewDialog
        activeImage={activeImage}
        activeImageCanRegenerate={activeImageCanRegenerate}
        canRegenerate={canRegenerate}
        isRegenerating={isRegenerating}
        onClose={() => setActiveImage(null)}
        onRegenerate={onRegenerate}
        regenerationError={regenerationError}
        regenerationPrompt={regenerationPrompt}
        setRegenerationPrompt={setRegenerationPrompt}
      />
    </Card>
  );
}
