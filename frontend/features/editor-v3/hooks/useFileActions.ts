import type { Dispatch, RefObject, SetStateAction } from "react";

import {
  createFileOrDir,
  deleteFile,
  listFiles,
  moveFile,
  restoreFile,
  uploadFile,
  type FileListItem
} from "../../../lib/fileApi";
import { showToast } from "../../../lib/toastStore";
import type { EditorDirtyState, EditorFileMeta, EditorSelectionState } from "../../../types/editor";

type UseFileActionsParams = {
  domainId: string;
  readOnly: boolean;
  files: EditorFileMeta[];
  selection: EditorSelectionState | null;
  selectedFolderPath: string;
  fileInputRef: RefObject<HTMLInputElement | null>;
  setFiles: Dispatch<SetStateAction<EditorFileMeta[]>>;
  setDeletedFiles: Dispatch<SetStateAction<EditorFileMeta[]>>;
  setSelection: Dispatch<SetStateAction<EditorSelectionState | null>>;
  setSelectedFolderPath: Dispatch<SetStateAction<string>>;
  setDirtyState: Dispatch<SetStateAction<EditorDirtyState>>;
  loadFile: (file: EditorFileMeta, options?: { line?: number }) => Promise<void>;
};

function toEditorFileMeta(item: FileListItem): EditorFileMeta {
  return {
    id: item.id,
    path: item.path,
    size: item.size,
    mimeType: item.mimeType,
    version: item.version || 1,
    isEditable: Boolean(item.isEditable),
    isBinary: Boolean(item.isBinary),
    width: item.width,
    height: item.height,
    lastEditedBy: item.lastEditedBy,
    updatedAt: item.updatedAt,
    editable: Boolean(item.isEditable),
  };
}

