import { useState } from "react";

import type { DomainSummaryResponse } from "../types/editor";
import type { EditorDirtyState, EditorFileMeta, EditorSelectionState } from "../../../types/editor";

type PreviewMode = "code" | "preview";
type PreviewSource = "buffer" | "published";

export function useEditorState(initialFocusLine?: number) {
  const [summary, setSummary] = useState<DomainSummaryResponse | null>(null);
  const [files, setFiles] = useState<EditorFileMeta[]>([]);
  const [deletedFiles, setDeletedFiles] = useState<EditorFileMeta[]>([]);
  const [selection, setSelection] = useState<EditorSelectionState | null>(null);
  const [dirtyState, setDirtyState] = useState<EditorDirtyState>({
    isDirty: false,
    originalContent: "",
    currentContent: "",
  });
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(true);
  const [fileLoading, setFileLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [focusLine, setFocusLine] = useState<number | undefined>(initialFocusLine);
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0);
  const [previewMode, setPreviewMode] = useState<PreviewMode>("code");
  const [previewSource, setPreviewSource] = useState<PreviewSource>("buffer");
  const [stylePreview, setStylePreview] = useState("");
  const [scriptPreview, setScriptPreview] = useState("");

  return {
    summary,
    setSummary,
    files,
    setFiles,
    deletedFiles,
    setDeletedFiles,
    selection,
    setSelection,
    dirtyState,
    setDirtyState,
    description,
    setDescription,
    loading,
    setLoading,
    fileLoading,
    setFileLoading,
    saving,
    setSaving,
    error,
    setError,
    focusLine,
    setFocusLine,
    historyRefreshKey,
    setHistoryRefreshKey,
    previewMode,
    setPreviewMode,
    previewSource,
    setPreviewSource,
    stylePreview,
    setStylePreview,
    scriptPreview,
    setScriptPreview,
  };
}
