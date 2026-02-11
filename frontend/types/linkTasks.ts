export type LinkTaskDTO = {
  id: string;
  domain_id: string;
  anchor_text: string;
  target_url: string;
  scheduled_for: string;
  action?: string;
  status: string;
  found_location?: string;
  generated_content?: string;
  error_message?: string;
  log_lines?: string[];
  attempts: number;
  created_by: string;
  created_at: string;
  completed_at?: string;
};

export type LinkTaskCreateInput = {
  anchorText: string;
  targetUrl: string;
  scheduledFor?: string | Date;
};

export type LinkTaskImportItem = {
  anchorText: string;
  targetUrl: string;
  scheduledFor?: string | Date;
};

export type LinkTaskImportInput = {
  items?: LinkTaskImportItem[];
  text?: string;
};

export type LinkTaskFilters = {
  status?: string;
  scheduledFrom?: string | Date;
  scheduledTo?: string | Date;
  limit?: number;
  page?: number;
  search?: string;
};

export type LinkTaskListParams = LinkTaskFilters & {
  domainId?: string;
  projectId?: string;
};
