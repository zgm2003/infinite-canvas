"use client";

import { useSyncThemeClass } from "@/stores/use-theme-store";

export function ThemeSync() {
  useSyncThemeClass();
  return null;
}
