import { authFetch } from "./http";

export type FileListItem = {
  id: string;
  path: string;
  size: number;
  mimeType: string;
  version: number;
  isEditable: boolean;
  isBinary: boolean;
  width?: number;
  height?: number;
  lastEditedBy?: string;
  updatedAt: string;
};

export type FileContent = {
  content: string;
  mimeType: string;
  version?: number;
};

export type FileEditHistoryItem = {
  id: string;
  editedBy: string;
  editType: string;
  description?: string;
  createdAt: string;
};

export type SaveFileResponse = {
  status: string;
  version?: number;
};

export type FileMeta = FileListItem;

export type FileRevisionDTO = {
  id: string;
  file_id: string;
  version: number;
  edited_by: string;
  source: "manual" | "ai" | "revert" | string;
  description?: string;
  content_hash: string;
  size_bytes: number;
  mime_type: string;
  content?: string;
  created_at: string;
};

export type AIDiffSummary = {
  old_bytes: number;
  new_bytes: number;
};

export type AIEditorSuggestionDTO = {
  suggested_content: string;
  diff_summary?: AIDiffSummary;
  warnings?: string[];
  prompt_trace?: Record<string, any>;
  token_usage?: Record<string, any>;
  mime_type?: string;
};

export type AIPageSuggestionFile = {
  path: string;
  content: string;
  mime_type: string;
};

export type AIPageSuggestionDTO = {
  files: AIPageSuggestionFile[];
  warnings?: string[];
  prompt_trace?: Record<string, any>;
  token_usage?: Record<string, any>;
};

const encodeDomainId = (domainId: string) => encodeURIComponent(domainId.trim());

const encodeFilePath = (value: string) => {
  const trimmed = value.replace(/^\/+|\/+$/g, "");
  if (!trimmed) {
    throw new Error("path is required");
  }
  return trimmed
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");
};

const buildFilesBase = (domainId: string) => `/api/domains/${encodeDomainId(domainId)}/files`;

export async function listFiles(domainId: string): Promise<FileListItem[]> {
  return authFetch<FileListItem[]>(buildFilesBase(domainId));
}

export async function getFile(domainId: string, path: string): Promise<FileContent> {
  const encodedPath = encodeFilePath(path);
  return authFetch<FileContent>(`${buildFilesBase(domainId)}/${encodedPath}`);
}

export async function saveFile(
  domainId: string,
  path: string,
  content: string,
  description?: string,
  opts?: { expectedVersion?: number; source?: "manual" | "ai" | "revert" }
): Promise<SaveFileResponse> {
  const encodedPath = encodeFilePath(path);
  const payload: { content: string; description?: string; expected_version?: number; source?: string } = { content };
  if (description && description.trim()) {
    payload.description = description.trim();
  }
  if (typeof opts?.expectedVersion === "number" && opts.expectedVersion > 0) {
    payload.expected_version = opts.expectedVersion;
  }
  if (opts?.source) {
    payload.source = opts.source;
  }
  return authFetch<SaveFileResponse>(`${buildFilesBase(domainId)}/${encodedPath}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

export async function getFileHistory(
  fileId: string,
  domainId?: string
): Promise<FileEditHistoryItem[]> {
  if (!domainId || !domainId.trim()) {
    throw new Error("domainId is required to fetch file history");
  }
  const encodedFileId = encodeURIComponent(fileId.trim());
  return authFetch<FileEditHistoryItem[]>(
    `${buildFilesBase(domainId)}/${encodedFileId}/history`
  );
}

export async function createFileOrDir(
  domainId: string,
  payload: { path: string; kind: "file" | "dir"; content?: string; mime_type?: string }
) {
  return authFetch<any>(buildFilesBase(domainId), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

export async function moveFile(domainId: string, path: string, newPath: string) {
  const encodedPath = encodeFilePath(path);
  return authFetch<any>(`${buildFilesBase(domainId)}/${encodedPath}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ op: "move", new_path: newPath }),
  });
}

export async function deleteFile(domainId: string, path: string) {
  const encodedPath = encodeFilePath(path);
  return authFetch<any>(`${buildFilesBase(domainId)}/${encodedPath}`, {
    method: "DELETE",
  });
}

export async function uploadFile(domainId: string, file: File, path?: string) {
  const fd = new FormData();
  fd.append("file", file);
  if (path && path.trim()) {
    fd.append("path", path.trim());
  }
  return authFetch<FileListItem>(`${buildFilesBase(domainId)}/upload`, {
    method: "POST",
    body: fd,
  });
}

export async function getFileMeta(domainId: string, path: string) {
  const encodedPath = encodeFilePath(path);
  return authFetch<FileMeta>(`${buildFilesBase(domainId)}/${encodedPath}/meta`);
}

export async function getFileRevisionsByPath(domainId: string, path: string) {
  const encodedPath = encodeFilePath(path);
  return authFetch<FileRevisionDTO[]>(`${buildFilesBase(domainId)}/${encodedPath}/history`);
}

export async function revertFileToRevision(
  domainId: string,
  path: string,
  revisionId: string,
  description?: string
) {
  const encodedPath = encodeFilePath(path);
  const payload: { revision_id: string; description?: string } = { revision_id: revisionId };
  if (description && description.trim()) {
    payload.description = description.trim();
  }
  return authFetch<{ status: string; version?: number }>(`${buildFilesBase(domainId)}/${encodedPath}/revert`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

export async function aiSuggestFile(
  domainId: string,
  path: string,
  payload: {
    instruction: string;
    model?: string;
    selection?: string;
    context_files?: string[];
  }
) {
  const encodedPath = encodeFilePath(path);
  return authFetch<AIEditorSuggestionDTO>(`${buildFilesBase(domainId)}/${encodedPath}/ai-suggest`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

export async function aiCreatePage(
  domainId: string,
  payload: {
    instruction: string;
    target_path: string;
    with_assets: boolean;
    model?: string;
  }
) {
  return authFetch<AIPageSuggestionDTO>(`/api/domains/${encodeDomainId(domainId)}/editor/ai-create-page`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}
