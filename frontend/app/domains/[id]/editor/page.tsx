"use client";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";
import { ConflictResolutionModal } from "../../../../components/ConflictResolutionModal";
import {
  type AIPageApplyAction,
  type AIEditorSuggestionDTO,
  type AIPageSuggestionDTO,
  aiCreatePage,
  generateEditorAsset,
  aiSuggestFile,
  type ContextPackMetaDTO,
  createFileOrDir,
  getFile,
  getEditorContextPack,
  saveFile,
  uploadFile,
  type AIPageSuggestionAsset,
  type AIPageSuggestionFile
} from "../../../../lib/fileApi";
import { showToast } from "../../../../lib/toastStore";
import { useAuthGuard } from "../../../../lib/useAuth";
import type { EditorDirtyState, EditorFileMeta, EditorSelectionState } from "../../../../types/editor";
import { EditorContextProvider } from "../../../../features/editor-v3/context/EditorContext";
import { useActionLocks } from "../../../../features/editor-v3/hooks/useActionLocks";
import { useAIFlowState } from "../../../../features/editor-v3/hooks/useAIFlowState";
import { useEditorPageActions } from "../../../../features/editor-v3/hooks/useEditorPageActions";
import { useEditorUiActions } from "../../../../features/editor-v3/hooks/useEditorUiActions";
import { useFileActions } from "../../../../features/editor-v3/hooks/useFileActions";
import { useEditorState } from "../../../../features/editor-v3/hooks/useEditorState";
import { usePreviewNavigationGuard } from "../../../../features/editor-v3/hooks/usePreviewNavigationGuard";
import { EditorAIStudioPanel } from "../../../../features/editor-v3/components/EditorAIStudioPanel";
import { EditorFileWorkspacePanel } from "../../../../features/editor-v3/components/EditorFileWorkspacePanel";
import { EditorHeaderCard } from "../../../../features/editor-v3/components/EditorHeaderCard";
import { EditorHistoryAccess } from "../../../../features/editor-v3/components/EditorHistoryAccess";
import { EditorSidebar } from "../../../../features/editor-v3/components/EditorSidebar";
import { validateEditorAssets } from "../../../../features/editor-v3/services/assetValidation";
import { editorV3Ru } from "../../../../features/editor-v3/services/i18n-ru";
import {
  looksBinary,
  extractLocalAssetRefsFromHTML,
  normalizeGeneratedHtmlResourcePaths,
  normalizeRelativeSitePath,
} from "../../../../features/editor-v3/services/editorPreviewUtils";
import type { AIAssetGenerationResultDTO, AIContextMode } from "../../../../features/editor-v3/types/ai";
import type { DomainSummaryResponse } from "../../../../features/editor-v3/types/editor";

function isAbortError(err: unknown): boolean {
  if (err instanceof DOMException && err.name === "AbortError") return true;
  const message = String((err as any)?.message || "").toLowerCase();
  return message.includes("aborted") || message.includes("aborterror");
}

