"use client";

import { useMemo, useState, useRef, useEffect, useCallback } from "react";
import {
  FiChevronDown,
  FiPlus,
  FiSettings,
  FiCheck,
  FiLoader,
  FiFileText,
  FiTrash2,
  FiRotateCcw,
} from "react-icons/fi";
import type { EditorFileMeta } from "@/types/editor";
import { isHtmlFile, getPageDisplayName } from "../services/pageNameMapping";
import { contentEditorRu } from "../services/i18n-content-ru";
import { CreatePageModal } from "./CreatePageModal";

type ContentEditorHeaderProps = {
  files: EditorFileMeta[];
  deletedFiles: EditorFileMeta[];
  selectedPath: string | undefined;
  onSelectFile: (file: EditorFileMeta) => void;
  readOnly: boolean;
  domainId: string;
  onPageCreated: (path: string) => void;
  onDeletePage: (file: EditorFileMeta) => void;
  onRestorePage: (file: EditorFileMeta) => void;
  settingsOpen: boolean;
  onToggleSettings: () => void;
  publishing: boolean;
  contentDirty: boolean;
  hasTemplate: boolean;
  onPublish: () => void;
};

const th = contentEditorRu.header;
const tc = contentEditorRu.compiler;
const ts = contentEditorRu.sidebar;

