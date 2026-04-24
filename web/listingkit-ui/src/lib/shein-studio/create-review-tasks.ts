import { createListingKitTask } from "@/lib/api/create-task";
import { uploadListingKitImages } from "@/lib/api/upload-images";
import { resolveGeneratedDesignSrc } from "@/lib/shein-studio/design-image";
import { loadSDSListingKitMetadata } from "@/lib/sds/product-metadata";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
} from "@/lib/types/shein-studio";

export function parsePositiveInt(input: string) {
  const parsed = Number.parseInt(input.trim(), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }
  return parsed;
}

async function buildDesignFile(design: SheinStudioGeneratedDesign, index: number) {
  const src = resolveGeneratedDesignSrc(design);
  if (!src) {
    throw new Error("Generated design image is missing.");
  }

  const response = await fetch(src, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`Load generated design failed: ${response.status}`);
  }
  const blob = await response.blob();
  return new File([blob], `${design.id || `style-${index + 1}`}-design.png`, {
    type: blob.type || "image/png",
  });
}

export async function createSheinReviewTasks(input: {
  prompt: string;
  sheinStoreId: string;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  onProgress?: (message: string) => void;
}) {
  const { designs, onProgress, prompt, selection, selectedIds, sheinStoreId } = input;

  if (!selection?.variantId) {
    throw new Error("Select an SDS variant first.");
  }

  const approved = designs.filter((design) => selectedIds.includes(design.id));
  if (approved.length === 0) {
    throw new Error("Approve at least one style before creating SHEIN tasks.");
  }

  const storeID = parsePositiveInt(sheinStoreId);
  const created: SheinStudioCreatedTask[] = [];
  onProgress?.("Loading SDS product metadata...");
  const sdsMetadata = await loadSDSListingKitMetadata({
    parentProductId: selection.parentProductId,
    variantId: selection.variantId,
  });

  for (let index = 0; index < approved.length; index += 1) {
    onProgress?.(`Uploading approved style ${index + 1} of ${approved.length}...`);
    const reviewFiles = [await buildDesignFile(approved[index], index)];
    const uploaded = await uploadListingKitImages(reviewFiles);
    const imageURLs = uploaded.image_urls ?? [];
    if (imageURLs.length === 0) {
      throw new Error("Uploaded review image URLs are missing.");
    }

    onProgress?.(`Creating SHEIN data task ${index + 1} of ${approved.length}...`);
    const task = await createListingKitTask({
      text: prompt.trim(),
      image_urls: imageURLs,
      platforms: ["shein"],
      ...(storeID ? { shein_store_id: storeID } : {}),
      options: {
        // The backend sends this flat design into SDS syncDesign, then replaces
        // SHEIN images with the SDS-rendered template mockups.
        process_images: false,
        sds: {
          ...sdsMetadata,
          variant_id: selection.variantId,
          parent_product_id: selection.parentProductId,
          prototype_group_id: selection.prototypeGroupId,
          layer_id: selection.layerId,
          blank_design_url: selection.blankDesignUrl ?? sdsMetadata.blank_design_url,
          template_image_url: selection.templateImageUrl ?? sdsMetadata.template_image_url,
          mask_image_url: selection.maskImageUrl ?? sdsMetadata.mask_image_url,
          design_type: "material",
          fit_level: 1,
          resize_mode: 0,
        },
      },
    });

    created.push({
      id: task.task_id,
      title: `Style ${index + 1}`,
      designId: approved[index].id,
    });
  }

  if (created.length === 0) {
    throw new Error("No SHEIN data tasks were created.");
  }
  onProgress?.(`Created ${created.length} SHEIN data task${created.length === 1 ? "" : "s"}.`);
  return created;
}
