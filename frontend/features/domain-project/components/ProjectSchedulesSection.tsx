import { useState } from 'react';
import { AlertTriangle, CheckCircle2, Clock, Play, Power, PowerOff, Settings2, Trash2, X, Plus, XCircle } from 'lucide-react';
import type { LinkEligibilityDTO } from '../../../types/schedules';
import { ScheduleForm } from '../../../components/ScheduleForm';
import { ScheduleTrigger } from '../../../components/ScheduleTrigger';
import { ScheduleRunHistory } from './ScheduleRunHistory';
import type { ScheduleFormValue } from '../../../lib/scheduleFormValidation';
import type { ScheduleDTO } from '../../../types/schedules';
import {
  canCancel,
  canDelete,
  canRun,
} from '../../../features/queue-monitoring/services/actionGuards';
import { getScheduleStrategyLabel } from '../../../features/queue-monitoring/services/statusMeta';

type ProjectSchedulesSectionProps = {
  projectId: string;
  schedulesMultiple: boolean;
  scheduleForm: ScheduleFormValue;
  schedulesLoading: boolean;
  schedulesError: string | null;
  editingSchedule: ScheduleDTO | null;
  resolvedProjectTimezone: string;
  schedules: ScheduleDTO[];
  schedulesPermission: boolean;
  onScheduleFormChange: (next: ScheduleFormValue) => void;
  onSubmitSchedule: (config: Record<string, unknown>) => Promise<void>;
  onRefreshSchedules: () => Promise<void>;
  onTriggerSchedule: (schedule: ScheduleDTO) => Promise<void>;
  onToggleSchedule: (schedule: ScheduleDTO) => Promise<void>;
  onEditSchedule: (schedule: ScheduleDTO) => void;
  onDeleteSchedule: (schedule: ScheduleDTO) => Promise<void>;
  linkScheduleForm: ScheduleFormValue;
  linkScheduleLoading: boolean;
  linkScheduleError: string | null;
  editingLinkSchedule: ScheduleDTO | null;
  linkSchedule: ScheduleDTO | null;
  linkSchedulePermission: boolean;
  onLinkScheduleFormChange: (next: ScheduleFormValue) => void;
  onSubmitLinkSchedule: (config: Record<string, unknown>) => Promise<void>;
  onRefreshLinkSchedule: () => Promise<void>;
  onTriggerLinkSchedule: (schedule: ScheduleDTO) => Promise<void>;
  onToggleLinkSchedule: (schedule: ScheduleDTO) => Promise<void>;
  onEditLinkSchedule: (schedule: ScheduleDTO) => void;
  onDeleteLinkSchedule: (schedule: ScheduleDTO) => Promise<void>;
  linkEligibility?: LinkEligibilityDTO | null;
  linkEligibilityLoading?: boolean;
};

const REASON_LABELS: Record<string, string> = {
  already_inserted: 'Ссылка уже вставлена',
  not_published: 'Сайт не опубликован',
  no_link_settings: 'Не настроен анкор/акцептор',
  active_task_running: 'Задача уже выполняется',
  link_ready_at_future: 'Ещё не готов (link_ready_at)',
};

