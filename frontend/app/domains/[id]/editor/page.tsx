"use client";

import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
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
  type AIEditorSuggestionDTO,
  type AIPageSuggestionDTO,
  aiCreatePage,
  aiSuggestFile,
  createFileOrDir,
  deleteFile,
  getFile,
  listFiles,
  moveFile,
  restoreFile,
  saveFile,
  uploadFile,
  type AIPageSuggestionFile,
  type FileListItem
} from "../../../../lib/fileApi";
import { showToast } from "../../../../lib/toastStore";
import { useAuthGuard } from "../../../../lib/useAuth";
import type { EditorDirtyState, EditorFileMeta, EditorSelectionState } from "../../../../types/editor";

type DomainSummaryResponse = {
  domain: {
    id: string;
    project_id: string;
    url: string;
    status: string;
  };
  project_name: string;
  my_role: "admin" | "owner" | "editor" | "viewer";
};

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
  const [aiInstruction, setAiInstruction] = useState("");
  const [aiOutput, setAiOutput] = useState("");
  const [aiBusy, setAiBusy] = useState(false);
  const [aiModel, setAiModel] = useState("");
  const [aiContextFiles, setAiContextFiles] = useState("");
  const [aiSuggestView, setAiSuggestView] = useState<"diff" | "content">("diff");
  const [aiSuggestMeta, setAiSuggestMeta] = useState<{
    source?: string;
    warnings: string[];
    tokenUsage?: Record<string, any>;
  } | null>(null);
  const [aiCreateInstruction, setAiCreateInstruction] = useState("");
  const [aiCreatePath, setAiCreatePath] = useState("new-page.html");
  const [aiCreateBusy, setAiCreateBusy] = useState(false);
  const [aiCreateModel, setAiCreateModel] = useState("");
  const [aiCreateFiles, setAiCreateFiles] = useState<AIPageSuggestionFile[]>([]);
  const [aiCreateSelectedPaths, setAiCreateSelectedPaths] = useState<string[]>([]);
  const [aiCreatePreviewPath, setAiCreatePreviewPath] = useState("");
  const [aiCreateMeta, setAiCreateMeta] = useState<{
    source?: string;
    warnings: string[];
    tokenUsage?: Record<string, any>;
  } | null>(null);
  const [conflictState, setConflictState] = useState<{
    currentVersion: number;
    currentHash?: string;
    updatedBy?: string;
    updatedAt?: string;
  } | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

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

  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirtyState.isDirty) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [dirtyState.isDirty]);

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
        { expectedVersion: selection.version, source: "manual" }
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
        { expectedVersion: conflictState.currentVersion, source: "manual" }
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

  const onAISuggest = async () => {
    if (!selection?.selectedPath || !aiInstruction.trim()) return;
    setAiBusy(true);
    setAiSuggestMeta(null);
    try {
      const contextFiles = aiContextFiles
        .split(",")
        .map((item) => item.trim())
        .filter((item) => item && item !== selection.selectedPath)
        .slice(0, 10);
      const result: AIEditorSuggestionDTO = await aiSuggestFile(domainId, selection.selectedPath, {
        instruction: aiInstruction.trim(),
        model: aiModel.trim() || undefined,
        context_files: contextFiles.length > 0 ? contextFiles : undefined,
      });
      setAiOutput(result.suggested_content || "");
      setAiSuggestView("diff");
      const promptSource =
        typeof result.prompt_trace?.resolved_source === "string" ? result.prompt_trace.resolved_source : undefined;
      setAiSuggestMeta({
        source: promptSource,
        warnings: Array.isArray(result.warnings) ? result.warnings : [],
        tokenUsage: result.token_usage,
      });
      showToast({ type: "success", title: "AI-предложение готово", message: selection.selectedPath });
    } catch (err: any) {
      showToast({ type: "error", title: "AI suggest error", message: err?.message || "unknown error" });
    } finally {
      setAiBusy(false);
    }
  };

  const onApplyAISuggest = () => {
    if (!aiOutput) return;
    setDirtyState((prev) => ({
      ...prev,
      currentContent: aiOutput,
      isDirty: aiOutput !== prev.originalContent,
    }));
  };

  const onAICreatePage = async () => {
    if (!aiCreateInstruction.trim() || !aiCreatePath.trim()) return;
    setAiCreateBusy(true);
    setAiCreateMeta(null);
    try {
      const result: AIPageSuggestionDTO = await aiCreatePage(domainId, {
        instruction: aiCreateInstruction.trim(),
        target_path: aiCreatePath.trim(),
        with_assets: true,
        model: aiCreateModel.trim() || undefined,
      });
      const files = result.files || [];
      setAiCreateFiles(files);
      setAiCreateSelectedPaths(files.map((item) => item.path));
      setAiCreatePreviewPath(files[0]?.path || "");
      const promptSource =
        typeof result.prompt_trace?.resolved_source === "string" ? result.prompt_trace.resolved_source : undefined;
      setAiCreateMeta({
        source: promptSource,
        warnings: Array.isArray(result.warnings) ? result.warnings : [],
        tokenUsage: result.token_usage,
      });
      showToast({ type: "success", title: "AI сгенерировал страницу", message: `${result.files?.length || 0} файлов` });
    } catch (err: any) {
      showToast({ type: "error", title: "AI create-page error", message: err?.message || "unknown error" });
    } finally {
      setAiCreateBusy(false);
    }
  };

  const onApplyCreatedFiles = async () => {
    if (aiCreateFiles.length === 0) return;
    const selectedFiles = aiCreateFiles.filter((file) => aiCreateSelectedPaths.includes(file.path));
    if (selectedFiles.length === 0) {
      showToast({ type: "error", title: "Нет выбранных файлов", message: "Отметьте хотя бы один файл для применения." });
      return;
    }
    const confirmed = window.confirm(`Применить ${selectedFiles.length} AI-файлов в проект?`);
    if (!confirmed) return;
    let saved = 0;
    for (const file of selectedFiles) {
      try {
        await createFileOrDir(domainId, {
          kind: "file",
          path: file.path,
          content: file.content,
          mime_type: file.mime_type,
        });
        saved += 1;
      } catch (err: any) {
        if (String(err?.message || "").startsWith("409") || String(err?.message || "").includes("exists")) {
          await saveFile(domainId, file.path, file.content, "ai create-page overwrite", { source: "ai" });
          saved += 1;
        }
      }
    }
    await loadFiles();
    showToast({ type: "success", title: "AI-файлы применены", message: `${saved}/${selectedFiles.length}` });
  };

  const onToggleCreatedFile = (path: string, checked: boolean) => {
    setAiCreateSelectedPaths((prev) => {
      if (checked) {
        if (prev.includes(path)) return prev;
        return [...prev, path];
      }
      return prev.filter((item) => item !== path);
    });
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
          </div>
          <h2 className="mb-1 text-sm font-semibold">Файлы сайта</h2>
          <FileTree
            files={files}
            selectedPath={selection?.selectedPath}
            loading={fileLoading}
            onSelect={onSelectFile}
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

          <div className="grid gap-3 xl:grid-cols-2">
            <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
              <h3 className="mb-2 text-sm font-semibold">AI: редактирование файла</h3>
              <input
                value={aiModel}
                onChange={(e) => setAiModel(e.target.value)}
                placeholder="Модель (опционально), например: gemini-2.5-pro"
                className="mb-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
              />
              <input
                value={aiContextFiles}
                onChange={(e) => setAiContextFiles(e.target.value)}
                placeholder="Контекст-файлы через запятую, например: style.css,script.js"
                className="mb-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
              />
              <textarea
                value={aiInstruction}
                onChange={(e) => setAiInstruction(e.target.value)}
                placeholder="Что нужно изменить в файле?"
                rows={3}
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
              />
              <div className="mt-2 flex items-center gap-2">
                <button
                  type="button"
                  onClick={onAISuggest}
                  disabled={aiBusy || !selection?.selectedPath || !aiInstruction.trim()}
                  className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1 text-xs font-semibold text-white disabled:opacity-50"
                >
                  <FiWind /> Suggest
                </button>
                <button
                  type="button"
                  onClick={onApplyAISuggest}
                  disabled={!aiOutput}
                  className="rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  Apply to buffer
                </button>
              </div>
              {aiOutput && (
                <>
                  <div className="mb-2 mt-2 flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setAiSuggestView("diff")}
                      className={`rounded-lg border px-2 py-1 text-[11px] font-semibold ${
                        aiSuggestView === "diff"
                          ? "border-indigo-600 bg-indigo-600 text-white"
                          : "border-slate-200 bg-white text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      }`}
                    >
                      View diff
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
                      View content
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
                    rows={8}
                    className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                  />
                  )}
                  <div className="mt-2 rounded-lg border border-slate-200 bg-slate-50 px-2 py-2 text-[11px] text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
                    <div>Prompt source: {aiSuggestMeta?.source || "unknown"}</div>
                    <div>Warnings: {aiSuggestMeta?.warnings?.length || 0}</div>
                    <div>Token usage: {aiSuggestMeta?.tokenUsage ? JSON.stringify(aiSuggestMeta.tokenUsage) : "n/a"}</div>
                  </div>
                </>
              )}
            </div>

            <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
              <h3 className="mb-2 text-sm font-semibold">AI: создать новую страницу</h3>
              <input
                value={aiCreateModel}
                onChange={(e) => setAiCreateModel(e.target.value)}
                className="mb-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                placeholder="Модель (опционально), например: gemini-2.5-pro"
              />
              <input
                value={aiCreatePath}
                onChange={(e) => setAiCreatePath(e.target.value)}
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
                placeholder="new-page.html"
              />
              <textarea
                value={aiCreateInstruction}
                onChange={(e) => setAiCreateInstruction(e.target.value)}
                placeholder="Опиши страницу, которую нужно создать"
                rows={3}
                className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-800"
              />
              <div className="mt-2 flex items-center gap-2">
                <button
                  type="button"
                  onClick={onAICreatePage}
                  disabled={aiCreateBusy || !aiCreateInstruction.trim() || !aiCreatePath.trim()}
                  className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1 text-xs font-semibold text-white disabled:opacity-50"
                >
                  <FiWind /> Generate files
                </button>
                <button
                  type="button"
                  onClick={onApplyCreatedFiles}
                  disabled={aiCreateFiles.length === 0}
                  className="rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  Apply all
                </button>
              </div>
              {aiCreateFiles.length > 0 && (
                <>
                  <div className="mt-2 flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setAiCreateSelectedPaths(aiCreateFiles.map((item) => item.path))}
                      className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-[11px] font-semibold text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    >
                      Select all
                    </button>
                    <button
                      type="button"
                      onClick={() => setAiCreateSelectedPaths([])}
                      className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-[11px] font-semibold text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    >
                      Clear
                    </button>
                    <span className="text-[11px] text-slate-500 dark:text-slate-400">
                      Выбрано: {aiCreateSelectedPaths.length}/{aiCreateFiles.length}
                    </span>
                  </div>
                  <div className="mt-2 max-h-44 space-y-1 overflow-auto rounded-lg border border-slate-200 p-2 text-xs dark:border-slate-700">
                    {aiCreateFiles.map((file) => (
                      <label key={file.path} className="flex cursor-pointer items-center gap-2 rounded bg-slate-100 px-2 py-1 dark:bg-slate-800">
                        <input
                          type="checkbox"
                          checked={aiCreateSelectedPaths.includes(file.path)}
                          onChange={(e) => onToggleCreatedFile(file.path, e.currentTarget.checked)}
                          className="h-3.5 w-3.5 rounded border-slate-300"
                        />
                        <button
                          type="button"
                          onClick={() => setAiCreatePreviewPath(file.path)}
                          className={`truncate text-left ${aiCreatePreviewPath === file.path ? "font-semibold text-indigo-600 dark:text-indigo-300" : ""}`}
                        >
                          {file.path} · {file.mime_type}
                        </button>
                      </label>
                    ))}
                  </div>
                  {aiCreatePreviewPath && (
                    <div className="mt-2 rounded-lg border border-slate-200 bg-white p-2 dark:border-slate-700 dark:bg-slate-900/60">
                      <div className="mb-1 truncate text-[11px] text-slate-500 dark:text-slate-400">
                        Preview: {aiCreatePreviewPath}
                      </div>
                      <textarea
                        value={aiCreateFiles.find((item) => item.path === aiCreatePreviewPath)?.content || ""}
                        readOnly
                        rows={8}
                        className="w-full rounded-lg border border-slate-200 bg-slate-50 px-2 py-2 font-mono text-[11px] dark:border-slate-700 dark:bg-slate-800"
                      />
                    </div>
                  )}
                </>
              )}
              {aiCreateMeta && (
                <div className="mt-2 rounded-lg border border-slate-200 bg-slate-50 px-2 py-2 text-[11px] text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
                  <div>Prompt source: {aiCreateMeta.source || "unknown"}</div>
                  <div>Warnings: {aiCreateMeta.warnings.length}</div>
                  <div>Token usage: {aiCreateMeta.tokenUsage ? JSON.stringify(aiCreateMeta.tokenUsage) : "n/a"}</div>
                </div>
              )}
            </div>
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
