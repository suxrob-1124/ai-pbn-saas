export type EditorFileMeta = {
  id: string;
  path: string;
  size: number;
  mimeType: string;
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
};

export type EditorDirtyState = {
  isDirty: boolean;
  originalContent: string;
  currentContent: string;
  lastSavedAt?: string;
};