function LinkEligibilityPanel({
  eligibility,
  loading,
}: {
  eligibility: LinkEligibilityDTO | null;
  loading: boolean;
}) {
  const [showSkipped, setShowSkipped] = useState(false);

  if (loading && !eligibility) {
    return (
      <div className="border border-slate-200 dark:border-slate-800 rounded-2xl p-4 text-sm text-slate-500 animate-pulse">
        Загрузка данных о доменах...
      </div>
    );
  }
  if (!eligibility) return null;

  const eligible = eligibility.domains.filter((d) => d.eligible);
  const skipped = eligibility.domains.filter((d) => !d.eligible);

  return (
    <div className="border border-slate-200 dark:border-slate-800 rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 bg-slate-50 dark:bg-slate-900/40 border-b border-slate-200 dark:border-slate-800">
        <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">
          Домены при следующем запуске
        </span>
        <div className="flex gap-3 text-xs text-slate-500">
          <span className="text-emerald-600 dark:text-emerald-400 font-medium">
            ✓ {eligibility.summary.eligible_count} запустятся
          </span>
          {skipped.length > 0 && (
            <span className="text-red-500 font-medium">
              ✗ {eligibility.summary.ineligible_count} пропускаются
            </span>
          )}
        </div>
      </div>

      <div className="divide-y divide-slate-100 dark:divide-slate-800">
        {eligible.map((d) => (
          <div key={d.id} className="flex items-center gap-2 px-4 py-2.5 text-sm">
            <CheckCircle2 className="w-4 h-4 text-emerald-500 flex-shrink-0" />
            <span className="text-slate-700 dark:text-slate-300 truncate">{d.url}</span>
          </div>
        ))}

        {skipped.length > 0 && (
          <>
            <button
              onClick={() => setShowSkipped((v) => !v)}
              className="w-full flex items-center justify-between px-4 py-2.5 text-xs text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-900/30 transition-colors">
              <span className="font-semibold text-red-500">
                Пропускаются ({skipped.length})
              </span>
              <span>{showSkipped ? '▲ скрыть' : '▼ показать'}</span>
            </button>
            {showSkipped &&
              skipped.map((d) => (
                <div key={d.id} className="flex items-start gap-2 px-4 py-2.5 text-sm bg-red-50/40 dark:bg-red-900/10">
                  <XCircle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
                  <div className="min-w-0">
                    <span className="text-slate-700 dark:text-slate-300 truncate block">{d.url}</span>
                    <span className="text-xs text-red-500 dark:text-red-400">
                      {REASON_LABELS[d.reason] ?? d.reason}
                    </span>
                  </div>
                </div>
              ))}
          </>
        )}

        {eligible.length === 0 && skipped.length === 0 && (
          <div className="px-4 py-6 text-center text-sm text-slate-400">
            <AlertTriangle className="w-5 h-5 mx-auto mb-1 text-slate-300" />
            Нет доменов в проекте
          </div>
        )}
      </div>
    </div>
  );
}

