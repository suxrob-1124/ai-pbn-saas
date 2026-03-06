"use client";

import { createContext, useContext } from "react";
import type { EditorMode } from "../types/content-editor";

type ContentModeContextValue = {
  mode: EditorMode;
  setMode: (mode: EditorMode) => void;
};

const ContentModeContext = createContext<ContentModeContextValue>({
  mode: "code",
  setMode: () => {},
});

export const ContentModeProvider = ContentModeContext.Provider;

export function useContentModeContext() {
  return useContext(ContentModeContext);
}
