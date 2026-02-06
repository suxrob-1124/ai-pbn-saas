import { authFetch } from "./http";
import type {
  ScheduleCreateInput,
  ScheduleDTO,
  ScheduleStrategy,
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

const encodeScheduleId = (scheduleId: string) => {
  const trimmed = scheduleId.trim();
  if (!trimmed) {
    throw new Error("scheduleId is required");
  }
  return encodeURIComponent(trimmed);
};

const buildSchedulesBase = (projectId: string) =>
  `/api/projects/${encodeProjectId(projectId)}/schedules`;

const normalizeConfig = (config: Record<string, unknown>) => {
  if (!config || Object.keys(config).length === 0) {
    throw new Error("config is required");
  }
  return config;
};

const buildSchedulePayload = (input: ScheduleCreateInput) => {
  const name = input.name?.trim();
  if (!name) {
    throw new Error("name is required");
  }
  const strategy = String(input.strategy || "").trim();
  if (!strategy) {
    throw new Error("strategy is required");
  }
  const payload: Record<string, unknown> = {
    name,
    strategy,
    config: normalizeConfig(input.config)
  };
  if (input.description && input.description.trim()) {
    payload.description = input.description.trim();
  }
  if (typeof input.isActive === "boolean") {
    payload.isActive = input.isActive;
  }
  return payload;
};

const buildScheduleUpdates = (updates: ScheduleUpdateInput) => {
  const payload: Record<string, unknown> = {};
  if (updates.name !== undefined) {
    const trimmed = updates.name.trim();
    if (!trimmed) {
      throw new Error("name cannot be empty");
    }
    payload.name = trimmed;
  }
  if (updates.description !== undefined) {
    const trimmed = updates.description.trim();
    payload.description = trimmed;
  }
  if (updates.strategy !== undefined) {
    const trimmed = String(updates.strategy).trim();
    if (!trimmed) {
      throw new Error("strategy cannot be empty");
    }
    payload.strategy = trimmed;
  }
  if (updates.config !== undefined) {
    payload.config = normalizeConfig(updates.config);
  }
  if (typeof updates.isActive === "boolean") {
    payload.isActive = updates.isActive;
  }
  if (Object.keys(payload).length === 0) {
    throw new Error("no updates provided");
  }
  return payload;
};

/** Получить список расписаний проекта. */
export async function listSchedules(projectId: string): Promise<ScheduleDTO[]> {
  return authFetch<ScheduleDTO[]>(buildSchedulesBase(projectId));
}

/** Получить одно расписание по ID. */
export async function getSchedule(
  projectId: string,
  scheduleId: string
): Promise<ScheduleDTO> {
  const encodedId = encodeScheduleId(scheduleId);
  return authFetch<ScheduleDTO>(`${buildSchedulesBase(projectId)}/${encodedId}`);
}

/** Создать расписание. */
export async function createSchedule(
  projectId: string,
  input: ScheduleCreateInput
): Promise<ScheduleDTO> {
  const payload = buildSchedulePayload(input);
  return authFetch<ScheduleDTO>(buildSchedulesBase(projectId), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

/** Обновить расписание. */
export async function updateSchedule(
  projectId: string,
  scheduleId: string,
  updates: ScheduleUpdateInput
): Promise<ScheduleDTO> {
  const encodedId = encodeScheduleId(scheduleId);
  const payload = buildScheduleUpdates(updates);
  return authFetch<ScheduleDTO>(`${buildSchedulesBase(projectId)}/${encodedId}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

/** Удалить расписание. */
export async function deleteSchedule(
  projectId: string,
  scheduleId: string
): Promise<{ status: string }> {
  const encodedId = encodeScheduleId(scheduleId);
  return authFetch<{ status: string }>(`${buildSchedulesBase(projectId)}/${encodedId}`, {
    method: "DELETE"
  });
}

/** Запустить расписание вручную. */
export async function triggerSchedule(
  projectId: string,
  scheduleId: string
): Promise<ScheduleTriggerResponse> {
  const encodedId = encodeScheduleId(scheduleId);
  return authFetch<ScheduleTriggerResponse>(
    `${buildSchedulesBase(projectId)}/${encodedId}/trigger`,
    { method: "POST" }
  );
}
