"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import type { UrlObject } from "url";
import { useParams, usePathname, useRouter, useSearchParams } from "next/navigation";
import { authFetch, authFetchCached, post, patch, del } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import {
  FiPlay,
  FiRefreshCw,
  FiList,
  FiClock,
  FiPauseCircle,
  FiCheck,
  FiTrash2,
  FiX,
  FiKey,
  FiAlertCircle,
  FiLink,
  FiInfo,
  FiActivity
} from "react-icons/fi";
import Link from "next/link";
import { showToast } from "../../../lib/toastStore";
import { createSchedule, deleteSchedule, listSchedules, triggerSchedule, updateSchedule } from "../../../lib/schedulesApi";
import { deleteLinkSchedule, getLinkSchedule, triggerLinkSchedule, upsertLinkSchedule } from "../../../lib/linkSchedulesApi";
import type { ScheduleDTO } from "../../../types/schedules";
import { ScheduleForm } from "../../../components/ScheduleForm";
import { ScheduleList } from "../../../components/ScheduleList";
import type { ScheduleFormValue } from "../../../lib/scheduleFormValidation";
import { Badge } from "../../../components/Badge";

type Project = {
  id: string;
  name: string;
  target_country?: string;
  target_language?: string;
  timezone?: string;
  status?: string;
  ownerHasApiKey?: boolean;
};

type Domain = {
  id: string;
  project_id: string;
  url: string;
  main_keyword?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  status: string;
  last_generation_id?: string;
  link_anchor_text?: string;
  link_acceptor_url?: string;
  link_status?: string;
  link_last_task_id?: string;
  link_ready_at?: string;
  updated_at?: string;
};

