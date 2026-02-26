import { ScheduleForm } from "../../../components/ScheduleForm";
import { ScheduleList } from "../../../components/ScheduleList";
import type { ScheduleFormValue } from "../../../lib/scheduleFormValidation";
import type { ScheduleDTO } from "../../../types/schedules";

type ProjectSchedulesSectionProps = {
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
};

export function ProjectSchedulesSection({
  schedulesMultiple,
  scheduleForm,
  schedulesLoading,
  schedulesError,
  editingSchedule,
  resolvedProjectTimezone,
  schedules,
  schedulesPermission,
  onScheduleFormChange,
  onSubmitSchedule,
  onRefreshSchedules,
  onTriggerSchedule,
  onToggleSchedule,
  onEditSchedule,
  onDeleteSchedule,
  linkScheduleForm,
  linkScheduleLoading,
  linkScheduleError,
  editingLinkSchedule,
  linkSchedule,
  linkSchedulePermission,
  onLinkScheduleFormChange,
  onSubmitLinkSchedule,
  onRefreshLinkSchedule,
  onTriggerLinkSchedule,
  onToggleLinkSchedule,
  onEditLinkSchedule,
  onDeleteLinkSchedule
}: ProjectSchedulesSectionProps) {
  return (
    <div className="space-y-4">
      <div className="space-y-3">
        <h3 className="text-base font-semibold">Расписание генерации</h3>
        {schedulesMultiple && (
          <div className="text-sm text-amber-600 dark:text-amber-400">
            Обнаружено несколько расписаний. Отображается и редактируется только первое.
          </div>
        )}
        <ScheduleForm
          key="generation-schedule-form"
          value={scheduleForm}
          loading={schedulesLoading}
          error={schedulesError}
          title={editingSchedule ? "Редактировать расписание генерации" : "Новое расписание генерации"}
          submitLabel={editingSchedule ? "Сохранить изменения" : "Создать расписание"}
          timezone={resolvedProjectTimezone}
          timezoneLabel={resolvedProjectTimezone}
          onChange={onScheduleFormChange}
          onSubmit={onSubmitSchedule}
        />
        <ScheduleList
          title="Расписание генерации"
          schedules={schedules.slice(0, 1)}
          loading={schedulesLoading}
          error={schedulesError}
          permissionDenied={schedulesPermission}
          timezone={resolvedProjectTimezone}
          onRefresh={onRefreshSchedules}
          onTrigger={onTriggerSchedule}
          onToggle={onToggleSchedule}
          onEdit={onEditSchedule}
          onDelete={onDeleteSchedule}
        />
      </div>

      <div className="space-y-3">
        <h3 className="text-base font-semibold">Расписание ссылок</h3>
        <ScheduleForm
          key="link-schedule-form"
          value={linkScheduleForm}
          loading={linkScheduleLoading}
          error={linkScheduleError}
          title={editingLinkSchedule ? "Редактировать расписание ссылок" : "Новое расписание ссылок"}
          submitLabel={editingLinkSchedule ? "Сохранить изменения" : "Создать расписание"}
          timezone={resolvedProjectTimezone}
          timezoneLabel={resolvedProjectTimezone}
          onChange={onLinkScheduleFormChange}
          onSubmit={onSubmitLinkSchedule}
        />
        <ScheduleList
          title="Расписание ссылок"
          schedules={linkSchedule ? [linkSchedule] : []}
          loading={linkScheduleLoading}
          error={linkScheduleError}
          permissionDenied={linkSchedulePermission}
          timezone={resolvedProjectTimezone}
          onRefresh={onRefreshLinkSchedule}
          onTrigger={onTriggerLinkSchedule}
          onToggle={onToggleLinkSchedule}
          onEdit={onEditLinkSchedule}
          onDelete={onDeleteLinkSchedule}
        />
      </div>
    </div>
  );
}
