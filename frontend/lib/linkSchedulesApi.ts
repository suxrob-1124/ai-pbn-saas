import { authFetch } from "./http";
import type {
  ScheduleCreateInput,
  ScheduleDTO,
  ScheduleTriggerResponse,
  ScheduleUpdateInput
} from "../types/schedules";

const encodeProjectId = (projectId: string) => {
  const trimmed = projectId.trim();
  if (!trimmed) {
    throw new Error("projectId is required");
  }
  return encodeURIComponent(trimmed);
};

const base = (projectId: string) =>
  `/api/projects/${encodeProjectId(projectId)}/link-schedule`;

const isNotFound = (err: unknown) => {
  const message = err instanceof Error ? err.message : String(err);
  return /not found|404/i.test(message);
};

const normalizeConfig = (config: Record<string, unknown>) => {
  if (!config || Object.keys(config).length === 0) {
    throw new Error("config is required");
  }
  return config;
};

const buildPayload = (input: ScheduleCreateInput | ScheduleUpdateInput) => {
  const payload: Record<string, unknown> = {};
  if ("name" in input && input.name !== undefined) {
    const trimmed = input.name.trim();
    if (!trimmed) {
      throw new Error("name is required");
    }
    payload.name = trimmed;
  }
  if ("description" in input && input.description !== undefined) {
    const trimmed = input.description?.trim();
    if (trimmed) {
      payload.description = trimmed;
    }
  }
  if ("strategy" in input && input.strategy !== undefined) {
    const trimmed = String(input.strategy).trim();
    if (!trimmed) {
      throw new Error("strategy is required");
    }
    payload.strategy = trimmed;
  }
  if ("config" in input && input.config !== undefined) {
    payload.config = normalizeConfig(input.config);
  }
  if ("isActive" in input && typeof input.isActive === "boolean") {
    payload.isActive = input.isActive;
  }
  return payload;
};

/** Получить расписание ссылок (одно на проект). */
export async function getLinkSchedule(projectId: string): Promise<ScheduleDTO | null> {
  try {
    return await authFetch<ScheduleDTO>(base(projectId));
  } catch (err) {
    if (isNotFound(err)) {
      return null;
    }
    throw err;
  }
}

/** Создать или обновить расписание ссылок (upsert). */
export async function upsertLinkSchedule(
  projectId: string,
  input: ScheduleCreateInput
): Promise<ScheduleDTO> {
  const payload = buildPayload(input);
  return authFetch<ScheduleDTO>(base(projectId), {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

/** Удалить расписание ссылок. */
export async function deleteLinkSchedule(projectId: string): Promise<{ status: string }> {
  return authFetch<{ status: string }>(base(projectId), { method: "DELETE" });
}

/** Запустить расписание ссылок вручную. */
export async function triggerLinkSchedule(
  projectId: string
): Promise<ScheduleTriggerResponse> {
  return authFetch<ScheduleTriggerResponse>(`${base(projectId)}/trigger`, {
    method: "POST"
  });
}