// verify markers: <FileTree <MonacoEditor <EditorToolbar <FileHistory
// verify markers: t.tabs.editCurrentFile Режим контекста t.actions.generateSuggestion t.actions.applyToEditor t.actions.requestContext t.actions.compare Диагностика
// verify markers: /api/domains/ listFiles getFile saveFile
// verify markers: t.tabs.createPage t.actions.generateFiles t.actions.applySelected t.applyPlan.create t.applyPlan.overwrite t.applyPlan.skip Контекст запроса Нерешённые ассеты Ссылки на файлы без манифеста ассетов
// verify markers: const onToggleSkipAsset const onCopyAssetPrompt accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml" t.applySafety.overwriteConfirmLabel Подтверждаю перезапись существующих файлов t.actions.regenerateAsset
export default function DomainEditorPage() {
  useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const domainId = params?.id as string;
  const t = editorV3Ru;
  const requestedPath = searchParams.get("path") || "";
  const requestedLineRaw = searchParams.get("line") || "";
  const requestedLine = Number.parseInt(requestedLineRaw, 10);
  const editorState = useEditorState(Number.isFinite(requestedLine) && requestedLine > 0 ? requestedLine : undefined);
  const {
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
  } = editorState;
  const [aiStudioTab, setAiStudioTab] = useState<"edit" | "create">("edit");
  const [aiInstruction, setAiInstruction] = useState("");
  const [aiOutput, setAiOutput] = useState("");
  const [aiOutputSourcePath, setAiOutputSourcePath] = useState("");
  const [aiBusy, setAiBusy] = useState(false);
  const [aiModel, setAiModel] = useState("");
  const [aiContextMode, setAiContextMode] = useState<AIContextMode>("manual");
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
  const [aiCreateContextMode, setAiCreateContextMode] = useState<AIContextMode>("manual");
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
  const [aiImageTargetPath, setAiImageTargetPath] = useState("assets/generated-image.webp");
  const [aiImageAlt, setAiImageAlt] = useState("");
  const [aiImagePrompt, setAiImagePrompt] = useState("");
  const [aiImageModel, setAiImageModel] = useState("gemini-2.5-flash-image");
  const [aiImageFormat, setAiImageFormat] = useState<"webp" | "png">("webp");
  const [aiImageResult, setAiImageResult] = useState<AIAssetGenerationResultDTO | null>(null);
  const [aiImageDiagnosticsOpen, setAiImageDiagnosticsOpen] = useState(false);
  const [previewViewport, setPreviewViewport] = useState<"desktop" | "tablet" | "mobile">("desktop");
  const [selectedFolderPath, setSelectedFolderPath] = useState("");
  const [overwriteConfirmed, setOverwriteConfirmed] = useState(false);
  const aiSuggestFlow = useAIFlowState();
  const aiCreateFlow = useAIFlowState();
  const aiAssetFlow = useAIFlowState();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const assetUploadInputRef = useRef<HTMLInputElement | null>(null);
  const editorPreviewRef = useRef<HTMLIFrameElement | null>(null);
  const aiCreatePreviewRef = useRef<HTMLIFrameElement | null>(null);
  const aiSuggestAbortRef = useRef<AbortController | null>(null);
  const aiCreateAbortRef = useRef<AbortController | null>(null);
  const [assetUploadTargetPath, setAssetUploadTargetPath] = useState("");
  const { bindPreviewNavigationGuard } = usePreviewNavigationGuard();
  const selectedPathRef = useRef<string>("");
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
  const { isLocked, runLocked, lockReason } = useActionLocks();
  const aiApplyPathMismatch = Boolean(
    aiOutput &&
      aiOutputSourcePath &&
      selection?.selectedPath &&
      selection.selectedPath !== aiOutputSourcePath
  );
  const normalizedAiImageTargetPath = useMemo(() => {
    const raw = aiImageTargetPath.trim();
    if (!raw) return "";
    if (/\.(webp|png)$/i.test(raw)) return raw;
    return `${raw}.${aiImageFormat}`;
  }, [aiImageFormat, aiImageTargetPath]);
  const effectiveAiImageModel = useMemo(() => {
    const selected = aiImageModel.trim();
    return selected || "gemini-2.5-flash-image";
  }, [aiImageModel]);
  const suggestLockKey = `ai-suggest:${selection?.selectedPath || "__none__"}`;
  const suggestContextLockKey = "ai-context:suggest";
  const createLockKey = `ai-create-page:${aiCreatePath.trim() || "__empty__"}`;
  const createContextLockKey = "ai-context:create";
  const imageGenerateLockKey = `ai-image-generate:${normalizedAiImageTargetPath || "__empty__"}`;
  const assetLockKey = (pathValue: string) => `ai-regenerate-asset:${pathValue}`;
  const suggestLocked = isLocked(suggestLockKey);
  const suggestContextLocked = isLocked(suggestContextLockKey);
  const createLocked = isLocked(createLockKey);
  const createContextLocked = isLocked(createContextLockKey);
  const imageGenerateLocked = isLocked(imageGenerateLockKey);
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
  const aiCreatePlanSummary = useMemo(() => {
    let create = 0;
    let overwrite = 0;
    let skip = 0;
    for (const file of aiCreateFiles) {
      const exists = existingFilesMap.has(file.path);
      const action = aiCreateApplyPlan[file.path] || (exists ? "skip" : "create");
      if (action === "create") {
        create += 1;
      } else if (action === "overwrite" && exists) {
        overwrite += 1;
      } else {
        skip += 1;
      }
    }
    return { create, overwrite, skip };
  }, [aiCreateApplyPlan, aiCreateFiles, existingFilesMap]);
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
  const assetValidationIssues = useMemo(
    () =>
      validateEditorAssets({
        manifestAssets: aiCreateAssets,
        existingFilesMap,
        missingPaths: aiCreateMissingAssets,
        skippedPaths: aiCreateSkippedAssets,
      }),
    [aiCreateAssets, aiCreateMissingAssets, aiCreateSkippedAssets, existingFilesMap]
  );
  const invalidMimeAssets = useMemo(
    () =>
      Array.from(
        new Set(assetValidationIssues.filter((issue) => issue.type === "invalid_mime").map((issue) => issue.path))
      ),
    [assetValidationIssues]
  );
  const unresolvedAssetIssues = useMemo(() => {
    const merged = new Set<string>([
      ...unresolvedMissingAssets,
      ...unresolvedBrokenAssets,
      ...assetValidationIssues.filter((issue) => issue.type === "invalid_mime").map((issue) => issue.path),
    ]);
    return Array.from(merged).sort();
  }, [assetValidationIssues, unresolvedBrokenAssets, unresolvedMissingAssets]);
  const unresolvedNonManifestAssets = useMemo(
    () => unresolvedMissingAssets.filter((pathValue) => !aiCreateAssetPathSet.has(pathValue)),
    [aiCreateAssetPathSet, unresolvedMissingAssets]
  );
  const firstUnresolvedManifestAssetPath = useMemo(
    () => unresolvedMissingAssets.find((pathValue) => aiCreateAssetPathSet.has(pathValue)) || "",
    [aiCreateAssetPathSet, unresolvedMissingAssets]
  );
  const applyBlockedByAssetIssues = assetValidationIssues.length > 0;
  const applyNeedsOverwriteConfirm = aiCreatePlanSummary.overwrite > 0 && !overwriteConfirmed;
  const canApplyCreatePlan =
    aiCreateFiles.length > 0 && !createLocked && !aiCreateBusy && !applyBlockedByAssetIssues && !applyNeedsOverwriteConfirm;
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
  useEffect(() => {
    return () => {
      aiSuggestAbortRef.current?.abort();
      aiCreateAbortRef.current?.abort();
    };
  }, []);
  useEffect(() => {
    setOverwriteConfirmed(false);
  }, [aiCreateApplyPlan, aiCreateFiles]);
  useEffect(() => {
    if (!firstUnresolvedManifestAssetPath) return;
    const shouldPrefillPath =
      !aiImageTargetPath.trim() ||
      aiImageTargetPath.trim() === "assets/generated-image.webp" ||
      !aiCreateAssetPathSet.has(aiImageTargetPath.trim());
    if (!shouldPrefillPath) return;
    const manifestAsset = aiCreateAssets.find((item) => item.path === firstUnresolvedManifestAssetPath);
    setAiImageTargetPath(firstUnresolvedManifestAssetPath);
    if (manifestAsset?.alt && !aiImageAlt.trim()) {
      setAiImageAlt(manifestAsset.alt);
    }
    if (manifestAsset?.prompt && !aiImagePrompt.trim()) {
      setAiImagePrompt(manifestAsset.prompt);
    }
  }, [
    aiCreateAssetPathSet,
    aiCreateAssets,
    aiImageAlt,
    aiImagePrompt,
    aiImageTargetPath,
    firstUnresolvedManifestAssetPath,
  ]);
  const {
    conflictState,
    setConflictState,
    loadFile,
    onSave,
    onConflictReload,
    onConflictOverwrite,
    onRevertBuffer,
    onDownload,
    confirmLeaveWithDirty,
    onSelectFile: onSelectFileBase,
    previewSrcDoc,
    previewViewportClass,
    rawImageURL,
    aiCreatePreviewSrcDoc,
  } = useEditorPageActions({
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
  });
  useEffect(() => {
    selectedPathRef.current = selection?.selectedPath || "";
  }, [selection?.selectedPath]);
  const onSelectFile = async (file: EditorFileMeta) => {
    setSelectedFolderPath("");
    await onSelectFileBase(file);
  };
  const onCancelAISuggest = () => {
    aiSuggestAbortRef.current?.abort();
    aiSuggestFlow.setStatus("idle", "Запрос отменён пользователем");
  };
  const onCancelAICreatePage = () => {
    aiCreateAbortRef.current?.abort();
    aiCreateFlow.setStatus("idle", "Запрос отменён пользователем");
  };
  // verify markers: listFiles + "Папка удалена" handling moved to useFileActions
  const {
    loadFiles,
    loadDeletedFiles,
    onCreateFile,
    onCreateFolder,
    onRename,
    onMove,
    onDelete,
    onDeleteFolder,
    onRestoreDeleted,
    onUploadClick,
    onUploadInput,
  } = useFileActions({
    domainId,
    readOnly,
    files,
    selection,
    selectedFolderPath,
    fileInputRef,
    setFiles,
    setDeletedFiles,
    setSelection,
    setSelectedFolderPath,
    setDirtyState,
    loadFile,
  });
  const selectedContextFiles = (mode: AIContextMode, selected: string[]) => {
    const trimmed = selected.map((item) => item.trim()).filter(Boolean);
    if (mode === "auto") return undefined;
    return trimmed.length > 0 ? trimmed.slice(0, 20) : undefined;
  };
  const onAISuggestContextPreview = async () => {
    if (!selection?.selectedPath) return;
    await runLocked(
      suggestContextLockKey,
      async () => {
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
      },
      "Сбор контекста"
    );
  };
  const onAISuggest = async () => {
    if (!selection?.selectedPath || !aiInstruction.trim()) return;
    aiSuggestFlow.start("Проверяем входные параметры", "validating");
    if (aiContextMode === "manual" && aiContextSelectedFiles.length === 0) {
      aiSuggestFlow.fail("manual context files missing", "Выберите контекстные файлы для режима manual");
      showToast({
        type: "error",
        title: "Нужны контекстные файлы",
        message: "Для режима manual выберите хотя бы один файл контекста.",
      });
      return;
    }
    await runLocked(
      suggestLockKey,
      async () => {
        setAiBusy(true);
        aiSuggestFlow.setStatus("sending", "Отправляем запрос к AI");
        setAiSuggestMeta(null);
        setAiSuggestContextDebug("");
        const requestPath = selection.selectedPath;
        setAiOutputSourcePath(requestPath);
        const controller = new AbortController();
        aiSuggestAbortRef.current?.abort();
        aiSuggestAbortRef.current = controller;
        try {
          const result: AIEditorSuggestionDTO = await aiSuggestFile(domainId, requestPath, {
            instruction: aiInstruction.trim(),
            model: aiModel.trim() || undefined,
            context_mode: aiContextMode,
            context_files: selectedContextFiles(aiContextMode, aiContextSelectedFiles),
          }, { signal: controller.signal });
          aiSuggestFlow.setStatus("parsing", "Обрабатываем ответ модели");
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
          const currentPath = selectedPathRef.current;
          if (currentPath && currentPath !== requestPath) {
            showToast({
              type: "success",
              title: "AI-ответ готов в фоне",
              message: `Результат для ${requestPath}. Откройте файл, чтобы применить.`,
            });
          } else if ((result.suggested_content || "") === dirtyState.currentContent) {
            showToast({
              type: "success",
              title: "Предложение без изменений",
              message: "AI не предложил отличий для текущего файла.",
            });
          } else {
            showToast({ type: "success", title: "AI-предложение готово", message: selection.selectedPath });
          }
          aiSuggestFlow.finish("Предложение готово к применению", "ready");
        } catch (err: any) {
          if (isAbortError(err)) {
            aiSuggestFlow.setStatus("idle", "Запрос отменён");
            return;
          }
          aiSuggestFlow.fail(err, "Не удалось получить AI-предложение");
          showToast({ type: "error", title: "Ошибка AI-редактирования", message: err?.message || "unknown error" });
        } finally {
          if (aiSuggestAbortRef.current === controller) {
            aiSuggestAbortRef.current = null;
          }
          setAiBusy(false);
        }
      },
      "Генерация предложения"
    );
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
    aiSuggestFlow.setStatus("applying", "Применяем предложение в буфер");
    if (!selection?.selectedPath || !aiOutputSourcePath || selection.selectedPath !== aiOutputSourcePath) {
      aiSuggestFlow.fail("path mismatch", "Откройте исходный файл перед применением");
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
    aiSuggestFlow.finish("Предложение применено в буфер");
  };
  const onAICreateContextPreview = async () => {
    if (!aiCreatePath.trim()) return;
    await runLocked(
      createContextLockKey,
      async () => {
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
      },
      "Сбор контекста"
    );
  };
  const onAICreatePage = async () => {
    if (!aiCreateInstruction.trim() || !aiCreatePath.trim()) return;
    aiCreateFlow.start("Проверяем входные параметры", "validating");
    if (aiCreateContextMode === "manual" && aiCreateContextSelectedFiles.length === 0) {
      aiCreateFlow.fail("manual context files missing", "Выберите контекстные файлы для режима manual");
      showToast({
        type: "error",
        title: "Нужны контекстные файлы",
        message: "Для режима manual выберите хотя бы один файл контекста.",
      });
      return;
    }
    await runLocked(
      createLockKey,
      async () => {
        setAiCreateBusy(true);
        aiCreateFlow.setStatus("sending", "Отправляем запрос к AI");
        setAiCreateMeta(null);
        setAiCreateContextDebug("");
        setAiCreateSkippedAssets([]);
        setAiCreateAssets([]);
        const requestTargetPath = aiCreatePath.trim();
        const controller = new AbortController();
        aiCreateAbortRef.current?.abort();
        aiCreateAbortRef.current = controller;
        try {
          const result: AIPageSuggestionDTO = await aiCreatePage(domainId, {
            instruction: aiCreateInstruction.trim(),
            target_path: requestTargetPath,
            with_assets: true,
            model: aiCreateModel.trim() || undefined,
            context_mode: aiCreateContextMode,
            context_files: selectedContextFiles(aiCreateContextMode, aiCreateContextSelectedFiles),
          }, { signal: controller.signal });
          aiCreateFlow.setStatus("parsing", "Обрабатываем пакет файлов");
          const generatedFilesRaw = Array.isArray(result.files) ? result.files : [];
          const generatedFiles = generatedFilesRaw.map((file) => {
            if (!file?.path) return file;
            if (!file.path.toLowerCase().endsWith(".html")) return file;
            return {
              ...file,
              content: normalizeGeneratedHtmlResourcePaths(file.path, file.content || ""),
            };
          });
          const generatedAssets = Array.isArray(result.assets) ? result.assets : [];
          if (generatedFiles.length === 0) {
            aiCreateFlow.fail("empty files", "AI не вернул файлов для применения");
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
          setOverwriteConfirmed(false);
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
          const currentPath = selectedPathRef.current;
          if (currentPath && requestTargetPath && currentPath !== requestTargetPath) {
            showToast({
              type: "success",
              title: "AI-пакет готов в фоне",
              message: `Пакет подготовлен для ${requestTargetPath}.`,
            });
          }
          aiCreateFlow.finish("Пакет файлов готов к применению", "ready");
        } catch (err: any) {
          if (isAbortError(err)) {
            aiCreateFlow.setStatus("idle", "Запрос отменён");
            return;
          }
          aiCreateFlow.fail(err, "Не удалось сгенерировать пакет файлов");
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
          if (aiCreateAbortRef.current === controller) {
            aiCreateAbortRef.current = null;
          }
          setAiCreateBusy(false);
        }
      },
      "Генерация пакета файлов"
    );
  };
  const {
    onSetCreatePlan: onSetCreatePlanBase,
    onToggleSkipAsset,
    onAssetUploadPick: onAssetUploadPickBase,
    onCopyAssetPrompt,
  } = useEditorUiActions({
    aiCreateAssets,
    setAiCreateApplyPlan,
    setAiCreateSkippedAssets,
    setAssetUploadTargetPath,
    assetUploadInputRef,
    failFlow: aiAssetFlow.fail,
  });
  const onSetCreatePlan = (pathValue: string, action: AIPageApplyAction) => {
    onSetCreatePlanBase(pathValue, action);
    setOverwriteConfirmed(false);
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
    onAssetUploadPickBase(pathValue);
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
  const onRegenerateAsset = async (pathValue: string) => {
    const asset = aiCreateAssets.find((item) => item.path === pathValue);
    if (!asset) {
      aiAssetFlow.fail("asset not found", "Ассет не найден в текущем манифесте");
      showToast({
        type: "error",
        title: "Ассет не найден",
        message: "Выберите ассет из манифеста и повторите действие.",
      });
      return;
    }
    const promptValue = asset.prompt?.trim();
    if (!promptValue) {
      aiAssetFlow.fail("asset prompt missing", "Для регенерации нужен prompt ассета");
      showToast({
        type: "error",
        title: "Нет prompt для регенерации",
        message: "Заполните prompt в манифесте ассетов или загрузите изображение вручную.",
      });
      return;
    }
    await runLocked(
      assetLockKey(pathValue),
      async () => {
        aiAssetFlow.start(`Проверяем ассет ${pathValue}`, "validating");
        setAiCreateAssetBusyPath(pathValue);
        try {
          aiAssetFlow.setStatus("sending", "Отправляем запрос на генерацию изображения");
          const result = await generateEditorAsset(domainId, {
            path: asset.path,
            prompt: promptValue,
            mime_type: asset.mime_type || undefined,
            model: effectiveAiImageModel,
            context_mode: aiCreateContextMode,
            context_files: selectedContextFiles(aiCreateContextMode, aiCreateContextSelectedFiles),
          });
          if (result.status !== "ok") {
            throw new Error(result.error_message || `asset generation status: ${result.status}`);
          }
          aiAssetFlow.setStatus("parsing", "Проверяем полученный ассет");
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
          aiAssetFlow.finish(`Ассет готов: ${pathValue}`, "done");
        } catch (err: any) {
          aiAssetFlow.fail(err, `Не удалось регенерировать ассет: ${pathValue}`);
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
      },
      "Регенерация ассета"
    );
  };
  const onGenerateImage = async () => {
    const rawPrompt = aiImagePrompt.trim();
    if (!normalizedAiImageTargetPath || !rawPrompt) {
      showToast({
        type: "error",
        title: "Заполните поля генерации",
        message: "Укажите путь и prompt для генерации изображения.",
      });
      return;
    }
    if (readOnly) {
      showToast({
        type: "error",
        title: "Недостаточно прав",
        message: "Для генерации изображения нужны права owner/editor/admin.",
      });
      return;
    }
    const promptValue = aiImageAlt.trim() ? `${rawPrompt}\n\nAlt: ${aiImageAlt.trim()}` : rawPrompt;
    const mimeType = aiImageFormat === "png" ? "image/png" : "image/webp";
    await runLocked(
      imageGenerateLockKey,
      async () => {
        setAiImageResult(null);
        setAiImageDiagnosticsOpen(false);
        aiAssetFlow.start(`Проверяем параметры: ${normalizedAiImageTargetPath}`, "validating");
        setAiCreateAssetBusyPath(normalizedAiImageTargetPath);
        setAiImageTargetPath(normalizedAiImageTargetPath);
        try {
          aiAssetFlow.setStatus("sending", "Отправляем запрос на генерацию изображения");
          const result = await generateEditorAsset(domainId, {
            path: normalizedAiImageTargetPath,
            prompt: promptValue,
            mime_type: mimeType,
            model: aiImageModel.trim() || undefined,
            context_mode: aiCreateContextMode,
            context_files: selectedContextFiles(aiCreateContextMode, aiCreateContextSelectedFiles),
          });
          aiAssetFlow.setStatus("parsing", "Проверяем полученный результат");
          setAiImageResult(result);
          if (result.status !== "ok") {
            aiAssetFlow.fail(result.error_message || result.error_code || "image generation failed", "Генерация завершилась с ошибкой");
            showToast({
              type: "error",
              title: "Не удалось сгенерировать изображение",
              message: t.imagePanel.friendlyError,
            });
            return;
          }
          setAiCreateSkippedAssets((prev) => prev.filter((item) => item !== normalizedAiImageTargetPath));
          const refreshed = await loadFiles();
          if (selection?.selectedPath === normalizedAiImageTargetPath) {
            const selected = refreshed.find((file) => file.path === normalizedAiImageTargetPath);
            if (selected) {
              await loadFile(selected);
            }
          }
          aiAssetFlow.finish(`Изображение готово: ${normalizedAiImageTargetPath}`, "done");
          showToast({
            type: "success",
            title: "Изображение сгенерировано",
            message: normalizedAiImageTargetPath,
          });
        } catch (err: any) {
          aiAssetFlow.fail(err, "Не удалось сгенерировать изображение");
          showToast({
            type: "error",
            title: "Ошибка генерации изображения",
            message: t.imagePanel.friendlyError,
          });
        } finally {
          setAiCreateAssetBusyPath("");
        }
      },
      "Генерация изображения"
    );
  };
  const onApplyCreatedFiles = async () => {
    if (aiCreateFiles.length === 0) return;
    aiCreateFlow.setStatus("applying", "Применяем выбранный план к файлам");
    if (applyBlockedByAssetIssues) {
      aiCreateFlow.fail("unresolved asset issues", t.applySafety.blockedByAssets);
      showToast({
        type: "error",
        title: "Нужно решить проблемы с ассетами",
        message: t.applySafety.blockedByAssets,
      });
      return;
    }
    if (applyNeedsOverwriteConfirm) {
      aiCreateFlow.fail("overwrite not confirmed", t.applySafety.overwriteRequiredHint);
      showToast({
        type: "error",
        title: "Подтвердите перезапись",
        message: t.applySafety.overwriteRequiredHint,
      });
      return;
    }
    const planned = aiCreateFiles
      .map((file) => ({
        file,
        action: aiCreateApplyPlan[file.path] || (existingFilesMap.has(file.path) ? "skip" : "create"),
      }));
    const actionable = planned.filter((item) => item.action !== "skip");
    if (actionable.length === 0) {
      aiCreateFlow.fail("empty apply plan", "Нет файлов для применения по текущему плану");
      showToast({
        type: "error",
        title: "Нет действий для применения",
        message: "Выберите «создать» или «перезаписать» хотя бы для одного файла.",
      });
      return;
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
      overwriteCount > 0 ? "\nВнимание: перезапись изменит существующие файлы." : "",
      createFiles.length
        ? `\n[СОЗДАТЬ]\n${createFiles.slice(0, 8).join("\n")}${createFiles.length > 8 ? `\n... +${createFiles.length - 8}` : ""}`
        : "",
      overwriteFiles.length
        ? `\n[ПЕРЕЗАПИСАТЬ]\n${overwriteFiles.slice(0, 8).join("\n")}${overwriteFiles.length > 8 ? `\n... +${overwriteFiles.length - 8}` : ""}`
        : "",
    ]
      .filter(Boolean)
      .join("\n");
    const confirmed = window.confirm(`${t.applySafety.summaryTitle}\n\n${summaryLines}\n\n${t.applySafety.continueQuestion}`);
    if (!confirmed) return;
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
      aiCreateFlow.fail("partial apply failed", "План применён частично, есть ошибки");
      showToast({
        type: "error",
        title: "Часть файлов не применена",
        message: `Успешно: ${applied}, пропущено: ${skipped}, ошибок: ${failed.length}. ${failed.slice(0, 2).join(" | ")}`,
      });
    } else {
      aiCreateFlow.finish("План успешно применён", "done");
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
  if (loading) {
    return <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка редактора...</div>;
  }
  return (
    <EditorContextProvider value={editorState}>
      <div className="space-y-4">
        <EditorHeaderCard
          domainId={domainId}
          summary={summary}
          readOnly={readOnly}
          confirmLeaveWithDirty={confirmLeaveWithDirty}
        />
      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600 dark:border-red-800 dark:bg-red-950/30 dark:text-red-300">
          {error}
        </div>
      )}
        <div className="grid gap-4 lg:grid-cols-[320px_1fr]">
          <EditorSidebar
            t={t}
            readOnly={readOnly}
            files={files}
            deletedFiles={deletedFiles}
            selection={selection}
            selectedFolderPath={selectedFolderPath}
            fileLoading={fileLoading}
            fileInputRef={fileInputRef}
            assetUploadInputRef={assetUploadInputRef}
            onCreateFile={onCreateFile}
            onCreateFolder={onCreateFolder}
            onRename={onRename}
            onMove={onMove}
            onDelete={onDelete}
            onUploadClick={onUploadClick}
            onUploadInput={onUploadInput}
            onAssetUploadSelected={onAssetUploadSelected}
            onSelectFile={onSelectFile}
            onSelectFolder={(pathValue) => {
              setSelectedFolderPath(pathValue);
              setSelection(null);
              setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
            }}
            onDeleteFolder={onDeleteFolder}
            onRestoreDeleted={onRestoreDeleted}
          />
        <section className="space-y-3">
          <EditorFileWorkspacePanel
            selection={selection}
            currentFile={currentFile}
            dirtyState={dirtyState}
            saving={saving}
            canSave={canSave}
            readOnly={readOnly}
            description={description}
            setDescription={setDescription}
            onSave={onSave}
            onRevertBuffer={onRevertBuffer}
            onDownload={onDownload}
            rawImageURL={rawImageURL}
            binaryPreview={binaryPreview}
            previewMode={previewMode}
            setPreviewMode={setPreviewMode}
            canPreviewCurrentFile={canPreviewCurrentFile}
            previewSource={previewSource}
            setPreviewSource={setPreviewSource}
            setPreviewViewport={setPreviewViewport}
            previewViewport={previewViewport}
            editorPreviewRef={editorPreviewRef}
            previewSrcDoc={previewSrcDoc}
            bindPreviewNavigationGuard={bindPreviewNavigationGuard}
            previewViewportClass={previewViewportClass}
            focusLine={focusLine}
            setDirtyState={setDirtyState}
          />
          <EditorAIStudioPanel
            t={t}
            aiStudioTab={aiStudioTab}
            setAiStudioTab={setAiStudioTab}
            aiModel={aiModel}
            setAiModel={setAiModel}
            aiContextMode={aiContextMode}
            setAiContextMode={setAiContextMode}
            selection={selection}
            aiContextSelectedFiles={aiContextSelectedFiles}
            setAiContextSelectedFiles={setAiContextSelectedFiles}
            selectableContextFiles={selectableContextFiles}
            aiInstruction={aiInstruction}
            setAiInstruction={setAiInstruction}
            onAISuggest={onAISuggest}
            onCancelAISuggest={onCancelAISuggest}
            aiBusy={aiBusy}
            suggestLocked={suggestLocked}
            onApplyAISuggest={onApplyAISuggest}
            aiOutput={aiOutput}
            aiApplyPathMismatch={aiApplyPathMismatch}
            onAISuggestContextPreview={onAISuggestContextPreview}
            aiSuggestContextBusy={aiSuggestContextBusy}
            suggestContextLocked={suggestContextLocked}
            lockReason={lockReason}
            suggestLockKey={suggestLockKey}
            suggestContextLockKey={suggestContextLockKey}
            aiSuggestFlow={aiSuggestFlow}
            onOpenAISourceFile={onOpenAISourceFile}
            aiOutputSourcePath={aiOutputSourcePath}
            aiSuggestMeta={aiSuggestMeta}
            aiSuggestView={aiSuggestView}
            setAiSuggestView={setAiSuggestView}
            dirtyState={dirtyState}
            setAiOutput={setAiOutput}
            aiSuggestDiagnosticsOpen={aiSuggestDiagnosticsOpen}
            setAiSuggestDiagnosticsOpen={setAiSuggestDiagnosticsOpen}
            aiSuggestContextDebug={aiSuggestContextDebug}
            aiCreateModel={aiCreateModel}
            setAiCreateModel={setAiCreateModel}
            aiCreateContextMode={aiCreateContextMode}
            setAiCreateContextMode={setAiCreateContextMode}
            aiCreatePath={aiCreatePath}
            setAiCreatePath={setAiCreatePath}
            aiCreateContextSelectedFiles={aiCreateContextSelectedFiles}
            setAiCreateContextSelectedFiles={setAiCreateContextSelectedFiles}
            aiCreateInstruction={aiCreateInstruction}
            setAiCreateInstruction={setAiCreateInstruction}
            onAICreatePage={onAICreatePage}
            onCancelAICreatePage={onCancelAICreatePage}
            aiCreateBusy={aiCreateBusy}
            createLocked={createLocked}
            onApplyCreatedFiles={onApplyCreatedFiles}
            canApplyCreatePlan={canApplyCreatePlan}
            onAICreateContextPreview={onAICreateContextPreview}
            aiCreateContextBusy={aiCreateContextBusy}
            createContextLocked={createContextLocked}
            createLockKey={createLockKey}
            createContextLockKey={createContextLockKey}
            aiCreatePlanSummary={aiCreatePlanSummary}
            overwriteConfirmed={overwriteConfirmed}
            setOverwriteConfirmed={setOverwriteConfirmed}
            applyBlockedByAssetIssues={applyBlockedByAssetIssues}
            applyNeedsOverwriteConfirm={applyNeedsOverwriteConfirm}
            aiCreateFlow={aiCreateFlow}
            aiCreateMeta={aiCreateMeta}
            aiImageTargetPath={aiImageTargetPath}
            setAiImageTargetPath={setAiImageTargetPath}
            aiImageFormat={aiImageFormat}
            setAiImageFormat={setAiImageFormat}
            aiImageAlt={aiImageAlt}
            setAiImageAlt={setAiImageAlt}
            aiImageModel={aiImageModel}
            setAiImageModel={setAiImageModel}
            aiImagePrompt={aiImagePrompt}
            setAiImagePrompt={setAiImagePrompt}
            onGenerateImage={onGenerateImage}
            readOnly={readOnly}
            imageGenerateLocked={imageGenerateLocked}
            aiCreateAssetBusyPath={aiCreateAssetBusyPath}
            normalizedAiImageTargetPath={normalizedAiImageTargetPath}
            effectiveAiImageModel={effectiveAiImageModel}
            imageGenerateLockKey={imageGenerateLockKey}
            aiImageResult={aiImageResult}
            aiImageDiagnosticsOpen={aiImageDiagnosticsOpen}
            setAiImageDiagnosticsOpen={setAiImageDiagnosticsOpen}
            onAssetUploadPick={onAssetUploadPick}
            unresolvedAssetIssues={unresolvedAssetIssues}
            assetValidationIssues={assetValidationIssues}
            aiCreateAssets={aiCreateAssets}
            aiCreateSkippedAssets={aiCreateSkippedAssets}
            onRegenerateAsset={onRegenerateAsset}
            isLocked={isLocked}
            assetLockKey={assetLockKey}
            onToggleSkipAsset={onToggleSkipAsset}
            invalidMimeAssets={invalidMimeAssets}
            unresolvedBrokenAssets={unresolvedBrokenAssets}
            unresolvedMissingAssets={unresolvedMissingAssets}
            onCopyAssetPrompt={onCopyAssetPrompt}
            aiAssetFlow={aiAssetFlow}
            unresolvedNonManifestAssets={unresolvedNonManifestAssets}
            aiCreateFiles={aiCreateFiles}
            existingFilesMap={existingFilesMap}
            aiCreateApplyPlan={aiCreateApplyPlan}
            setAiCreatePreviewPath={setAiCreatePreviewPath}
            aiCreatePreviewPath={aiCreatePreviewPath}
            onSetCreatePlan={onSetCreatePlan}
            aiCreatePreviewFile={aiCreatePreviewFile}
            aiCreateView={aiCreateView}
            setAiCreateView={setAiCreateView}
            aiCreateExistingContent={aiCreateExistingContent}
            setAiCreateFiles={setAiCreateFiles}
            aiCreatePreviewSrcDoc={aiCreatePreviewSrcDoc}
            previewViewport={previewViewport}
            setPreviewViewport={setPreviewViewport}
            aiCreatePreviewRef={aiCreatePreviewRef}
            bindPreviewNavigationGuard={bindPreviewNavigationGuard}
            previewViewportClass={previewViewportClass}
            aiCreateDiagnosticsOpen={aiCreateDiagnosticsOpen}
            setAiCreateDiagnosticsOpen={setAiCreateDiagnosticsOpen}
            aiCreateContextDebug={aiCreateContextDebug}
          />
          <EditorHistoryAccess
            domainId={domainId}
            selection={selection}
            readOnly={readOnly}
            historyRefreshKey={historyRefreshKey}
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
    </EditorContextProvider>
  );
}
