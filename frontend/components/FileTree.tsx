"use client";

import { useEffect, useMemo, useState } from "react";
import { FiCode, FiFile, FiFileText, FiFolder, FiImage, FiTrash2 } from "react-icons/fi";

import { isImageLikeFile } from "../features/editor-v3/services/editorPreviewUtils";
import type { EditorFileMeta, EditorFileNode } from "../types/editor";

type FileTreeProps = {
  files: EditorFileMeta[];
  selectedPath?: string;
  selectedFolderPath?: string;
  loading?: boolean;
  onSelect: (file: EditorFileMeta) => void;
  onSelectFolder?: (path: string) => void;
  onDeleteFolder?: (path: string) => void;
  canManageFolders?: boolean;
};

function fileLabelIcon(file: EditorFileMeta) {
  const mime = (file.mimeType || "").toLowerCase();
  const path = file.path.toLowerCase();
  if (isImageLikeFile(path, mime)) {
    return <FiImage className="h-4 w-4" />;
  }
  if (path.endsWith(".js") || path.endsWith(".ts") || path.endsWith(".tsx") || path.endsWith(".jsx") || path.endsWith(".css")) {
    return <FiCode className="h-4 w-4" />;
  }
  if (mime.startsWith("text/") || path.endsWith(".html") || path.endsWith(".xml") || path.endsWith(".json") || path.endsWith(".md")) {
    return <FiFileText className="h-4 w-4" />;
  }
  return <FiFile className="h-4 w-4" />;
}

function buildTree(files: EditorFileMeta[]): EditorFileNode[] {
  const root: EditorFileNode = { name: "", path: "", isDir: true, children: [] };

  for (const file of files) {
    const normalizedPath = file.path.replace(/\/+$/, "");
    const parts = normalizedPath.split("/").filter(Boolean);
    if (parts.length === 0) {
      continue;
    }
    const isDirEntry = (file.mimeType || "").toLowerCase() === "inode/directory";
    let cursor = root;
    for (let i = 0; i < parts.length; i += 1) {
      const part = parts[i];
      const isLast = i === parts.length - 1;
      const fullPath = parts.slice(0, i + 1).join("/");
      const nodeIsDir = !isLast || (isLast && isDirEntry);
      cursor.children = cursor.children || [];
      let child = cursor.children.find((item) => item.name === part);
      if (!child) {
        child = {
          name: part,
          path: fullPath,
          isDir: nodeIsDir,
          children: nodeIsDir ? [] : undefined,
          file: isLast && !nodeIsDir ? file : undefined
        };
        cursor.children.push(child);
      }
      if (nodeIsDir) {
        child.isDir = true;
        child.children = child.children || [];
      }
      if (isLast && !nodeIsDir) {
        child.file = file;
      }
      cursor = child;
    }
  }

  const sortNodes = (nodes: EditorFileNode[]): EditorFileNode[] => {
    nodes.sort((a, b) => {
      if (a.isDir !== b.isDir) {
        return a.isDir ? -1 : 1;
      }
      return a.name.localeCompare(b.name, "ru");
    });
    for (const node of nodes) {
      if (node.children && node.children.length > 0) {
        node.children = sortNodes(node.children);
      }
    }
    return nodes;
  };

  return sortNodes(root.children || []);
}

function parentPaths(pathValue: string): string[] {
  const parts = pathValue.split("/").filter(Boolean);
  const out: string[] = [];
  for (let i = 0; i < parts.length - 1; i += 1) {
    out.push(parts.slice(0, i + 1).join("/"));
  }
  return out;
}

export function FileTree({
  files,
  selectedPath,
  selectedFolderPath,
  loading,
  onSelect,
  onSelectFolder,
  onDeleteFolder,
  canManageFolders
}: FileTreeProps) {
  const tree = useMemo(() => buildTree(files), [files]);
  const [openPaths, setOpenPaths] = useState<Record<string, boolean>>({});

  useEffect(() => {
    if (!selectedPath) {
      return;
    }
    const parents = parentPaths(selectedPath);
    if (parents.length === 0) {
      return;
    }
    setOpenPaths((prev) => {
      const next = { ...prev };
      for (const p of parents) {
        next[p] = true;
      }
      return next;
    });
  }, [selectedPath]);

  const toggleFolder = (pathValue: string) => {
    setOpenPaths((prev) => ({ ...prev, [pathValue]: !prev[pathValue] }));
  };

  const renderNode = (node: EditorFileNode, depth: number) => {
    if (node.isDir) {
      const isOpen = openPaths[node.path] ?? depth < 1;
      const selected = Boolean(node.path) && selectedFolderPath === node.path;
      return (
        <div key={node.path}>
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => {
                toggleFolder(node.path);
                if (node.path) {
                  onSelectFolder?.(node.path);
                }
              }}
              className={`w-full flex items-center gap-2 rounded-md px-2 py-1 text-left text-sm ${
                selected
                  ? "bg-indigo-600 text-white"
                  : "text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800"
              }`}
              style={{ paddingLeft: `${8 + depth * 14}px` }}
            >
              <FiFolder className="h-4 w-4" />
              <span className="truncate">{node.name}</span>
            </button>
            {canManageFolders && onDeleteFolder && node.path ? (
              <button
                type="button"
                onClick={(event) => {
                  event.stopPropagation();
                  onDeleteFolder(node.path);
                }}
                title={`Удалить папку ${node.path}`}
                className="shrink-0 rounded-md border border-red-200 bg-red-50 px-1.5 py-1 text-red-700 hover:bg-red-100 dark:border-red-900 dark:bg-red-900/20 dark:text-red-200"
              >
                <FiTrash2 className="h-3.5 w-3.5" />
              </button>
            ) : null}
          </div>
          {isOpen && node.children && node.children.length > 0 && (
            <div>{node.children.map((child) => renderNode(child, depth + 1))}</div>
          )}
        </div>
      );
    }

    const file = node.file;
    if (!file) {
      return null;
    }
    const selected = selectedPath === file.path;
    return (
      <button
        key={file.path}
        type="button"
        onClick={() => onSelect(file)}
        className={`w-full flex items-center gap-2 rounded-md px-2 py-1 text-left text-sm ${
          selected
            ? "bg-indigo-600 text-white"
            : "text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800"
        }`}
        style={{ paddingLeft: `${8 + depth * 14}px` }}
      >
        {fileLabelIcon(file)}
        <span className="truncate">{node.name}</span>
        {isImageLikeFile(file.path, file.mimeType) && file.width && file.height && (
          <span className={`ml-auto text-[10px] ${selected ? "text-indigo-100" : "text-slate-400"}`}>
            {file.width}x{file.height}
          </span>
        )}
      </button>
    );
  };

  if (loading) {
    return <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка файлов...</div>;
  }

  if (files.length === 0) {
    return <div className="text-sm text-slate-500 dark:text-slate-400">Файлы не найдены.</div>;
  }

  return <div className="space-y-0.5">{tree.map((node) => renderNode(node, 0))}</div>;
}
