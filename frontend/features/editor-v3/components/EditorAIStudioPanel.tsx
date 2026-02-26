"use client";

import { FiWind } from "react-icons/fi";

import { MonacoDiffEditor } from "../../../components/MonacoDiffEditor";
import { AI_CONTEXT_MODE_OPTIONS, EDITOR_IMAGE_MODEL_OPTIONS, EDITOR_MODEL_OPTIONS } from "../services/constants";
import {
  AI_FLOW_STATUS_LABELS,
  assetStatusClass,
  assetStatusLabel,
  detectLanguage,
  getFlowToneClass,
} from "../services/editorPreviewUtils";

type EditorAIStudioPanelProps = Record<string, any>;

export function EditorAIStudioPanel(props: EditorAIStudioPanelProps) {
  const {
    t,
    aiStudioTab,
    setAiStudioTab,
    aiModel,
    setAiModel,
    aiContextMode,
    setAiContextMode,
    selection,
    aiContextSelectedFiles,
    setAiContextSelectedFiles,
    selectableContextFiles,
    aiInstruction,
    setAiInstruction,
    onAISuggest,
    aiBusy,
    suggestLocked,
    onApplyAISuggest,
    aiOutput,
    aiApplyPathMismatch,
    onAISuggestContextPreview,
    aiSuggestContextBusy,
    suggestContextLocked,
    lockReason,
    suggestLockKey,
    suggestContextLockKey,
    aiSuggestFlow,
    onOpenAISourceFile,
    aiOutputSourcePath,
    aiSuggestMeta,
    aiSuggestView,
    setAiSuggestView,
    dirtyState,
    setAiOutput,
    aiSuggestDiagnosticsOpen,
    setAiSuggestDiagnosticsOpen,
    aiSuggestContextDebug,
    aiCreateModel,
    setAiCreateModel,
    aiCreateContextMode,
    setAiCreateContextMode,
    aiCreatePath,
    setAiCreatePath,
    aiCreateContextSelectedFiles,
    setAiCreateContextSelectedFiles,
    aiCreateInstruction,
    setAiCreateInstruction,
    onAICreatePage,
    aiCreateBusy,
    createLocked,
    onApplyCreatedFiles,
    canApplyCreatePlan,
    onAICreateContextPreview,
    aiCreateContextBusy,
    createContextLocked,
    createLockKey,
    createContextLockKey,
    aiCreatePlanSummary,
    overwriteConfirmed,
    setOverwriteConfirmed,
    applyBlockedByAssetIssues,
    applyNeedsOverwriteConfirm,
    aiCreateFlow,
    aiCreateMeta,
    aiImageTargetPath,
    setAiImageTargetPath,
    aiImageFormat,
    setAiImageFormat,
    aiImageAlt,
    setAiImageAlt,
    aiImageModel,
    setAiImageModel,
    aiImagePrompt,
    setAiImagePrompt,
    onGenerateImage,
    readOnly,
    imageGenerateLocked,
    aiCreateAssetBusyPath,
    normalizedAiImageTargetPath,
    effectiveAiImageModel,
    imageGenerateLockKey,
    aiImageResult,
    aiImageDiagnosticsOpen,
    setAiImageDiagnosticsOpen,
    onAssetUploadPick,
    unresolvedAssetIssues,
    assetValidationIssues,
    aiCreateAssets,
    aiCreateSkippedAssets,
    onRegenerateAsset,
    isLocked,
    assetLockKey,
    onToggleSkipAsset,
    invalidMimeAssets,
    unresolvedBrokenAssets,
    unresolvedMissingAssets,
    onCopyAssetPrompt,
    aiAssetFlow,
    unresolvedNonManifestAssets,
    aiCreateFiles,
    existingFilesMap,
    aiCreateApplyPlan,
    setAiCreatePreviewPath,
    aiCreatePreviewPath,
    onSetCreatePlan,
    aiCreatePreviewFile,
    aiCreateView,
    setAiCreateView,
    aiCreateExistingContent,
    setAiCreateFiles,
    aiCreatePreviewSrcDoc,
    previewViewport,
    setPreviewViewport,
    aiCreatePreviewRef,
    bindPreviewNavigationGuard,
    aiCreateDiagnosticsOpen,
    setAiCreateDiagnosticsOpen,
    aiCreateContextDebug,
  } = props;
  const previewViewportStyle =
    previewViewport === "mobile"
      ? { maxWidth: 390 }
      : previewViewport === "tablet"
        ? { maxWidth: 820 }
        : undefined;

  return (
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
          {t.tabs.editCurrentFile}
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
          {t.tabs.createPage}
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
                onChange={(e) => setAiContextMode(e.target.value)}
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
                {selectableContextFiles.map((pathValue: string) => (
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
              disabled={aiBusy || suggestLocked || !selection?.selectedPath || !aiInstruction.trim()}
              className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white disabled:opacity-50"
            >
              <FiWind /> {suggestLocked ? t.actions.generating : t.actions.generateSuggestion}
            </button>
            <button
              type="button"
              onClick={onApplyAISuggest}
              disabled={!aiOutput || aiApplyPathMismatch || suggestLocked}
              className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              {t.actions.applyToEditor}
            </button>
            <button
              type="button"
              onClick={onAISuggestContextPreview}
              disabled={aiSuggestContextBusy || suggestContextLocked || !selection?.selectedPath}
              className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              {suggestContextLocked ? t.actions.loadingContext : t.actions.requestContext}
            </button>
          </div>
          {(suggestLocked || suggestContextLocked) && (
            <div className="text-[11px] text-slate-500 dark:text-slate-400">
              {suggestLocked ? lockReason(suggestLockKey) || t.actions.generating : lockReason(suggestContextLockKey)}
            </div>
          )}
          {aiSuggestFlow.flow.status !== "idle" && (
            <div
              className={`rounded-lg border px-3 py-2 text-[11px] ${getFlowToneClass(aiSuggestFlow.flow.status)}`}
            >
              <div className="font-semibold">
                {t.flowTitles.suggest}: {AI_FLOW_STATUS_LABELS[aiSuggestFlow.flow.status as keyof typeof AI_FLOW_STATUS_LABELS]}
              </div>
              <div>{aiSuggestFlow.flow.message || "Выполняем операцию..."}</div>
              {aiSuggestFlow.flow.error ? <div className="mt-1">Причина: {aiSuggestFlow.flow.error}</div> : null}
            </div>
          )}

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

          {aiOutput && !aiApplyPathMismatch && (
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
                  {t.actions.compare}
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
              <div>Источник промпта: {aiSuggestMeta?.source || t.diagnostics.unknown}</div>
              <div>Предупреждений: {aiSuggestMeta?.warnings?.length || 0}</div>
              <div>Использование токенов: {aiSuggestMeta?.tokenUsage ? JSON.stringify(aiSuggestMeta.tokenUsage) : t.diagnostics.tokenUsageNA}</div>
              {aiSuggestMeta?.warnings?.length ? <div>{t.diagnostics.warningsLabel}: {aiSuggestMeta.warnings.join(" | ")}</div> : null}
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
                onChange={(e) => setAiCreateContextMode(e.target.value)}
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
                {selectableContextFiles.map((pathValue: string) => (
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
              disabled={aiCreateBusy || createLocked || !aiCreateInstruction.trim() || !aiCreatePath.trim()}
              className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white disabled:opacity-50"
            >
              <FiWind /> {createLocked ? t.actions.generating : t.actions.generateFiles}
            </button>
            <button
              type="button"
              onClick={onApplyCreatedFiles}
              disabled={!canApplyCreatePlan}
              title={t.tooltips.applySelectedDanger}
              className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              {t.actions.applySelected}
            </button>
            <button
              type="button"
              onClick={onAICreateContextPreview}
              disabled={aiCreateContextBusy || createContextLocked || !aiCreatePath.trim()}
              className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              {createContextLocked ? t.actions.loadingContext : t.actions.requestContext}
            </button>
          </div>
          {(createLocked || createContextLocked) && (
            <div className="text-[11px] text-slate-500 dark:text-slate-400">
              {createLocked ? lockReason(createLockKey) || t.actions.generating : lockReason(createContextLockKey)}
            </div>
          )}
          {aiCreatePlanSummary.overwrite > 0 && (
            <label className="inline-flex items-center gap-2 rounded-lg border border-amber-300 bg-amber-50 px-3 py-1.5 text-[11px] text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
              <input
                type="checkbox"
                checked={overwriteConfirmed}
                onChange={(event) => setOverwriteConfirmed(event.currentTarget.checked)}
              />
              {t.applySafety.overwriteConfirmLabel}
            </label>
          )}
          {(applyBlockedByAssetIssues || applyNeedsOverwriteConfirm) && (
            <div className="text-[11px] text-amber-700 dark:text-amber-300">
              {applyBlockedByAssetIssues ? t.applySafety.blockedByAssets : t.applySafety.overwriteRequiredHint}
            </div>
          )}
          {aiCreateFlow.flow.status !== "idle" && (
            <div
              className={`rounded-lg border px-3 py-2 text-[11px] ${getFlowToneClass(aiCreateFlow.flow.status)}`}
            >
              <div className="font-semibold">
                {t.flowTitles.createPage}: {AI_FLOW_STATUS_LABELS[aiCreateFlow.flow.status as keyof typeof AI_FLOW_STATUS_LABELS]}
              </div>
              <div>{aiCreateFlow.flow.message || "Выполняем операцию..."}</div>
              {aiCreateFlow.flow.error ? <div className="mt-1">Причина: {aiCreateFlow.flow.error}</div> : null}
            </div>
          )}

          {aiCreateMeta?.contextPack && (
            <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
              Контекст: файлов {aiCreateMeta.contextPack.files_used}, объем {aiCreateMeta.contextPack.bytes_used} байт
              {aiCreateMeta.contextPack.truncated ? ", контекст урезан по лимитам" : ""}
            </div>
          )}

          <div className="rounded-lg border border-slate-200 bg-slate-50/70 p-3 text-xs dark:border-slate-700 dark:bg-slate-800/50">
            <div className="mb-2 text-sm font-semibold text-slate-700 dark:text-slate-200">Генерация изображения</div>
            <div className="grid gap-3 md:grid-cols-2">
              <label className="block text-xs text-slate-500 dark:text-slate-400">
                Путь файла
                <input
                  value={aiImageTargetPath}
                  onChange={(e) => setAiImageTargetPath(e.target.value)}
                  placeholder="assets/hero-image.webp"
                  className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-900/60"
                />
              </label>
              <label className="block text-xs text-slate-500 dark:text-slate-400">
                Формат
                <select
                  value={aiImageFormat}
                  onChange={(e) => setAiImageFormat(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-900/60"
                >
                  <option value="webp">webp</option>
                  <option value="png">png</option>
                </select>
              </label>
              <label className="block text-xs text-slate-500 dark:text-slate-400">
                Alt / описание
                <input
                  value={aiImageAlt}
                  onChange={(e) => setAiImageAlt(e.target.value)}
                  placeholder="Описание изображения для страницы"
                  className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-900/60"
                />
              </label>
              <label className="block text-xs text-slate-500 dark:text-slate-400">
                Модель (изображения)
                <select
                  value={aiImageModel}
                  onChange={(e) => setAiImageModel(e.target.value)}
                  className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs dark:border-slate-700 dark:bg-slate-900/60"
                >
                  {EDITOR_IMAGE_MODEL_OPTIONS.map((item) => (
                    <option key={`ai-image-model-${item.value || "default"}`} value={item.value}>
                      {item.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <label className="mt-3 block text-xs text-slate-500 dark:text-slate-400">
              Prompt / style intent
              <textarea
                value={aiImagePrompt}
                onChange={(e) => setAiImagePrompt(e.target.value)}
                rows={3}
                placeholder="Опиши стиль, композицию и требования к изображению"
                className="mt-1 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-900/60"
              />
            </label>
            <div className="mt-3 flex items-center gap-2">
              <button
                type="button"
                onClick={() => void onGenerateImage()}
                disabled={readOnly || imageGenerateLocked || Boolean(aiCreateAssetBusyPath) || !normalizedAiImageTargetPath || !aiImagePrompt.trim()}
                className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white disabled:opacity-50"
              >
                <FiWind /> {imageGenerateLocked ? t.actions.generating : t.imagePanel.generate}
              </button>
              <span className="inline-flex items-center rounded border border-slate-200 bg-slate-50 px-2 py-1 text-[11px] text-slate-600 dark:border-slate-700 dark:bg-slate-800/70 dark:text-slate-300">
                Модель для изображений: <span className="ml-1 font-mono font-semibold">{effectiveAiImageModel}</span>
              </span>
              {imageGenerateLocked && (
                <span className="text-[11px] text-slate-500 dark:text-slate-400">
                  {lockReason(imageGenerateLockKey) || "Генерация изображения"}
                </span>
              )}
            </div>
            {aiImageResult && (
              <div className="mt-3 rounded-lg border border-slate-200 bg-white/80 p-2 text-[11px] dark:border-slate-700 dark:bg-slate-900/60">
                <div className="flex flex-wrap items-center gap-2">
                  <span className={`rounded px-1.5 py-0.5 font-semibold ${assetStatusClass(aiImageResult.status)}`}>
                    {assetStatusLabel(aiImageResult.status)}
                  </span>
                  <span className="text-slate-600 dark:text-slate-300">mime: {aiImageResult.mime_type || "—"}</span>
                  <span className="text-slate-600 dark:text-slate-300">
                    size: {typeof aiImageResult.size_bytes === "number" ? `${aiImageResult.size_bytes} bytes` : "—"}
                  </span>
                </div>
                {aiImageResult.status !== "ok" && (
                  <div className="mt-2 rounded border border-rose-200 bg-rose-50 px-2 py-1.5 text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">
                    {t.imagePanel.friendlyError}
                  </div>
                )}
                {aiImageResult.status !== "ok" && (
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      onClick={() => void onGenerateImage()}
                      disabled={readOnly || imageGenerateLocked || Boolean(aiCreateAssetBusyPath)}
                      className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                    >
                      {t.imagePanel.retry}
                    </button>
                    <button
                      type="button"
                      onClick={() => onAssetUploadPick(normalizedAiImageTargetPath)}
                      disabled={readOnly || !normalizedAiImageTargetPath}
                      className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                    >
                      {t.imagePanel.uploadManual}
                    </button>
                  </div>
                )}
                <details
                  open={aiImageDiagnosticsOpen}
                  onToggle={(event) => setAiImageDiagnosticsOpen((event.currentTarget as HTMLDetailsElement).open)}
                  className="mt-2 rounded border border-slate-200 bg-slate-50 px-2 py-1.5 dark:border-slate-700 dark:bg-slate-800/60"
                >
                  <summary className="cursor-pointer font-semibold text-slate-700 dark:text-slate-200">
                    {t.imagePanel.diagnostics}
                  </summary>
                  <div className="mt-1 space-y-1 text-slate-600 dark:text-slate-300">
                    <div>error_code: {aiImageResult.error_code || "—"}</div>
                    <div>error_message: {aiImageResult.error_message || "—"}</div>
                    <div>{t.imagePanel.warningsCount}: {aiImageResult.warnings.length}</div>
                    {aiImageResult.warnings.length > 0 ? (
                      <div className="text-amber-700 dark:text-amber-300">{aiImageResult.warnings.join(" | ")}</div>
                    ) : null}
                    <div>token_usage: {aiImageResult.token_usage ? JSON.stringify(aiImageResult.token_usage) : t.imagePanel.diagnosticsEmpty}</div>
                  </div>
                </details>
              </div>
            )}
          </div>

          {unresolvedAssetIssues.length > 0 && (
            <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300">
              Нерешённые ассеты: {unresolvedAssetIssues.slice(0, 8).join(", ")}
              {unresolvedAssetIssues.length > 8 ? ` (+${unresolvedAssetIssues.length - 8})` : ""}
            </div>
          )}
          {assetValidationIssues.length > 0 && (
            <div className="rounded-lg border border-red-200 bg-red-50/70 p-2 text-xs dark:border-red-900 dark:bg-red-950/20">
              <div className="mb-1 font-semibold text-red-700 dark:text-red-300">
                Проблемы ассетов перед применением
              </div>
              <div className="space-y-1">
                {assetValidationIssues.map((issue: any) => {
                  const manifestAsset = aiCreateAssets.find((asset: any) => asset.path === issue.path);
                  const skipped = aiCreateSkippedAssets.includes(issue.path);
                  return (
                    <div
                      key={`asset-issue-${issue.path}-${issue.type}`}
                      className="flex flex-wrap items-center justify-between gap-2 rounded border border-red-200 bg-white/80 px-2 py-1 dark:border-red-900 dark:bg-slate-900/50"
                    >
                      <div className="min-w-0">
                        <div className="truncate font-mono text-[11px] text-red-800 dark:text-red-300">{issue.path}</div>
                        <div className="text-[11px] text-red-700/90 dark:text-red-300/90">
                          {issue.type}: {issue.message}
                          {issue.expectedMimeType || issue.actualMimeType
                            ? ` (expected: ${issue.expectedMimeType || "—"}, actual: ${issue.actualMimeType || "—"})`
                            : ""}
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-1">
                        <button
                          type="button"
                          onClick={() => onAssetUploadPick(issue.path)}
                          disabled={readOnly}
                          className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                        >
                          Загрузить вручную
                        </button>
                        {manifestAsset?.prompt ? (
                          <button
                            type="button"
                            onClick={() => void onRegenerateAsset(issue.path)}
                            disabled={Boolean(aiCreateAssetBusyPath) || isLocked(assetLockKey(issue.path))}
                            className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                          >
                            Повторить генерацию
                          </button>
                        ) : null}
                        <button
                          type="button"
                          onClick={() => onToggleSkipAsset(issue.path)}
                          className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                        >
                          {skipped ? "Вернуть ассет" : "Пропустить ассет"}
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
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
                    {aiCreateAssets.map((asset: any) => {
                      const broken = unresolvedBrokenAssets.includes(asset.path);
                      const unresolved = unresolvedMissingAssets.includes(asset.path);
                      const invalidMime = invalidMimeAssets.includes(asset.path);
                      const skipped = aiCreateSkippedAssets.includes(asset.path);
                      const status = skipped
                        ? "пропущен"
                        : invalidMime
                          ? "mime mismatch"
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
                                  : invalidMime
                                    ? "bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300"
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
                                {t.actions.upload}
                              </button>
                              <button
                                type="button"
                                onClick={() => void onRegenerateAsset(asset.path)}
                                disabled={
                                  Boolean(aiCreateAssetBusyPath) || isLocked(assetLockKey(asset.path)) || !asset.prompt
                                }
                                title={t.tooltips.regenerateAsset}
                                className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                              >
                                {aiCreateAssetBusyPath === asset.path || isLocked(assetLockKey(asset.path))
                                  ? t.actions.regeneratingAsset
                                  : t.actions.regenerateAsset}
                              </button>
                              <button
                                type="button"
                                onClick={() => onToggleSkipAsset(asset.path)}
                                className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                              >
                                {skipped ? t.actions.restore : t.actions.skip}
                              </button>
                              <button
                                type="button"
                                onClick={() => void onCopyAssetPrompt(asset.path)}
                                disabled={!asset.prompt}
                                className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                              >
                                {t.actions.copyPrompt}
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
                Изображения из манифеста не применяются автоматически как бинарные файлы. Добавьте их через {t.actions.upload} или пометьте как {t.actions.skip.toLowerCase()}.
              </div>
              {aiAssetFlow.flow.status !== "idle" && (
                <div
                  className={`mt-2 rounded-lg border px-2 py-1.5 text-[11px] ${getFlowToneClass(aiAssetFlow.flow.status)}`}
                >
                  <div className="font-semibold">
                    {t.flowTitles.regenerateAsset}: {AI_FLOW_STATUS_LABELS[aiAssetFlow.flow.status as keyof typeof AI_FLOW_STATUS_LABELS]}
                  </div>
                  <div>{aiAssetFlow.flow.message || "Выполняем операцию..."}</div>
                  {aiAssetFlow.flow.error ? <div className="mt-1">Причина: {aiAssetFlow.flow.error}</div> : null}
                </div>
              )}
            </div>
          )}

          {unresolvedNonManifestAssets.length > 0 && (
            <div className="rounded-lg border border-amber-200 bg-amber-50/70 p-2 text-xs dark:border-amber-900 dark:bg-amber-950/20">
              <div className="mb-1 font-semibold text-amber-800 dark:text-amber-300">
                Ссылки на файлы без манифеста ассетов
              </div>
              <div className="space-y-1">
                {unresolvedNonManifestAssets.map((pathValue: string) => {
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
                          {t.actions.upload}
                        </button>
                        <button
                          type="button"
                          onClick={() => onToggleSkipAsset(pathValue)}
                          className="rounded border border-slate-300 bg-white px-2 py-0.5 text-[11px] font-semibold text-slate-700 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100"
                        >
                          {skipped ? t.actions.restore : t.actions.skip}
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
                    {aiCreateFiles.map((file: any) => {
                      const exists = existingFilesMap.has(file.path);
                      const selectedAction = aiCreateApplyPlan[file.path] || (exists ? "skip" : "create");
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
                            {exists && selectedAction === "overwrite" ? (
                              <span className="rounded bg-rose-100 px-1.5 py-0.5 text-[11px] text-rose-800 dark:bg-rose-900/40 dark:text-rose-300">
                                {t.applySafety.overwriteBadge}
                              </span>
                            ) : exists ? (
                              <span className="rounded bg-amber-100 px-1.5 py-0.5 text-[11px] text-amber-800 dark:bg-amber-900/40 dark:text-amber-300">
                                {t.applySafety.existingBadge}
                              </span>
                            ) : (
                              <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-[11px] text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300">
                                {t.applySafety.newBadge}
                              </span>
                            )}
                          </td>
                          <td className="px-2 py-2">
                            <select
                              value={selectedAction}
                              onChange={(e) => onSetCreatePlan(file.path, e.target.value)}
                              title={selectedAction === "overwrite" ? t.tooltips.overwriteAction : undefined}
                              className="w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-700 dark:bg-slate-800"
                            >
                              <option value="create">{t.applyPlan.create}</option>
                              <option value="overwrite">{t.applyPlan.overwrite}</option>
                              <option value="skip">{t.applyPlan.skip}</option>
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
                      {t.actions.compare}
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
                      {t.actions.preview}
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
                      <div className="space-y-2">
                        <div className="inline-flex rounded-lg border border-slate-200 p-1 text-[11px] dark:border-slate-700">
                          <button
                            type="button"
                            onClick={() => setPreviewViewport("desktop")}
                            className={`rounded px-2 py-1 ${previewViewport === "desktop" ? "bg-indigo-600 text-white" : "text-slate-600 dark:text-slate-300"}`}
                          >
                            Desktop
                          </button>
                          <button
                            type="button"
                            onClick={() => setPreviewViewport("tablet")}
                            className={`rounded px-2 py-1 ${previewViewport === "tablet" ? "bg-indigo-600 text-white" : "text-slate-600 dark:text-slate-300"}`}
                          >
                            Tablet
                          </button>
                          <button
                            type="button"
                            onClick={() => setPreviewViewport("mobile")}
                            className={`rounded px-2 py-1 ${previewViewport === "mobile" ? "bg-indigo-600 text-white" : "text-slate-600 dark:text-slate-300"}`}
                          >
                            Mobile
                          </button>
                        </div>
                        <div className="mx-auto w-full" style={previewViewportStyle}>
                          <iframe
                            ref={aiCreatePreviewRef}
                            title="ai-create-preview"
                            sandbox="allow-same-origin allow-scripts"
                            srcDoc={aiCreatePreviewSrcDoc}
                            onLoad={() => bindPreviewNavigationGuard(aiCreatePreviewRef.current)}
                            className="h-[56vh] w-full rounded-lg border border-slate-200 dark:border-slate-700"
                          />
                        </div>
                      </div>
                    ) : (
                      <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-800/50 dark:text-slate-300">
                        {t.hints.previewOnlyForHTML}
                      </div>
                    ))}
                  {aiCreateView === "code" && (
                    <textarea
                      value={aiCreatePreviewFile.content || ""}
                      onChange={(e) => {
                        const nextContent = e.target.value;
                        setAiCreateFiles((prev: any[]) =>
                          prev.map((item: any) =>
                            item.path === aiCreatePreviewFile.path
                              ? {
                                  ...item,
                                  content: nextContent,
                                }
                              : item
                          )
                        );
                      }}
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
              <div>Источник промпта: {aiCreateMeta?.source || t.diagnostics.unknown}</div>
              <div>Предупреждений: {aiCreateMeta?.warnings?.length || 0}</div>
              <div>Использование токенов: {aiCreateMeta?.tokenUsage ? JSON.stringify(aiCreateMeta.tokenUsage) : t.diagnostics.tokenUsageNA}</div>
              {aiCreateMeta?.warnings?.length ? <div>{t.diagnostics.warningsLabel}: {aiCreateMeta.warnings.join(" | ")}</div> : null}
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
  );
}
