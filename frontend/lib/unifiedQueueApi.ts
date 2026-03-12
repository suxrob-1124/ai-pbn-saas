import { authFetch } from './http';

export type UnifiedQueueItem = {
  id: string;
  type: 'generation' | 'link';
  domain_id: string;
  domain_url?: string;
  project_id?: string;
  status: 'pending' | 'processing' | 'completed' | 'failed';
  status_detail: string;
  error_message?: string;
  scheduled_for: string;
  created_at: string;
  anchor_text?: string;
  target_url?: string;
};

type UnifiedQueueParams = {
  type?: 'all' | 'generation' | 'link';
  status?: 'all' | 'pending' | 'processing' | 'completed' | 'failed';
  limit?: number;
  page?: number;
};

export async function getProjectUnifiedQueue(
  projectId: string,
  params: UnifiedQueueParams = {}
): Promise<UnifiedQueueItem[]> {
  const qs = new URLSearchParams();
  if (params.type && params.type !== 'all') qs.set('type', params.type);
  if (params.status && params.status !== 'all') qs.set('status', params.status);
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.page) qs.set('page', String(params.page));
  const query = qs.toString();
  return authFetch<UnifiedQueueItem[]>(
    `/api/projects/${encodeURIComponent(projectId)}/queue/unified${query ? `?${query}` : ''}`
  );
}

export async function getGlobalUnifiedQueue(
  params: UnifiedQueueParams = {}
): Promise<UnifiedQueueItem[]> {
  const qs = new URLSearchParams();
  if (params.type && params.type !== 'all') qs.set('type', params.type);
  if (params.status && params.status !== 'all') qs.set('status', params.status);
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.page) qs.set('page', String(params.page));
  const query = qs.toString();
  return authFetch<UnifiedQueueItem[]>(`/api/queue/unified${query ? `?${query}` : ''}`);
}