export function ContentEditorHeader({
  files,
  deletedFiles,
  selectedPath,
  onSelectFile,
  readOnly,
  domainId,
  onPageCreated,
  onDeletePage,
  onRestorePage,
  settingsOpen,
  onToggleSettings,
  publishing,
  contentDirty,
  hasTemplate,
  onPublish,
}: ContentEditorHeaderProps) {
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const pages = useMemo(
    () =>
      files
        .filter((f) => isHtmlFile(f.path))
        .map((f) => ({
          file: f,
          displayName: getPageDisplayName(f.path),
          isRootIndex: f.path === "index.html",
        }))
        .sort((a, b) => {
          if (a.isRootIndex && !b.isRootIndex) return -1;
          if (!a.isRootIndex && b.isRootIndex) return 1;
          return a.displayName.localeCompare(b.displayName, "ru");
        }),
    [files],
  );

  const deletedPages = useMemo(
    () =>
      deletedFiles
        .filter((f) => isHtmlFile(f.path))
        .map((f) => ({
          file: f,
          displayName: getPageDisplayName(f.path),
        })),
    [deletedFiles],
  );

  const selectedDisplay = useMemo(() => {
    if (!selectedPath) return th.noPage;
    return getPageDisplayName(selectedPath);
  }, [selectedPath]);

  // Click-outside to close dropdown
  useEffect(() => {
    if (!dropdownOpen) return;
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [dropdownOpen]);

  const handleSelectPage = useCallback(
    (file: EditorFileMeta) => {
      onSelectFile(file);
      setDropdownOpen(false);
    },
    [onSelectFile],
  );

  const handleDeletePage = useCallback(
    (e: React.MouseEvent, file: EditorFileMeta) => {
      e.stopPropagation();
      if (!confirm(`${ts.confirmDelete} "${getPageDisplayName(file.path)}"?`)) return;
      onDeletePage(file);
      setDropdownOpen(false);
    },
    [onDeletePage],
  );

  const handleRestorePage = useCallback(
    (file: EditorFileMeta) => {
      onRestorePage(file);
    },
    [onRestorePage],
  );

  return (
    <>
      <div className="flex items-center justify-between gap-3 border-b border-slate-200 bg-white px-5 py-3 dark:border-slate-800 dark:bg-[#0b0f19]">
        {/* Left: Page selector + New page */}
        <div className="flex items-center gap-2">
          {/* Page selector dropdown */}
          <div ref={dropdownRef} className="relative">
            <button
              type="button"
              onClick={() => setDropdownOpen((prev) => !prev)}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
            >
              <FiFileText className="h-4 w-4 text-slate-400" />
              <span className="max-w-[200px] truncate">{selectedDisplay}</span>
              <FiChevronDown
                className={`h-3.5 w-3.5 text-slate-400 transition-transform ${dropdownOpen ? "rotate-180" : ""}`}
              />
            </button>

            {dropdownOpen && (
              <div className="absolute left-0 top-full z-30 mt-1 w-72 rounded-xl border border-slate-200 bg-white py-1 shadow-lg dark:border-slate-700 dark:bg-[#0f1523]">
                {pages.length === 0 ? (
                  <p className="px-3 py-2 text-xs text-slate-400 dark:text-slate-500">
                    {ts.noPages}
                  </p>
                ) : (
                  <ul className="max-h-64 overflow-y-auto">
                    {pages.map(({ file, displayName, isRootIndex }) => {
                      const active = file.path === selectedPath;
                      return (
                        <li key={file.path}>
                          <div
                            className={`group flex w-full items-center gap-2 px-3 py-2 text-sm transition-colors ${
                              active
                                ? "bg-indigo-50 text-indigo-700 font-medium dark:bg-indigo-500/10 dark:text-indigo-400"
                                : "text-slate-600 hover:bg-slate-50 dark:text-slate-300 dark:hover:bg-slate-800/50"
                            }`}
                          >
                            <button
                              type="button"
                              onClick={() => handleSelectPage(file)}
                              className="flex min-w-0 flex-1 items-center gap-2 text-left"
                            >
                              <FiFileText className="h-3.5 w-3.5 flex-shrink-0 opacity-60" />
                              <span className="truncate">{displayName}</span>
                              {isRootIndex && (
                                <span className="flex-shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                                  index
                                </span>
                              )}
                            </button>
                            {!readOnly && (
                              <button
                                type="button"
                                onClick={(e) => handleDeletePage(e, file)}
                                title={ts.deletePage}
                                className="flex-shrink-0 rounded p-1 text-slate-400 opacity-0 transition-opacity hover:bg-red-50 hover:text-red-500 group-hover:opacity-100 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                              >
                                <FiTrash2 className="h-3.5 w-3.5" />
                              </button>
                            )}
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                )}

                {/* Deleted pages (trash) */}
                {!readOnly && deletedPages.length > 0 && (
                  <>
                    <div className="mx-3 my-1 border-t border-slate-200 dark:border-slate-700" />
                    <div className="px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
                      {ts.trash}
                    </div>
                    <ul className="max-h-32 overflow-y-auto">
                      {deletedPages.map(({ file, displayName }) => (
                        <li key={`deleted-${file.id}`}>
                          <div className="flex items-center gap-2 px-3 py-1.5 text-sm text-slate-400 dark:text-slate-500">
                            <FiFileText className="h-3.5 w-3.5 flex-shrink-0 opacity-40" />
                            <span className="truncate line-through">{displayName}</span>
                            <button
                              type="button"
                              onClick={() => handleRestorePage(file)}
                              title={ts.restore}
                              className="ml-auto flex-shrink-0 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium text-emerald-600 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-900/20"
                            >
                              <FiRotateCcw className="h-3 w-3" />
                              {ts.restore}
                            </button>
                          </div>
                        </li>
                      ))}
                    </ul>
                  </>
                )}
              </div>
            )}
          </div>

          {/* New page button */}
          {!readOnly && (
            <button
              type="button"
              onClick={() => setModalOpen(true)}
              className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-2.5 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-indigo-500"
            >
              <FiPlus className="h-3.5 w-3.5" />
              {th.newPage}
            </button>
          )}
        </div>

        {/* Right: Settings toggle + Publish */}
        <div className="flex items-center gap-2">
          {/* Settings toggle */}
          <button
            type="button"
            onClick={onToggleSettings}
            title={th.settings}
            className={`rounded-lg p-2 transition-colors ${
              settingsOpen
                ? "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-400"
                : "text-slate-500 hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
            }`}
          >
            <FiSettings className="h-4 w-4" />
          </button>

          {/* Publish button */}
          {!readOnly && selectedPath && (
            <button
              type="button"
              onClick={onPublish}
              disabled={publishing || !contentDirty || !hasTemplate}
              className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3 py-1.5 text-sm font-semibold text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-40"
            >
              {publishing ? (
                <>
                  <FiLoader className="h-3.5 w-3.5 animate-spin" />
                  {tc.publishing}
                </>
              ) : (
                <>
                  <FiCheck className="h-3.5 w-3.5" />
                  {tc.publish}
                </>
              )}
            </button>
          )}
        </div>
      </div>

      {/* Create page modal */}
      {modalOpen && (
        <CreatePageModal
          domainId={domainId}
          onClose={() => setModalOpen(false)}
          onCreated={(path) => {
            setModalOpen(false);
            onPageCreated(path);
          }}
        />
      )}
    </>
  );
}
