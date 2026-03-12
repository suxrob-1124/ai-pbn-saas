"use client";

import { useEffect, useState } from "react";

import { showToast } from "../../../lib/toastStore";
import {
  createSchedule,
  deleteSchedule,
  listSchedules,
  triggerSchedule,
  updateSchedule,
} from "../../../lib/schedulesApi";
import {
  deleteLinkSchedule,
  getLinkSchedule,
  getLinkScheduleEligibility,
  triggerLinkSchedule,
  upsertLinkSchedule,
} from "../../../lib/linkSchedulesApi";
import type { ScheduleFormValue } from "../../../lib/scheduleFormValidation";
import type { LinkEligibilityDTO, ScheduleDTO } from "../../../types/schedules";
import {
  createDefaultScheduleForm,
  deriveScheduleStrategy,
  normalizeSchedule,
  type ProjectTab,
} from "../services/projectPageUtils";

type UseProjectSchedulesParams = {
  projectId: string;
  activeTab: ProjectTab;
  setTab: (tab: ProjectTab) => void;
  resolvedProjectTimezone: string;
};

const isPermissionError = (message: string) =>
  /permission|access denied|admin only|forbidden/i.test(message);

export function useProjectSchedules({
  projectId,
  activeTab,
  setTab,
  resolvedProjectTimezone,
}: UseProjectSchedulesParams) {
  const [schedules, setSchedules] = useState<ScheduleDTO[]>([]);
  const [scheduleForm, setScheduleForm] =
    useState<ScheduleFormValue>(createDefaultScheduleForm);
  const [editingSchedule, setEditingSchedule] = useState<ScheduleDTO | null>(null);
  const [schedulesMultiple, setSchedulesMultiple] = useState(false);
  const [schedulesLoading, setSchedulesLoading] = useState(false);
  const [schedulesError, setSchedulesError] = useState<string | null>(null);
  const [schedulesPermission, setSchedulesPermission] = useState(false);
  const [linkSchedule, setLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleForm, setLinkScheduleForm] =
    useState<ScheduleFormValue>(createDefaultScheduleForm);
  const [editingLinkSchedule, setEditingLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleLoading, setLinkScheduleLoading] = useState(false);
  const [linkScheduleError, setLinkScheduleError] = useState<string | null>(null);
  const [linkSchedulePermission, setLinkSchedulePermission] = useState(false);
  const [linkEligibility, setLinkEligibility] = useState<LinkEligibilityDTO | null>(null);
  const [linkEligibilityLoading, setLinkEligibilityLoading] = useState(false);

  const resetScheduleForm = () => {
    setScheduleForm(createDefaultScheduleForm());
    setEditingSchedule(null);
  };

  const resetLinkScheduleForm = () => {
    setLinkScheduleForm(createDefaultScheduleForm());
    setEditingLinkSchedule(null);
  };

  const applyScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    const strategy = schedule.strategy || deriveScheduleStrategy(config as Record<string, unknown>);
    const weeklyDay =
      typeof (config as any).weekday === "string"
        ? String((config as any).weekday)
        : typeof (config as any).day === "string"
          ? String((config as any).day)
          : "mon";
    setScheduleForm({
      name: schedule.name,
      description: schedule.description ?? "",
      strategy,
      isActive: schedule.isActive,
      dailyLimit: typeof config.limit === "number" ? String(config.limit) : "5",
      dailyTime: typeof config.time === "string" ? config.time : "09:00",
      weeklyLimit: typeof config.limit === "number" ? String(config.limit) : "3",
      weeklyDay,
      weeklyTime: typeof config.time === "string" ? config.time : "09:00",
      customCron: typeof config.cron === "string" ? config.cron : "0 9 * * *",
      delayMinutes: typeof (config as any).delay_minutes === "number" ? String((config as any).delay_minutes) : "5",
    });
    setEditingSchedule(schedule);
  };

  const applyLinkScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    const strategy = schedule.strategy || deriveScheduleStrategy(config as Record<string, unknown>);
    const weeklyDay =
      typeof (config as any).weekday === "string"
        ? String((config as any).weekday)
        : typeof (config as any).day === "string"
          ? String((config as any).day)
          : "mon";
    setLinkScheduleForm({
      name: schedule.name,
      description: schedule.description ?? "",
      strategy,
      isActive: schedule.isActive,
      dailyLimit: typeof config.limit === "number" ? String(config.limit) : "5",
      dailyTime: typeof config.time === "string" ? config.time : "09:00",
      weeklyLimit: typeof config.limit === "number" ? String(config.limit) : "3",
      weeklyDay,
      weeklyTime: typeof config.time === "string" ? config.time : "09:00",
      customCron: typeof config.cron === "string" ? config.cron : "0 9 * * *",
      delayMinutes: typeof (config as any).delay_minutes === "number" ? String((config as any).delay_minutes) : "5",
    });
    setEditingLinkSchedule(schedule);
  };

  const loadSchedules = async () => {
    if (!projectId) return;
    setSchedulesLoading(true);
    setSchedulesError(null);
    setSchedulesPermission(false);
    try {
      const list = await listSchedules(projectId);
      const normalized = Array.isArray(list) ? list : [];
      setSchedulesMultiple(normalized.length > 1);
      setSchedules(normalized);
      if (normalized.length > 0) {
        applyScheduleToForm(normalized[0]);
      } else {
        resetScheduleForm();
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить расписания";
      if (isPermissionError(msg)) {
        setSchedulesPermission(true);
      } else {
        setSchedulesError(msg);
      }
    } finally {
      setSchedulesLoading(false);
    }
  };

  const loadLinkSchedule = async () => {
    if (!projectId) return;
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    setLinkSchedulePermission(false);
    try {
      const schedule = await getLinkSchedule(projectId);
      const normalized = normalizeSchedule(schedule);
      setLinkSchedule(normalized);
      if (normalized) {
        applyLinkScheduleToForm(normalized);
      } else {
        resetLinkScheduleForm();
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить расписание ссылок";
      setLinkSchedule(null);
      resetLinkScheduleForm();
      if (isPermissionError(msg)) {
        setLinkSchedulePermission(true);
      } else {
        setLinkScheduleError(msg);
      }
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleSubmitSchedule = async (config: Record<string, unknown>) => {
    setSchedulesLoading(true);
    setSchedulesError(null);
    const isEdit = Boolean(editingSchedule);
    try {
      if (editingSchedule) {
        const updated = await updateSchedule(projectId, editingSchedule.id, {
          name: scheduleForm.name,
          description: scheduleForm.description || undefined,
          strategy: scheduleForm.strategy,
          config,
          isActive: scheduleForm.isActive,
          timezone: editingSchedule.timezone || resolvedProjectTimezone,
        });
        showToast({
          type: "success",
          title: "Расписание обновлено",
          message: updated.name,
        });
      } else {
        const created = await createSchedule(projectId, {
          name: scheduleForm.name,
          description: scheduleForm.description || undefined,
          strategy: scheduleForm.strategy,
          config,
          isActive: scheduleForm.isActive,
          timezone: resolvedProjectTimezone,
        });
        showToast({
          type: "success",
          title: "Расписание создано",
          message: created.name,
        });
      }
      resetScheduleForm();
      await loadSchedules();
    } catch (err: any) {
      const fallback = isEdit ? "Не удалось обновить расписание" : "Не удалось создать расписание";
      const msg = err?.message || fallback;
      setSchedulesError(msg);
      showToast({
        type: "error",
        title: isEdit ? "Ошибка обновления" : "Ошибка создания",
        message: msg,
      });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleEditSchedule = (schedule: ScheduleDTO) => {
    applyScheduleToForm(schedule);
    if (activeTab !== "schedules") {
      setTab("schedules");
    }
  };

  const handleToggleSchedule = async (sched: ScheduleDTO) => {
    setSchedulesLoading(true);
    setSchedulesError(null);
    try {
      const updated = await updateSchedule(projectId, sched.id, {
        isActive: !sched.isActive,
      });
      showToast({
        type: "success",
        title: "Расписание обновлено",
        message: `${updated.name} · ${updated.isActive ? "активно" : "пауза"}`,
      });
      await loadSchedules();
    } catch (err: any) {
      const msg = err?.message || "Не удалось обновить расписание";
      setSchedulesError(msg);
      showToast({ type: "error", title: "Ошибка обновления", message: msg });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleTriggerSchedule = async (sched: ScheduleDTO) => {
    setSchedulesLoading(true);
    setSchedulesError(null);
    try {
      const res = await triggerSchedule(projectId, sched.id);
      const enqueued = res.enqueued ?? 0;
      if (enqueued > 0) {
        showToast({
          type: "success",
          title: "Запуск инициирован",
          message: `${sched.name} · ${enqueued} задач`,
        });
      } else {
        showToast({
          type: "warning",
          title: "Нечего запускать",
          message: `${sched.name} · нет доменов для запуска`,
        });
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось запустить расписание";
      setSchedulesError(msg);
      showToast({ type: "error", title: "Ошибка запуска", message: msg });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleDeleteSchedule = async (sched: ScheduleDTO) => {
    if (!confirm(`Удалить расписание ${sched.name}?`)) return;
    setSchedulesLoading(true);
    setSchedulesError(null);
    try {
      await deleteSchedule(projectId, sched.id);
      showToast({
        type: "success",
        title: "Расписание удалено",
        message: sched.name,
      });
      await loadSchedules();
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить расписание";
      setSchedulesError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setSchedulesLoading(false);
    }
  };

  const handleSubmitLinkSchedule = async (config: Record<string, unknown>) => {
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      const saved = await upsertLinkSchedule(projectId, {
        name: linkScheduleForm.name,
        description: linkScheduleForm.description || undefined,
        strategy: linkScheduleForm.strategy,
        config,
        isActive: linkScheduleForm.isActive,
        timezone: linkSchedule?.timezone || resolvedProjectTimezone,
      });
      showToast({
        type: "success",
        title: "Расписание ссылок сохранено",
        message: saved.name,
      });
      const normalized = normalizeSchedule(saved);
      setLinkSchedule(normalized);
      if (normalized) {
        applyLinkScheduleToForm(normalized);
      }
    } catch (err: any) {
      const msg = err?.message || "Не удалось сохранить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка сохранения", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleEditLinkSchedule = (schedule: ScheduleDTO) => {
    applyLinkScheduleToForm(schedule);
    if (activeTab !== "schedules") {
      setTab("schedules");
    }
  };

  const handleToggleLinkSchedule = async (schedule: ScheduleDTO) => {
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      const saved = await upsertLinkSchedule(projectId, {
        name: schedule.name,
        description: schedule.description ?? undefined,
        strategy: schedule.strategy,
        config: schedule.config,
        isActive: !schedule.isActive,
      });
      showToast({
        type: "success",
        title: "Расписание ссылок обновлено",
        message: `${saved.name} · ${saved.isActive ? "активно" : "пауза"}`,
      });
      setLinkSchedule(saved);
      applyLinkScheduleToForm(saved);
    } catch (err: any) {
      const msg = err?.message || "Не удалось обновить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка обновления", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleTriggerLinkSchedule = async (schedule: ScheduleDTO) => {
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      const res = await triggerLinkSchedule(projectId);
      showToast({
        type: "success",
        title: "Запуск ссылок инициирован",
        message: `${schedule.name} · ${res.enqueued ?? 0} задач`,
      });
    } catch (err: any) {
      const msg = err?.message || "Не удалось запустить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка запуска", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const handleDeleteLinkSchedule = async (schedule: ScheduleDTO) => {
    if (!confirm(`Удалить расписание ссылок ${schedule.name}?`)) return;
    setLinkScheduleLoading(true);
    setLinkScheduleError(null);
    try {
      await deleteLinkSchedule(projectId);
      showToast({
        type: "success",
        title: "Расписание ссылок удалено",
        message: schedule.name,
      });
      setLinkSchedule(null);
      resetLinkScheduleForm();
      await loadLinkSchedule();
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить расписание ссылок";
      setLinkScheduleError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setLinkScheduleLoading(false);
    }
  };

  const loadLinkEligibility = async () => {
    if (!projectId) return;
    setLinkEligibilityLoading(true);
    try {
      const data = await getLinkScheduleEligibility(projectId);
      setLinkEligibility(data);
    } catch {
      setLinkEligibility(null);
    } finally {
      setLinkEligibilityLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === "schedules") {
      void loadSchedules();
      void loadLinkSchedule();
      void loadLinkEligibility();
    }
  }, [activeTab, projectId]);

  return {
    schedulesMultiple,
    scheduleForm,
    setScheduleForm,
    schedulesLoading,
    schedulesError,
    editingSchedule,
    schedules,
    schedulesPermission,
    loadSchedules,
    handleSubmitSchedule,
    handleTriggerSchedule,
    handleToggleSchedule,
    handleEditSchedule,
    handleDeleteSchedule,
    linkScheduleForm,
    setLinkScheduleForm,
    linkScheduleLoading,
    linkScheduleError,
    editingLinkSchedule,
    linkSchedule,
    linkSchedulePermission,
    loadLinkSchedule,
    handleSubmitLinkSchedule,
    handleTriggerLinkSchedule,
    handleToggleLinkSchedule,
    handleEditLinkSchedule,
    handleDeleteLinkSchedule,
    linkEligibility,
    linkEligibilityLoading,
    loadLinkEligibility,
  };
}

