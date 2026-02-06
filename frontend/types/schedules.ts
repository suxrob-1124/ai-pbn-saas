export type ScheduleStrategy = "immediate" | "daily" | "weekly" | "custom" | string;

export type ScheduleDTO = {
  id: string;
  project_id: string;
  name: string;
  description?: string;
  strategy: string;
  config: Record<string, unknown>;
  isActive: boolean;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  nextRunAt?: string;
  lastRunAt?: string;
  timezone?: string;
};

export type ScheduleCreateInput = {
  name: string;
  description?: string;
  strategy: ScheduleStrategy;
  config: Record<string, unknown>;
  isActive?: boolean;
};

export type ScheduleUpdateInput = {
  name?: string;
  description?: string;
  strategy?: ScheduleStrategy;
  config?: Record<string, unknown>;
  isActive?: boolean;
};

export type ScheduleTriggerResponse = {
  status: string;
  enqueued?: number;
};
