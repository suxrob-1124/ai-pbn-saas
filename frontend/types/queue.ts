export type QueueItemDTO = {
  id: string;
  domain_id: string;
  domain_url?: string;
  schedule_id?: string;
  priority: number;
  scheduled_for: string;
  status: string;
  error_message?: string;
  created_at: string;
  processed_at?: string;
};