type Generation = {
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

type ProjectSummary = {
  project: Project;
  domains: Domain[];
  members: Array<{ email: string; role: string; createdAt: string }>;
};

type ProjectTab = "domains" | "schedules" | "errors" | "settings";
const PROJECT_TABS: ProjectTab[] = ["domains", "schedules", "errors", "settings"];

export default function ProjectDetailPage() {
  const { me } = useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const projectId = params?.id as string;
  const [project, setProject] = useState<Project | null>(null);
  const [domains, setDomains] = useState<Domain[]>([]);
  const [domainSearch, setDomainSearch] = useState("");
  const domainById = useMemo(() => {
    const map: Record<string, Domain> = {};
    domains.forEach((domain) => {
      map[domain.id] = domain;
    });
    return map;
  }, [domains]);
  const [gens, setGens] = useState<Record<string, Generation[]>>({});
  const [openRuns, setOpenRuns] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [projectName, setProjectName] = useState("");
  const [projectCountry, setProjectCountry] = useState("");
  const [projectLanguage, setProjectLanguage] = useState("");
  const [projectTimezone, setProjectTimezone] = useState("");
  const [timezoneQuery, setTimezoneQuery] = useState("");
  const [recentTimezones, setRecentTimezones] = useState<string[]>([]);
  const [projectSettingsLoading, setProjectSettingsLoading] = useState(false);
  const [projectSettingsError, setProjectSettingsError] = useState<string | null>(null);
  const timezoneFallback = useMemo(
    () => ["UTC", "Europe/Moscow", "Europe/Paris", "Europe/London", "Europe/Berlin", "Asia/Bangkok", "Asia/Tokyo", "Asia/Singapore", "America/New_York", "America/Los_Angeles"],
    []
  );
  const availableTimezones = useMemo(() => {
    let zones: string[] = [];
    try {
      const supported = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf;
      if (typeof supported === "function") {
        zones = supported("timeZone") || [];
      }
    } catch {
      zones = [];
    }
    if (zones.length === 0) {
      zones = timezoneFallback;
    }
    const unique = Array.from(new Set(zones)).sort();
    const current = (projectTimezone || "").trim();
    if (current && !unique.includes(current)) {
      unique.unshift(current);
    }
    return unique;
  }, [projectTimezone, timezoneFallback]);

  const [url, setUrl] = useState("");
  const [keyword, setKeyword] = useState("");
  const [country, setCountry] = useState("");
  const [language, setLanguage] = useState("");
  const [exclude, setExclude] = useState("");
  const [importText, setImportText] = useState("");
  const [keywordEdits, setKeywordEdits] = useState<Record<string, string>>({});
  const [linkEdits, setLinkEdits] = useState<Record<string, { anchor: string; acceptor: string }>>({});
  const [activeTab, setActiveTab] = useState<ProjectTab>("domains");
  const [members, setMembers] = useState<Array<{ email: string; role: string; createdAt: string }>>([]);
  const [newMemberEmail, setNewMemberEmail] = useState("");
  const [newMemberRole, setNewMemberRole] = useState("editor");
  const [schedules, setSchedules] = useState<ScheduleDTO[]>([]);
  const [scheduleForm, setScheduleForm] = useState<ScheduleFormValue>({
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
  const [editingSchedule, setEditingSchedule] = useState<ScheduleDTO | null>(null);
  const [schedulesMultiple, setSchedulesMultiple] = useState(false);
  const [schedulesLoading, setSchedulesLoading] = useState(false);
  const [schedulesError, setSchedulesError] = useState<string | null>(null);
  const [schedulesPermission, setSchedulesPermission] = useState(false);
  const [linkSchedule, setLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleForm, setLinkScheduleForm] = useState<ScheduleFormValue>({
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
  const [editingLinkSchedule, setEditingLinkSchedule] = useState<ScheduleDTO | null>(null);
  const [linkScheduleLoading, setLinkScheduleLoading] = useState(false);
  const [linkScheduleError, setLinkScheduleError] = useState<string | null>(null);
  const [linkSchedulePermission, setLinkSchedulePermission] = useState(false);
  const [linkLoadingId, setLinkLoadingId] = useState<string | null>(null);
  const [projectErrors, setProjectErrors] = useState<Generation[]>([]);
  const [projectErrorsLoading, setProjectErrorsLoading] = useState(false);
  const [projectErrorsError, setProjectErrorsError] = useState<string | null>(null);

  const buildTabHref = (tab: ProjectTab): UrlObject => {
    const params = new URLSearchParams(searchParams);
    params.set("tab", tab);
    const qs = params.toString();
    return {
      pathname: projectId ? `/projects/${projectId}` : pathname,
      search: qs ? `?${qs}` : undefined
    };
  };
  const setTab = (tab: ProjectTab) => {
    setActiveTab(tab);
    if (!projectId) {
      return;
    }
    const params = new URLSearchParams(searchParams);
    params.set("tab", tab);
    const qs = params.toString();
    const nextUrl = qs ? `${pathname}?${qs}` : pathname;
    if (typeof window !== "undefined") {
      window.history.replaceState(null, "", nextUrl);
    } else {
      router.replace(nextUrl as unknown as never, { scroll: false });
    }
  };

  const filteredDomains = useMemo(() => {
    const term = domainSearch.trim().toLowerCase();
    if (!term) {
      return domains;
    }
    return domains.filter((d) => {
      const label = (d.url || d.id || "").toLowerCase();
      return label.includes(term);
    });
  }, [domains, domainSearch]);

  useEffect(() => {
    const tab = searchParams.get("tab");
    if (tab === "queue" && projectId) {
      router.replace(`/projects/${projectId}/queue`, { scroll: false });
      return;
    }
    if (tab === "members" && projectId) {
      router.replace(`/projects/${projectId}?tab=settings`, { scroll: false });
      return;
    }
    if (tab && PROJECT_TABS.includes(tab as ProjectTab) && tab !== activeTab) {
      setActiveTab(tab as ProjectTab);
    }
  }, [searchParams, activeTab, projectId, router]);

  const load = async (force = false) => {
    setLoading(true);
    setError(null);
    try {
      const summary = await authFetchCached<ProjectSummary>(`/api/projects/${projectId}/summary`, undefined, {
        ttlMs: 15000,
        bypassCache: force
      });
      const p = summary?.project || null;
      const d = Array.isArray(summary?.domains) ? summary.domains : [];
      const m = Array.isArray(summary?.members) ? summary.members : [];
      setProject(p);
      setCountry(p?.target_country || "");
      setLanguage(p?.target_language || "");
      setProjectName(p?.name || "");
      setProjectCountry(p?.target_country || "");
      setProjectLanguage(p?.target_language || "");
      setProjectTimezone(p?.timezone || "UTC");
      setDomains(d);
      setMembers(m);
      const edits: Record<string, string> = {};
      const linkDrafts: Record<string, { anchor: string; acceptor: string }> = {};
      d.forEach((item) => {
        edits[item.id] = item.main_keyword || "";
        linkDrafts[item.id] = {
          anchor: item.link_anchor_text || "",
          acceptor: item.link_acceptor_url || ""
        };
      });
      setKeywordEdits(edits);
      setLinkEdits(linkDrafts);
    } catch (err: any) {
      setProject(null);
      setDomains([]);
      setMembers([]);
      setError(err?.message || "Не удалось загрузить проект");
    } finally {
      setLoading(false);
    }
  };

  const isPermissionError = (message: string) =>
    /permission|access denied|admin only|forbidden/i.test(message);

  const resolvedProjectTimezone = (projectTimezone || project?.timezone || "UTC").trim() || "UTC";

  const filteredTimezones = useMemo(() => {
    const q = timezoneQuery.trim().toLowerCase();
    if (!q) return availableTimezones;
    const filtered = availableTimezones.filter((tz) => tz.toLowerCase().includes(q));
    const current = (projectTimezone || "").trim();
    if (current && !filtered.includes(current)) {
      return [current, ...filtered];
    }
    return filtered;
  }, [availableTimezones, projectTimezone, timezoneQuery]);

  const timezoneGroups = useMemo(() => {
    const groups = new Map<string, string[]>();
    filteredTimezones.forEach((tz) => {
      const parts = tz.split("/");
      const group = parts.length > 1 ? parts[0] : "Other";
      const list = groups.get(group) || [];
      list.push(tz);
      groups.set(group, list);
    });
    return Array.from(groups.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [filteredTimezones]);

  const recentFiltered = useMemo(() => {
    const q = timezoneQuery.trim().toLowerCase();
    const list = recentTimezones.filter((tz) => availableTimezones.includes(tz));
    if (!q) return list;
    return list.filter((tz) => tz.toLowerCase().includes(q));
  }, [availableTimezones, recentTimezones, timezoneQuery]);

  const getTimezoneOffsetLabel = useMemo(() => {
    const cache = new Map<string, string>();
    return (tz: string) => {
      if (cache.has(tz)) return cache.get(tz) as string;
      try {
        const now = new Date();
        const formatter = new Intl.DateTimeFormat("en-US", {
          timeZone: tz,
          hour12: false,
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit"
        });
        const parts = formatter.formatToParts(now);
        const partMap: Record<string, string> = {};
        parts.forEach((p) => {
          if (p.type !== "literal") partMap[p.type] = p.value;
        });
        const asUTC = Date.UTC(
          Number(partMap.year),
          Number(partMap.month) - 1,
          Number(partMap.day),
          Number(partMap.hour),
          Number(partMap.minute),
          Number(partMap.second)
        );
        const offsetMinutes = Math.round((asUTC - now.getTime()) / 60000);
        const sign = offsetMinutes >= 0 ? "+" : "-";
        const abs = Math.abs(offsetMinutes);
        const hh = String(Math.floor(abs / 60)).padStart(2, "0");
        const mm = String(abs % 60).padStart(2, "0");
        const label = `UTC${sign}${hh}:${mm}`;
        cache.set(tz, label);
        return label;
      } catch {
        cache.set(tz, "");
        return "";
      }
    };
  }, []);

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem("obz_recent_timezones");
      if (raw) {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) {
          setRecentTimezones(parsed.filter((v) => typeof v === "string"));
        }
      }
    } catch {
      // ignore
    }
  }, []);

  const updateRecentTimezone = (tz: string) => {
    setRecentTimezones((prev) => {
      const next = [tz, ...prev.filter((v) => v !== tz)].slice(0, 5);
      try {
        window.localStorage.setItem("obz_recent_timezones", JSON.stringify(next));
      } catch {
        // ignore
      }
      return next;
    });
  };

  const deriveStrategy = (config: Record<string, unknown> | undefined) => {
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

  const normalizeSchedule = (schedule: ScheduleDTO | null) => {
    if (!schedule) return null;
    const config = schedule.config && typeof schedule.config === "object" ? schedule.config : {};
    return {
      ...schedule,
      config,
      strategy: schedule.strategy || deriveStrategy(config)
    };
  };

  const formatDateTime = (value?: string, tzOverride?: string) => {
    if (!value) return "—";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "—";
    const tz = (tzOverride || resolvedProjectTimezone || "").trim();
    if (tz) {
      try {
        return new Intl.DateTimeFormat("ru-RU", {
          dateStyle: "short",
          timeStyle: "medium",
          timeZone: tz
        }).format(date);
      } catch {
        // fallback to local
      }
    }
    return date.toLocaleString();
  };

  const formatRelativeTime = (target: Date) => {
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

  const loadProjectErrors = async (force = false) => {
    if (!projectId) return;
    setProjectErrorsLoading(true);
    setProjectErrorsError(null);
    try {
      const list = await authFetchCached<Generation[]>(`/api/generations?limit=100&lite=1`, undefined, {
        ttlMs: 15000,
        bypassCache: force
      });
      const normalized = Array.isArray(list) ? list : [];
      const domainIDs = new Set(domains.map((d) => d.id));
      const errors = normalized
        .filter((g) => g.status === "error" && g.domain_id && domainIDs.has(g.domain_id))
        .sort((a, b) => {
          const aTime = new Date((a.updated_at || a.finished_at || a.started_at || a.created_at || "") as string).getTime() || 0;
          const bTime = new Date((b.updated_at || b.finished_at || b.started_at || b.created_at || "") as string).getTime() || 0;
          return bTime - aTime;
        })
        .slice(0, 20);
      setProjectErrors(errors);
    } catch (err: any) {
      setProjectErrorsError(err?.message || "Не удалось загрузить ошибки");
      setProjectErrors([]);
    } finally {
      setProjectErrorsLoading(false);
    }
  };

  const saveProjectSettings = async () => {
    if (!projectId) return;
    const name = projectName.trim();
    if (!name) {
      setProjectSettingsError("Название проекта не может быть пустым");
      return;
    }
    setProjectSettingsLoading(true);
    setProjectSettingsError(null);
    try {
      const payload = {
        name,
        country: projectCountry.trim(),
        language: projectLanguage.trim(),
        status: project?.status || "draft",
        timezone: resolvedProjectTimezone
      };
      const updated = await authFetch<Project>(`/api/projects/${projectId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });
      setProject(updated);
      setProjectName(updated.name || "");
      setProjectCountry(updated.target_country || "");
      setProjectLanguage(updated.target_language || "");
      setProjectTimezone(updated.timezone || "UTC");
      showToast({
        type: "success",
        title: "Настройки проекта сохранены",
        message: updated.name
      });
    } catch (err: any) {
      const msg = err?.message || "Не удалось сохранить настройки проекта";
      setProjectSettingsError(msg);
      showToast({ type: "error", title: "Ошибка сохранения", message: msg });
    } finally {
      setProjectSettingsLoading(false);
    }
  };

  const resetScheduleForm = () => {
    setScheduleForm({
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
    setEditingSchedule(null);
  };

  const resetLinkScheduleForm = () => {
    setLinkScheduleForm({
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
    setEditingLinkSchedule(null);
  };

  const applyScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    const strategy = schedule.strategy || deriveStrategy(config as Record<string, unknown>);
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
      customCron: typeof config.cron === "string" ? config.cron : "0 9 * * *"
    });
    setEditingSchedule(schedule);
  };

  const applyLinkScheduleToForm = (schedule: ScheduleDTO) => {
    const config = schedule.config || {};
    const strategy = schedule.strategy || deriveStrategy(config as Record<string, unknown>);
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
      customCron: typeof config.cron === "string" ? config.cron : "0 9 * * *"
    });
    setEditingLinkSchedule(schedule);
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
          timezone: editingSchedule.timezone || resolvedProjectTimezone
        });
        showToast({
          type: "success",
          title: "Расписание обновлено",
          message: updated.name
        });
      } else {
        const created = await createSchedule(projectId, {
          name: scheduleForm.name,
          description: scheduleForm.description || undefined,
          strategy: scheduleForm.strategy,
          config,
          isActive: scheduleForm.isActive,
          timezone: resolvedProjectTimezone
        });
        showToast({
          type: "success",
          title: "Расписание создано",
          message: created.name
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
        message: msg
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
        isActive: !sched.isActive
      });
      showToast({
        type: "success",
        title: "Расписание обновлено",
        message: `${updated.name} · ${updated.isActive ? "активно" : "пауза"}`
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
          message: `${sched.name} · ${enqueued} задач`
        });
      } else {
        showToast({
          type: "warning",
          title: "Нечего запускать",
          message: `${sched.name} · нет доменов для запуска`
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
        message: sched.name
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
        timezone: linkSchedule?.timezone || resolvedProjectTimezone
      });
      showToast({
        type: "success",
        title: "Расписание ссылок сохранено",
        message: saved.name
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
        isActive: !schedule.isActive
      });
      showToast({
        type: "success",
        title: "Расписание ссылок обновлено",
        message: `${saved.name} · ${saved.isActive ? "активно" : "пауза"}`
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
        message: `${schedule.name} · ${res.enqueued ?? 0} задач`
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
        message: schedule.name
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

  useEffect(() => {
    if (projectId) {
      load();
    }
  }, [projectId]);

  useEffect(() => {
    if (activeTab === "schedules") {
      loadSchedules();
      loadLinkSchedule();
    }
  }, [activeTab, projectId]);

  useEffect(() => {
    if (activeTab === "errors") {
      loadProjectErrors();
    }
  }, [activeTab, projectId, domains]);

  const addDomain = async () => {
    if (!url.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains`, { url, keyword, country, language, exclude_domains: exclude });
      setUrl("");
      setKeyword("");
      setExclude("");
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось добавить домен");
    } finally {
      setLoading(false);
    }
  };

  const importDomains = async () => {
    if (!importText.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains/import`, { text: importText });
      setImportText("");
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось импортировать");
    } finally {
      setLoading(false);
    }
  };

  const runGeneration = async (id: string) => {
    const domain = domains.find((d) => d.id === id);
    if (!(keywordEdits[id] || "").trim() && !(domain?.main_keyword || "").trim()) {
      setError("Сначала задайте ключевое слово");
      return;
    }
    if (domain?.status === "processing" || domain?.status === "pending") {
      setError("У этого домена уже есть запущенная генерация");
      return;
    }
    // Проверяем наличие API ключа у владельца проекта
    if (project && project.ownerHasApiKey === false) {
      setError("API ключ не настроен у владельца проекта. Настройте ключ в профиле для запуска генерации.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await post(`/api/domains/${id}/generate`);
      await load(true);
    } catch (err: any) {
      const errMsg = err?.message || "Не удалось запустить генерацию";
      // Улучшаем сообщение об ошибке, если это связано с API ключом
      if (errMsg.includes("API key") || errMsg.includes("api key")) {
        setError(`${errMsg} Настройте API ключ в профиле.`);
      } else {
        setError(errMsg);
      }
    } finally {
      setLoading(false);
    }
  };

  const updateKeyword = async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/domains/${id}`, { keyword: keywordEdits[id] || "" });
      showToast({
        type: "success",
        title: "Ключевое слово сохранено",
        message: domainById[id]?.url || ""
      });
      await load(true);
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось сохранить ключевое слово",
        message: err?.message || "Попробуйте позже"
      });
      setError(err?.message || "Не удалось обновить ключевое слово");
    } finally {
      setLoading(false);
    }
  };

  const updateLinkSettings = async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      const entry = linkEdits[id] || { anchor: "", acceptor: "" };
      await patch(`/api/domains/${id}`, {
        link_anchor_text: entry.anchor?.trim() || "",
        link_acceptor_url: entry.acceptor?.trim() || ""
      });
      showToast({
        type: "success",
        title: "Ссылка сохранена",
        message: domainById[id]?.url || ""
      });
      await load(true);
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось сохранить ссылку",
        message: err?.message || "Попробуйте позже"
      });
      setError(err?.message || "Не удалось обновить ссылку");
    } finally {
      setLoading(false);
    }
  };

  const deleteDomain = async (id: string) => {
    if (!confirm("Удалить домен?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/domains/${id}`);
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить домен");
    } finally {
      setLoading(false);
    }
  };

  const runLinkTask = async (id: string) => {
    const domain = domainById[id];
    if (!domain) return;
    const linkStatus = (domain.link_status || "").toLowerCase();
    const hasActiveLink = ["inserted", "generated"].includes(linkStatus);
    const anchor = (domain.link_anchor_text || "").trim();
    const acceptor = (domain.link_acceptor_url || "").trim();
    const draft = linkEdits[id] || { anchor, acceptor };
    const draftAnchor = (draft.anchor || "").trim();
    const draftAcceptor = (draft.acceptor || "").trim();
    if (draftAnchor !== anchor || draftAcceptor !== acceptor) {
      showToast({
        type: "error",
        title: "Сначала сохраните ссылку",
        message: "В полях есть несохранённые изменения."
      });
      return;
    }
    if (!anchor || !acceptor) {
      showToast({
        type: "error",
        title: "Ссылка не настроена",
        message: "Заполните анкор и акцептор в настройках домена."
      });
      return;
    }
    setLinkLoadingId(id);
    try {
      await post(`/api/domains/${id}/link/run`);
      showToast({
        type: "success",
        title: hasActiveLink ? "Ссылка обновляется" : "Ссылка добавляется",
        message: domain.url
      });
      await load(true);
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось запустить ссылку",
        message: err?.message || "Попробуйте позже"
      });
    } finally {
      setLinkLoadingId(null);
    }
  };

  const removeLinkTask = async (id: string) => {
    const domain = domainById[id];
    if (!domain) return;
    const linkStatus = (domain.link_status || "").toLowerCase();
    const canRemoveLink = ["inserted", "generated"].includes(linkStatus);
    const anchor = (domain.link_anchor_text || "").trim();
    const acceptor = (domain.link_acceptor_url || "").trim();
    const draft = linkEdits[id] || { anchor, acceptor };
    const draftAnchor = (draft.anchor || "").trim();
    const draftAcceptor = (draft.acceptor || "").trim();
    if (draftAnchor !== anchor || draftAcceptor !== acceptor) {
      showToast({
        type: "error",
        title: "Сначала сохраните ссылку",
        message: "В полях есть несохранённые изменения."
      });
      return;
    }
    if (!canRemoveLink) {
      showToast({
        type: "error",
        title: "Удалять нечего",
        message: "Ссылка на сайте не найдена."
      });
      return;
    }
    if (!confirm(`Удалить ссылку с сайта ${domain.url}?`)) return;
    setLinkLoadingId(id);
    try {
      await post(`/api/domains/${id}/link/remove`);
      showToast({
        type: "success",
        title: "Ссылка удаляется",
        message: domain.url
      });
      await load(true);
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Не удалось удалить ссылку",
        message: err?.message || "Попробуйте позже"
      });
    } finally {
      setLinkLoadingId(null);
    }
  };

  const deleteProject = async () => {
    if (!confirm("Удалить проект и все его домены?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/projects/${projectId}`);
      router.push("/projects");
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить проект");
    } finally {
      setLoading(false);
    }
  };

  const addMember = async () => {
    if (!newMemberEmail.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/members`, { email: newMemberEmail.trim(), role: newMemberRole });
      setNewMemberEmail("");
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось добавить участника");
    } finally {
      setLoading(false);
    }
  };

  const updateMemberRole = async (email: string, role: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`, { role });
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось обновить роль");
    } finally {
      setLoading(false);
    }
  };

  const removeMember = async (email: string) => {
    if (!confirm(`Удалить участника ${email}?`)) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`);
      await load(true);
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить участника");
    } finally {
      setLoading(false);
    }
  };

  const loadGens = async (id: string) => {
    try {
      const list = await authFetch<Generation[]>(`/api/domains/${id}/generations`);
      setGens((prev) => ({ ...prev, [id]: Array.isArray(list) ? list : [] }));
      // Переключаем состояние открытия/закрытия
      setOpenRuns((prev) => ({ ...prev, [id]: !prev[id] }));
    } catch {
      /* ignore */
    }
  };


  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold">{project?.name || "Проект"}</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Страна: {project?.target_country || "—"} · Язык: {project?.target_language || "—"}
            </p>
          </div>
          <div className="flex gap-2">
            <Link
              href={
                projectId
                  ? ({ pathname: "/projects/[id]/queue", query: { id: projectId, tab: "domains" } } as UrlObject)
                  : "/projects"
              }
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiList /> Очередь проекта
            </Link>
            {projectId && (
              <Link
                href={`/monitoring/indexing?projectId=${encodeURIComponent(projectId)}`}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                <FiActivity /> Indexing
              </Link>
            )}
            <button
              onClick={() => load(true)}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            <button
              onClick={deleteProject}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
            >
              <FiTrash2 /> Удалить
            </button>
          </div>
        </div>
        {error && <div className="text-red-500 text-sm mt-2">{error}</div>}
        
        {/* Индикатор API ключа */}
        {project && (
          <div className="mt-4">
            {project.ownerHasApiKey === false ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/20 p-3">
                <div className="flex items-start gap-2">
                  <FiAlertCircle className="text-amber-600 dark:text-amber-400 mt-0.5" />
                  <div className="flex-1">
                    <div className="text-sm font-semibold text-amber-800 dark:text-amber-200">
                      ⚠️ API ключ не настроен
                    </div>
                    <div className="text-xs text-amber-700 dark:text-amber-300 mt-1">
                      Генерация не будет работать без API ключа владельца проекта.{" "}
                      <a href="/me" className="underline hover:no-underline">
                        Настроить в профиле →
                      </a>
                    </div>
                  </div>
                </div>
              </div>
            ) : project.ownerHasApiKey === true ? (
              <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/40 p-3">
                <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-400">
                  <FiKey className="text-emerald-600 dark:text-emerald-400" />
                  <span>API ключ настроен. Генерация будет использовать ключ владельца проекта.</span>
                </div>
              </div>
            ) : null}
          </div>
        )}
      </div>

      {/* Tabs */}
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 dark:border-slate-800 mb-4">
          <div className="flex flex-wrap gap-2">
          <Link
            href={buildTabHref("domains")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "domains"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Домены
          </Link>
          <Link
            href={buildTabHref("schedules")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "schedules"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Расписания
          </Link>
          <Link
            href={buildTabHref("errors")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "errors"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Ошибки
          </Link>
          <Link
            href={buildTabHref("settings")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "settings"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Настройки
          </Link>
          </div>
        </div>

        {activeTab === "domains" && (
          <>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3 mb-3">
        <h3 className="font-semibold">Добавить домен</h3>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="example.com"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Ключевое слово"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
          <div className="flex gap-2">
            <button
              onClick={addDomain}
              disabled={loading || !url.trim()}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
            >
              Добавить
            </button>
          </div>
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Страна (по умолчанию из проекта)"
            value={country}
            onChange={(e) => setCountry(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Язык (по умолчанию из проекта)"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Исключить домены (через запятую)"
            value={exclude}
            onChange={(e) => setExclude(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Импорт списком (url[,ключевое слово] на строку). Пример: <code>example.com,casino</code>
          </p>
          <textarea
            className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            rows={4}
            placeholder="example.com,ключевое слово&#10;example.org"
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
          />
          <button
            onClick={importDomains}
            disabled={loading || !importText.trim()}
            className="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-4 py-2 text-sm font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
          >
            Импортировать
          </button>
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <h3 className="font-semibold">Домены</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">
            Показано: {filteredDomains.length} из {domains.length}
          </span>
        </div>
        <input
          type="search"
          value={domainSearch}
          onChange={(e) => setDomainSearch(e.target.value)}
          placeholder="Поиск по домену"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        />
        {filteredDomains.length === 0 && (
          <div className="text-sm text-slate-500 dark:text-slate-400">Домены не найдены.</div>
        )}
        <div className="space-y-3">
          {filteredDomains.map((d) => {
            const linkStatus = (d.link_status || "").toLowerCase();
            const hasActiveLink = ["inserted", "generated"].includes(linkStatus);
            const canRemoveLink = hasActiveLink;
            const linkReadyAtDate = d.link_ready_at ? new Date(d.link_ready_at) : null;
            const linkReadyValid = linkReadyAtDate && !Number.isNaN(linkReadyAtDate.getTime());
            const linkReadyFuture = Boolean(linkReadyValid && linkReadyAtDate!.getTime() > Date.now());
            const linkReadyBadge = !linkReadyValid
              ? { label: "не задано", tone: "slate" as const }
              : linkReadyFuture
                ? { label: "ожидает", tone: "amber" as const }
                : { label: "готово", tone: "green" as const };
            return (
              <div
                key={d.id}
                className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white/60 dark:bg-slate-900/40 p-4 shadow-sm space-y-3"
              >
              <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Link href={`/domains/${d.id}`} className="text-indigo-600 hover:underline font-semibold">
                      {d.url}
                    </Link>
                    <StatusBadge status={d.status} />
                    <LinkStatusBadge domain={d} />
                  </div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">
                    Обновлено: {formatDateTime(d.updated_at)}
                  </div>
                  <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                    <span>
                      Готовность ссылок: {linkReadyValid ? formatDateTime(d.link_ready_at) : "не задано"}
                    </span>
                    <Badge label={linkReadyBadge.label} tone={linkReadyBadge.tone} className="text-[11px]" />
                    {linkReadyFuture && linkReadyAtDate && (
                      <span className="text-amber-600 dark:text-amber-300">
                        через {formatRelativeTime(linkReadyAtDate)}
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-slate-400">
                    Страна: {d.target_country || "—"} · Язык: {d.target_language || "—"}
                  </div>
                  {d.exclude_domains && <div className="text-xs text-slate-400">Исключить: {d.exclude_domains}</div>}
                </div>
                <div className="flex flex-wrap items-center gap-2 md:justify-end">
                  <button
                    onClick={() => runLinkTask(d.id)}
                    disabled={loading || linkLoadingId === d.id}
                    className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <FiLink />
                    {hasActiveLink ? "Обновить ссылку" : "Добавить ссылку"}
                  </button>
                  <button
                    onClick={() => runLinkTask(d.id)}
                    disabled={loading || linkLoadingId === d.id}
                    className="inline-flex sm:hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    title={hasActiveLink ? "Обновить ссылку" : "Добавить ссылку"}
                    aria-label={hasActiveLink ? "Обновить ссылку" : "Добавить ссылку"}
                  >
                    <FiLink />
                  </button>
                  {canRemoveLink ? (
                    <>
                      <button
                        onClick={() => removeLinkTask(d.id)}
                        disabled={loading || linkLoadingId === d.id}
                        className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                      >
                        <FiTrash2 />
                        Удалить ссылку
                      </button>
                      <button
                        onClick={() => removeLinkTask(d.id)}
                        disabled={loading || linkLoadingId === d.id}
                        className="inline-flex sm:hidden items-center justify-center rounded-lg border border-red-200 bg-white px-2 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                        title="Удалить ссылку"
                        aria-label="Удалить ссылку"
                      >
                        <FiTrash2 />
                      </button>
                    </>
                  ) : (
                    <span className="hidden sm:inline-flex items-center gap-1 rounded-full border border-slate-200 bg-slate-50 px-2 py-1 text-[11px] font-semibold text-slate-500 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300">
                      <FiInfo className="h-3 w-3" /> Нет ссылки
                    </span>
                  )}
                  <button
                    onClick={() => loadGens(d.id)}
                    className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <FiList /> Запуски {openRuns[d.id] && gens[d.id] && `(${gens[d.id].length})`}
                  </button>
                  <button
                    onClick={() => loadGens(d.id)}
                    className="inline-flex sm:hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    title={`Запуски ${openRuns[d.id] && gens[d.id] ? `(${gens[d.id].length})` : ""}`}
                    aria-label="Запуски"
                  >
                    <FiList />
                  </button>
                  <Link
                    href={`/monitoring/indexing?domainId=${encodeURIComponent(d.id)}`}
                    className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    <FiActivity /> Index checks
                  </Link>
                  <Link
                    href={`/monitoring/indexing?domainId=${encodeURIComponent(d.id)}`}
                    className="inline-flex sm:hidden items-center justify-center rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    title="Index checks"
                    aria-label="Index checks"
                  >
                    <FiActivity />
                  </Link>
                  <button
                    onClick={() => deleteDomain(d.id)}
                    disabled={loading}
                    className="hidden sm:inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                  >
                    Удалить
                  </button>
                  <button
                    onClick={() => deleteDomain(d.id)}
                    disabled={loading}
                    className="inline-flex sm:hidden items-center justify-center rounded-lg border border-red-200 bg-white px-2 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                    title="Удалить"
                    aria-label="Удалить"
                  >
                    <FiTrash2 />
                  </button>
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                {(() => {
                  const keywordValue = keywordEdits[d.id] ?? "";
                  const keywordDirty = keywordValue.trim() !== (d.main_keyword || "").trim();
                  return (
                    <div className="space-y-1">
                      <div className="text-xs uppercase tracking-wide text-slate-400 flex items-center gap-2">
                        Ключевое слово
                        {keywordDirty && (
                          <span className="rounded-full bg-amber-900/30 px-2 py-0.5 text-[10px] text-amber-300">
                            несохранено
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <input
                          className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${keywordDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""}`}
                          value={keywordValue}
                          onChange={(e) => setKeywordEdits((prev) => ({ ...prev, [d.id]: e.target.value }))}
                          placeholder="Ключевое слово"
                        />
                        <button
                          onClick={() => updateKeyword(d.id)}
                          disabled={loading || !keywordDirty}
                          className="hidden sm:inline-flex items-center gap-1 rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                        >
                          Сохранить
                        </button>
                        <button
                          onClick={() => updateKeyword(d.id)}
                          disabled={loading || !keywordDirty}
                          className="inline-flex sm:hidden items-center justify-center rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                          title="Сохранить ключевое слово"
                          aria-label="Сохранить ключевое слово"
                        >
                          <FiCheck />
                        </button>
                      </div>
                    </div>
                  );
                })()}
                {(() => {
                  const link = linkEdits[d.id] || { anchor: d.link_anchor_text || "", acceptor: d.link_acceptor_url || "" };
                  const anchorValue = link.anchor ?? "";
                  const acceptorValue = link.acceptor ?? "";
                  const linkDirty =
                    anchorValue.trim() !== (d.link_anchor_text || "").trim() ||
                    acceptorValue.trim() !== (d.link_acceptor_url || "").trim();
                  return (
                    <div className="space-y-1">
                      <div className="text-xs uppercase tracking-wide text-slate-400 flex items-center gap-2">
                        Ссылка
                        {linkDirty && (
                          <span className="rounded-full bg-amber-900/30 px-2 py-0.5 text-[10px] text-amber-300">
                            несохранено
                          </span>
                        )}
                      </div>
                      <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                        <input
                          className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${linkDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""}`}
                          value={anchorValue}
                          onChange={(e) =>
                            setLinkEdits((prev) => ({
                              ...prev,
                              [d.id]: { anchor: e.target.value, acceptor: prev[d.id]?.acceptor ?? "" }
                            }))
                          }
                          placeholder="Анкор"
                        />
                        <input
                          className={`w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 ${linkDirty ? "border-indigo-400 ring-1 ring-indigo-400/40" : ""}`}
                          value={acceptorValue}
                          onChange={(e) =>
                            setLinkEdits((prev) => ({
                              ...prev,
                              [d.id]: { anchor: prev[d.id]?.anchor ?? "", acceptor: e.target.value }
                            }))
                          }
                          placeholder="https://acceptor.example"
                        />
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => updateLinkSettings(d.id)}
                            disabled={loading || !linkDirty}
                            className="hidden sm:inline-flex items-center gap-1 rounded-lg bg-slate-200 px-3 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                          >
                            Сохранить
                          </button>
                          <button
                            onClick={() => updateLinkSettings(d.id)}
                            disabled={loading || !linkDirty}
                            className="inline-flex sm:hidden items-center justify-center rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 disabled:opacity-50 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                            title="Сохранить ссылку"
                            aria-label="Сохранить ссылку"
                          >
                            <FiCheck />
                          </button>
                        </div>
                      </div>
                    </div>
                  );
                })()}
              </div>

              {openRuns[d.id] && gens[d.id] && <RunsList runs={gens[d.id]} />}
              </div>
            );
          })}
        </div>
      </div>
          </>
        )}

        {activeTab === "schedules" && (
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
                onChange={setScheduleForm}
                onSubmit={handleSubmitSchedule}
              />
              <ScheduleList
                title="Расписание генерации"
                schedules={schedules.slice(0, 1)}
                loading={schedulesLoading}
                error={schedulesError}
                permissionDenied={schedulesPermission}
                timezone={resolvedProjectTimezone}
                onRefresh={loadSchedules}
                onTrigger={handleTriggerSchedule}
                onToggle={handleToggleSchedule}
                onEdit={handleEditSchedule}
                onDelete={handleDeleteSchedule}
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
                onChange={setLinkScheduleForm}
                onSubmit={handleSubmitLinkSchedule}
              />
              <ScheduleList
                title="Расписание ссылок"
                schedules={linkSchedule ? [linkSchedule] : []}
                loading={linkScheduleLoading}
                error={linkScheduleError}
                permissionDenied={linkSchedulePermission}
                timezone={resolvedProjectTimezone}
                onRefresh={loadLinkSchedule}
                onTrigger={handleTriggerLinkSchedule}
                onToggle={handleToggleLinkSchedule}
                onEdit={handleEditLinkSchedule}
                onDelete={handleDeleteLinkSchedule}
              />
            </div>
          </div>
        )}

        {activeTab === "errors" && (
          <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="font-semibold">Ошибки генерации</h3>
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  Последние сбои генерации по доменам проекта с быстрым перезапуском.
                </p>
              </div>
              <button
                onClick={() => loadProjectErrors(true)}
                disabled={projectErrorsLoading}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                <FiRefreshCw /> Обновить
              </button>
            </div>
            {projectErrorsLoading && (
              <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка ошибок...</div>
            )}
            {!projectErrorsLoading && projectErrorsError && (
              <div className="text-sm text-red-500">{projectErrorsError}</div>
            )}
            {!projectErrorsLoading && !projectErrorsError && projectErrors.length === 0 && (
              <div className="text-sm text-slate-500 dark:text-slate-400">Ошибок пока нет.</div>
            )}
            {!projectErrorsLoading && !projectErrorsError && projectErrors.length > 0 && (
              <div className="space-y-3">
                {projectErrors.map((g) => {
                  const domain = g.domain_id ? domainById[g.domain_id] : undefined;
                  const label = domain?.url || g.domain_url || g.domain_id || "Неизвестный домен";
                  const when = g.updated_at || g.finished_at || g.started_at || g.created_at;
                  const timeLabel = formatDateTime(when);
                  const message = (g.error || "Ошибка не указана").trim();
                  const shortMessage = message.length > 160 ? `${message.slice(0, 160)}…` : message;
                  return (
                    <div
                      key={g.id}
                      className="flex flex-col gap-3 rounded-lg border border-slate-200 bg-white/90 px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-900/70 md:flex-row md:items-center md:justify-between"
                    >
                      <div className="space-y-1">
                        <div className="font-semibold text-slate-900 dark:text-slate-100">{label}</div>
                        <div className="text-xs text-slate-500 dark:text-slate-400">
                          {timeLabel} · {shortMessage}
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Link
                          href={`/queue/${g.id}`}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        >
                          Открыть
                        </Link>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {activeTab === "settings" && (
          <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <h3 className="font-semibold">Настройки проекта</h3>
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  Основные параметры, таймзона и доступы команды.
                </p>
              </div>
              <button
                onClick={saveProjectSettings}
                disabled={projectSettingsLoading || !projectName.trim()}
                className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
              >
                {projectSettingsLoading ? "Сохраняем..." : "Сохранить настройки"}
              </button>
            </div>
            {projectSettingsError && (
              <div className="text-sm text-red-500">{projectSettingsError}</div>
            )}

            <div className="grid gap-4 lg:grid-cols-[1fr_1.2fr]">
              <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
                <div>
                  <h4 className="font-semibold">Профиль проекта</h4>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    Название и региональные параметры.
                  </p>
                </div>
                <div className="grid gap-3">
                  <div className="space-y-1">
                    <div className="text-xs uppercase tracking-wide text-slate-400">Название</div>
                    <input
                      className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                      value={projectName}
                      onChange={(e) => setProjectName(e.target.value)}
                      placeholder="Название проекта"
                    />
                  </div>
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="space-y-1">
                      <div className="text-xs uppercase tracking-wide text-slate-400">Страна</div>
                      <input
                        className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                        value={projectCountry}
                        onChange={(e) => setProjectCountry(e.target.value)}
                        placeholder="se"
                      />
                    </div>
                    <div className="space-y-1">
                      <div className="text-xs uppercase tracking-wide text-slate-400">Язык</div>
                      <input
                        className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                        value={projectLanguage}
                        onChange={(e) => setProjectLanguage(e.target.value)}
                        placeholder="sv"
                      />
                    </div>
                  </div>
                </div>
              </div>

              <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
                <div>
                  <h4 className="font-semibold">Таймзона проекта</h4>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    Используется для расписаний и отображения времени.
                  </p>
                </div>
                <div className="space-y-3">
                  <input
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                    value={timezoneQuery}
                    onChange={(e) => setTimezoneQuery(e.target.value)}
                    placeholder="Поиск таймзоны (например, Asia/Bangkok)"
                  />
                  <div className="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-950 space-y-3">
                    <div>
                      <div className="text-[11px] uppercase tracking-wide text-slate-400">Выбранная зона</div>
                      <div className="mt-1 flex items-center justify-between rounded-md border border-slate-200 px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:text-slate-100">
                        <span>{resolvedProjectTimezone || "UTC"}</span>
                        <span className="text-xs text-slate-500 dark:text-slate-400">
                          {getTimezoneOffsetLabel(resolvedProjectTimezone)}
                        </span>
                      </div>
                    </div>
                    {recentFiltered.length > 0 && (
                      <div>
                        <div className="text-[11px] uppercase tracking-wide text-slate-400">Недавние</div>
                        <div className="mt-1 flex flex-wrap gap-2">
                          {recentFiltered.map((tz) => (
                            <button
                              key={`recent-${tz}`}
                              type="button"
                              onClick={() => {
                                setProjectTimezone(tz);
                                updateRecentTimezone(tz);
                              }}
                              className="inline-flex items-center gap-2 rounded-full border border-slate-200 px-3 py-1 text-xs text-slate-700 hover:bg-slate-100 dark:border-slate-800 dark:text-slate-200 dark:hover:bg-slate-900"
                            >
                              <span>{tz}</span>
                              <span className="text-[10px] text-slate-500 dark:text-slate-400">
                                {getTimezoneOffsetLabel(tz)}
                              </span>
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                    <div>
                      <div className="text-[11px] uppercase tracking-wide text-slate-400">Все таймзоны</div>
                      <div className="mt-2 max-h-64 space-y-3 overflow-auto pr-2">
                        {timezoneGroups.length === 0 && (
                          <div className="text-sm text-slate-500 dark:text-slate-400">Ничего не найдено</div>
                        )}
                        {timezoneGroups.map(([group, items]) => (
                          <div key={group} className="space-y-2">
                            <div className="text-xs font-semibold uppercase text-slate-500 dark:text-slate-400">
                              {group}
                            </div>
                            <div className="grid gap-2">
                              {items.map((tz) => (
                                <button
                                  key={tz}
                                  type="button"
                                  onClick={() => {
                                    setProjectTimezone(tz);
                                    updateRecentTimezone(tz);
                                  }}
                                  className={`flex items-center justify-between rounded-md border px-3 py-2 text-sm ${
                                    tz === projectTimezone
                                      ? "border-indigo-400 bg-indigo-50 text-indigo-700 dark:border-indigo-500/60 dark:bg-indigo-500/10 dark:text-indigo-200"
                                      : "border-slate-200 text-slate-700 hover:bg-slate-100 dark:border-slate-800 dark:text-slate-200 dark:hover:bg-slate-900"
                                  }`}
                                >
                                  <span>{tz}</span>
                                  <span className="text-xs text-slate-500 dark:text-slate-400">
                                    {getTimezoneOffsetLabel(tz)}
                                  </span>
                                </button>
                              ))}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div className="border-t border-slate-200 dark:border-slate-800 pt-4 space-y-4">
              <h3 className="font-semibold">Участники проекта</h3>
              <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
                <h4 className="font-semibold">Добавить участника</h4>
                <div className="grid gap-3 md:grid-cols-3">
                  <input
                    className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                    placeholder="email@example.com"
                    value={newMemberEmail}
                    onChange={(e) => setNewMemberEmail(e.target.value)}
                  />
                  <select
                    className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                    value={newMemberRole}
                    onChange={(e) => setNewMemberRole(e.target.value)}
                  >
                    <option value="viewer">Наблюдатель</option>
                    <option value="editor">Редактор</option>
                    <option value="owner">Владелец</option>
                  </select>
                  <button
                    onClick={addMember}
                    disabled={loading || !newMemberEmail.trim()}
                    className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
                  >
                    Добавить
                  </button>
                </div>
              </div>

              <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
                <div className="flex items-center justify-between">
                  <h4 className="font-semibold">Список участников</h4>
                  <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {members.length}</span>
                </div>
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                        <th className="py-2 pr-4">Почта</th>
                        <th className="py-2 pr-4">Роль</th>
                        <th className="py-2 pr-4">Добавлен</th>
                        <th className="py-2 pr-4 text-right">Действия</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                      {members.map((m) => (
                        <tr key={m.email}>
                          <td className="py-3 pr-4 font-medium">{m.email}</td>
                          <td className="py-3 pr-4">
                            {m.role === "owner" ? (
                              <span className="text-sm text-slate-600 dark:text-slate-400">Владелец</span>
                            ) : (
                              <select
                                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                                value={m.role}
                                onChange={(e) => updateMemberRole(m.email, e.target.value)}
                              >
                                <option value="viewer">Наблюдатель</option>
                                <option value="editor">Редактор</option>
                                <option value="owner">Владелец</option>
                              </select>
                            )}
                          </td>
                          <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                            {formatDateTime(m.createdAt)}
                          </td>
                          <td className="py-3 pr-4 text-right">
                            {m.role !== "owner" && (
                              <button
                                onClick={() => removeMember(m.email)}
                                disabled={loading}
                                className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                              >
                                <FiX /> Удалить
                              </button>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {members.length === 0 && <div className="text-sm text-slate-500 mt-2">Участников пока нет.</div>}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function RunsList({ runs }: { runs: Generation[] }) {
  if (!Array.isArray(runs) || runs.length === 0) return null;
  // Показываем только последние 4 запуска
  const displayRuns = runs.slice(0, 4);
  return (
    <div className="mt-2 text-left text-xs bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700 rounded-lg p-2 space-y-1">
      {displayRuns.map((r) => (
        <Link
          key={r.id}
          href={`/queue/${r.id}`}
          className="flex items-center justify-between rounded-lg px-2 py-1 hover:bg-slate-100 dark:hover:bg-slate-700/60"
        >
          <span className="font-semibold">{r.id.slice(0, 8)}</span>
          <div className="flex items-center gap-2">
            <StatusBadge status={r.status} />
            <span className="text-slate-500 dark:text-slate-400">{r.progress}%</span>
            {r.error && <span className="text-red-500">ошибка</span>}
          </div>
        </Link>
      ))}
      {runs.length > 4 && (
        <div className="text-xs text-slate-500 dark:text-slate-400 px-2 py-1">
          ... и еще {runs.length - 4} запусков
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; tone: "slate" | "amber" | "green" | "yellow" | "orange" | "red"; icon: ReactNode }> = {
    waiting: { text: "Ожидает генерацию", tone: "slate", icon: <FiClock /> },
    processing: { text: "Генерация", tone: "amber", icon: <FiPlay /> },
    published: { text: "Опубликован", tone: "green", icon: <FiPlay /> },
    draft: { text: "Черновик", tone: "slate", icon: <FiPauseCircle /> },
    active: { text: "Активен", tone: "green", icon: <FiPlay /> },
    paused: { text: "Приостановлено", tone: "slate", icon: <FiPauseCircle /> },
    pause_requested: { text: "Пауза запрошена", tone: "yellow", icon: <FiPauseCircle /> },
    cancelling: { text: "Отмена...", tone: "orange", icon: <FiPauseCircle /> },
    cancelled: { text: "Отменено", tone: "red", icon: <FiPauseCircle /> },
    pending: { text: "В очереди", tone: "amber", icon: <FiClock /> },
    success: { text: "Готово", tone: "green", icon: <FiCheck /> },
    error: { text: "Ошибка", tone: "red", icon: <FiPauseCircle /> },
    running: { text: "В работе", tone: "amber", icon: <FiPlay /> },
  };
  const cfg = map[status] || { text: status, tone: "slate" as const, icon: <FiPauseCircle /> };
  return <Badge label={cfg.text} tone={cfg.tone} icon={cfg.icon} className="text-xs" />;
}

function LinkStatusBadge({ domain }: { domain: { link_anchor_text?: string; link_acceptor_url?: string; link_status?: string; link_last_task_id?: string } }) {
  const hasSettings = Boolean((domain.link_anchor_text || "").trim()) && Boolean((domain.link_acceptor_url || "").trim());
  if (!hasSettings) {
    return <Badge label="Ожидает настройки" tone="slate" icon={<FiClock />} className="text-xs" />;
  }
  const status = domain.link_status || (domain.link_last_task_id ? "pending" : "ready");
  const map: Record<string, { text: string; tone: "slate" | "amber" | "blue" | "orange" | "sky" | "green" | "yellow" | "red"; icon: ReactNode }> = {
    ready: { text: "Готово к запуску", tone: "slate", icon: <FiClock /> },
    pending: { text: "Ожидает добавления", tone: "amber", icon: <FiClock /> },
    searching: { text: "Поиск места", tone: "blue", icon: <FiRefreshCw /> },
    removing: { text: "Удаление", tone: "orange", icon: <FiRefreshCw /> },
    found: { text: "Место найдено", tone: "sky", icon: <FiCheck /> },
    inserted: { text: "Ссылка вставлена", tone: "green", icon: <FiCheck /> },
    generated: { text: "Вставлено (ген. текст)", tone: "yellow", icon: <FiCheck /> },
    removed: { text: "Ссылка удалена", tone: "slate", icon: <FiCheck /> },
    failed: { text: "Ошибка ссылки", tone: "red", icon: <FiPauseCircle /> },
  };
  const cfg = map[status] || map.ready;
  return <Badge label={cfg.text} tone={cfg.tone} icon={cfg.icon} className="text-xs" />;
}
