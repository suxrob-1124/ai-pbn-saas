"use client";

import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";
import {
  FiArrowLeft,
  FiCode,
  FiEdit3,
  FiEye,
  FiFolder,
  FiMove,
  FiPlus,
  FiTrash2,
  FiUpload,
  FiWind
} from "react-icons/fi";

import { EditorToolbar } from "../../../../components/EditorToolbar";
import { FileHistory } from "../../../../components/FileHistory";
import { FileTree } from "../../../../components/FileTree";
import { ConflictResolutionModal } from "../../../../components/ConflictResolutionModal";
import { MonacoDiffEditor } from "../../../../components/MonacoDiffEditor";
import { MonacoEditor } from "../../../../components/MonacoEditor";
import { apiBase, authFetch } from "../../../../lib/http";
import {
  SaveConflictError,
  type AIPageApplyAction,
  type AIEditorSuggestionDTO,
  aiRegenerateAsset,
  type AIPageSuggestionDTO,
  aiCreatePage,
  aiSuggestFile,
  type ContextPackMetaDTO,
  createFileOrDir,
  deleteFile,
  getFile,
  getEditorContextPack,
  listFiles,
  moveFile,
  restoreFile,
  saveFile,
  uploadFile,
  type AIPageSuggestionAsset,
  type AIPageSuggestionFile,
  type FileListItem
} from "../../../../lib/fileApi";
import { showToast } from "../../../../lib/toastStore";
import { useAuthGuard } from "../../../../lib/useAuth";
import type { EditorDirtyState, EditorFileMeta, EditorSelectionState } from "../../../../types/editor";
import { AI_CONTEXT_MODE_OPTIONS, EDITOR_MODEL_OPTIONS } from "../../../../features/editor-v3/services/constants";
import type { AIContextMode } from "../../../../features/editor-v3/types/ai";
import type { DomainSummaryResponse } from "../../../../features/editor-v3/types/editor";

const detectLanguage = (pathValue: string) => {
  const path = pathValue.toLowerCase();
  if (path.endsWith(".html") || path.endsWith(".htm")) return "html";
  if (path.endsWith(".css")) return "css";
  if (path.endsWith(".js") || path.endsWith(".mjs") || path.endsWith(".cjs")) return "javascript";
  if (path.endsWith(".ts")) return "typescript";
  if (path.endsWith(".tsx")) return "typescript";
  if (path.endsWith(".json")) return "json";
  if (path.endsWith(".xml") || path.endsWith(".svg")) return "xml";
  if (path.endsWith(".md") || path.endsWith(".markdown")) return "markdown";
  return "plaintext";
};

const looksBinary = (value: string) => value.includes("\u0000");

const normalizeRelativeSitePath = (baseFilePath: string, refValue: string) => {
  const trimmed = refValue.trim();
  if (!trimmed || trimmed.startsWith("#")) return "";
  if (/^(data:|mailto:|tel:|javascript:|https?:)/i.test(trimmed)) return "";
  const pathPart = trimmed.split("#")[0].split("?")[0].trim();
  if (!pathPart) return "";
  const rawSegments = (pathPart.startsWith("/")
    ? pathPart.replace(/^\/+/, "").split("/")
    : [...baseFilePath.split("/").filter(Boolean).slice(0, -1), ...pathPart.split("/")]).filter(Boolean);
  const normalized: string[] = [];
  for (const segment of rawSegments) {
    if (segment === ".") continue;
    if (segment === "..") {
      normalized.pop();
      continue;
    }
    normalized.push(segment);
  }
  return normalized.join("/");
};

