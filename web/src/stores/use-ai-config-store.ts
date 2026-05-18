import { create } from "zustand";
import { persist } from "zustand/middleware";

import { CONFIG_STORE_KEY, defaultConfig, type AiConfig } from "@/lib/ai-config";

type AiConfigStore = {
  config: AiConfig;
  updateConfig: <K extends keyof AiConfig>(key: K, value: AiConfig[K]) => void;
};

export const useAiConfigStore = create<AiConfigStore>()(
  persist(
    (set) => ({
      config: defaultConfig,
      updateConfig: (key, value) =>
        set((state) => ({
          config: {
            ...state.config,
            [key]: value,
          },
        })),
    }),
    {
      name: CONFIG_STORE_KEY,
      merge: (persisted, current) => {
        const config = { ...defaultConfig, ...((persisted as Partial<AiConfigStore>).config || {}) };
        return { ...current, config: { ...config, imageModel: config.imageModel || config.model, textModel: config.textModel || config.model } };
      },
    },
  ),
);
