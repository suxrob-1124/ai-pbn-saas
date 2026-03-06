export interface LegacyImportJob {
  ID: string;
  ProjectID: string;
  RequestedBy: string;
  ForceOverwrite: boolean;
  Status: string; // pending | running | completed | failed
  TotalItems: number;
  CompletedItems: number;
  FailedItems: number;
  SkippedItems: number;
  Error: { String: string; Valid: boolean } | null;
  CreatedAt: string;
  StartedAt: { Time: string; Valid: boolean } | null;
  FinishedAt: { Time: string; Valid: boolean } | null;
  UpdatedAt: string;
}

export interface LegacyImportItem {
  ID: string;
  JobID: string;
  DomainID: string;
  DomainURL: string;
  Status: string; // pending | running | success | failed | skipped
  Step: string;   // ssh_probe | file_sync | link_decode | link_baseline | artifacts | inventory_update
  Progress: number;
  Error: { String: string; Valid: boolean } | null;
  Actions: string[] | null;
  Warnings: string[] | null;
  FileCount: number;
  TotalSizeBytes: number;
  StartedAt: { Time: string; Valid: boolean } | null;
  FinishedAt: { Time: string; Valid: boolean } | null;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface LegacyImportJobDetail extends LegacyImportJob {
  items: LegacyImportItem[];
}

export interface LegacyImportPreviewItem {
  domain_id: string;
  domain_url: string;
  has_files: boolean;
  has_link: boolean;
  link_anchor?: string;
  link_target?: string;
  has_legacy_artifacts: boolean;
  has_non_legacy_gen: boolean;
}
