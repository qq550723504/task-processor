import type { Metadata } from "next";

const socialImage = "/sumi/fd824975-1e65-4585-9ebf-212d68cb1507.png";

export function createSumiLegalMetadata(title: string, description: string): Metadata {
  const fullTitle = `${title} | 硕米智能引擎`;

  return {
    title: fullTitle,
    description,
    openGraph: {
      title: fullTitle,
      description,
      siteName: "硕米智能引擎",
      type: "website",
      images: [socialImage],
    },
    twitter: {
      card: "summary_large_image",
      title: fullTitle,
      description,
      images: [socialImage],
    },
  };
}
