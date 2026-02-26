"use client";

import { FiEdit3, FiEye } from "react-icons/fi";

import { FileHistory } from "../../../components/FileHistory";
import type { EditorSelectionState } from "../../../types/editor";

type EditorHistoryAccessProps = {
  domainId: string;
  selection: EditorSelectionState | null;
  readOnly: boolean;
  historyRefreshKey: number;
  onReverted: () => Promise<void>;
};

export function EditorHistoryAccess({
  domainId,
  selection,
  readOnly,
  historyRefreshKey,
  onReverted,
}: EditorHistoryAccessProps) {
  return (
    <>
      <FileHistory
        domainId={domainId}
        fileId={selection?.selectedFileId}
        filePath={selection?.selectedPath}
        canWrite={!readOnly}
        refreshKey={historyRefreshKey}
        onReverted={onReverted}
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
    </>
  );
}
