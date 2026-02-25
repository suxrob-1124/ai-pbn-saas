export type EditorFileMeta = {
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
  editable: boolean;
};

export type EditorFileNode = {
  name: string;
  path: string;
  isDir: boolean;
  children?: EditorFileNode[];
  file?: EditorFileMeta;
};

export type EditorSelectionState = {
  selectedPath: string;
  selectedFileId: string;
  language: string;
  mimeType: string;
  version: number;
};

export type EditorDirtyState = {
  isDirty: boolean;
  originalContent: string;
  currentContent: string;
  lastSavedAt?: string;
};

export type EditorRevision = {
  id: string;
  version: number;
  editedBy: string;
  source: string;
  description?: string;
  createdAt: string;
};
