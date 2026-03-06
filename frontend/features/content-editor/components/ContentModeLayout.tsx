"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { EditorDirtyState, EditorFileMeta, EditorSelectionState } from "@/types/editor";
import type { DomainSummaryResponse } from "@/features/editor-v3/types/editor";
import { saveFile } from "@/lib/fileApi";
import { apiBase } from "@/lib/http";
import { showToast } from "@/lib/toastStore";
import { ContentEditorHeader } from "./ContentEditorHeader";
import { ContentEditor } from "./ContentEditor";
import { ContentSettingsPanel, type SettingsTab } from "./ContentSettingsPanel";
import { EditorHistoryAccess } from "@/features/editor-v3/components/EditorHistoryAccess";
import { useHtmlTextExtractor } from "../hooks/useHtmlTextExtractor";
import { useSeoParser } from "../hooks/useSeoParser";
import { usePageMeta } from "../hooks/usePageMeta";
import { useCssVariables } from "../hooks/useCssVariables";
import { assembleFullHtml, convertAiBlocksToImg, unrewriteEditorImageUrls } from "../services/htmlTextExtraction";
import { extractCssFromHtml, applyCssVariablesToHtml } from "../services/cssVariableExtraction";
import { isHtmlFile } from "../services/pageNameMapping";
import { contentEditorRu } from "../services/i18n-content-ru";

type ContentModeLayoutProps = {
  domainId: string;
  files: EditorFileMeta[];
  deletedFiles: EditorFileMeta[];
  selection: EditorSelectionState | null;
  onSelectFile: (file: EditorFileMeta) => void;
  dirtyState: EditorDirtyState;
  readOnly: boolean;
  summary: DomainSummaryResponse | null;
  stylePreview: string;
  scriptPreview: string;
  onFilesRefresh: () => Promise<EditorFileMeta[]>;
  onLoadFile: (file: EditorFileMeta) => Promise<void>;
  onDeletePage: (file: EditorFileMeta) => void;
  onRestorePage: (file: EditorFileMeta) => void;
  historyRefreshKey: number;
  onHistoryReverted: () => Promise<void>;
};

const t = contentEditorRu.editor;
const tc = contentEditorRu.compiler;

