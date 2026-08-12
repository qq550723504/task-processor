import type { ProcessImagesData } from "@/lib/api/generated";

const imageURLRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
  image_urls: ["source.jpg"],
  country: "US",
  scene: {
    scene_category: "shoes",
    scene_style: "lifestyle",
    background_tone: "warm",
    composition: "close_up",
    props_level: "light",
    audience_hint: "sporty",
    custom_scene_hint: "show subtle motion energy",
  },
};

const productURLRequest: ProcessImagesData["body"] = {
  marketplace: "shein",
  product_url: "product-ref",
  country: "DE",
  scene: { scene_category: "home" },
};

const combinedSourceRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
  marketplace: "shein",
  image_urls: ["source.jpg"],
  product_url: "product-ref",
  country: "SG",
  scene: { composition: "centered" },
};

// @ts-expect-error Generated request type must require image_urls or product_url.
const missingSourceRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
};

// @ts-expect-error Generated request type must require target_platform or marketplace.
const missingTargetRequest: ProcessImagesData["body"] = {
  image_urls: ["source.jpg"],
};

const invalidSceneRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
  image_urls: ["source.jpg"],
  scene: {
    // @ts-expect-error Generated scene shape must reject unsupported fields.
    arbitrary_runtime_option: "unsupported",
  },
};

void imageURLRequest;
void productURLRequest;
void combinedSourceRequest;
void missingSourceRequest;
void missingTargetRequest;
void invalidSceneRequest;