const extractLocalAssetRefsFromHTML = (html: string) => {
  const refs = new Set<string>();
  const re = /\b(?:src|href)\s*=\s*["']([^"']+)["']/gi;
  let match: RegExpExecArray | null;
  while ((match = re.exec(html)) !== null) {
    const value = (match[1] || "").trim();
    if (!value) continue;
    if (/^(data:|mailto:|tel:|javascript:|https?:|#)/i.test(value)) continue;
    refs.add(value);
  }
  return Array.from(refs);
};

const encodePath = (value: string) =>
  value
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");

function rewriteHtmlAssetRefs(html: string, domainId: string) {
  const base = apiBase();
  return html.replace(/\b(src|href)\s*=\s*["']([^"']+)["']/gi, (full, attr, rawValue: string) => {
    const value = rawValue.trim();
    if (!value || value.startsWith("#")) return full;
    if (/^(data:|mailto:|tel:|javascript:|https?:)/i.test(value)) return full;
    const normalized = value.replace(/^\.\//, "").replace(/^\//, "");
    if (!normalized) return full;
    const [pathPart, hashPart = ""] = normalized.split("#");
    const [purePath, queryPart = ""] = pathPart.split("?");
    if (!purePath) return full;
    const encodedPath = purePath
      .split("/")
      .filter(Boolean)
      .map((part) => encodeURIComponent(part))
      .join("/");
    if (!encodedPath) return full;
    const query = queryPart ? `&${queryPart}` : "";
    const hash = hashPart ? `#${hashPart}` : "";
    const url = `${base}/api/domains/${domainId}/files/${encodedPath}?raw=1${query}${hash}`;
    return `${attr}="${url}"`;
  });
}

function injectRuntimeAssets(indexHtml: string, styleContent: string, scriptContent: string) {
  let html = indexHtml || "";
  if (styleContent) {
    html = html.replace(/<link[^>]*href=["']style\.css["'][^>]*>/gi, "");
    if (/<\/head>/i.test(html)) {
      html = html.replace(/<\/head>/i, `<style data-live-preview="style.css">\n${styleContent}\n</style>\n</head>`);
    } else {
      html = `<style data-live-preview="style.css">\n${styleContent}\n</style>\n${html}`;
    }
  }
  if (scriptContent) {
    html = html.replace(/<script[^>]*src=["']script\.js["'][^>]*>\s*<\/script>/gi, "");
    if (/<\/body>/i.test(html)) {
      html = html.replace(/<\/body>/i, `<script data-live-preview="script.js">\n${scriptContent}\n</script>\n</body>`);
    } else {
      html = `${html}\n<script data-live-preview="script.js">\n${scriptContent}\n</script>`;
    }
  }
  return html;
}

export default function DomainEditorPage() {
  useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const domainId = params?.id as string;

  const requestedPath = searchParams.get("path") || "";
  const requestedLineRaw = searchParams.get("line") || "";
  const requestedLine = Number.parseInt(requestedLineRaw, 10);

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
  const [focusLine, setFocusLine] = useState<number | undefined>(
    Number.isFinite(requestedLine) && requestedLine > 0 ? requestedLine : undefined
  );
  const [historyRefreshKey, setHistoryRefreshKey] = useState(0);
  const [previewMode, setPreviewMode] = useState<"code" | "preview">("code");
  const [previewSource, setPreviewSource] = useState<"buffer" | "published">("buffer");
  const [stylePreview, setStylePreview] = useState("");
  const [scriptPreview, setScriptPreview] = useState("");
  const [aiStudioTab, setAiStudioTab] = useState<"edit" | "create">("edit");
  const [aiInstruction, setAiInstruction] = useState("");
  const [aiOutput, setAiOutput] = useState("");
  const [aiOutputSourcePath, setAiOutputSourcePath] = useState("");
  const [aiBusy, setAiBusy] = useState(false);
  const [aiModel, setAiModel] = useState("");
  const [aiContextMode, setAiContextMode] = useState<AIContextMode>("auto");
  const [aiContextSelectedFiles, setAiContextSelectedFiles] = useState<string[]>([]);
  const [aiSuggestView, setAiSuggestView] = useState<"diff" | "content">("diff");
  const [aiSuggestMeta, setAiSuggestMeta] = useState<{
    source?: string;
    warnings: string[];
    tokenUsage?: Record<string, any>;
    contextPack?: ContextPackMetaDTO;
  } | null>(null);
  const [aiSuggestDiagnosticsOpen, setAiSuggestDiagnosticsOpen] = useState(false);
  const [aiSuggestContextBusy, setAiSuggestContextBusy] = useState(false);
  const [aiSuggestContextDebug, setAiSuggestContextDebug] = useState("");
  const [aiCreateInstruction, setAiCreateInstruction] = useState("");
  const [aiCreatePath, setAiCreatePath] = useState("new-page.html");
  const [aiCreateBusy, setAiCreateBusy] = useState(false);
  const [aiCreateAssetBusyPath, setAiCreateAssetBusyPath] = useState("");
  const [aiCreateModel, setAiCreateModel] = useState("");
  const [aiCreateContextMode, setAiCreateContextMode] = useState<AIContextMode>("auto");
  const [aiCreateContextSelectedFiles, setAiCreateContextSelectedFiles] = useState<string[]>([]);
  const [aiCreateFiles, setAiCreateFiles] = useState<AIPageSuggestionFile[]>([]);
  const [aiCreateAssets, setAiCreateAssets] = useState<AIPageSuggestionAsset[]>([]);
  const [aiCreateSkippedAssets, setAiCreateSkippedAssets] = useState<string[]>([]);
  const [aiCreateApplyPlan, setAiCreateApplyPlan] = useState<Record<string, AIPageApplyAction>>({});
  const [aiCreateExistingContent, setAiCreateExistingContent] = useState<Record<string, string>>({});
  const [aiCreatePreviewPath, setAiCreatePreviewPath] = useState("");
  const [aiCreateView, setAiCreateView] = useState<"diff" | "preview" | "code">("diff");
  const [aiCreateMeta, setAiCreateMeta] = useState<{
    source?: string;
    warnings: string[];
    tokenUsage?: Record<string, any>;
    contextPack?: ContextPackMetaDTO;
  } | null>(null);
  const [aiCreateDiagnosticsOpen, setAiCreateDiagnosticsOpen] = useState(false);
  const [aiCreateContextBusy, setAiCreateContextBusy] = useState(false);
  const [aiCreateContextDebug, setAiCreateContextDebug] = useState("");
  const [conflictState, setConflictState] = useState<{
    currentVersion: number;
    currentHash?: string;
    updatedBy?: string;
    updatedAt?: string;
  } | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const assetUploadInputRef = useRef<HTMLInputElement | null>(null);
  const [assetUploadTargetPath, setAssetUploadTargetPath] = useState("");

  const currentFile = useMemo(
    () => files.find((item) => item.path === selection?.selectedPath),
    [files, selection?.selectedPath]
  );
  const canPreviewCurrentFile = Boolean(selection?.selectedPath?.toLowerCase().endsWith(".html"));
  const readOnly = (summary?.my_role || "viewer") === "viewer";
  const binaryPreview = looksBinary(dirtyState.currentContent);
  const canSave =
    !readOnly &&
    Boolean(currentFile?.isEditable) &&
    !binaryPreview &&
    dirtyState.isDirty &&
    !saving;
  const aiApplyPathMismatch = Boolean(
    aiOutput &&
      aiOutputSourcePath &&
      selection?.selectedPath &&
      selection.selectedPath !== aiOutputSourcePath
  );
  const selectableContextFiles = useMemo(
    () =>
      files
        .filter((item) => item.isEditable && item.size <= 250_000)
        .map((item) => item.path)
        .slice(0, 120),
    [files]
  );
  const existingFilesMap = useMemo(() => {
    const map = new Map<string, EditorFileMeta>();
    for (const file of files) {
      map.set(file.path, file);
    }
    return map;
  }, [files]);
  const aiCreatePreviewFile = useMemo(
    () => aiCreateFiles.find((item) => item.path === aiCreatePreviewPath) || null,
    [aiCreateFiles, aiCreatePreviewPath]
  );
  const aiCreateAssetPathSet = useMemo(() => new Set(aiCreateAssets.map((item) => item.path)), [aiCreateAssets]);
  const aiCreateFilePathSet = useMemo(() => new Set(aiCreateFiles.map((item) => item.path)), [aiCreateFiles]);
  const aiCreateMissingAssets = useMemo(() => {
    const knownPaths = new Set<string>([
      ...Array.from(existingFilesMap.keys()),
      ...Array.from(aiCreateFilePathSet.values()),
      ...aiCreateAssets.map((item) => item.path),
    ]);
    const missing = new Set<string>();
    for (const file of aiCreateFiles) {
      if (!file.path.toLowerCase().endsWith(".html")) continue;
      const refs = extractLocalAssetRefsFromHTML(file.content || "");
      for (const ref of refs) {
        const resolved = normalizeRelativeSitePath(file.path, ref);
        if (!resolved) continue;
        if (!knownPaths.has(resolved)) {
          missing.add(resolved);
        }
      }
    }
    for (const asset of aiCreateAssets) {
      if (!existingFilesMap.has(asset.path) && !aiCreateFilePathSet.has(asset.path)) {
        missing.add(asset.path);
      }
    }
    return Array.from(missing).sort();
  }, [aiCreateAssets, aiCreateFiles, aiCreateFilePathSet, existingFilesMap]);
  const unresolvedMissingAssets = useMemo(
    () => aiCreateMissingAssets.filter((pathValue) => !aiCreateSkippedAssets.includes(pathValue)),
    [aiCreateMissingAssets, aiCreateSkippedAssets]
  );
  const unresolvedBrokenAssets = useMemo(
    () =>
      aiCreateAssets
        .filter((asset) => {
          if (aiCreateSkippedAssets.includes(asset.path)) return false;
          const existing = existingFilesMap.get(asset.path);
          if (!existing) return false;
          return !existing.mimeType.toLowerCase().startsWith("image/");
        })
        .map((asset) => asset.path),
    [aiCreateAssets, aiCreateSkippedAssets, existingFilesMap]
  );
  const unresolvedAssetIssues = useMemo(() => {
    const merged = new Set<string>([...unresolvedMissingAssets, ...unresolvedBrokenAssets]);
    return Array.from(merged).sort();
  }, [unresolvedBrokenAssets, unresolvedMissingAssets]);
  const unresolvedNonManifestAssets = useMemo(
    () => unresolvedMissingAssets.filter((pathValue) => !aiCreateAssetPathSet.has(pathValue)),
    [aiCreateAssetPathSet, unresolvedMissingAssets]
  );

  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirtyState.isDirty) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [dirtyState.isDirty]);

  useEffect(() => {
    setAiCreateSkippedAssets((prev) => prev.filter((pathValue) => aiCreateMissingAssets.includes(pathValue)));
  }, [aiCreateMissingAssets]);

  const updatePathQuery = (pathValue: string) => {
    const query = new URLSearchParams(searchParams.toString());
    query.set("path", pathValue);
    query.delete("line");
    const qs = query.toString();
    router.replace((`/domains/${domainId}/editor${qs ? `?${qs}` : ""}` as any), { scroll: false });
  };

  const toEditorFileMeta = (item: FileListItem): EditorFileMeta => ({
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
    });

  const loadFiles = async () => {
    const fileList = await listFiles(domainId);
    const prepared: EditorFileMeta[] = (Array.isArray(fileList) ? fileList : []).map((item: FileListItem) => toEditorFileMeta(item));
    setFiles(prepared);
    return prepared;
  };

  const loadDeletedFiles = async () => {
    const fileList = await listFiles(domainId, { includeDeleted: true });
    const prepared: EditorFileMeta[] = (Array.isArray(fileList) ? fileList : [])
      .filter((item: FileListItem) => Boolean(item.deletedAt))
      .map((item: FileListItem) => ({
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
    }));
    setDeletedFiles(prepared);
    return prepared;
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
      try {
        const [styleResp, scriptResp] = await Promise.all([
          getFile(domainId, "style.css").catch(() => null),
          getFile(domainId, "script.js").catch(() => null),
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
  }, [domainId, selection?.selectedPath, historyRefreshKey]);

  useEffect(() => {
    if (previewMode === "preview" && !canPreviewCurrentFile) {
      setPreviewMode("code");
    }
  }, [previewMode, canPreviewCurrentFile]);

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
      setDirtyState((prev) => ({
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
      setDirtyState((prev) => ({
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
    setDirtyState((prev) => ({
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

  const onCreateFile = async () => {
    if (readOnly) return;
    const nextPath = prompt("Путь нового файла (например: pages/about.html)");
    if (!nextPath) return;
    try {
      await createFileOrDir(domainId, { kind: "file", path: nextPath, content: "" });
      const nextFiles = await loadFiles();
      const created = nextFiles.find((item) => item.path === nextPath);
      if (created) {
        await loadFile(created);
      }
      showToast({ type: "success", title: "Файл создан", message: nextPath });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось создать файл", message: err?.message || "unknown error" });
    }
  };

  const onCreateFolder = async () => {
    if (readOnly) return;
    const nextPath = prompt("Путь новой папки (например: pages/blog)");
    if (!nextPath) return;
    try {
      await createFileOrDir(domainId, { kind: "dir", path: nextPath });
      await loadFiles();
      showToast({ type: "success", title: "Папка создана", message: nextPath });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось создать папку", message: err?.message || "unknown error" });
    }
  };

  const onRename = async () => {
    if (readOnly || !selection?.selectedPath) return;
    const currentPath = selection.selectedPath;
    const parts = currentPath.split("/").filter(Boolean);
    const currentName = parts.pop() || currentPath;
    const parent = parts.join("/");
    const nextName = (prompt("Новое имя файла", currentName) || "").trim();
    if (!nextName || nextName === currentName) return;
    if (nextName.includes("/")) {
      showToast({ type: "error", title: "Некорректное имя", message: "Имя файла не должно содержать /" });
      return;
    }
    const nextPath = parent ? `${parent}/${nextName}` : nextName;
    try {
      await moveFile(domainId, currentPath, nextPath);
      const nextFiles = await loadFiles();
      const moved = nextFiles.find((item) => item.path === nextPath);
      if (moved) {
        await loadFile(moved);
      }
      showToast({ type: "success", title: "Файл переименован", message: `${currentName} → ${nextName}` });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось переименовать файл", message: err?.message || "unknown error" });
    }
  };

  const onMove = async () => {
    if (readOnly || !selection?.selectedPath) return;
    const currentPath = selection.selectedPath;
    const parts = currentPath.split("/").filter(Boolean);
    const currentName = parts.pop() || currentPath;
    const currentDir = parts.join("/");
    const destinationRaw = prompt(
      "Папка назначения (например: pages/archive). Пусто = корень.",
      currentDir
    );
    if (destinationRaw === null) return;
    const destination = destinationRaw.trim().replace(/^\/+|\/+$/g, "");
    const nextPath = destination ? `${destination}/${currentName}` : currentName;
    if (nextPath === currentPath) return;
    try {
      await moveFile(domainId, currentPath, nextPath);
      const nextFiles = await loadFiles();
      const moved = nextFiles.find((item) => item.path === nextPath);
      if (moved) {
        await loadFile(moved);
      }
      showToast({ type: "success", title: "Файл перемещен", message: `${currentPath} → ${nextPath}` });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось переместить файл", message: err?.message || "unknown error" });
    }
  };

  const onDelete = async () => {
    if (readOnly || !selection?.selectedPath) return;
    if (!confirm(`Удалить "${selection.selectedPath}"?`)) return;
    try {
      await deleteFile(domainId, selection.selectedPath);
      setSelection(null);
      setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
      await loadFiles();
      await loadDeletedFiles();
      showToast({ type: "success", title: "Файл удален", message: selection.selectedPath });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось удалить файл", message: err?.message || "unknown error" });
    }
  };

  const onDeleteFolder = async (folderPath: string) => {
    if (readOnly) return;
    const normalized = folderPath.trim().replace(/^\/+|\/+$/g, "");
    if (!normalized) return;
    const hasChildren = files.some((item) => item.path.startsWith(`${normalized}/`));
    const confirmed = hasChildren
      ? confirm(`Папка "${normalized}" содержит файлы. Удалить папку рекурсивно?`)
      : confirm(`Удалить пустую папку "${normalized}"?`);
    if (!confirmed) return;
    try {
      await deleteFile(domainId, normalized, { recursive: hasChildren });
      if (selection?.selectedPath === normalized || selection?.selectedPath?.startsWith(`${normalized}/`)) {
        setSelection(null);
        setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
      }
      await loadFiles();
      await loadDeletedFiles();
      showToast({ type: "success", title: "Папка удалена", message: normalized });
    } catch (err: any) {
      if (String(err?.message || "").includes("recursive")) {
        const retry = confirm(`Папка "${normalized}" не пустая. Выполнить рекурсивное удаление?`);
        if (!retry) return;
        try {
          await deleteFile(domainId, normalized, { recursive: true });
          await loadFiles();
          await loadDeletedFiles();
          showToast({ type: "success", title: "Папка удалена", message: `${normalized} (recursive)` });
          return;
        } catch (retryErr: any) {
          showToast({ type: "error", title: "Не удалось удалить папку", message: retryErr?.message || "unknown error" });
          return;
        }
      }
      showToast({ type: "error", title: "Не удалось удалить папку", message: err?.message || "unknown error" });
    }
  };

  const onRestoreDeleted = async (file: EditorFileMeta) => {
    try {
      await restoreFile(domainId, file.path);
      const active = await loadFiles();
      await loadDeletedFiles();
      const restored = active.find((item) => item.path === file.path);
      if (restored) {
        await loadFile(restored);
      }
      showToast({ type: "success", title: "Файл восстановлен", message: file.path });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось восстановить файл", message: err?.message || "unknown error" });
    }
  };

  const onUploadClick = () => fileInputRef.current?.click();

  const onUploadInput = async (file?: File | null) => {
    if (readOnly || !file) return;
    const destination = prompt("Куда загрузить файл? (путь или папка)", file.name) || file.name;
    try {
      await uploadFile(domainId, file, destination);
      await loadFiles();
      showToast({ type: "success", title: "Файл загружен", message: destination });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось загрузить файл", message: err?.message || "unknown error" });
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const selectedContextFiles = (mode: AIContextMode, selected: string[]) => {
    const trimmed = selected.map((item) => item.trim()).filter(Boolean);
    if (mode === "auto") return undefined;
    return trimmed.length > 0 ? trimmed.slice(0, 20) : undefined;
  };

  const onAISuggestContextPreview = async () => {
    if (!selection?.selectedPath) return;
    setAiSuggestContextBusy(true);
    try {
      const payload = {
        target_path: selection.selectedPath,
        context_mode: aiContextMode,
        context_files: selectedContextFiles(aiContextMode, aiContextSelectedFiles),
      } as const;
      const debug = await getEditorContextPack(domainId, payload);
      setAiSuggestContextDebug(debug.site_context || "");
      setAiSuggestMeta((prev) => ({
        source: prev?.source,
        warnings: prev?.warnings || [],
        tokenUsage: prev?.tokenUsage,
        contextPack: debug.context_pack_meta,
      }));
      setAiSuggestDiagnosticsOpen(true);
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось собрать контекст", message: err?.message || "unknown error" });
    } finally {
      setAiSuggestContextBusy(false);
    }
  };

  const onAISuggest = async () => {
    if (!selection?.selectedPath || !aiInstruction.trim()) return;
    if (aiContextMode === "manual" && aiContextSelectedFiles.length === 0) {
      showToast({
        type: "error",
        title: "Нужны контекстные файлы",
        message: "Для режима manual выберите хотя бы один файл контекста.",
      });
      return;
    }
    setAiBusy(true);
    setAiSuggestMeta(null);
    setAiSuggestContextDebug("");
    setAiOutputSourcePath(selection.selectedPath);
    try {
      const result: AIEditorSuggestionDTO = await aiSuggestFile(domainId, selection.selectedPath, {
        instruction: aiInstruction.trim(),
        model: aiModel.trim() || undefined,
        context_mode: aiContextMode,
        context_files: selectedContextFiles(aiContextMode, aiContextSelectedFiles),
      });
      setAiOutput(result.suggested_content || "");
      setAiSuggestView("diff");
      const promptSource =
        typeof result.prompt_trace?.resolved_source === "string" ? result.prompt_trace.resolved_source : undefined;
      setAiSuggestMeta({
        source: promptSource,
        warnings: Array.isArray(result.warnings) ? result.warnings : [],
        tokenUsage: result.token_usage,
        contextPack: result.context_pack_meta,
      });
      if ((result.suggested_content || "") === dirtyState.currentContent) {
        showToast({
          type: "success",
          title: "Предложение без изменений",
          message: "AI не предложил отличий для текущего файла.",
        });
      } else {
        showToast({ type: "success", title: "AI-предложение готово", message: selection.selectedPath });
      }
    } catch (err: any) {
      showToast({ type: "error", title: "Ошибка AI-редактирования", message: err?.message || "unknown error" });
    } finally {
      setAiBusy(false);
    }
  };

  const onOpenAISourceFile = async () => {
    if (!aiOutputSourcePath) return;
    const sourceFile = files.find((item) => item.path === aiOutputSourcePath);
    if (!sourceFile) {
      showToast({
        type: "error",
        title: "Исходный файл не найден",
        message: "Файл для AI-предложения был удален или перемещен.",
      });
      return;
    }
    await loadFile(sourceFile);
  };

  const onApplyAISuggest = () => {
    if (!aiOutput) return;
    if (!selection?.selectedPath || !aiOutputSourcePath || selection.selectedPath !== aiOutputSourcePath) {
      showToast({
        type: "error",
        title: "Неверный файл для применения",
        message: `Предложение относится к "${aiOutputSourcePath}", откройте этот файл перед применением.`,
      });
      return;
    }
    setDirtyState((prev) => ({
      ...prev,
      currentContent: aiOutput,
      isDirty: aiOutput !== prev.originalContent,
    }));
  };

  const onAICreateContextPreview = async () => {
    if (!aiCreatePath.trim()) return;
    setAiCreateContextBusy(true);
    try {
      const payload = {
        target_path: aiCreatePath.trim(),
        context_mode: aiCreateContextMode,
        context_files: selectedContextFiles(aiCreateContextMode, aiCreateContextSelectedFiles),
      } as const;
      const debug = await getEditorContextPack(domainId, payload);
      setAiCreateContextDebug(debug.site_context || "");
      setAiCreateMeta((prev) => ({
        source: prev?.source,
        warnings: prev?.warnings || [],
        tokenUsage: prev?.tokenUsage,
        contextPack: debug.context_pack_meta,
      }));
      setAiCreateDiagnosticsOpen(true);
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось собрать контекст", message: err?.message || "unknown error" });
    } finally {
      setAiCreateContextBusy(false);
    }
  };

  const onAICreatePage = async () => {
    if (!aiCreateInstruction.trim() || !aiCreatePath.trim()) return;
    if (aiCreateContextMode === "manual" && aiCreateContextSelectedFiles.length === 0) {
      showToast({
        type: "error",
        title: "Нужны контекстные файлы",
        message: "Для режима manual выберите хотя бы один файл контекста.",
      });
      return;
    }
    setAiCreateBusy(true);
    setAiCreateMeta(null);
    setAiCreateContextDebug("");
    setAiCreateSkippedAssets([]);
    setAiCreateAssets([]);
    try {
      const result: AIPageSuggestionDTO = await aiCreatePage(domainId, {
        instruction: aiCreateInstruction.trim(),
        target_path: aiCreatePath.trim(),
        with_assets: true,
        model: aiCreateModel.trim() || undefined,
        context_mode: aiCreateContextMode,
        context_files: selectedContextFiles(aiCreateContextMode, aiCreateContextSelectedFiles),
      });
      const generatedFiles = Array.isArray(result.files) ? result.files : [];
      const generatedAssets = Array.isArray(result.assets) ? result.assets : [];
      if (generatedFiles.length === 0) {
        showToast({ type: "error", title: "AI не вернул файлов", message: "Повторите генерацию с другим запросом." });
        return;
      }
      setAiCreateFiles(generatedFiles);
      setAiCreateAssets(generatedAssets);
      setAiCreatePreviewPath(generatedFiles[0]?.path || "");
      setAiCreateView("diff");
      const defaultPlan: Record<string, AIPageApplyAction> = {};
      for (const file of generatedFiles) {
        defaultPlan[file.path] = existingFilesMap.has(file.path) ? "skip" : "create";
      }
      setAiCreateApplyPlan(defaultPlan);
      const existingMap: Record<string, string> = {};
      const existingPaths = generatedFiles.filter((item) => existingFilesMap.has(item.path)).map((item) => item.path);
      await Promise.all(
        existingPaths.map(async (filePath) => {
          try {
            const payload = await getFile(domainId, filePath);
            existingMap[filePath] = payload.content || "";
          } catch {
            existingMap[filePath] = "";
          }
        })
      );
      setAiCreateExistingContent(existingMap);
      const promptSource =
        typeof result.prompt_trace?.resolved_source === "string" ? result.prompt_trace.resolved_source : undefined;
      setAiCreateMeta({
        source: promptSource,
        warnings: Array.isArray(result.warnings) ? result.warnings : [],
        tokenUsage: result.token_usage,
        contextPack: result.context_pack_meta,
      });
      const existingCount = generatedFiles.filter((item) => existingFilesMap.has(item.path)).length;
      showToast({
        type: "success",
        title: "AI сгенерировал пакет файлов",
        message: `${generatedFiles.length} файлов, конфликтов по пути: ${existingCount}`,
      });
    } catch (err: any) {
      const status = Number(err?.status || err?.response?.status || 0);
      if (status === 422 || String(err?.message || "").toLowerCase().includes("ai_response_invalid_format")) {
        showToast({
          type: "error",
          title: "AI вернул невалидный формат",
          message: "Модель не вернула корректный JSON-контракт. Уточните инструкцию и повторите.",
        });
      } else {
        showToast({ type: "error", title: "Ошибка создания страницы", message: err?.message || "unknown error" });
      }
    } finally {
      setAiCreateBusy(false);
    }
  };

  const onSetCreatePlan = (pathValue: string, action: AIPageApplyAction) => {
    setAiCreateApplyPlan((prev) => ({ ...prev, [pathValue]: action }));
  };

  const onToggleSkipAsset = (pathValue: string) => {
    setAiCreateSkippedAssets((prev) =>
      prev.includes(pathValue) ? prev.filter((item) => item !== pathValue) : [...prev, pathValue]
    );
  };

  const onAssetUploadPick = (pathValue: string) => {
    if (readOnly) {
      showToast({
        type: "error",
        title: "Недостаточно прав",
        message: "Для загрузки ассета нужны права owner/editor/admin.",
      });
      return;
    }
    setAssetUploadTargetPath(pathValue);
    assetUploadInputRef.current?.click();
  };

  const onAssetUploadSelected = async (event: ChangeEvent<HTMLInputElement>) => {
    const picked = event.target.files?.[0];
    event.currentTarget.value = "";
    if (!picked || !assetUploadTargetPath) return;
    try {
      await uploadFile(domainId, picked, assetUploadTargetPath);
      setAiCreateSkippedAssets((prev) => prev.filter((item) => item !== assetUploadTargetPath));
      await loadFiles();
      showToast({
        type: "success",
        title: "Ассет загружен",
        message: `Файл ${assetUploadTargetPath} успешно добавлен.`,
      });
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось загрузить ассет",
        message: err?.message || "unknown error",
      });
    } finally {
      setAssetUploadTargetPath("");
    }
  };

  const onCopyAssetPrompt = async (pathValue: string) => {
    const asset = aiCreateAssets.find((item) => item.path === pathValue);
    if (!asset?.prompt?.trim()) {
      showToast({
        type: "error",
        title: "Нет prompt для ассета",
        message: "В манифесте не указан prompt для регенерации изображения.",
      });
      return;
    }
    const text = asset.prompt.trim();
    try {
      await navigator.clipboard.writeText(text);
      showToast({
        type: "success",
        title: "Промпт скопирован",
        message: "Используйте prompt в image-generation пайплайне или внешнем инструменте.",
      });
    } catch {
      showToast({
        type: "error",
        title: "Не удалось скопировать",
        message: "Скопируйте prompt вручную из таблицы ассетов.",
      });
    }
  };

  const onRegenerateAsset = async (pathValue: string) => {
    const asset = aiCreateAssets.find((item) => item.path === pathValue);
    if (!asset) {
      showToast({
        type: "error",
        title: "Ассет не найден",
        message: "Выберите ассет из манифеста и повторите действие.",
      });
      return;
    }
    const promptValue = asset.prompt?.trim();
    if (!promptValue) {
      showToast({
        type: "error",
        title: "Нет prompt для регенерации",
        message: "Заполните prompt в манифесте ассетов или загрузите изображение вручную.",
      });
      return;
    }
    setAiCreateAssetBusyPath(pathValue);
    try {
      const result = await aiRegenerateAsset(domainId, {
        path: asset.path,
        prompt: promptValue,
        mime_type: asset.mime_type || undefined,
        model: aiCreateModel.trim() || undefined,
        context_mode: aiCreateContextMode,
        context_files: selectedContextFiles(aiCreateContextMode, aiCreateContextSelectedFiles),
      });
      setAiCreateSkippedAssets((prev) => prev.filter((item) => item !== pathValue));
      const refreshed = await loadFiles();
      if (selection?.selectedPath === pathValue) {
        const selected = refreshed.find((file) => file.path === pathValue);
        if (selected) {
          await loadFile(selected);
        }
      }
      showToast({
        type: "success",
        title: "Изображение сгенерировано",
        message: pathValue,
      });
      if (result.warnings && result.warnings.length > 0) {
        showToast({
          type: "error",
          title: "Предупреждения регенерации",
          message: result.warnings.slice(0, 2).join(" | "),
        });
      }
    } catch (err: any) {
      const status = Number(err?.status || err?.response?.status || 0);
      if (status === 422) {
        showToast({
          type: "error",
          title: "Невалидное изображение от AI",
          message: err?.message || "AI вернул неподходящий формат, попробуйте другую модель или prompt.",
        });
      } else {
        showToast({
          type: "error",
          title: "Ошибка регенерации ассета",
          message: err?.message || "unknown error",
        });
      }
    } finally {
      setAiCreateAssetBusyPath("");
    }
  };

  const onApplyCreatedFiles = async () => {
    if (aiCreateFiles.length === 0) return;
    const planned = aiCreateFiles
      .map((file) => ({
        file,
        action: aiCreateApplyPlan[file.path] || (existingFilesMap.has(file.path) ? "skip" : "create"),
      }));
    const actionable = planned.filter((item) => item.action !== "skip");
    if (actionable.length === 0) {
      showToast({
        type: "error",
        title: "Нет действий для применения",
        message: "Выберите create или overwrite хотя бы для одного файла.",
      });
      return;
    }
    if (unresolvedAssetIssues.length > 0) {
      const allowWithMissing = window.confirm(
        `Обнаружены нерешённые ассеты (${unresolvedAssetIssues.length}):\n${unresolvedAssetIssues.slice(0, 6).join("\n")}\n\nПрименить изменения всё равно?`
      );
      if (!allowWithMissing) return;
    }
    const overwriteCount = actionable.filter((item) => item.action === "overwrite").length;
    const createCount = actionable.length - overwriteCount;
    const skipCount = planned.length - actionable.length;
    const createFiles = actionable.filter((item) => item.action === "create").map((item) => item.file.path);
    const overwriteFiles = actionable.filter((item) => item.action === "overwrite").map((item) => item.file.path);
    const summaryLines = [
      `Создать: ${createCount}`,
      `Перезаписать: ${overwriteCount}`,
      `Пропустить: ${skipCount}`,
      createFiles.length ? `\n[CREATE]\n${createFiles.slice(0, 8).join("\n")}${createFiles.length > 8 ? `\n... +${createFiles.length - 8}` : ""}` : "",
      overwriteFiles.length ? `\n[OVERWRITE]\n${overwriteFiles.slice(0, 8).join("\n")}${overwriteFiles.length > 8 ? `\n... +${overwriteFiles.length - 8}` : ""}` : "",
    ]
      .filter(Boolean)
      .join("\n");
    const confirmed = window.confirm(
      `Проверьте план применения:\n\n${summaryLines}\n\nПродолжить?`
    );
    if (!confirmed) return;
    if (overwriteCount > 0) {
      const overwriteConfirmed = window.confirm(
        `Подтвердите перезапись ${overwriteCount} файлов.\nЭто изменит существующие файлы без возможности авто-отката.`
      );
      if (!overwriteConfirmed) return;
    }
    let applied = 0;
    let skipped = 0;
    const skippedExisting: string[] = [];
    const failed: string[] = [];
    for (const item of actionable) {
      const file = item.file;
      try {
        const exists = existingFilesMap.has(file.path);
        if (item.action === "create") {
          if (exists) {
            skipped += 1;
            skippedExisting.push(file.path);
            continue;
          }
          await createFileOrDir(domainId, {
            kind: "file",
            path: file.path,
            content: file.content,
            mime_type: file.mime_type,
          });
          applied += 1;
          continue;
        }
        if (item.action === "overwrite") {
          if (exists) {
            await saveFile(domainId, file.path, file.content, "ai create-page overwrite", {
              expectedPath: file.path,
              source: "ai",
            });
          } else {
            await createFileOrDir(domainId, {
              kind: "file",
              path: file.path,
              content: file.content,
              mime_type: file.mime_type,
            });
          }
          applied += 1;
        }
      } catch (err: any) {
        failed.push(`${file.path}: ${err?.message || "unknown error"}`);
      }
    }
    const nextFiles = await loadFiles();
    if (aiCreatePreviewPath) {
      const target = nextFiles.find((file) => file.path === aiCreatePreviewPath);
      if (target) {
        await loadFile(target);
      }
    }
    if (failed.length > 0) {
      showToast({
        type: "error",
        title: "Часть файлов не применена",
        message: `Успешно: ${applied}, пропущено: ${skipped}, ошибок: ${failed.length}. ${failed.slice(0, 2).join(" | ")}`,
      });
    } else {
      showToast({
        type: "success",
        title: "AI-файлы применены",
        message: `Успешно: ${applied}, пропущено: ${skipped}`,
      });
    }
    if (skippedExisting.length > 0) {
      showToast({
        type: "error",
        title: "Часть create-применений пропущена",
        message: `Файл уже существовал: ${skippedExisting.slice(0, 3).join(", ")}${skippedExisting.length > 3 ? "..." : ""}`,
      });
    }
  };

  const onSelectFile = async (file: EditorFileMeta) => {
    await loadFile(file);
  };

  const previewSrcDoc = useMemo(() => {
    if (!selection || !currentFile) return "";
    const selectedPath = selection.selectedPath.toLowerCase();
    if (currentFile.mimeType.toLowerCase().startsWith("image/")) return "";
    if (!selectedPath.endsWith(".html")) return "";
    const html = previewSource === "buffer" ? dirtyState.currentContent : dirtyState.originalContent;
    const withAssets = injectRuntimeAssets(html, stylePreview, scriptPreview);
    return rewriteHtmlAssetRefs(withAssets, domainId);
  }, [
    selection,
    currentFile,
    previewSource,
    dirtyState.currentContent,
    dirtyState.originalContent,
    stylePreview,
    scriptPreview,
    domainId,
  ]);

  const rawImageURL = useMemo(() => {
    if (!selection || !currentFile || !currentFile.mimeType.toLowerCase().startsWith("image/")) return "";
    return `${apiBase()}/api/domains/${domainId}/files/${encodePath(selection.selectedPath)}?raw=1`;
  }, [selection, currentFile, domainId]);

  const aiCreatePreviewSrcDoc = useMemo(() => {
    if (!aiCreatePreviewFile) return "";
    if (!aiCreatePreviewFile.path.toLowerCase().endsWith(".html")) return "";
    return rewriteHtmlAssetRefs(
      injectRuntimeAssets(aiCreatePreviewFile.content || "", stylePreview, scriptPreview),
      domainId
    );
  }, [aiCreatePreviewFile, stylePreview, scriptPreview, domainId]);

  if (loading) {
    return <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка редактора...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-slate-200 bg-white/80 p-4 shadow dark:border-slate-800 dark:bg-slate-900/60">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h1 className="flex items-center gap-2 text-xl font-semibold">
              <FiFolder /> Редактор сайта v2
            </h1>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              {summary?.domain.url || "—"} • Проект: {summary?.project_name || "—"}
            </p>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              Роль: {summary?.my_role || "viewer"} {readOnly ? "(только чтение)" : "(редактирование включено)"}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Link
              href={`/domains/${domainId}`}
              onClick={(event) => {
                if (!confirmLeaveWithDirty()) {
                  event.preventDefault();
                }
              }}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiArrowLeft /> К домену
            </Link>
            {summary?.domain.project_id && (
              <Link
                href={`/projects/${summary.domain.project_id}`}
                onClick={(event) => {
                  if (!confirmLeaveWithDirty()) {
                    event.preventDefault();
                  }
                }}
                className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
              >
                <FiArrowLeft /> К проекту
              </Link>
            )}
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-800 dark:bg-red-950/30 dark:text-red-300">
          {error}
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-[320px_1fr]">
        <aside className="space-y-3 rounded-xl border border-slate-200 bg-white/80 p-3 shadow dark:border-slate-800 dark:bg-slate-900/60">
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={onCreateFile}
              disabled={readOnly}
              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiPlus /> File
            </button>
            <button
              type="button"
              onClick={onCreateFolder}
              disabled={readOnly}
              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiFolder /> Folder
            </button>
            <button
              type="button"
              onClick={onRename}
              disabled={readOnly || !selection}
              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiEdit3 /> Rename
            </button>
            <button
              type="button"
              onClick={onMove}
              disabled={readOnly || !selection}
              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiMove /> Move
            </button>
            <button
              type="button"
              onClick={onDelete}
              disabled={readOnly || !selection}
              className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-red-50 px-2 py-1 text-xs font-semibold text-red-700 disabled:opacity-50 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
            >
              <FiTrash2 /> Delete
            </button>
            <button
              type="button"
              onClick={onUploadClick}
              disabled={readOnly}
              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiUpload /> Upload
            </button>
            <input
              ref={fileInputRef}
              type="file"
              className="hidden"
              onChange={(e) => onUploadInput(e.target.files?.[0] || null)}
            />
            <input
              ref={assetUploadInputRef}
              type="file"
              accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml"
              className="hidden"
              onChange={onAssetUploadSelected}
            />
          </div>
          <h2 className="mb-1 text-sm font-semibold">Файлы сайта</h2>
          <FileTree
            files={files}
            selectedPath={selection?.selectedPath}
            loading={fileLoading}
            onSelect={onSelectFile}
            canManageFolders={!readOnly}
            onDeleteFolder={onDeleteFolder}
          />
          <div className="mt-3 rounded-lg border border-slate-200 bg-slate-50/70 p-2 dark:border-slate-700 dark:bg-slate-800/40">
            <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
              Корзина
            </div>
            {deletedFiles.length === 0 && (
              <div className="text-xs text-slate-500 dark:text-slate-400">Удаленных файлов нет</div>
            )}
            {deletedFiles.length > 0 && (
              <div className="max-h-40 space-y-1 overflow-auto">
                {deletedFiles.map((file) => (
                  <div key={`trash-${file.id}`} className="rounded-md border border-slate-200 bg-white/80 px-2 py-1 dark:border-slate-700 dark:bg-slate-900/60">
                    <div className="truncate text-xs text-slate-700 dark:text-slate-200">{file.path}</div>
                    <button
                      type="button"
                      onClick={() => onRestoreDeleted(file)}
                      disabled={readOnly}
                      className="mt-1 inline-flex items-center gap-1 rounded-md border border-emerald-300 bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 disabled:opacity-50 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300"
                    >
                      Восстановить
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </aside>

        <section className="space-y-3">
          <EditorToolbar
            currentPath={selection?.selectedPath}
            dirty={dirtyState.isDirty}
            saving={saving}
            canSave={canSave}
            readOnly={readOnly}
            description={description}
            onDescriptionChange={setDescription}
            onSave={onSave}
            onRevert={onRevertBuffer}
            onDownload={onDownload}
          />

          {!selection && (
            <div className="rounded-xl border border-slate-200 bg-white/80 p-6 text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400">
              Выберите файл для просмотра.
            </div>
          )}

          {selection && currentFile && currentFile.mimeType.toLowerCase().startsWith("image/") && (
            <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
              <div className="mb-2 text-xs text-slate-500 dark:text-slate-400">
                {selection.selectedPath} · {currentFile.width || "?"}x{currentFile.height || "?"}
              </div>
              <img src={rawImageURL} alt={selection.selectedPath} className="max-h-[70vh] rounded-lg border border-slate-200 dark:border-slate-700" />
            </div>
          )}

          {selection && currentFile && currentFile.isEditable && !binaryPreview && !currentFile.mimeType.toLowerCase().startsWith("image/") && (
            <div className="rounded-xl border border-slate-200 bg-white/80 p-2 dark:border-slate-800 dark:bg-slate-900/60">
              <div className="mb-2 flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setPreviewMode("code")}
                  className={`rounded-lg border px-3 py-1 text-xs font-semibold ${previewMode === "code" ? "bg-indigo-600 text-white border-indigo-600" : "border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-100"}`}
                >
                  <FiCode className="inline mr-1" /> Code
                </button>
                <button
                  type="button"
                  onClick={() => setPreviewMode("preview")}
                  disabled={!canPreviewCurrentFile}
                  className={`rounded-lg border px-3 py-1 text-xs font-semibold ${previewMode === "preview" ? "bg-indigo-600 text-white border-indigo-600" : "border-slate-200 text-slate-700 dark:border-slate-700 dark:text-slate-100"}`}
                >
                  <FiEye className="inline mr-1" /> Preview
                </button>
                {previewMode === "preview" && canPreviewCurrentFile && (
                  <div className="ml-auto inline-flex rounded-lg border border-slate-200 p-1 dark:border-slate-700">
                    <button
                      type="button"
                      onClick={() => setPreviewSource("buffer")}
                      className={`rounded px-2 py-1 text-[11px] ${previewSource === "buffer" ? "bg-indigo-600 text-white" : "text-slate-600 dark:text-slate-300"}`}
                    >
                      Buffer preview
                    </button>
                    <button
                      type="button"
                      onClick={() => setPreviewSource("published")}
                      className={`rounded px-2 py-1 text-[11px] ${previewSource === "published" ? "bg-indigo-600 text-white" : "text-slate-600 dark:text-slate-300"}`}
                    >
                      Published preview
                    </button>
                  </div>
                )}
              </div>

              {previewMode === "code" && (
                <MonacoEditor
                  content={dirtyState.currentContent}
                  language={selection.language}
                  readOnly={readOnly || !currentFile.isEditable}
                  scrollLine={focusLine}
                  onChange={(value) => {
                    setDirtyState((prev) => ({
                      ...prev,
                      currentContent: value,
                      isDirty: value !== prev.originalContent,
                    }));
                  }}
                />
              )}

              {previewMode === "preview" && (
                <iframe
                  title="editor-preview"
                  sandbox="allow-same-origin allow-scripts"
                  srcDoc={previewSrcDoc}
                  className="h-[62vh] w-full rounded-lg border border-slate-200 dark:border-slate-700"
                />
              )}
            </div>
          )}

          {selection && currentFile && (!currentFile.isEditable || binaryPreview) && !currentFile.mimeType.toLowerCase().startsWith("image/") && (
            <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
              <div className="font-semibold">Файл недоступен для редактирования в текстовом редакторе</div>
              <div className="mt-1">Тип: {selection.mimeType || currentFile.mimeType || "unknown"}</div>
              <div className="mt-1">Размер: {currentFile.size} bytes</div>
            </div>
          )}

          <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => setAiStudioTab("edit")}
                className={`rounded-lg border px-3 py-1.5 text-xs font-semibold ${
                  aiStudioTab === "edit"
                    ? "border-indigo-600 bg-indigo-600 text-white"
                    : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                }`}
              >
                AI Studio: изменить текущий файл
              </button>
              <button
                type="button"
                onClick={() => setAiStudioTab("create")}
                className={`rounded-lg border px-3 py-1.5 text-xs font-semibold ${
                  aiStudioTab === "create"
                    ? "border-indigo-600 bg-indigo-600 text-white"
                    : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                }`}
              >
                AI Studio: создать страницу
              </button>
            </div>

            {aiStudioTab === "edit" && (
              <div className="space-y-3">
                <div className="grid gap-3 md:grid-cols-3">
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Модель
                    <select
                      value={aiModel}
                      onChange={(e) => setAiModel(e.target.value)}
                      className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                    >
                      {EDITOR_MODEL_OPTIONS.map((item) => (
                        <option key={`ai-suggest-model-${item.value || "default"}`} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Режим контекста
                    <select
                      value={aiContextMode}
                      onChange={(e) => setAiContextMode(e.target.value as AIContextMode)}
                      className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                    >
                      {AI_CONTEXT_MODE_OPTIONS.map((item) => (
                        <option key={`ai-edit-context-mode-${item.value}`} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Текущий файл
                    <input
                      value={selection?.selectedPath || ""}
                      readOnly
                      className="mt-1 w-full rounded-lg border border-slate-200 bg-slate-100 px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                    />
                  </label>
                </div>

                {aiContextMode !== "auto" && (
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Контекстные файлы (Ctrl/Cmd для мультивыбора)
                    <select
                      multiple
                      value={aiContextSelectedFiles}
                      onChange={(e) =>
                        setAiContextSelectedFiles(
                          Array.from(e.currentTarget.selectedOptions)
                            .map((item) => item.value)
                            .slice(0, 20)
                        )
                      }
                      className="mt-1 h-28 w-full rounded-lg border border-slate-200 bg-white px-2 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                    >
                      {selectableContextFiles.map((pathValue) => (
                        <option key={`ctx-edit-${pathValue}`} value={pathValue}>
                          {pathValue}
                        </option>
                      ))}
                    </select>
                  </label>
                )}

                <label className="block text-xs text-slate-500 dark:text-slate-400">
                  Что нужно изменить
                  <textarea
                    value={aiInstruction}
                    onChange={(e) => setAiInstruction(e.target.value)}
                    placeholder="Например: сделай эту страницу в стиле главной и сохрани шведский язык"
                    rows={3}
                    className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800"
                  />
                </label>

                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    onClick={onAISuggest}
                    disabled={aiBusy || !selection?.selectedPath || !aiInstruction.trim()}
                    className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white disabled:opacity-50"
                  >
                    <FiWind /> Сгенерировать предложение
                  </button>
                  <button
                    type="button"
                    onClick={onApplyAISuggest}
                    disabled={!aiOutput || aiApplyPathMismatch}
                    className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    Применить в буфер
                  </button>
                  <button
                    type="button"
                    onClick={onAISuggestContextPreview}
                    disabled={aiSuggestContextBusy || !selection?.selectedPath}
                    className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    Контекст запроса
                  </button>
                </div>

                {aiApplyPathMismatch && (
                  <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
                    Предложение относится к файлу <span className="font-semibold">{aiOutputSourcePath}</span>, сейчас открыт{" "}
                    <span className="font-semibold">{selection?.selectedPath}</span>.
                    <button
                      type="button"
                      onClick={() => void onOpenAISourceFile()}
                      className="ml-2 inline-flex rounded border border-amber-300 bg-white px-2 py-0.5 font-semibold text-amber-800 dark:border-amber-700 dark:bg-amber-900/20 dark:text-amber-200"
                    >
                      Открыть исходный файл
                    </button>
                  </div>
                )}

                {aiSuggestMeta?.contextPack && (
                  <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
                    Контекст: файлов {aiSuggestMeta.contextPack.files_used}, объем {aiSuggestMeta.contextPack.bytes_used} байт
                    {aiSuggestMeta.contextPack.truncated ? ", контекст урезан по лимитам" : ""}
                  </div>
                )}

                {aiOutput && (
                  <>
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => setAiSuggestView("diff")}
                        className={`rounded-lg border px-2 py-1 text-[11px] font-semibold ${
                          aiSuggestView === "diff"
                            ? "border-indigo-600 bg-indigo-600 text-white"
                            : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        }`}
                      >
                        Diff
                      </button>
                      <button
                        type="button"
                        onClick={() => setAiSuggestView("content")}
                        className={`rounded-lg border px-2 py-1 text-[11px] font-semibold ${
                          aiSuggestView === "content"
                            ? "border-indigo-600 bg-indigo-600 text-white"
                            : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        }`}
                      >
                        Код
                      </button>
                    </div>
                    {aiSuggestView === "diff" ? (
                      <MonacoDiffEditor
                        original={dirtyState.currentContent}
                        modified={aiOutput}
                        language={selection?.language || "plaintext"}
                      />
                    ) : (
                      <textarea
                        value={aiOutput}
                        onChange={(e) => setAiOutput(e.target.value)}
                        rows={10}
                        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                      />
                    )}
                  </>
                )}

                <details
                  open={aiSuggestDiagnosticsOpen}
                  onToggle={(event) => setAiSuggestDiagnosticsOpen((event.currentTarget as HTMLDetailsElement).open)}
                  className="rounded-lg border border-slate-200 bg-slate-50/70 px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800/50"
                >
                  <summary className="cursor-pointer font-semibold text-slate-700 dark:text-slate-200">Диагностика</summary>
                  <div className="mt-2 space-y-1 text-slate-600 dark:text-slate-300">
                    <div>Источник промпта: {aiSuggestMeta?.source || "unknown"}</div>
                    <div>Предупреждений: {aiSuggestMeta?.warnings?.length || 0}</div>
                    <div>Token usage: {aiSuggestMeta?.tokenUsage ? JSON.stringify(aiSuggestMeta.tokenUsage) : "n/a"}</div>
                    {aiSuggestMeta?.warnings?.length ? <div>Warnings: {aiSuggestMeta.warnings.join(" | ")}</div> : null}
                    {aiSuggestContextDebug ? (
                      <textarea
                        readOnly
                        value={aiSuggestContextDebug}
                        rows={8}
                        className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-2 py-2 font-mono text-[11px] dark:border-slate-700 dark:bg-slate-900/60"
                      />
                    ) : null}
                  </div>
                </details>
              </div>
            )}

            {aiStudioTab === "create" && (
              <div className="space-y-3">
                <div className="grid gap-3 md:grid-cols-3">
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Модель
                    <select
                      value={aiCreateModel}
                      onChange={(e) => setAiCreateModel(e.target.value)}
                      className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                    >
                      {EDITOR_MODEL_OPTIONS.map((item) => (
                        <option key={`ai-create-model-${item.value || "default"}`} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Режим контекста
                    <select
                      value={aiCreateContextMode}
                      onChange={(e) => setAiCreateContextMode(e.target.value as AIContextMode)}
                      className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                    >
                      {AI_CONTEXT_MODE_OPTIONS.map((item) => (
                        <option key={`ai-create-context-mode-${item.value}`} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Целевой путь
                    <input
                      value={aiCreatePath}
                      onChange={(e) => setAiCreatePath(e.target.value)}
                      className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                      placeholder="about.html"
                    />
                  </label>
                </div>

                {aiCreateContextMode !== "auto" && (
                  <label className="block text-xs text-slate-500 dark:text-slate-400">
                    Контекстные файлы (Ctrl/Cmd для мультивыбора)
                    <select
                      multiple
                      value={aiCreateContextSelectedFiles}
                      onChange={(e) =>
                        setAiCreateContextSelectedFiles(
                          Array.from(e.currentTarget.selectedOptions)
                            .map((item) => item.value)
                            .slice(0, 20)
                        )
                      }
                      className="mt-1 h-28 w-full rounded-lg border border-slate-200 bg-white px-2 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                    >
                      {selectableContextFiles.map((pathValue) => (
                        <option key={`ctx-create-${pathValue}`} value={pathValue}>
                          {pathValue}
                        </option>
                      ))}
                    </select>
                  </label>
                )}

                <label className="block text-xs text-slate-500 dark:text-slate-400">
                  Что нужно создать
                  <textarea
                    value={aiCreateInstruction}
                    onChange={(e) => setAiCreateInstruction(e.target.value)}
                    placeholder="Например: создай страницу about на языке сайта и в стиле главной"
                    rows={3}
                    className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800"
                  />
                </label>

                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    onClick={onAICreatePage}
                    disabled={aiCreateBusy || !aiCreateInstruction.trim() || !aiCreatePath.trim()}
                    className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white disabled:opacity-50"
                  >
                    <FiWind /> Сгенерировать пакет файлов
                  </button>
                  <button
                    type="button"
                    onClick={onApplyCreatedFiles}
                    disabled={aiCreateFiles.length === 0}
                    className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    Применить план
                  </button>
                  <button
                    type="button"
                    onClick={onAICreateContextPreview}
                    disabled={aiCreateContextBusy || !aiCreatePath.trim()}
                    className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    Контекст запроса
                  </button>
                </div>

                {aiCreateMeta?.contextPack && (
                  <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
                    Контекст: файлов {aiCreateMeta.contextPack.files_used}, объем {aiCreateMeta.contextPack.bytes_used} байт
                    {aiCreateMeta.contextPack.truncated ? ", контекст урезан по лимитам" : ""}
                  </div>
                )}

                {unresolvedAssetIssues.length > 0 && (
                  <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300">
                    Нерешённые ассеты: {unresolvedAssetIssues.slice(0, 8).join(", ")}
                    {unresolvedAssetIssues.length > 8 ? ` (+${unresolvedAssetIssues.length - 8})` : ""}
                  </div>
                )}

                {aiCreateAssets.length > 0 && (
                  <div className="rounded-lg border border-slate-200 bg-slate-50/70 p-2 text-xs dark:border-slate-700 dark:bg-slate-800/50">
                    <div className="mb-1 font-semibold text-slate-700 dark:text-slate-200">
                      Манифест ассетов (изображения)
                    </div>
                    <div className="max-h-40 overflow-auto">
                      <table className="min-w-full text-left text-[11px]">
                        <thead className="text-slate-500 dark:text-slate-400">
                          <tr>
                            <th className="px-2 py-1">Путь</th>
                            <th className="px-2 py-1">Тип</th>
                            <th className="px-2 py-1">Alt</th>
                            <th className="px-2 py-1">Промпт</th>
                            <th className="px-2 py-1">Статус</th>
                            <th className="px-2 py-1">Действия</th>
                          </tr>
                        </thead>
                        <tbody>
                          {aiCreateAssets.map((asset) => {
                            const broken = unresolvedBrokenAssets.includes(asset.path);
                            const unresolved = unresolvedMissingAssets.includes(asset.path);
                            const skipped = aiCreateSkippedAssets.includes(asset.path);
                            const status = skipped
                              ? "пропущен"
                              : broken
                                ? "битый"
                                : unresolved
                                ? "требует файл"
                                : "готов";
                            return (
                              <tr key={`asset-${asset.path}`} className="border-t border-slate-200 dark:border-slate-700">
                                <td className="px-2 py-1 font-mono">{asset.path}</td>
                                <td className="px-2 py-1">{asset.mime_type}</td>
                                <td className="px-2 py-1">{asset.alt || "—"}</td>
                                <td className="px-2 py-1">
                                  <div className="line-clamp-2">{asset.prompt || "—"}</div>
                                </td>
                                <td className="px-2 py-1">
                                  <span
                                    className={`rounded px-1.5 py-0.5 text-[11px] ${
                                      skipped
                                        ? "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300"
                                        : broken
                                          ? "bg-rose-100 text-rose-800 dark:bg-rose-900/40 dark:text-rose-300"
                                          : unresolved
                                          ? "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                                          : "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300"
                                    }`}
                                  >
                                    {status}
                                  </span>
                                </td>
                                <td className="px-2 py-1">
                                  <div className="flex flex-wrap gap-1">
                                    <button
                                      type="button"
                                      onClick={() => onAssetUploadPick(asset.path)}
                                      disabled={readOnly}
                                      className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                                    >
                                      Upload
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => void onRegenerateAsset(asset.path)}
                                      disabled={Boolean(aiCreateAssetBusyPath) || !asset.prompt}
                                      className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                                    >
                                      {aiCreateAssetBusyPath === asset.path ? "Генерация..." : "Регенерировать"}
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => onToggleSkipAsset(asset.path)}
                                      className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                                    >
                                      {skipped ? "Вернуть" : "Skip"}
                                    </button>
                                    <button
                                      type="button"
                                      onClick={() => void onCopyAssetPrompt(asset.path)}
                                      disabled={!asset.prompt}
                                      className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                                    >
                                      Copy prompt
                                    </button>
                                  </div>
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                    <div className="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
                      Изображения из манифеста не применяются автоматически как бинарные файлы. Добавьте их через Upload или пометьте Skip.
                    </div>
                  </div>
                )}

                {unresolvedNonManifestAssets.length > 0 && (
                  <div className="rounded-lg border border-amber-200 bg-amber-50/70 p-2 text-xs dark:border-amber-900 dark:bg-amber-950/20">
                    <div className="mb-1 font-semibold text-amber-800 dark:text-amber-300">
                      Ссылки на файлы без манифеста ассетов
                    </div>
                    <div className="space-y-1">
                      {unresolvedNonManifestAssets.map((pathValue) => {
                        const skipped = aiCreateSkippedAssets.includes(pathValue);
                        return (
                          <div
                            key={`missing-ref-${pathValue}`}
                            className="flex flex-wrap items-center justify-between gap-2 rounded border border-amber-200 bg-white/70 px-2 py-1 dark:border-amber-900 dark:bg-slate-900/40"
                          >
                            <span className="font-mono text-[11px]">{pathValue}</span>
                            <div className="flex flex-wrap gap-1">
                              <button
                                type="button"
                                onClick={() => onAssetUploadPick(pathValue)}
                                disabled={readOnly}
                                className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                              >
                                Upload
                              </button>
                              <button
                                type="button"
                                onClick={() => onToggleSkipAsset(pathValue)}
                                className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                              >
                                {skipped ? "Вернуть" : "Skip"}
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                {aiCreateFiles.length > 0 && (
                  <>
                    <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-700">
                      <table className="min-w-full text-left text-xs">
                        <thead className="bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                          <tr>
                            <th className="px-2 py-2">Файл</th>
                            <th className="px-2 py-2">Статус</th>
                            <th className="px-2 py-2">Действие</th>
                          </tr>
                        </thead>
                        <tbody>
                          {aiCreateFiles.map((file) => {
                            const exists = existingFilesMap.has(file.path);
                            return (
                              <tr key={`plan-${file.path}`} className="border-t border-slate-200 dark:border-slate-700">
                                <td className="px-2 py-2">
                                  <button
                                    type="button"
                                    onClick={() => setAiCreatePreviewPath(file.path)}
                                    className={`truncate text-left ${
                                      aiCreatePreviewPath === file.path ? "font-semibold text-indigo-600 dark:text-indigo-300" : ""
                                    }`}
                                  >
                                    {file.path}
                                  </button>
                                  <div className="text-[11px] text-slate-500 dark:text-slate-400">{file.mime_type}</div>
                                </td>
                                <td className="px-2 py-2">
                                  {exists ? (
                                    <span className="rounded bg-amber-100 px-1.5 py-0.5 text-[11px] text-amber-800 dark:bg-amber-900/40 dark:text-amber-300">
                                      уже существует
                                    </span>
                                  ) : (
                                    <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-[11px] text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300">
                                      новый
                                    </span>
                                  )}
                                </td>
                                <td className="px-2 py-2">
                                  <select
                                    value={aiCreateApplyPlan[file.path] || (exists ? "skip" : "create")}
                                    onChange={(e) => onSetCreatePlan(file.path, e.target.value as AIPageApplyAction)}
                                    className="w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
                                  >
                                    <option value="create">create</option>
                                    <option value="overwrite">overwrite</option>
                                    <option value="skip">skip</option>
                                  </select>
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>

                    {aiCreatePreviewFile && (
                      <>
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={() => setAiCreateView("diff")}
                            className={`rounded-lg border px-2 py-1 text-[11px] font-semibold ${
                              aiCreateView === "diff"
                                ? "border-indigo-600 bg-indigo-600 text-white"
                                : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                            }`}
                          >
                            Diff
                          </button>
                          <button
                            type="button"
                            onClick={() => setAiCreateView("preview")}
                            className={`rounded-lg border px-2 py-1 text-[11px] font-semibold ${
                              aiCreateView === "preview"
                                ? "border-indigo-600 bg-indigo-600 text-white"
                                : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                            }`}
                          >
                            Preview
                          </button>
                          <button
                            type="button"
                            onClick={() => setAiCreateView("code")}
                            className={`rounded-lg border px-2 py-1 text-[11px] font-semibold ${
                              aiCreateView === "code"
                                ? "border-indigo-600 bg-indigo-600 text-white"
                                : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                            }`}
                          >
                            Код
                          </button>
                        </div>
                        {aiCreateView === "diff" && (
                          <MonacoDiffEditor
                            original={aiCreateExistingContent[aiCreatePreviewFile.path] || ""}
                            modified={aiCreatePreviewFile.content || ""}
                            language={detectLanguage(aiCreatePreviewFile.path)}
                          />
                        )}
                        {aiCreateView === "preview" &&
                          (aiCreatePreviewFile.path.toLowerCase().endsWith(".html") ? (
                            <iframe
                              title="ai-create-preview"
                              sandbox="allow-same-origin allow-scripts"
                              srcDoc={aiCreatePreviewSrcDoc}
                              className="h-[56vh] w-full rounded-lg border border-slate-200 dark:border-slate-700"
                            />
                          ) : (
                            <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-800/50 dark:text-slate-300">
                              Preview доступен только для HTML-файлов.
                            </div>
                          ))}
                        {aiCreateView === "code" && (
                          <textarea
                            readOnly
                            value={aiCreatePreviewFile.content || ""}
                            rows={12}
                            className="w-full rounded-lg border border-slate-200 bg-slate-50 px-2 py-2 font-mono text-[11px] dark:border-slate-700 dark:bg-slate-800"
                          />
                        )}
                      </>
                    )}
                  </>
                )}

                <details
                  open={aiCreateDiagnosticsOpen}
                  onToggle={(event) => setAiCreateDiagnosticsOpen((event.currentTarget as HTMLDetailsElement).open)}
                  className="rounded-lg border border-slate-200 bg-slate-50/70 px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800/50"
                >
                  <summary className="cursor-pointer font-semibold text-slate-700 dark:text-slate-200">Диагностика</summary>
                  <div className="mt-2 space-y-1 text-slate-600 dark:text-slate-300">
                    <div>Источник промпта: {aiCreateMeta?.source || "unknown"}</div>
                    <div>Предупреждений: {aiCreateMeta?.warnings?.length || 0}</div>
                    <div>Token usage: {aiCreateMeta?.tokenUsage ? JSON.stringify(aiCreateMeta.tokenUsage) : "n/a"}</div>
                    {aiCreateMeta?.warnings?.length ? <div>Warnings: {aiCreateMeta.warnings.join(" | ")}</div> : null}
                    {aiCreateContextDebug ? (
                      <textarea
                        readOnly
                        value={aiCreateContextDebug}
                        rows={8}
                        className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-2 py-2 font-mono text-[11px] dark:border-slate-700 dark:bg-slate-900/60"
                      />
                    ) : null}
                  </div>
                </details>
              </div>
            )}
          </div>

          <FileHistory
            domainId={domainId}
            fileId={selection?.selectedFileId}
            filePath={selection?.selectedPath}
            canWrite={!readOnly}
            refreshKey={historyRefreshKey}
            onReverted={async () => {
              setHistoryRefreshKey((value) => value + 1);
              const refreshed = await loadFiles();
              const selectedPath = selection?.selectedPath;
              if (selectedPath) {
                const file = refreshed.find((item) => item.path === selectedPath);
                if (file) {
                  await loadFile(file);
                }
              }
            }}
          />

          {readOnly && (
            <div className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-semibold text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
              <FiEye /> Режим просмотра: сохранение отключено для роли viewer
            </div>
          )}
          {!readOnly && (
            <div className="inline-flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs font-semibold text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300">
              <FiEdit3 /> Редактирование доступно
            </div>
          )}
        </section>
      </div>
      <ConflictResolutionModal
        open={Boolean(conflictState)}
        currentVersion={conflictState?.currentVersion || selection?.version || 1}
        updatedBy={conflictState?.updatedBy}
        updatedAt={conflictState?.updatedAt}
        busy={saving}
        onReload={() => void onConflictReload()}
        onOverwrite={() => void onConflictOverwrite()}
        onCancel={() => setConflictState(null)}
      />
    </div>
  );
}
