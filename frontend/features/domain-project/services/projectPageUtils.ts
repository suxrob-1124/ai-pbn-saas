import type { ScheduleDTO } from "../../../types/schedules";
import type { ScheduleFormValue } from "../../../lib/scheduleFormValidation";

export type Project = {
  id: string;
  name: string;
  target_country?: string;
  target_language?: string;
  timezone?: string;
  status?: string;
  ownerHasApiKey?: boolean;
};

export type Domain = {
  id: string;
  project_id: string;
  url: string;
  main_keyword?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  status: string;
  last_attempt_generation_id?: string;
  last_success_generation_id?: string;
  published_path?: string;
  file_count?: number;
  total_size_bytes?: number;
  deployment_mode?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_status?: string;
  link_status_effective?: string;
  link_status_source?: "domain" | "active_task";
  link_last_task_id?: string;
  link_ready_at?: string;
  updated_at?: string;
};

export type Generation = {
  id: string;
  domain_id?: string;
  domain_url?: string | null;
  status: string;
  progress: number;
  error?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
};

export type ProjectSummary = {
  project: Project;
  domains: Domain[];
  members: Array<{ email: string; role: string; createdAt: string }>;
  my_role?: "admin" | "owner" | "editor" | "viewer";
};

export type ProjectTab = "domains" | "schedules" | "errors" | "settings";
export const PROJECT_TABS: ProjectTab[] = ["domains", "schedules", "errors", "settings"];

export const createDefaultScheduleForm = (): ScheduleFormValue => ({
  name: "",
  description: "",
  strategy: "daily",
  isActive: true,
  dailyLimit: "5",
  dailyTime: "09:00",
  weeklyLimit: "3",
  weeklyDay: "mon",
  weeklyTime: "09:00",
  customCron: "0 9 * * *"
});

export const deriveScheduleStrategy = (config: Record<string, unknown> | undefined) => {
  const cfg = config || {};
  if (typeof (cfg as any).cron === "string" || typeof (cfg as any).interval === "string") {
    return "custom";
  }
  if (
    typeof (cfg as any).weekday === "string" ||
    typeof (cfg as any).day === "string" ||
    typeof (cfg as any).day === "number"
  ) {
    return "weekly";
  }
  if (typeof (cfg as any).time === "string") {
    return "daily";
  }
  return "immediate";
};

export const normalizeSchedule = (schedule: ScheduleDTO | null) => {
  if (!schedule) return null;
  const config = schedule.config && typeof schedule.config === "object" ? schedule.config : {};
  return {
    ...schedule,
    config,
    strategy: schedule.strategy || deriveScheduleStrategy(config)
  };
};

export const formatDateTime = (value?: string, timezone?: string) => {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  const tz = (timezone || "").trim();
  if (tz) {
    try {
      return new Intl.DateTimeFormat("ru-RU", {
        dateStyle: "short",
        timeStyle: "medium",
        timeZone: tz
      }).format(date);
    } catch {
      // fallback to local timezone
    }
  }
  return date.toLocaleString();
};

export const formatRelativeTime = (target: Date) => {
  const diffMs = target.getTime() - Date.now();
  if (!Number.isFinite(diffMs) || diffMs <= 0) return "сейчас";
  const totalMinutes = Math.ceil(diffMs / 60000);
  if (totalMinutes < 60) {
    return `${totalMinutes} мин`;
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours < 24) {
    return minutes ? `${hours} ч ${minutes} мин` : `${hours} ч`;
  }
  const days = Math.floor(hours / 24);
  const remHours = hours % 24;
  return remHours ? `${days} д ${remHours} ч` : `${days} д`;
};
