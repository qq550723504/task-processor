import { buildSelectionSummary } from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
} from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

type BuildSheinStudioDraftInputArgs = {
  prompt: string;
  styleCount: string;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds: boolean;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
};

export function buildSheinStudioDraftInput(
  args: BuildSheinStudioDraftInputArgs,
): SheinStudioSaveInput {
  return {
    prompt: args.prompt,
    styleCount: args.styleCount,
    productImageCount: args.productImageCount,
    productImagePrompt: args.productImagePrompt,
    productImagePrompts: args.productImagePrompts,
    artworkModel: args.artworkModel,
    transparentBackground: args.transparentBackground,
    sheinStoreId: args.sheinStoreId,
    imageStrategy: args.imageStrategy,
    selectedSdsImages: args.selectedSdsImages,
    renderSizeImagesWithSds: args.renderSizeImagesWithSds,
    selection: buildSelectionSummary(args.selection),
    designs: args.designs,
    selectedIds: args.selectedIds,
    createdTasks: args.createdTasks,
  };
}
