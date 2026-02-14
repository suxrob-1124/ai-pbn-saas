export type IndexCheckStatus =
  | "pending"
  | "checking"
  | "success"
  | "failed_investigation"
  | string;

export type IndexCheckDTO = {
  id: string;
  domain_id: string;
  project_id?: string;
  domain_url?: string;
  check_date: string;
  status: IndexCheckStatus;
  is_indexed?: boolean | null;
  attempts: number;
  last_attempt_at?: string | null;
  next_retry_at?: string | null;
  error_message?: string | null;
  completed_at?: string | null;
  created_at: string;
  run_now_enqueued?: boolean;
  run_now_error?: string | null;
};

export type IndexCheckHistoryDTO = {
  id: string;
  check_id: string;
  attempt_number: number;
  result?: "success" | "error" | "timeout" | string | null;
  response_data?: Record<string, unknown> | null;
  error_message?: string | null;
  duration_ms?: number | null;
  created_at: string;
};

export type IndexCheckCalendarDayDTO = {
  date: string;
  total: number;
  indexed_true: number;
  indexed_false: number;
  pending: number;
  checking: number;
  failed_investigation: number;
  success: number;
};

export type IndexCheckStatsDTO = {
  from: string;
  to: string;
  total_checks: number;
  total_resolved: number;
  indexed_true: number;
  percent_indexed: number;
  avg_attempts_to_success: number;
  failed_investigation: number;
  daily: IndexCheckCalendarDayDTO[];
};

export type IndexChecksFilters = {
  status?: IndexCheckStatus;
  isIndexed?: boolean;
  from?: string | Date;
  to?: string | Date;
  limit?: number;
  offset?: number;
  page?: number;
  search?: string;
  sort?: string;
  domainId?: string;
};

export type IndexChecksResponse = {
  items: IndexCheckDTO[];
  total: number;
};

export type IndexCheckHistoryResponse = IndexCheckHistoryDTO[];

export type IndexCheckBatchResponse = {
  created: number;
  updated: number;
  skipped: number;
  upsert_failed?: number;
  enqueued?: number;
  enqueue_failed?: number;
};

export type IndexCheckCalendarResponse = IndexCheckCalendarDayDTO[];

export type IndexCheckStatsResponse = IndexCheckStatsDTO;

export type IndexCheck = IndexCheckDTO;
