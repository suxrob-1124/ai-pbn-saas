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
  timezone?: string;
};

export type ScheduleUpdateInput = {
  name?: string;
  description?: string;
  strategy?: ScheduleStrategy;
  config?: Record<string, unknown>;
  isActive?: boolean;
  timezone?: string;
};

export type ScheduleTriggerResponse = {
  status: string;
  enqueued?: number;
};

export type LinkDomainEligibility = {
  id: string;
  url: string;
  eligible: boolean;
  reason: string;
  link_status: string | null;
};

export type LinkEligibilityDTO = {
  schedule: {
    is_active: boolean;
    next_run_at: string | null;
    last_run_at: string | null;
  } | null;
  domains: LinkDomainEligibility[];
  summary: {
    eligible_count: number;
    ineligible_count: number;
    active_task_count: number;
  };
};

export type SkipDetail = {
  domain_id: string;
  domain_url: string;
  reason: string;
};

export type ScheduleRunLog = {
  id: string;
  schedule_id: string;
  schedule_type: string;
  project_id: string;
  run_at: string;
  total_domains: number;
  eligible_count: number;
  enqueued_count: number;
  skipped_count: number;
  skip_details: SkipDetail[];
  error_message?: string;
  next_run_at?: string;
  created_at: string;
};

export type SchedulePreviewDomain = {
  id: string;
  url: string;
};

export type SchedulePreviewSection = {
  has_schedule: boolean;
  is_active: boolean;
  next_run_at?: string;
  eligible_domains: SchedulePreviewDomain[];
  would_skip: SkipDetail[];
  would_enqueue: number;
};

export type SchedulePreviewResponse = {
  generation: SchedulePreviewSection;
  link: SchedulePreviewSection;
};
