import { create } from "zustand";

type ConfigDialogStore = {
  isOpen: boolean;
  shouldPromptContinue: boolean;
  openConfigDialog: (shouldPromptContinue?: boolean) => void;
  setConfigDialogOpen: (isOpen: boolean) => void;
  clearPromptContinue: () => void;
};

export const useConfigDialogStore = create<ConfigDialogStore>()((set) => ({
  isOpen: false,
  shouldPromptContinue: false,
  openConfigDialog: (shouldPromptContinue = false) =>
    set({
      isOpen: true,
      shouldPromptContinue,
    }),
  setConfigDialogOpen: (isOpen) => set({ isOpen }),
  clearPromptContinue: () => set({ shouldPromptContinue: false }),
}));
