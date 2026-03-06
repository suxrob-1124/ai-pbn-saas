"use client";

import { useMemo, useState } from "react";
import { FiFileText, FiPlus, FiTrash2, FiRotateCcw } from "react-icons/fi";
import type { EditorFileMeta } from "@/types/editor";
import { getPageDisplayName, isHtmlFile } from "../services/pageNameMapping";
import { contentEditorRu } from "../services/i18n-content-ru";
import { CreatePageModal } from "./CreatePageModal";

type ContentSidebarProps = {
  files: EditorFileMeta[];
  deletedFiles?: EditorFileMeta[];
  selectedPath: string | undefined;
  onSelectFile: (file: EditorFileMeta) => void;
  readOnly: boolean;
  domainId: string;
  onPageCreated?: (path: string) => void;
  onDeletePage?: (file: EditorFileMeta) => void;
  onRestorePage?: (file: EditorFileMeta) => void;
};

const t = contentEditorRu.sidebar;

export function ContentSidebar({
  files,
  deletedFiles = [],
  selectedPath,
  onSelectFile,
  readOnly,
  domainId,
  onPageCreated,
  onDeletePage,
  onRestorePage,
}: ContentSidebarProps) {
  const [modalOpen, setModalOpen] = useState(false);

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

  return (
    <aside className="space-y-2">
      <div className="rounded-xl border border-slate-200 bg-white/80 p-3 shadow dark:border-slate-800 dark:bg-slate-900/60">
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200">{t.title}</h3>
          {!readOnly && (
            <button
              type="button"
              onClick={() => setModalOpen(true)}
              className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-indigo-500 transition-colors"
            >
              <FiPlus className="h-3.5 w-3.5" />
              {t.createPage}
            </button>
          )}
        </div>

        {pages.length === 0 ? (
          <p className="text-xs text-slate-400 dark:text-slate-500">{t.noPages}</p>
        ) : (
          <ul className="space-y-0.5">
            {pages.map(({ file, displayName, isRootIndex }) => {
              const active = file.path === selectedPath;
              return (
                <li key={file.path}>
                  <div
                    className={`group flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-sm transition-colors ${
                      active
                        ? "bg-indigo-50 text-indigo-700 font-medium dark:bg-indigo-500/10 dark:text-indigo-400"
                        : "text-slate-600 hover:bg-slate-50 dark:text-slate-300 dark:hover:bg-slate-800/50"
                    }`}
                  >
                    <button
                      type="button"
                      onClick={() => onSelectFile(file)}
                      className="flex min-w-0 flex-1 items-center gap-2 text-left"
                    >
                      <FiFileText className="h-4 w-4 flex-shrink-0 opacity-60" />
                      <span className="truncate">{displayName}</span>
                      {isRootIndex && (
                        <span className="flex-shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                          index
                        </span>
                      )}
                    </button>
                    {!readOnly && onDeletePage && (
                      <button
                        type="button"
                        onClick={() => {
                          if (confirm(`${t.confirmDelete} "${displayName}"?`)) {
                            onDeletePage(file);
                          }
                        }}
                        title={t.deletePage}
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
          <div className="mt-3 border-t border-slate-200 pt-3 dark:border-slate-700">
            <div className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
              {t.trash}
            </div>
            <ul className="space-y-1">
              {deletedPages.map(({ file, displayName }) => (
                <li key={`deleted-${file.id}`} className="flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm text-slate-400 dark:text-slate-500">
                  <FiFileText className="h-3.5 w-3.5 flex-shrink-0 opacity-40" />
                  <span className="truncate line-through">{displayName}</span>
                  {onRestorePage && (
                    <button
                      type="button"
                      onClick={() => onRestorePage(file)}
                      title={t.restore}
                      className="ml-auto flex-shrink-0 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-medium text-emerald-600 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-900/20"
                    >
                      <FiRotateCcw className="h-3 w-3" />
                      {t.restore}
                    </button>
                  )}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>

      {modalOpen && (
        <CreatePageModal
          domainId={domainId}
          onClose={() => setModalOpen(false)}
          onCreated={(path) => {
            setModalOpen(false);
            onPageCreated?.(path);
          }}
        />
      )}
    </aside>
  );
}
