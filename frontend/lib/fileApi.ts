import { authFetch } from "./http";

export type FileListItem = {
  id: string;
  path: string;
  size: number;
  mimeType: string;
  updatedAt: string;
};

export type FileContent = {
  content: string;
  mimeType: string;
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
  description?: string
): Promise<SaveFileResponse> {
  const encodedPath = encodeFilePath(path);
  const payload: { content: string; description?: string } = { content };
  if (description && description.trim()) {
    payload.description = description.trim();
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
