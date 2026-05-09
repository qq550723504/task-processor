import { useMemo, useState } from "react";

import { Card } from "@/components/shared/card";
import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";
import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

type ImageRole = "main" | "gallery" | "swatch" | "size_map" | "skc";

export function SheinDataImageGallery({
  images,
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
  const [roleOverrides, setRoleOverrides] = useState<
    Record<string, ImageRole>
  >({});

  const defaults = useMemo(() => {
    const finalByUrl = new Map(finalImages?.map((image) => [image.url, image]));
    const nextOrder = images.map((image) => image.url);
    const nextRoles: Record<string, ImageRole> = {};
    let nextMain = finalImages?.find((image) => image.main)?.url ?? images[0]?.url;
    for (const image of images) {
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
        ...suggestImageRoles(images, nextRoles, nextMain),
      };
    }
    return { order: nextOrder, mainUrl: nextMain, roles: nextRoles };
  }, [finalImages, images]);

  const order = orderOverride ?? defaults.order;
  const mainUrl = mainOverride ?? defaults.mainUrl;
  const roleByUrl = { ...defaults.roles, ...roleOverrides };

  const visibleImages = useMemo(() => {
    const byUrl = new Map(images.map((image) => [image.url, image]));
    return order
      .map((url) => byUrl.get(url))
      .filter((image): image is SheinPreviewImage => Boolean(image))
      .filter((image) => !deletedUrls.includes(image.url));
  }, [deletedUrls, images, order]);
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

  if (images.length === 0 && mockupImages.length === 0) {
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
            最终提交 {visibleImages.length} / {images.length} 张
          </span>
        </div>

        {images.length === 0 ? (
          <p className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-800">
            暂未生成 SHEIN 成品图。下方 SDS mockup 仅作为渲染参考，不会自动进入最终提交图包。
          </p>
        ) : null}

        {onSaveImageControls && images.length > 0 ? (
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
                <button
                  className="block w-full text-left"
                  onClick={() => {
                    onSelect(image);
                    setActiveImage(image);
                    setRegenerationPrompt("");
                  }}
                  type="button"
                >
                <div className="relative aspect-square overflow-hidden rounded-xl bg-zinc-50">
                  {/* The URLs come from SHEIN/SDS payloads and may not be known to Next image config. */}
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    alt={image.label}
                    className="h-full w-full object-contain transition group-hover:scale-[1.02]"
                    src={toImageProxyUrl(image.url)}
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
                </button>
                {onSaveImageControls ? (
                  <div className="mt-3 grid grid-cols-2 gap-2">
                    <button className="rounded-xl border border-zinc-200 px-2 py-1 text-xs font-medium text-zinc-700 disabled:text-zinc-300" disabled={index === 0} onClick={() => setOrderOverride((current) => moveItem(current ?? defaults.order, image.url, -1))} type="button">上移</button>
                    <button className="rounded-xl border border-zinc-200 px-2 py-1 text-xs font-medium text-zinc-700 disabled:text-zinc-300" disabled={index === visibleImages.length - 1} onClick={() => setOrderOverride((current) => moveItem(current ?? defaults.order, image.url, 1))} type="button">下移</button>
                    <button className="rounded-xl border border-zinc-200 px-2 py-1 text-xs font-medium text-zinc-700" onClick={() => updateRole(image.url, "main")} type="button">设为主图</button>
                    <select className="rounded-xl border border-zinc-200 px-2 py-1 text-xs font-medium text-zinc-700" value={role} onChange={(event) => updateRole(image.url, event.target.value as ImageRole)}>
                      <option value="main">主图</option>
                      <option value="gallery">图库</option>
                      <option value="swatch">色块图</option>
                      <option value="size_map">尺寸图</option>
                      <option value="skc">SKC 图</option>
                    </select>
                    <button className="col-span-2 rounded-xl border border-rose-200 px-2 py-1 text-xs font-medium text-rose-600" onClick={() => setDeletedUrls((current) => [...current, image.url])} type="button">从提交中移除</button>
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
        ) : null}

        {mockupImages.length > 0 ? (
          <div className="space-y-3 border-t border-zinc-100 pt-4">
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
                <button
                  className="group min-w-0 rounded-2xl border border-dashed border-zinc-200 bg-zinc-50 p-2 text-left transition hover:border-zinc-400"
                  key={`${image.id}-${image.url}`}
                  onClick={() => {
                    onSelect(image);
                    setActiveImage(image);
                    setRegenerationPrompt("");
                  }}
                  type="button"
                >
                  <div className="relative aspect-square overflow-hidden rounded-xl bg-white">
                    {/* The URLs come from SDS payloads and may not be known to Next image config. */}
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      alt={image.label}
                      className="h-full w-full object-contain transition group-hover:scale-[1.02]"
                      src={toImageProxyUrl(image.url)}
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
                </button>
              ))}
            </div>
          </div>
        ) : null}
        {onSaveImageControls && images.length > 0 ? (
          <div className="flex flex-wrap justify-end gap-2 border-t border-zinc-100 pt-4">
            {deletedUrls.length ? (
              <button className="rounded-xl border border-zinc-200 px-3 py-2 text-xs font-medium text-zinc-700" onClick={() => setDeletedUrls([])} type="button">
                恢复已移除图片
              </button>
            ) : null}
            <button
              className="rounded-xl border border-zinc-200 px-3 py-2 text-xs font-medium text-zinc-700"
              onClick={applySuggestedRoles}
              type="button"
            >
              自动建议图片角色
            </button>
            <button
              className="rounded-xl bg-zinc-950 px-4 py-2 text-xs font-semibold text-white disabled:bg-zinc-300"
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
            </button>
            {saveDisabledReason ? (
              <p className="basis-full text-right text-xs text-amber-700">
                {saveDisabledReason}
              </p>
            ) : null}
          </div>
        ) : null}
        {saveMessage ? (
          <p className="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {saveMessage}
          </p>
        ) : null}
        {saveErrorMessage ? (
          <p className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {saveErrorMessage}
          </p>
        ) : null}
      </div>

      {activeImage ? (
        <div
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/78 px-4 py-6 backdrop-blur-sm"
          role="dialog"
        >
          <div className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[1.75rem] bg-white shadow-2xl">
            <div className="flex flex-wrap items-start justify-between gap-3 border-b border-zinc-200 px-5 py-4">
              <div className="min-w-0">
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  SHEIN 图片预览
                </p>
                <h3 className="mt-1 truncate text-lg font-semibold text-zinc-950">
                  {activeImage.label}
                </h3>
                <p className="mt-1 break-all text-xs text-zinc-500">
                  {activeImage.url}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <a
                  className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
                  href={activeImage.url}
                  rel="noreferrer"
                  target="_blank"
                >
                  打开原图
                </a>
                <button
                  className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
                  onClick={() => setActiveImage(null)}
                  type="button"
                >
                  关闭
                </button>
              </div>
            </div>
            <div className="grid min-h-0 flex-1 gap-4 overflow-auto bg-zinc-100 p-4 lg:grid-cols-[minmax(0,1fr)_360px]">
              <div className="min-h-0">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  alt={activeImage.label}
                  className="mx-auto max-h-[76vh] max-w-full rounded-2xl bg-white object-contain shadow-sm"
                  src={toImageProxyUrl(activeImage.url)}
                />
              </div>
              {onRegenerate && activeImageCanRegenerate ? (
                <form
                  className="space-y-3 rounded-2xl border border-zinc-200 bg-white p-4 shadow-sm"
                  onSubmit={async (event) => {
                    event.preventDefault();
                    if (!activeImage || !canRegenerate) return;
                    try {
                      await onRegenerate(activeImage, regenerationPrompt.trim());
                      setRegenerationPrompt("");
                      setActiveImage(null);
                    } catch {
                      // The parent surfaces the API message in this panel.
                    }
                  }}
                >
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                      重新生成这张图
                    </p>
                    <p className="mt-1 text-xs leading-5 text-zinc-600">
                      描述这张图哪里不合适。系统会把当前图片作为问题参考一起发给模型。
                    </p>
                  </div>
                  <textarea
                    className="min-h-32 w-full resize-y rounded-2xl border border-zinc-200 bg-white px-3 py-3 text-sm text-zinc-900 outline-none transition placeholder:text-zinc-400 focus:border-zinc-950 focus:ring-2 focus:ring-zinc-950/10"
                    onChange={(event) => setRegenerationPrompt(event.target.value)}
                    placeholder="例如：保持图案不变，去掉多余文字，商品放大一点，修复边缘裁切。"
                    value={regenerationPrompt}
                  />
                  {regenerationError ? (
                    <p className="rounded-xl bg-red-50 px-3 py-2 text-xs leading-5 text-red-700">
                      {regenerationError}
                    </p>
                  ) : null}
                  <button
                    className="inline-flex h-10 w-full items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-300"
                    disabled={!canRegenerate}
                    type="submit"
                  >
                    {isRegenerating ? "重新生成中..." : "重新生成并替换"}
                  </button>
                </form>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </Card>
  );
}

function buildImageRoleOverrides(
  imageUrls: string[],
  roleByUrl: Record<string, ImageRole>,
  mainUrl?: string,
) {
  const overrides: Record<string, ImageRole> = {};
  for (const url of imageUrls) {
    const role = roleByUrl[url] ?? "gallery";
    overrides[url] = url === mainUrl && role === "gallery" ? "main" : role;
  }
  return overrides;
}

function suggestImageRoles(
  images: SheinPreviewImage[],
  currentRoles: Record<string, ImageRole>,
  currentMainUrl?: string,
) {
  const roles: Record<string, ImageRole> = {};
  for (const image of images) {
    roles[image.url] = currentRoles[image.url] ?? "gallery";
  }
  const mainUrl =
    images.find((image) => image.url === currentMainUrl)?.url ?? images[0]?.url;
  if (mainUrl) {
    roles[mainUrl] = "main";
  }
  const sizeImage = images.find((image) => isLikelySizeMapImage(image, roles[image.url]));
  if (sizeImage) {
    roles[sizeImage.url] = "size_map";
  }
  const skcImage = images.find((image) => image.url !== mainUrl && roles[image.url] === "skc");
  const swatchSource = skcImage ?? images.find((image) => image.url !== mainUrl && roles[image.url] === "gallery");
  if (swatchSource && roles[swatchSource.url] !== "size_map") {
    roles[swatchSource.url] = "swatch";
  }
  return { mainUrl, roles };
}

function isLikelySizeMapImage(image: SheinPreviewImage, role?: ImageRole) {
  if (role === "size_map") {
    return true;
  }
  const text = `${image.label} ${image.id}`.toLowerCase();
  return (
    text.includes("size") ||
    text.includes("dimension") ||
    text.includes("尺寸") ||
    text.includes("尺码")
  );
}

function normalizeImageRole(role?: string): ImageRole | undefined {
  switch (role) {
    case "main":
    case "gallery":
    case "swatch":
    case "size_map":
    case "skc":
      return role;
    default:
      return undefined;
  }
}

function hasSavedImageRole(
  finalImages?: Array<{
    url?: string;
    role?: string;
    main?: boolean;
    swatch?: boolean;
    size_map?: boolean;
  }>,
) {
  return Boolean(
    finalImages?.some(
      (image) =>
        image.main ||
        image.swatch ||
        image.size_map ||
        normalizeImageRole(image.role) !== undefined,
    ),
  );
}

function ImageRoleStatus({
  count,
  fallbackReady = false,
  label,
  required = false,
}: {
  count: number;
  fallbackReady?: boolean;
  label: string;
  required?: boolean;
}) {
  const ready = count > 0 || fallbackReady;
  return (
    <div
      className={[
        "rounded-xl border px-3 py-2",
        ready
          ? "border-emerald-200 bg-emerald-50 text-emerald-800"
          : required
            ? "border-amber-200 bg-amber-50 text-amber-800"
            : "border-zinc-200 bg-white text-zinc-600",
      ].join(" ")}
    >
      <div className="font-semibold">{label}</div>
      <div className="mt-1 text-[11px]">
        {count > 0
          ? `${count} 张已设置`
          : fallbackReady
            ? "默认使用首图"
            : required
              ? "需要设置"
              : "未设置"}
      </div>
    </div>
  );
}

function roleLabel(role: string) {
  switch (role) {
    case "main":
      return "主图";
    case "swatch":
      return "色块图";
    case "size_map":
      return "尺寸图";
    case "skc":
      return "SKC 图";
    default:
      return "图库";
  }
}

function moveItem(items: string[], value: string, delta: -1 | 1) {
  const next = [...items];
  const index = next.indexOf(value);
  if (index < 0) {
    return next;
  }
  const target = index + delta;
  if (target < 0 || target >= next.length) {
    return next;
  }
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}