export function ContentModeLayout({
  domainId,
  files,
  deletedFiles,
  selection,
  onSelectFile,
  dirtyState,
  readOnly,
  summary,
  stylePreview,
  scriptPreview,
  onFilesRefresh,
  onLoadFile,
  onDeletePage,
  onRestorePage,
  historyRefreshKey,
  onHistoryReverted,
}: ContentModeLayoutProps) {
  const selectedPath = selection?.selectedPath;
  const isHtml = selectedPath ? isHtmlFile(selectedPath) : false;
  const rawHtml = isHtml ? dirtyState.originalContent : "";

  const { editorContent, setEditorContent, contentDirty, template, classMap } = useHtmlTextExtractor(rawHtml, domainId);
  const { seo, updateSeoField, seoDirty } = useSeoParser(rawHtml);
  const { meta, metaDirty, updateMetaField, updateNavLink, addNavLink, removeNavLink } = usePageMeta(rawHtml);

  // CSS-переменные: сначала из отдельного style.css, fallback — из inline <style> в HTML
  const effectiveCss = stylePreview || extractCssFromHtml(rawHtml);
  const cssSourceIsFile = !!stylePreview.trim();
  const { variables: cssVariables, cssDirty, updateVariable: updateCssVariable, buildUpdatedCss } = useCssVariables(effectiveCss);
  const [publishing, setPublishing] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(true);
  const [settingsTab, setSettingsTab] = useState<SettingsTab>("seo");
  const isDirty = contentDirty || seoDirty || metaDirty || cssDirty;

  // Auto-select index.html (or first HTML file) when no HTML page is selected.
  // Reset the guard when selection is cleared (e.g. after page deletion)
  // so auto-selection can re-trigger for the remaining pages.
  const autoSelectedRef = useRef(false);
  useEffect(() => {
    if (!selectedPath) {
      autoSelectedRef.current = false;
    }
  }, [selectedPath]);
  useEffect(() => {
    if (autoSelectedRef.current) return;
    if (selectedPath && isHtml) return;
    const htmlFiles = files.filter((f) => isHtmlFile(f.path));
    if (htmlFiles.length === 0) return;
    const indexFile = htmlFiles.find(
      (f) => f.path === "index.html" || f.path.endsWith("/index.html"),
    );
    const target = indexFile || htmlFiles[0];
    autoSelectedRef.current = true;
    onSelectFile(target);
    void onLoadFile(target);
  }, [files, selectedPath, isHtml, onSelectFile, onLoadFile]);

  const handlePublish = useCallback(async () => {
    if (!selectedPath || !template) return;

    // Конвертируем AI image blocks → <img> перед сохранением
    const withImages = convertAiBlocksToImg(editorContent);
    const contentForSave = unrewriteEditorImageUrls(withImages, apiBase(), domainId);
    let fullHtml = assembleFullHtml(template, contentForSave, seo, meta, classMap);

    // CSS-переменные: inline → модифицируем HTML, file → сохраняем style.css отдельно
    if (cssDirty && !cssSourceIsFile) {
      fullHtml = applyCssVariablesToHtml(fullHtml, cssVariables);
    }

    setPublishing(true);
    try {
      await saveFile(domainId, selectedPath, fullHtml, "Content mode publish", {
        expectedVersion: selection?.version,
        source: "manual",
      });

      // Сохраняем CSS-файл если переменные из отдельного style.css
      if (cssDirty && cssSourceIsFile) {
        await saveFile(domainId, "style.css", buildUpdatedCss(), "CSS variables update", {
          source: "manual",
        });
      }

      showToast({ type: "success", title: tc.success });

      const refreshed = await onFilesRefresh();
      const file = refreshed.find((f) => f.path === selectedPath);
      if (file) await onLoadFile(file);
    } catch (err: any) {
      showToast({
        type: "error",
        title: tc.error,
        message: err?.message || String(err),
      });
    } finally {
      setPublishing(false);
    }
  }, [selectedPath, template, editorContent, seo, meta, classMap, domainId, selection?.version, onFilesRefresh, onLoadFile, cssDirty, cssSourceIsFile, cssVariables, buildUpdatedCss]);

  const handlePageCreated = useCallback(
    async (path: string) => {
      const refreshed = await onFilesRefresh();
      const file = refreshed.find((f) => f.path === path);
      if (file) {
        onSelectFile(file);
        await onLoadFile(file);
      }
    },
    [onFilesRefresh, onSelectFile, onLoadFile],
  );

  return (
    <div className="flex flex-col gap-3">
      {/* Sticky header bar */}
      <ContentEditorHeader
        files={files}
        deletedFiles={deletedFiles}
        selectedPath={selectedPath}
        onSelectFile={onSelectFile}
        readOnly={readOnly}
        domainId={domainId}
        onPageCreated={handlePageCreated}
        onDeletePage={onDeletePage}
        onRestorePage={onRestorePage}
        settingsOpen={settingsOpen}
        onToggleSettings={() => setSettingsOpen((prev) => !prev)}
        publishing={publishing}
        contentDirty={isDirty}
        hasTemplate={!!template}
        onPublish={handlePublish}
      />

      {/* Main content area: editor + optional settings panel */}
      <div className="flex gap-4">
        {/* Editor column */}
        <section className="min-w-0 flex-1">
          {selectedPath && isHtml ? (
            <ContentEditor
              content={editorContent}
              onChange={setEditorContent}
              readOnly={readOnly}
              domainId={domainId}
            />
          ) : (
            <div className="flex min-h-[400px] items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white/50 dark:border-slate-700 dark:bg-slate-900/30">
              <p className="text-sm text-slate-400 dark:text-slate-500">{t.noFileSelected}</p>
            </div>
          )}
        </section>

        {/* Collapsible settings panel */}
        {settingsOpen && selectedPath && isHtml && (
          <ContentSettingsPanel
            activeTab={settingsTab}
            onTabChange={setSettingsTab}
            seo={seo}
            onUpdateSeo={updateSeoField}
            currentPath={selectedPath}
            readOnly={readOnly}
            meta={meta}
            domainId={domainId}
            onUpdateMetaField={updateMetaField}
            onUpdateNavLink={updateNavLink}
            onAddNavLink={addNavLink}
            onRemoveNavLink={removeNavLink}
            cssVariables={cssVariables}
            onUpdateCssVariable={updateCssVariable}
          />
        )}
      </div>

      {/* History — full width below editor */}
      {selectedPath && isHtml && (
        <EditorHistoryAccess
          domainId={domainId}
          selection={selection}
          readOnly={readOnly}
          historyRefreshKey={historyRefreshKey}
          onReverted={onHistoryReverted}
        />
      )}
    </div>
  );
}
