import assert from "node:assert/strict";

import type { ScheduleDTO } from "../types/schedules";
import type { QueueItemDTO } from "../types/queue";
import type { LinkTaskDTO } from "../types/linkTasks";

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

const assertString = (value: unknown, field: string) => {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`invalid ${field}`);
  }
};

const assertNumber = (value: unknown, field: string) => {
  if (typeof value !== "number" || Number.isNaN(value)) {
    throw new Error(`invalid ${field}`);
  }
};

const assertScheduleDTO = (value: unknown) => {
  if (!isRecord(value)) {
    throw new Error("schedule must be object");
  }
  assertString(value.id, "id");
  assertString(value.project_id, "project_id");
  assertString(value.name, "name");
  assertString(value.strategy, "strategy");
  if (!isRecord(value.config)) {
    throw new Error("invalid config");
  }
  if (typeof value.isActive !== "boolean") {
    throw new Error("invalid isActive");
  }
  assertString(value.createdBy, "createdBy");
  assertString(value.createdAt, "createdAt");
  assertString(value.updatedAt, "updatedAt");
};

const assertQueueItemDTO = (value: unknown) => {
  if (!isRecord(value)) {
    throw new Error("queue item must be object");
  }
  assertString(value.id, "id");
  assertString(value.domain_id, "domain_id");
  assertNumber(value.priority, "priority");
  assertString(value.scheduled_for, "scheduled_for");
  assertString(value.status, "status");
  assertString(value.created_at, "created_at");
};

const assertLinkTaskDTO = (value: unknown) => {
  if (!isRecord(value)) {
    throw new Error("link task must be object");
  }
  assertString(value.id, "id");
  assertString(value.domain_id, "domain_id");
  assertString(value.anchor_text, "anchor_text");
  assertString(value.target_url, "target_url");
  assertString(value.scheduled_for, "scheduled_for");
  assertString(value.status, "status");
  assertNumber(value.attempts, "attempts");
  assertString(value.created_by, "created_by");
  assertString(value.created_at, "created_at");
};

const validSchedule: ScheduleDTO = {
  id: "sched-1",
  project_id: "project-1",
  name: "Daily",
  description: "desc",
  strategy: "daily",
  config: { cron: "0 9 * * *" },
  isActive: true,
  createdBy: "admin@example.com",
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString()
};

const validQueueItem: QueueItemDTO = {
  id: "queue-1",
  domain_id: "domain-1",
  schedule_id: "sched-1",
  priority: 0,
  scheduled_for: new Date().toISOString(),
  status: "pending",
  created_at: new Date().toISOString()
};

const validLinkTask: LinkTaskDTO = {
  id: "task-1",
  domain_id: "domain-1",
  anchor_text: "Example",
  target_url: "https://example.com",
  scheduled_for: new Date().toISOString(),
  status: "pending",
  attempts: 0,
  created_by: "admin@example.com",
  created_at: new Date().toISOString()
};

assertScheduleDTO(validSchedule);
assertQueueItemDTO(validQueueItem);
assertLinkTaskDTO(validLinkTask);

assert.throws(() => assertScheduleDTO({}), /invalid id/);
assert.throws(() => assertQueueItemDTO({ id: "x" }), /domain_id/);
assert.throws(() => assertLinkTaskDTO({ id: "x" }), /domain_id/);

console.log("OK");
