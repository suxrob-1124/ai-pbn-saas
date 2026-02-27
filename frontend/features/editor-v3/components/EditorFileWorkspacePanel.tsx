"use client";

import { FiCode, FiEye } from "react-icons/fi";

import { EditorToolbar } from "../../../components/EditorToolbar";
import { MonacoEditor } from "../../../components/MonacoEditor";
import { isImageLikeFile } from "../services/editorPreviewUtils";

type EditorFileWorkspacePanelProps = Record<string, any>;

export function EditorFileWorkspacePanel(props: EditorFileWorkspacePanelProps) {
  const {
    selection,
    currentFile,
    dirtyState,
    saving,
    canSave,
    readOnly,
    description,
    setDescription,
    onSave,
    onRevertBuffer,
    onDownload,
    rawImageURL,
    binaryPreview,
    previewMode,
    setPreviewMode,
    canPreviewCurrentFile,
    previewSource,
    setPreviewSource,
    setPreviewViewport,
    previewViewport,
    editorPreviewRef,
    previewSrcDoc,
    bindPreviewNavigationGuard,
    focusLine,
    setDirtyState,
  } = props;
  const isImageSelected = Boolean(selection && currentFile && isImageLikeFile(selection.selectedPath, currentFile.mimeType));
  const previewViewportStyle =
    previewViewport === "mobile"
      ? { maxWidth: 390 }
      : previewViewport === "tablet"
        ? { maxWidth: 820 }
        : undefined;
  const previewFrameStyle = { height: "80vh", minHeight: "680px" } as const;

  return (
    <>
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

      {isImageSelected && (
        <div className="rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
          <div className="mb-2 text-xs text-slate-500 dark:text-slate-400">
            {selection.selectedPath} · {currentFile.width || "?"}x{currentFile.height || "?"}
          </div>
          <img src={rawImageURL} alt={selection.selectedPath} className="max-h-[70vh] rounded-lg border border-slate-200 dark:border-slate-700" />
        </div>
      )}

      {selection && currentFile && currentFile.isEditable && !binaryPreview && !isImageSelected && (
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
                setDirtyState((prev: any) => ({
                  ...prev,
                  currentContent: value,
                  isDirty: value !== prev.originalContent,
                }));
              }}
            />
          )}

          {previewMode === "preview" && (
            <div className="space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="text-[11px] text-slate-500 dark:text-slate-400">
                  Якорные ссылки работают в пределах preview. Остальная навигация ограничена для безопасности редактора.
                </div>
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
              </div>
              <div className="mx-auto w-full" style={previewViewportStyle}>
                <iframe
                  ref={editorPreviewRef}
                  title="editor-preview"
                  sandbox="allow-same-origin allow-scripts"
                  srcDoc={previewSrcDoc}
                  onLoad={() => bindPreviewNavigationGuard(editorPreviewRef.current)}
                  style={previewFrameStyle}
                  className="w-full rounded-lg border border-slate-200 dark:border-slate-700"
                />
              </div>
            </div>
          )}
        </div>
      )}

      {selection && currentFile && (!currentFile.isEditable || binaryPreview) && !isImageSelected && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300">
          <div className="font-semibold">Файл недоступен для редактирования в текстовом редакторе</div>
          <div className="mt-1">Тип: {selection.mimeType || currentFile.mimeType || "unknown"}</div>
          <div className="mt-1">Размер: {currentFile.size} bytes</div>
        </div>
      )}
    </>
  );
}
