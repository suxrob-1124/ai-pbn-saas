export type LLMUsageEventDTO = {
  id: string;
  created_at: string;
  provider: string;
  operation: string;
  stage?: string | null;
  model: string;
  status: "success" | "error" | string;
  requester_email: string;
  key_owner_email?: string | null;
  key_type?: "user" | "global" | string | null;
  project_id?: string | null;
  domain_id?: string | null;
  generation_id?: string | null;
  link_task_id?: string | null;
  file_path?: string | null;
  prompt_tokens?: number | null;
  completion_tokens?: number | null;
  total_tokens?: number | null;
  token_source: "provider" | "estimated" | "mixed" | string;
  estimated_cost_usd?: number | null;
};

export type LLMUsageListDTO = {
  items: LLMUsageEventDTO[];
  total: number;
};

export type LLMUsageBucketDTO = {
  key: string;
  requests: number;
  tokens: number;
  cost_usd: number;
};

export type LLMUsageStatsDTO = {
  total_requests: number;
  total_tokens: number;
  total_cost_usd: number;
  by_day: LLMUsageBucketDTO[];
  by_model: LLMUsageBucketDTO[];
  by_operation: LLMUsageBucketDTO[];
  by_user: LLMUsageBucketDTO[];
};

export type LLMPricingDTO = {
  id: string;
  provider: string;
  model: string;
  input_usd_per_million: number;
  output_usd_per_million: number;
  active_from: string;
  active_to?: string | null;
  is_active: boolean;
  updated_by: string;
  updated_at: string;
};

export type LLMUsageFilters = {
  from?: string | Date;
  to?: string | Date;
  userEmail?: string;
  projectId?: string;
  domainId?: string;
  model?: string;
  operation?: string;
  status?: string;
  page?: number;
  limit?: number;
};
