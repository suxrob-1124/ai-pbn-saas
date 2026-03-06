import { authFetch, post } from "./http";
import type {
  LegacyImportJob,
  LegacyImportJobDetail,
  LegacyImportPreviewItem,
} from "../types/legacyImport";

export function previewLegacyImport(
  projectId: string,
  domainIds: string[]
): Promise<LegacyImportPreviewItem[]> {
  return post<LegacyImportPreviewItem[]>(
    `/api/projects/${encodeURIComponent(projectId)}/legacy-import/preview`,
    { domain_ids: domainIds }
  );
}

export function startLegacyImport(
  projectId: string,
  domainIds: string[],
  force: boolean
): Promise<{ job_id: string }> {
  return post<{ job_id: string }>(
    `/api/projects/${encodeURIComponent(projectId)}/legacy-import`,
    { domain_ids: domainIds, force }
  );
}

export function getLegacyImportJob(
  projectId: string,
  jobId: string
): Promise<LegacyImportJobDetail> {
  return authFetch<LegacyImportJobDetail>(
    `/api/projects/${encodeURIComponent(projectId)}/legacy-import/${encodeURIComponent(jobId)}`
  );
}

export function listLegacyImportJobs(
  projectId: string
): Promise<LegacyImportJob[]> {
  return authFetch<LegacyImportJob[]>(
    `/api/projects/${encodeURIComponent(projectId)}/legacy-import`
  );
}
