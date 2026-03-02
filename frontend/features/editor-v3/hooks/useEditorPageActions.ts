"use client";

import { useEffect, useMemo, useState } from "react";

import { apiBase, authFetch } from "../../../lib/http";
import { getFile, listFiles, SaveConflictError, saveFile, type FileListItem } from "../../../lib/fileApi";
import { showToast } from "../../../lib/toastStore";
import type { EditorFileMeta, EditorSelectionState } from "../../../types/editor";
import {
  detectLanguage,
  encodePath,
  injectRuntimeAssets,
  isImageLikeFile,
  rewriteHtmlAssetRefs,
} from "../services/editorPreviewUtils";
import type { DomainSummaryResponse } from "../types/editor";

type UseEditorPageActionsParams = {
  domainId: string;
  router: { replace: (href: any, options?: { scroll?: boolean }) => void };
  searchParams: { toString: () => string };
  requestedPath: string;
  requestedLine: number;
  setSummary: (value: DomainSummaryResponse | null) => void;
  setFiles: React.Dispatch<React.SetStateAction<EditorFileMeta[]>>;
  setDeletedFiles: React.Dispatch<React.SetStateAction<EditorFileMeta[]>>;
  files: EditorFileMeta[];
  selection: EditorSelectionState | null;
  setSelection: React.Dispatch<React.SetStateAction<EditorSelectionState | null>>;
  dirtyState: { isDirty: boolean; originalContent: string; currentContent: string };
  setDirtyState: React.Dispatch<React.SetStateAction<any>>;
  description: string;
  setDescription: (value: string) => void;
  setLoading: (value: boolean) => void;
  setFileLoading: (value: boolean) => void;
  setSaving: (value: boolean) => void;
  setError: (value: string | null) => void;
  setFocusLine: (value?: number) => void;
  setHistoryRefreshKey: React.Dispatch<React.SetStateAction<number>>;
  previewMode: "code" | "preview";
  setPreviewMode: (value: "code" | "preview") => void;
  previewSource: "buffer" | "published";
  stylePreview: string;
  setStylePreview: (value: string) => void;
  scriptPreview: string;
  setScriptPreview: (value: string) => void;
  previewViewport: "desktop" | "tablet" | "mobile";
  currentFile: EditorFileMeta | undefined;
  aiCreatePreviewFile: { path: string; content: string } | null;
  readOnly: boolean;
};

function toEditorFileMeta(item: FileListItem): EditorFileMeta {
  return {
    id: item.id,
    path: item.path,
    size: item.size,
    mimeType: item.mimeType,
    version: item.version || 1,
    isEditable: Boolean(item.isEditable),
    isBinary: Boolean(item.isBinary),
    width: item.width,
    height: item.height,
    lastEditedBy: item.lastEditedBy,
    updatedAt: item.updatedAt,
    editable: Boolean(item.isEditable),
  };
}

