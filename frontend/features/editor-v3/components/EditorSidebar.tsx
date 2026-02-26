"use client";

import type { ChangeEvent, RefObject } from "react";
import { FiEdit3, FiFolder, FiMove, FiPlus, FiTrash2, FiUpload } from "react-icons/fi";

import { FileTree } from "../../../components/FileTree";
import { editorV3Ru } from "../services/i18n-ru";
import type { EditorFileMeta, EditorSelectionState } from "../../../types/editor";

type EditorSidebarProps = {
  t: typeof editorV3Ru;
  readOnly: boolean;
  files: EditorFileMeta[];
  deletedFiles: EditorFileMeta[];
  selection: EditorSelectionState | null;
  selectedFolderPath: string;
  fileLoading: boolean;
  fileInputRef: RefObject<HTMLInputElement>;
  assetUploadInputRef: RefObject<HTMLInputElement>;
  onCreateFile: () => void;
  onCreateFolder: () => void;
  onRename: () => void;
  onMove: () => void;
  onDelete: () => void;
  onUploadClick: () => void;
  onUploadInput: (file: File | null) => void;
  onAssetUploadSelected: (event: ChangeEvent<HTMLInputElement>) => void;
  onSelectFile: (file: EditorFileMeta) => Promise<void>;
  onSelectFolder: (path: string) => void;
  onDeleteFolder: (path: string) => Promise<void>;
  onRestoreDeleted: (file: EditorFileMeta) => Promise<void>;
};

export function EditorSidebar({
  t,
  readOnly,
  files,
  deletedFiles,
  selection,
  selectedFolderPath,
  fileLoading,
  fileInputRef,
  assetUploadInputRef,
  onCreateFile,
  onCreateFolder,
  onRename,
  onMove,
  onDelete,
  onUploadClick,
  onUploadInput,
  onAssetUploadSelected,
  onSelectFile,
  onSelectFolder,
  onDeleteFolder,
  onRestoreDeleted,
}: EditorSidebarProps) {
  const hasTreeSelection = Boolean(selection || selectedFolderPath);
  return (
    <aside className="space-y-3 rounded-xl border border-slate-200 bg-white/80 p-3 shadow dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={onCreateFile}
          disabled={readOnly}
          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiPlus /> {t.sidebar.file}
        </button>
        <button
          type="button"
          onClick={onCreateFolder}
          disabled={readOnly}
          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiFolder /> {t.sidebar.folder}
        </button>
        <button
          type="button"
          onClick={onRename}
          disabled={readOnly || !hasTreeSelection}
          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiEdit3 /> {t.sidebar.rename}
        </button>
        <button
          type="button"
          onClick={onMove}
          disabled={readOnly || !hasTreeSelection}
          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiMove /> {t.sidebar.move}
        </button>
        <button
          type="button"
          onClick={onDelete}
          disabled={readOnly || !hasTreeSelection}
          title={t.tooltips.deleteDanger}
          className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-red-50 px-2 py-1 text-xs font-semibold text-red-700 disabled:opacity-50 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
        >
          <FiTrash2 /> {t.sidebar.delete}
        </button>
        <button
          type="button"
          onClick={onUploadClick}
          disabled={readOnly}
          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiUpload /> {t.sidebar.upload}
        </button>
        <input
          ref={fileInputRef as RefObject<HTMLInputElement>}
          type="file"
          className="hidden"
          onChange={(e) => onUploadInput(e.target.files?.[0] || null)}
        />
        <input
          ref={assetUploadInputRef as RefObject<HTMLInputElement>}
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
        selectedFolderPath={selectedFolderPath}
        loading={fileLoading}
        onSelect={onSelectFile}
        onSelectFolder={onSelectFolder}
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
  );
}