export function ProjectSchedulesSection({
  projectId,
  schedulesMultiple,
  scheduleForm,
  schedulesLoading,
  schedulesError,
  editingSchedule,
  resolvedProjectTimezone,
  schedules,
  onScheduleFormChange,
  onSubmitSchedule,
  onTriggerSchedule,
  onToggleSchedule,
  onEditSchedule,
  onDeleteSchedule,
  linkScheduleForm,
  linkScheduleLoading,
  linkScheduleError,
  editingLinkSchedule,
  linkSchedule,
  onLinkScheduleFormChange,
  onSubmitLinkSchedule,
  onTriggerLinkSchedule,
  onToggleLinkSchedule,
  onEditLinkSchedule,
  onDeleteLinkSchedule,
  linkEligibility,
  linkEligibilityLoading,
}: ProjectSchedulesSectionProps) {
  // Состояния для модалок
  const [isGenModalOpen, setIsGenModalOpen] = useState(false);
  const [isLinkModalOpen, setIsLinkModalOpen] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<{
    type: 'gen' | 'link';
    schedule: ScheduleDTO;
  } | null>(null);

  const genSchedule = schedules[0]; // Берем только первое, так как оно одно

  const formatDateTime = (value?: string) => {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    if (resolvedProjectTimezone) {
      try {
        return new Intl.DateTimeFormat('ru-RU', {
          dateStyle: 'short',
          timeStyle: 'short',
          timeZone: resolvedProjectTimezone,
        }).format(date);
      } catch {}
    }
    return date.toLocaleString();
  };

  const handleOpenGenModal = () => {
    if (genSchedule) onEditSchedule(genSchedule);
    setIsGenModalOpen(true);
  };

  const handleOpenLinkModal = () => {
    if (linkSchedule) onEditLinkSchedule(linkSchedule);
    setIsLinkModalOpen(true);
  };

  const submitGenForm = async (config: Record<string, unknown>) => {
    await onSubmitSchedule(config);
    setIsGenModalOpen(false);
  };

  const submitLinkForm = async (config: Record<string, unknown>) => {
    await onSubmitLinkSchedule(config);
    setIsLinkModalOpen(false);
  };

  const confirmDelete = async () => {
    if (!pendingDelete) return;
    if (pendingDelete.type === 'gen') await onDeleteSchedule(pendingDelete.schedule);
    if (pendingDelete.type === 'link') await onDeleteLinkSchedule(pendingDelete.schedule);
    setPendingDelete(null);
  };

  // Компонент карточки расписания
  const ScheduleCard = ({
    type,
    title,
    schedule,
    loading,
    onEdit,
    onToggle,
    onTrigger,
    onDelete,
  }: any) => {
    if (!schedule) {
      return (
        <div className="flex flex-col items-center justify-center p-8 border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-2xl bg-white/50 dark:bg-slate-900/20">
          <Clock className="w-8 h-8 text-slate-300 dark:text-slate-700 mb-3" />
          <h4 className="text-sm font-bold text-slate-700 dark:text-slate-300">
            {title} не настроено
          </h4>
          <p className="text-xs text-slate-500 mt-1 mb-4 text-center max-w-xs">
            Настройте расписание, чтобы система начала автоматическую работу.
          </p>
          <button
            onClick={onEdit}
            className="inline-flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white px-4 py-2 rounded-xl text-sm font-medium transition-colors">
            <Plus className="w-4 h-4" /> Настроить
          </button>
        </div>
      );
    }

    const nextRun = schedule.nextRunAt || schedule.next_run_at;
    const isLocked = canRun({ busy: loading }).disabled;
    const toggleGuard = schedule.isActive
      ? canCancel({ busy: loading, allowed: true })
      : canRun({ busy: loading });

    return (
      <div
        className={`flex flex-col justify-between p-6 rounded-2xl border transition-all shadow-sm ${schedule.isActive ? 'bg-white dark:bg-[#0f1523] border-slate-200 dark:border-slate-700' : 'bg-slate-50 dark:bg-[#0a1020] border-slate-100 dark:border-slate-800 opacity-75'}`}>
        <div>
          <div className="flex items-start justify-between mb-4">
            <div>
              <h4 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
                {title}
                {schedule.isActive ? (
                  <span className="w-2.5 h-2.5 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)]"></span>
                ) : (
                  <span className="w-2.5 h-2.5 rounded-full bg-slate-300 dark:bg-slate-600"></span>
                )}
              </h4>
              <p className="text-sm font-medium text-slate-500 dark:text-slate-400 mt-1">
                {schedule.name} • {getScheduleStrategyLabel(schedule.strategy)}
              </p>
            </div>

            <button
              onClick={() => onToggle(schedule)}
              disabled={toggleGuard.disabled}
              title={toggleGuard.reason}
              className={`p-2.5 rounded-xl transition-colors ${schedule.isActive ? 'bg-emerald-50 text-emerald-600 hover:bg-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-400 dark:hover:bg-emerald-500/20' : 'bg-slate-100 text-slate-500 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-400'}`}>
              {schedule.isActive ? <Power className="w-5 h-5" /> : <PowerOff className="w-5 h-5" />}
            </button>
          </div>

          <div className="flex items-center gap-2 text-sm font-medium text-slate-600 dark:text-slate-300 bg-slate-50 dark:bg-slate-800/50 p-3 rounded-xl border border-slate-100 dark:border-slate-700/50">
            <Clock className="w-4 h-4 opacity-70 text-indigo-500" />
            <span>
              Следующий запуск:{' '}
              <strong className="text-slate-900 dark:text-white">{formatDateTime(nextRun)}</strong>
            </span>
          </div>
        </div>

        <div className="flex items-center justify-between mt-6 pt-5 border-t border-slate-100 dark:border-slate-800">
          <div className="flex gap-2">
            <ScheduleTrigger
              onClick={() => onTrigger(schedule)}
              disabled={isLocked}
              disabledReason="Загрузка"
            />
            <button
              onClick={onEdit}
              disabled={isLocked}
              className="inline-flex items-center gap-2 px-3 py-2 rounded-xl bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 transition-colors text-xs font-semibold">
              <Settings2 className="w-4 h-4" /> Настроить
            </button>
          </div>
          <button
            onClick={() => setPendingDelete({ type, schedule })}
            disabled={isLocked}
            className="p-2.5 rounded-xl text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors"
            title="Удалить расписание">
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {schedulesMultiple && (
        <div className="p-4 bg-amber-50 text-amber-800 border border-amber-200 rounded-xl text-sm dark:bg-amber-900/20 dark:border-amber-800 dark:text-amber-300">
          ⚠️ Обнаружено несколько расписаний генерации. Система поддерживает только одно.
          Отображается и редактируется первое в списке.
        </div>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        <ScheduleCard
          type="gen"
          title="Генерация сайтов"
          schedule={genSchedule}
          loading={schedulesLoading}
          onEdit={handleOpenGenModal}
          onToggle={onToggleSchedule}
          onTrigger={onTriggerSchedule}
        />

        <ScheduleCard
          type="link"
          title="Вставка ссылок (Link Flow)"
          schedule={linkSchedule}
          loading={linkScheduleLoading}
          onEdit={handleOpenLinkModal}
          onToggle={onToggleLinkSchedule}
          onTrigger={onTriggerLinkSchedule}
        />
      </div>

      {/* Домены при следующем запуске */}
      {(linkEligibility || linkEligibilityLoading) && (
        <LinkEligibilityPanel eligibility={linkEligibility ?? null} loading={!!linkEligibilityLoading} />
      )}

      {/* История запусков */}
      <ScheduleRunHistory
        projectId={projectId}
        isScheduleActive={genSchedule?.isActive || linkSchedule?.isActive}
      />

      {/* МОДАЛКА: Настройка Генерации */}
      {isGenModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
          <div className="w-full max-w-2xl animate-in fade-in zoom-in-95 duration-200">
            <ScheduleForm
              value={scheduleForm}
              loading={schedulesLoading}
              error={schedulesError}
              title={genSchedule ? 'Настройка генерации сайтов' : 'Новое расписание генерации'}
              submitLabel={genSchedule ? 'Сохранить изменения' : 'Создать расписание'}
              timezone={resolvedProjectTimezone}
              timezoneLabel={resolvedProjectTimezone}
              onChange={onScheduleFormChange}
              onSubmit={submitGenForm}
              onCancel={() => setIsGenModalOpen(false)}
              projectId={projectId}
              scheduleType="generation"
            />
          </div>
        </div>
      )}

      {/* МОДАЛКА: Настройка Ссылок */}
      {isLinkModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
          <div className="w-full max-w-2xl animate-in fade-in zoom-in-95 duration-200">
            <ScheduleForm
              value={linkScheduleForm}
              loading={linkScheduleLoading}
              error={linkScheduleError}
              title={linkSchedule ? 'Настройка вставки ссылок' : 'Новое расписание ссылок'}
              submitLabel={linkSchedule ? 'Сохранить изменения' : 'Создать расписание'}
              timezone={resolvedProjectTimezone}
              timezoneLabel={resolvedProjectTimezone}
              onChange={onLinkScheduleFormChange}
              onSubmit={submitLinkForm}
              onCancel={() => setIsLinkModalOpen(false)}
              projectId={projectId}
              scheduleType="link"
            />
          </div>
        </div>
      )}

      {/* МОДАЛКА: Удаление */}
      {pendingDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 backdrop-blur-sm p-4">
          <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl w-full max-w-sm p-6 animate-in fade-in zoom-in-95">
            <h4 className="text-lg font-bold text-slate-900 dark:text-white">
              Удалить расписание?
            </h4>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
              Автоматическая работа по этому процессу будет остановлена.
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <button
                onClick={() => setPendingDelete(null)}
                className="px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                Отмена
              </button>
              <button
                onClick={confirmDelete}
                className="px-5 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-500 rounded-xl shadow-sm">
                Удалить
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
