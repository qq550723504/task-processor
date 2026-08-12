import type { ProcessImagesData } from "@/lib/api/generated";

const imageURLRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
  image_urls: ["source.jpg"],
};

const productURLRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
  product_url: "product-ref",
};

const combinedSourceRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
  image_urls: ["source.jpg"],
  product_url: "product-ref",
};

// @ts-expect-error Generated request type must require image_urls or product_url.
const missingSourceRequest: ProcessImagesData["body"] = {
  target_platform: "shein",
};

void imageURLRequest;
void productURLRequest;
void combinedSourceRequest;
void missingSourceRequest;
