"use client";

import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { PropsWithChildren } from "react";

export function ThemeProvider({ children, defaultTheme = "light" }: PropsWithChildren<{ defaultTheme?: "light" | "dark" }>) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme={defaultTheme}
      disableTransitionOnChange
      enableSystem={false}
      storageKey="listingkit-theme"
    >
      {children}
    </NextThemesProvider>
  );
}