export function useEditorPageActions(params: UseEditorPageActionsParams) {
  const {
    domainId,
    router,
    searchParams,
    requestedPath,
    requestedLine,
    setSummary,
    setFiles,
    setDeletedFiles,
    files,
    selection,
    setSelection,
    dirtyState,
    setDirtyState,
    description,
    setDescription,
    setLoading,
    setFileLoading,
    setSaving,
    setError,
    setFocusLine,
    setHistoryRefreshKey,
    previewMode,
    setPreviewMode,
    previewSource,
    stylePreview,
    setStylePreview,
    scriptPreview,
    setScriptPreview,
    previewViewport,
    currentFile,
    aiCreatePreviewFile,
    readOnly,
  } = params;

  const [conflictState, setConflictState] = useState<{
    currentVersion: number;
    currentHash?: string;
    updatedBy?: string;
    updatedAt?: string;
  } | null>(null);

  const canPreviewCurrentFile = Boolean(selection?.selectedPath?.toLowerCase().endsWith(".html"));
  const existingPathSet = useMemo(
    () =>
      new Set(
        files
          .map((file) => (file.path || "").trim().replace(/^\/+/, "").toLowerCase())
          .filter(Boolean)
      ),
    [files]
  );

  const loadFiles = async () => {
    const fileList = await listFiles(domainId);
    const prepared: EditorFileMeta[] = (Array.isArray(fileList) ? fileList : []).map((item: FileListItem) =>
      toEditorFileMeta(item)
    );
    setFiles(prepared);
    return prepared;
  };

  const loadDeletedFiles = async () => {
    const fileList = await listFiles(domainId, { includeDeleted: true });
    const prepared: EditorFileMeta[] = (Array.isArray(fileList) ? fileList : [])
      .filter((item: FileListItem) => Boolean(item.deletedAt))
      .map((item: FileListItem) => toEditorFileMeta(item));
    setDeletedFiles(prepared);
    return prepared;
  };

  const updatePathQuery = (pathValue: string) => {
    const query = new URLSearchParams(searchParams.toString());
    query.set("path", pathValue);
    query.delete("line");
    const qs = query.toString();
    router.replace((`/domains/${domainId}/editor${qs ? `?${qs}` : ""}` as any), { scroll: false });
  };

  const loadFile = async (file: EditorFileMeta, options?: { line?: number }) => {
    if (dirtyState.isDirty) {
      const confirmed = window.confirm("Есть несохраненные изменения. Переключить файл без сохранения?");
      if (!confirmed) {
        return;
      }
    }
    setFileLoading(true);
    setError(null);
    try {
      const payload = await getFile(domainId, file.path);
      const content = payload?.content ?? "";
      const mimeType = payload?.mimeType || file.mimeType;
      const language = detectLanguage(file.path);
      setSelection({
        selectedPath: file.path,
        selectedFileId: file.id,
        language,
        mimeType,
        version: typeof payload?.version === "number" ? payload.version : file.version || 1,
      });
      setDirtyState({
        isDirty: false,
        originalContent: content,
        currentContent: content,
      });
      setDescription("");
      setFocusLine(options?.line);
      updatePathQuery(file.path);
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить файл");
    } finally {
      setFileLoading(false);
    }
  };

  useEffect(() => {
    if (!domainId) return;
    let alive = true;
    const run = async () => {
      setLoading(true);
      setError(null);
      try {
        const summaryResp = await authFetch<DomainSummaryResponse>(
          `/api/domains/${domainId}/summary?gen_limit=1&link_limit=1`
        );
        if (!alive) return;
        setSummary(summaryResp);
        const prepared = await loadFiles();
        await loadDeletedFiles();
        if (!alive) return;
        if (prepared.length === 0) {
          setSelection(null);
          setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
          return;
        }
        const queryTarget = requestedPath ? prepared.find((item) => item.path === requestedPath) : null;
        const firstEditable = prepared.find((item) => item.isEditable);
        const fallback = firstEditable || prepared[0];
        const target = queryTarget || fallback;
        await loadFile(target, {
          line: Number.isFinite(requestedLine) && requestedLine > 0 ? requestedLine : undefined,
        });
      } catch (err: any) {
        if (!alive) return;
        setError(err?.message || "Не удалось загрузить редактор");
      } finally {
        if (alive) setLoading(false);
      }
    };
    void run();
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [domainId]);

  useEffect(() => {
    let cancelled = false;
    const refreshAssets = async () => {
      if (!selection || !selection.selectedPath.toLowerCase().endsWith(".html")) {
        setStylePreview("");
        setScriptPreview("");
        return;
      }
      const hasStyle = existingPathSet.has("style.css");
      const hasScript = existingPathSet.has("script.js");
      if (!hasStyle && !hasScript) {
        setStylePreview("");
        setScriptPreview("");
        return;
      }
      try {
        const [styleResp, scriptResp] = await Promise.all([
          hasStyle ? getFile(domainId, "style.css").catch(() => null) : Promise.resolve(null),
          hasScript ? getFile(domainId, "script.js").catch(() => null) : Promise.resolve(null),
        ]);
        if (cancelled) return;
        setStylePreview(styleResp?.content || "");
        setScriptPreview(scriptResp?.content || "");
      } catch {
        if (cancelled) return;
        setStylePreview("");
        setScriptPreview("");
      }
    };
    void refreshAssets();
    return () => {
      cancelled = true;
    };
  }, [domainId, selection?.selectedPath, existingPathSet, setScriptPreview, setStylePreview]);

  useEffect(() => {
    if (previewMode === "preview" && !canPreviewCurrentFile) {
      setPreviewMode("code");
    }
  }, [previewMode, canPreviewCurrentFile, setPreviewMode]);

  const onSave = async () => {
    if (!selection || !currentFile) return;
    if (readOnly) {
      showToast({ type: "error", title: "Недостаточно прав", message: "Viewer может только просматривать файлы." });
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const result = await saveFile(
        domainId,
        selection.selectedPath,
        dirtyState.currentContent,
        description.trim() || undefined,
        { expectedVersion: selection.version, expectedPath: selection.selectedPath, source: "manual" }
      );
      const nextVersion = typeof result.version === "number" ? result.version : selection.version + 1;
      setDirtyState((prev: any) => ({
        ...prev,
        isDirty: false,
        originalContent: prev.currentContent,
        lastSavedAt: new Date().toISOString(),
      }));
      setSelection((prev) => (prev ? { ...prev, version: nextVersion } : prev));
      setDescription("");
      setHistoryRefreshKey((value) => value + 1);
      await loadFiles();
      showToast({ type: "success", title: "Файл сохранен", message: selection.selectedPath });
    } catch (err: any) {
      const message = err?.message || "Не удалось сохранить файл";
      if (err instanceof SaveConflictError) {
        setConflictState({
          currentVersion:
            typeof err.conflict.current_version === "number" ? err.conflict.current_version : selection.version,
          currentHash: err.conflict.current_hash,
          updatedBy: err.conflict.updated_by,
          updatedAt: err.conflict.updated_at,
        });
        showToast({
          type: "error",
          title: "Конфликт версий",
          message: "Файл был изменен в другом сеансе. Выберите действие в модальном окне.",
        });
        setError(message);
        return;
      }
      showToast({ type: "error", title: "Ошибка сохранения", message });
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  const onConflictReload = async () => {
    if (!selection?.selectedPath) {
      setConflictState(null);
      return;
    }
    const target = files.find((item) => item.path === selection.selectedPath);
    if (target) {
      await loadFile(target);
    }
    setConflictState(null);
  };

  const onConflictOverwrite = async () => {
    if (!selection?.selectedPath || !conflictState) {
      setConflictState(null);
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const result = await saveFile(
        domainId,
        selection.selectedPath,
        dirtyState.currentContent,
        description.trim() || "manual overwrite after conflict",
        { expectedVersion: conflictState.currentVersion, expectedPath: selection.selectedPath, source: "manual" }
      );
      const nextVersion =
        typeof result.version === "number" ? result.version : Math.max(selection.version + 1, conflictState.currentVersion + 1);
      setDirtyState((prev: any) => ({
        ...prev,
        isDirty: false,
        originalContent: prev.currentContent,
        lastSavedAt: new Date().toISOString(),
      }));
      setSelection((prev) => (prev ? { ...prev, version: nextVersion } : prev));
      setDescription("");
      setHistoryRefreshKey((value) => value + 1);
      setConflictState(null);
      await loadFiles();
      showToast({ type: "success", title: "Файл перезаписан", message: selection.selectedPath });
    } catch (err: any) {
      if (err instanceof SaveConflictError) {
        setConflictState({
          currentVersion:
            typeof err.conflict.current_version === "number" ? err.conflict.current_version : conflictState.currentVersion,
          currentHash: err.conflict.current_hash,
          updatedBy: err.conflict.updated_by,
          updatedAt: err.conflict.updated_at,
        });
        showToast({
          type: "error",
          title: "Повторный конфликт",
          message: "Файл снова обновился. Сначала выполните Reload.",
        });
        return;
      }
      showToast({ type: "error", title: "Ошибка сохранения", message: err?.message || "unknown error" });
    } finally {
      setSaving(false);
    }
  };

  const onRevertBuffer = () => {
    setDirtyState((prev: any) => ({
      ...prev,
      isDirty: false,
      currentContent: prev.originalContent,
    }));
    setDescription("");
  };

  const onDownload = () => {
    if (!selection?.selectedPath) return;
    const blob = new Blob([dirtyState.currentContent], { type: selection.mimeType || "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = selection.selectedPath.split("/").pop() || "file.txt";
    a.click();
    URL.revokeObjectURL(url);
  };

  const confirmLeaveWithDirty = () => {
    if (!dirtyState.isDirty) return true;
    return window.confirm("Есть несохраненные изменения. Выйти без сохранения?");
  };

  const onSelectFile = async (file: EditorFileMeta) => {
    await loadFile(file);
  };

  const previewSrcDoc = useMemo(() => {
    if (!selection || !currentFile) return "";
    const selectedPath = selection.selectedPath.toLowerCase();
    if (isImageLikeFile(selection.selectedPath, currentFile.mimeType)) return "";
    if (!selectedPath.endsWith(".html")) return "";
    const html = previewSource === "buffer" ? dirtyState.currentContent : dirtyState.originalContent;
    const withAssets = injectRuntimeAssets(html, stylePreview, scriptPreview);
    return rewriteHtmlAssetRefs(withAssets, domainId, existingPathSet);
  }, [
    selection,
    currentFile,
    previewSource,
    dirtyState.currentContent,
    dirtyState.originalContent,
    stylePreview,
    scriptPreview,
    domainId,
    existingPathSet,
  ]);

  const previewViewportClass = useMemo(() => {
    if (previewViewport === "mobile") return "mx-auto w-full max-w-[390px]";
    if (previewViewport === "tablet") return "mx-auto w-full max-w-[820px]";
    return "w-full";
  }, [previewViewport]);

  const rawImageURL = useMemo(() => {
    if (!selection || !currentFile || !isImageLikeFile(selection.selectedPath, currentFile.mimeType)) return "";
    return `${apiBase()}/api/domains/${domainId}/files/${encodePath(selection.selectedPath)}?raw=1`;
  }, [selection, currentFile, domainId]);

  const aiCreatePreviewSrcDoc = useMemo(() => {
    if (!aiCreatePreviewFile) return "";
    if (!aiCreatePreviewFile.path.toLowerCase().endsWith(".html")) return "";
    return rewriteHtmlAssetRefs(
      injectRuntimeAssets(aiCreatePreviewFile.content || "", stylePreview, scriptPreview),
      domainId,
      existingPathSet
    );
  }, [aiCreatePreviewFile, stylePreview, scriptPreview, domainId, existingPathSet]);

  return {
    conflictState,
    setConflictState,
    loadFile,
    onSave,
    onConflictReload,
    onConflictOverwrite,
    onRevertBuffer,
    onDownload,
    confirmLeaveWithDirty,
    onSelectFile,
    previewSrcDoc,
    previewViewportClass,
    rawImageURL,
    aiCreatePreviewSrcDoc,
  };
}
