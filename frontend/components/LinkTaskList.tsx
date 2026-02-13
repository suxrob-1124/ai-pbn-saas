"use client";

import { useMemo, useState } from "react";
import { FiRefreshCw } from "react-icons/fi";
import type { LinkTaskDTO } from "../types/linkTasks";
import { canDeleteLinkTask, canEditLinkTask, canRetryLinkTask, getLinkTaskStatusMeta, normalizeLinkTaskStatus } from "../lib/linkTaskStatus";

type LinkTaskListProps = {
  tasks: LinkTaskDTO[];
  loading: boolean;
  error?: string | null;
  permissionDenied?: boolean;
  onRefresh: () => void;
  onRetry: (task: LinkTaskDTO) => void;
  onEdit: (task: LinkTaskDTO) => void;
  onDelete: (task: LinkTaskDTO) => void;
  onBulkRetry: (tasks: LinkTaskDTO[]) => void;
  onBulkDelete: (tasks: LinkTaskDTO[]) => void;
};

const STATUS_TONE_CLASSES: Record<string, string> = {
  amber: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200",
  blue: "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200",
  orange: "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-200",
  green: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200",
  yellow: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-200",
  slate: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200",
  red: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200"
};

export function LinkTaskList({
  tasks,
  loading,
  error,
  permissionDenied,
  onRefresh,
  onRetry,
  onEdit,
  onDelete,
  onBulkRetry,
  onBulkDelete
}: LinkTaskListProps) {
  const [statusFilter, setStatusFilter] = useState("all");
  const [selected, setSelected] = useState<Record<string, boolean>>({});

  const availableStatuses = useMemo(() => {
    const unique = new Set<string>();
    tasks.forEach((task) => {
      const normalized = normalizeLinkTaskStatus(task.status);
      unique.add(normalized || task.status);
    });
    return ["all", ...Array.from(unique)];
  }, [tasks]);

  const filteredTasks = useMemo(() => {
    if (statusFilter === "all") {
      return tasks;
    }
    return tasks.filter((task) => {
      const normalized = normalizeLinkTaskStatus(task.status);
      return (normalized || task.status) === statusFilter;
    });
  }, [tasks, statusFilter]);

  const selectedTasks = useMemo(
    () => filteredTasks.filter((task) => selected[task.id]),
    [filteredTasks, selected]
  );
  const bulkRetryTasks = useMemo(
    () => selectedTasks.filter((task) => canRetryLinkTask(task.status)),
    [selectedTasks]
  );
  const bulkDeleteTasks = useMemo(
    () => selectedTasks.filter((task) => canDeleteLinkTask(task.status)),
    [selectedTasks]
  );

  const toggleAll = () => {
    if (selectedTasks.length === filteredTasks.length && filteredTasks.length > 0) {
      const next = { ...selected };
      filteredTasks.forEach((task) => {
        delete next[task.id];
      });
      setSelected(next);
      return;
    }
    const next = { ...selected };
    filteredTasks.forEach((task) => {
      next[task.id] = true;
    });
    setSelected(next);
  };

  const toggleOne = (taskId: string) => {
    setSelected((prev) => ({ ...prev, [taskId]: !prev[taskId] }));
  };

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold">Задачи по ссылкам</h3>
        <div className="flex items-center gap-2">
          <label className="text-xs text-slate-500 dark:text-slate-400">
            Фильтр по статусу:
            <select
              className="ml-2 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              value={statusFilter}
              onChange={(event) => {
                setStatusFilter(event.target.value);
                setSelected({});
              }}
              disabled={loading}
            >
              {availableStatuses.map((status) => (
                <option key={status} value={status}>
                  {status === "all" ? "Все" : getLinkTaskStatusMeta(status).text}
                </option>
              ))}
            </select>
          </label>
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiRefreshCw /> Обновить
          </button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
        <span>Выбрано: {selectedTasks.length}</span>
        <button
          onClick={() => onBulkRetry(bulkRetryTasks)}
          disabled={loading || bulkRetryTasks.length === 0}
          title={bulkRetryTasks.length === 0 ? "Повтор доступен только для задач со статусом Ошибка." : undefined}
          className="inline-flex items-center gap-1 rounded-lg border border-emerald-200 bg-white px-3 py-1 text-xs font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:bg-slate-800 dark:text-emerald-300 disabled:opacity-50"
        >
          Массовый повтор
        </button>
        <button
          onClick={() => onBulkDelete(bulkDeleteTasks)}
          disabled={loading || bulkDeleteTasks.length === 0}
          title={bulkDeleteTasks.length === 0 ? "Удаление недоступно для активных задач (ожидает/поиск/удаление)." : undefined}
          className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200 disabled:opacity-50"
        >
          Массовое удаление
        </button>
      </div>

      {loading && (
        <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка задач...</div>
      )}
      {!loading && permissionDenied && (
        <div className="text-sm text-amber-600 dark:text-amber-400">
          Недостаточно прав для просмотра задач по ссылкам.
        </div>
      )}
      {!loading && !permissionDenied && error && (
        <div className="text-sm text-red-500">{error}</div>
      )}
      {!loading && !permissionDenied && !error && filteredTasks.length === 0 && (
        <div className="text-sm text-slate-500 dark:text-slate-400">Задач по ссылкам пока нет.</div>
      )}
      {!loading && !permissionDenied && !error && filteredTasks.length > 0 && (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                <th className="py-2 pr-2">
                  <input
                    type="checkbox"
                    checked={selectedTasks.length === filteredTasks.length && filteredTasks.length > 0}
                    onChange={toggleAll}
                    disabled={loading}
                  />
                </th>
                <th className="py-2 pr-4">Анкор</th>
                <th className="py-2 pr-4">URL</th>
                <th className="py-2 pr-4">Статус</th>
                <th className="py-2 pr-4">Время</th>
                <th className="py-2 pr-4 text-right">Действия</th>
              </tr>
            </thead>
	            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
	              {filteredTasks.map((task) => {
	                const canRetry = canRetryLinkTask(task.status);
	                const canEdit = canEditLinkTask(task.status);
	                const canDelete = canDeleteLinkTask(task.status);
	                return (
	                  <tr key={task.id}>
	                    <td className="py-3 pr-2">
	                      <input
	                        type="checkbox"
	                        checked={Boolean(selected[task.id])}
	                        onChange={() => toggleOne(task.id)}
	                        disabled={loading}
	                      />
	                    </td>
	                    <td className="py-3 pr-4 font-medium">{task.anchor_text}</td>
	                    <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">{task.target_url}</td>
	                    <td className="py-3 pr-4">
	                      {(() => {
	                        const meta = getLinkTaskStatusMeta(task.status);
	                        const style = STATUS_TONE_CLASSES[meta.tone] || STATUS_TONE_CLASSES.slate;
	                        return (
	                          <span
	                            className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-semibold ${style}`}
	                          >
	                            {meta.text}
	                          </span>
	                        );
	                      })()}
	                    </td>
	                    <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
	                      {new Date(task.scheduled_for).toLocaleString()}
	                    </td>
	                    <td className="py-3 pr-4 text-right space-x-2">
	                      <button
	                        onClick={() => onRetry(task)}
	                        disabled={loading || !canRetry}
	                        title={!canRetry ? "Повтор доступен только для задач со статусом Ошибка." : undefined}
	                        className="inline-flex items-center gap-1 rounded-lg border border-emerald-200 bg-white px-3 py-1 text-xs font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:bg-slate-800 dark:text-emerald-300"
	                      >
	                        Повторить
	                      </button>
	                      <button
	                        onClick={() => onEdit(task)}
	                        disabled={loading || !canEdit}
	                        title={!canEdit ? "Редактирование доступно только для статусов Ожидает и Ошибка." : undefined}
	                        className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
	                      >
	                        Изменить
	                      </button>
	                      <button
	                        onClick={() => onDelete(task)}
	                        disabled={loading || !canDelete}
	                        title={!canDelete ? "Удаление недоступно для активных задач (ожидает/поиск/удаление)." : undefined}
	                        className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
	                      >
	                        Удалить
	                      </button>
	                    </td>
	                  </tr>
	                );
	              })}
	            </tbody>
	          </table>
	        </div>
      )}
    </div>
  );
}
