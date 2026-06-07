"use client";

import { Moon, Sun } from "lucide-react";
import { useTheme } from "next-themes";

import { Button } from "@/components/ui/button";

export function ThemeToggleButton() {
  const { resolvedTheme, setTheme } = useTheme();
  const isDark = resolvedTheme === "dark";

  return (
    <Button
      aria-label={isDark ? "切换到白天模式" : "切换到夜间模式"}
      className="h-auto min-w-0 rounded-xl px-3 py-2"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      size="sm"
      type="button"
      variant="outline"
    >
      {isDark ? (
        <Sun className="size-4 shrink-0" data-icon="inline-start" />
      ) : (
        <Moon className="size-4 shrink-0" data-icon="inline-start" />
      )}
      <span className="hidden sm:inline">{isDark ? "白天模式" : "夜间模式"}</span>
    </Button>
  );
}
