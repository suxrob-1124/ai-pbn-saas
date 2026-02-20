import { createContext, useContext, type ReactNode } from "react";

import type { useEditorState } from "../hooks/useEditorState";

export type EditorContextValue = ReturnType<typeof useEditorState>;

const EditorContext = createContext<EditorContextValue | null>(null);

type EditorContextProviderProps = {
  value: EditorContextValue;
  children: ReactNode;
};

export function EditorContextProvider({ value, children }: EditorContextProviderProps) {
  return <EditorContext.Provider value={value}>{children}</EditorContext.Provider>;
}

export function useEditorContext() {
  const ctx = useContext(EditorContext);
  if (!ctx) {
    throw new Error("useEditorContext must be used within EditorContextProvider");
  }
  return ctx;
}
