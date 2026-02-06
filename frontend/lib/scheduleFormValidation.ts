import { parseExpression } from "cron-parser";

export type ScheduleStrategy = "immediate" | "daily" | "weekly" | "custom" | string;

export type ScheduleFormValue = {
  name: string;
  description: string;
  strategy: ScheduleStrategy;
  isActive: boolean;
  dailyLimit: string;
  dailyTime: string;
  weeklyLimit: string;
  weeklyDay: string;
  weeklyTime: string;
  customCron: string;
};

export type ScheduleValidationResult =
  | { ok: true; config: Record<string, unknown> }
  | { ok: false; error: string };

const timePattern = /^([01]\d|2[0-3]):[0-5]\d$/;
const validWeekDays = new Set(["mon", "tue", "wed", "thu", "fri", "sat", "sun"]);

const parseLimit = (raw: string): number | null => {
  const value = Number.parseInt(raw, 10);
  if (!Number.isFinite(value) || value <= 0) {
    return null;
  }
  return value;
};

const validateTime = (value: string): boolean => timePattern.test(value);

export const buildScheduleConfig = (value: ScheduleFormValue): ScheduleValidationResult => {
  const strategy = String(value.strategy || "").trim();
  switch (strategy) {
    case "daily": {
      const limit = parseLimit(value.dailyLimit);
      if (!limit) {
        return { ok: false, error: "Лимит должен быть больше 0" };
      }
      if (!validateTime(value.dailyTime)) {
        return { ok: false, error: "Некорректное время для ежедневного расписания" };
      }
      return { ok: true, config: { limit, time: value.dailyTime } };
    }
    case "weekly": {
      const limit = parseLimit(value.weeklyLimit);
      if (!limit) {
        return { ok: false, error: "Лимит должен быть больше 0" };
      }
      const day = value.weeklyDay.toLowerCase();
      if (!validWeekDays.has(day)) {
        return { ok: false, error: "Некорректный день недели" };
      }
      if (!validateTime(value.weeklyTime)) {
        return { ok: false, error: "Некорректное время для еженедельного расписания" };
      }
      return { ok: true, config: { limit, day, time: value.weeklyTime } };
    }
    case "custom": {
      const cron = value.customCron.trim();
      if (!cron) {
        return { ok: false, error: "CRON выражение обязательно" };
      }
      try {
        parseExpression(cron);
      } catch {
        return { ok: false, error: "Некорректное CRON выражение" };
      }
      return { ok: true, config: { cron } };
    }
    case "immediate":
    default:
      return { ok: true, config: { mode: "immediate" } };
  }
};
