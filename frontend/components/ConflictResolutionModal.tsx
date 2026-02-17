"use client";

type ConflictResolutionModalProps = {
  open: boolean;
  currentVersion: number;
  updatedBy?: string;
  updatedAt?: string;
  onReload: () => void;
  onOverwrite: () => void;
  onCancel: () => void;
  busy?: boolean;
};

export function ConflictResolutionModal({
  open,
  currentVersion,
  updatedBy,
  updatedAt,
  onReload,
  onOverwrite,
  onCancel,
  busy = false,
}: ConflictResolutionModalProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/55 p-4">
      <div className="w-full max-w-xl rounded-xl border border-amber-300 bg-white p-4 shadow-2xl dark:border-amber-800 dark:bg-slate-900">
        <h3 className="text-base font-semibold text-slate-900 dark:text-slate-100">Конфликт версий файла</h3>
        <p className="mt-2 text-sm text-slate-700 dark:text-slate-300">
          Файл изменился в другом сеансе. Выберите, как продолжить работу.
        </p>
        <div className="mt-3 rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
          <div>Текущая версия на сервере: v{currentVersion}</div>
          {updatedBy && <div>Кем изменен: {updatedBy}</div>}
          {updatedAt && <div>Когда: {new Date(updatedAt).toLocaleString()}</div>}
        </div>
        <div className="mt-4 flex flex-wrap items-center justify-end gap-2">
          <button
            type="button"
            onClick={onReload}
            disabled={busy}
            className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            Reload
          </button>
          <button
            type="button"
            onClick={onOverwrite}
            disabled={busy}
            className="rounded-lg border border-amber-400 bg-amber-100 px-3 py-1.5 text-xs font-semibold text-amber-900 disabled:opacity-50 dark:border-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
          >
            Overwrite
          </button>
          <button
            type="button"
            onClick={onCancel}
            disabled={busy}
            className="rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