export function useFileActions(params: UseFileActionsParams) {
  const {
    domainId,
    readOnly,
    files,
    selection,
    selectedFolderPath,
    fileInputRef,
    setFiles,
    setDeletedFiles,
    setSelection,
    setSelectedFolderPath,
    setDirtyState,
    loadFile,
  } = params;

  const loadFiles = async () => {
    const fileList = await listFiles(domainId);
    const prepared: EditorFileMeta[] = (Array.isArray(fileList) ? fileList : []).map((item: FileListItem) =>
      toEditorFileMeta(item)
    );
    setFiles(prepared);
    return prepared;
  };

  const loadDeletedFiles = async () => {
    const fileList = await listFiles(domainId, { includeDeleted: true });
    const prepared: EditorFileMeta[] = (Array.isArray(fileList) ? fileList : [])
      .filter((item: FileListItem) => Boolean(item.deletedAt))
      .map((item: FileListItem) => toEditorFileMeta(item));
    setDeletedFiles(prepared);
    return prepared;
  };

  const onCreateFile = async () => {
    if (readOnly) return;
    const nextPath = prompt("Путь нового файла (например: pages/about.html)");
    if (!nextPath) return;
    try {
      await createFileOrDir(domainId, { kind: "file", path: nextPath, content: "" });
      const nextFiles = await loadFiles();
      const created = nextFiles.find((item) => item.path === nextPath);
      if (created) {
        await loadFile(created);
      }
      showToast({ type: "success", title: "Файл создан", message: nextPath });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось создать файл", message: err?.message || "unknown error" });
    }
  };

  const onCreateFolder = async () => {
    if (readOnly) return;
    const nextPath = prompt("Путь новой папки (например: pages/blog)");
    if (!nextPath) return;
    try {
      await createFileOrDir(domainId, { kind: "dir", path: nextPath });
      await loadFiles();
      setSelectedFolderPath(nextPath.replace(/^\/+|\/+$/g, ""));
      setSelection(null);
      showToast({ type: "success", title: "Папка создана", message: nextPath });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось создать папку", message: err?.message || "unknown error" });
    }
  };

  const onRename = async () => {
    if (readOnly) return;
    const currentPath = selection?.selectedPath || selectedFolderPath;
    if (!currentPath) return;
    const isFolder = !selection?.selectedPath && Boolean(selectedFolderPath);
    const parts = currentPath.split("/").filter(Boolean);
    const currentName = parts.pop() || currentPath;
    const parent = parts.join("/");
    const nextName = (prompt(isFolder ? "Новое имя папки" : "Новое имя файла", currentName) || "").trim();
    if (!nextName || nextName === currentName) return;
    if (nextName.includes("/")) {
      showToast({ type: "error", title: "Некорректное имя", message: "Имя не должно содержать /" });
      return;
    }
    const nextPath = parent ? `${parent}/${nextName}` : nextName;
    try {
      await moveFile(domainId, currentPath, nextPath);
      const nextFiles = await loadFiles();
      if (isFolder) {
        setSelectedFolderPath(nextPath);
        setSelection(null);
        setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
        showToast({ type: "success", title: "Папка переименована", message: `${currentName} → ${nextName}` });
      } else {
        const moved = nextFiles.find((item) => item.path === nextPath);
        if (moved) {
          await loadFile(moved);
        }
        showToast({ type: "success", title: "Файл переименован", message: `${currentName} → ${nextName}` });
      }
    } catch (err: any) {
      showToast({
        type: "error",
        title: isFolder ? "Не удалось переименовать папку" : "Не удалось переименовать файл",
        message: err?.message || "unknown error",
      });
    }
  };

  const onMove = async () => {
    if (readOnly) return;
    const currentPath = selection?.selectedPath || selectedFolderPath;
    if (!currentPath) return;
    const isFolder = !selection?.selectedPath && Boolean(selectedFolderPath);
    const parts = currentPath.split("/").filter(Boolean);
    const currentName = parts.pop() || currentPath;
    const currentDir = parts.join("/");
    const destinationRaw = prompt(
      "Папка назначения (например: pages/archive). Пусто = корень.",
      currentDir
    );
    if (destinationRaw === null) return;
    const destination = destinationRaw.trim().replace(/^\/+|\/+$/g, "");
    const nextPath = destination ? `${destination}/${currentName}` : currentName;
    if (nextPath === currentPath) return;
    try {
      await moveFile(domainId, currentPath, nextPath);
      const nextFiles = await loadFiles();
      if (isFolder) {
        setSelectedFolderPath(nextPath);
        setSelection(null);
        setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
      } else {
        const moved = nextFiles.find((item) => item.path === nextPath);
        if (moved) {
          await loadFile(moved);
        }
      }
      showToast({
        type: "success",
        title: isFolder ? "Папка перемещена" : "Файл перемещен",
        message: `${currentPath} → ${nextPath}`,
      });
    } catch (err: any) {
      showToast({
        type: "error",
        title: isFolder ? "Не удалось переместить папку" : "Не удалось переместить файл",
        message: err?.message || "unknown error",
      });
    }
  };

  const onDelete = async () => {
    if (readOnly) return;
    const targetPath = selection?.selectedPath || "";
    if (targetPath) {
      if (!confirm(`Удалить "${targetPath}"?`)) return;
    } else if (selectedFolderPath) {
      await onDeleteFolder(selectedFolderPath);
      return;
    } else {
      return;
    }
    try {
      await deleteFile(domainId, targetPath);
      setSelection(null);
      setSelectedFolderPath("");
      setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
      await loadFiles();
      await loadDeletedFiles();
      showToast({ type: "success", title: "Файл удален", message: targetPath });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось удалить файл", message: err?.message || "unknown error" });
    }
  };

  const onDeleteFolder = async (folderPath: string) => {
    if (readOnly) return;
    const normalized = folderPath.trim().replace(/^\/+|\/+$/g, "");
    if (!normalized) return;
    const hasChildren = files.some((item) => item.path.startsWith(`${normalized}/`));
    const confirmed = hasChildren
      ? confirm(`Папка "${normalized}" содержит файлы. Удалить папку рекурсивно?`)
      : confirm(`Удалить пустую папку "${normalized}"?`);
    if (!confirmed) return;
    try {
      await deleteFile(domainId, normalized, { recursive: hasChildren });
      if (selection?.selectedPath === normalized || selection?.selectedPath?.startsWith(`${normalized}/`)) {
        setSelection(null);
        setDirtyState({ isDirty: false, originalContent: "", currentContent: "" });
      }
      if (selectedFolderPath === normalized || selectedFolderPath.startsWith(`${normalized}/`)) {
        setSelectedFolderPath("");
      }
      await loadFiles();
      await loadDeletedFiles();
      showToast({ type: "success", title: "Папка удалена", message: normalized });
    } catch (err: any) {
      if (String(err?.message || "").includes("recursive")) {
        const retry = confirm(`Папка "${normalized}" не пустая. Выполнить рекурсивное удаление?`);
        if (!retry) return;
        try {
          await deleteFile(domainId, normalized, { recursive: true });
          await loadFiles();
          await loadDeletedFiles();
          showToast({ type: "success", title: "Папка удалена", message: `${normalized} (recursive)` });
          return;
        } catch (retryErr: any) {
          showToast({ type: "error", title: "Не удалось удалить папку", message: retryErr?.message || "unknown error" });
          return;
        }
      }
      showToast({ type: "error", title: "Не удалось удалить папку", message: err?.message || "unknown error" });
    }
  };

  const onRestoreDeleted = async (file: EditorFileMeta) => {
    try {
      await restoreFile(domainId, file.path);
      const active = await loadFiles();
      await loadDeletedFiles();
      const restored = active.find((item) => item.path === file.path);
      if (restored) {
        await loadFile(restored);
      }
      showToast({ type: "success", title: "Файл восстановлен", message: file.path });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось восстановить файл", message: err?.message || "unknown error" });
    }
  };

  const onUploadClick = () => fileInputRef.current?.click();

  const onUploadInput = async (file?: File | null) => {
    if (readOnly || !file) return;
    const destination = prompt("Куда загрузить файл? (путь или папка)", file.name) || file.name;
    try {
      await uploadFile(domainId, file, destination);
      await loadFiles();
      showToast({ type: "success", title: "Файл загружен", message: destination });
    } catch (err: any) {
      showToast({ type: "error", title: "Не удалось загрузить файл", message: err?.message || "unknown error" });
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  return {
    loadFiles,
    loadDeletedFiles,
    onCreateFile,
    onCreateFolder,
    onRename,
    onMove,
    onDelete,
    onDeleteFolder,
    onRestoreDeleted,
    onUploadClick,
    onUploadInput,
  };
}
