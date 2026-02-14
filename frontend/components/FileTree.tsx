"use client";

import { useEffect, useMemo, useState } from "react";
import { FiCode, FiFile, FiFileText, FiFolder, FiImage } from "react-icons/fi";

import type { EditorFileMeta, EditorFileNode } from "../types/editor";

type FileTreeProps = {
  files: EditorFileMeta[];
  selectedPath?: string;
  loading?: boolean;
  onSelect: (file: EditorFileMeta) => void;
};

function fileLabelIcon(file: EditorFileMeta) {
  const mime = (file.mimeType || "").toLowerCase();
  const path = file.path.toLowerCase();
  if (mime.startsWith("image/")) {
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
    const parts = file.path.split("/").filter(Boolean);
    let cursor = root;
    for (let i = 0; i < parts.length; i += 1) {
      const part = parts[i];
      const isLast = i === parts.length - 1;
      const fullPath = parts.slice(0, i + 1).join("/");
      cursor.children = cursor.children || [];
      let child = cursor.children.find((item) => item.name === part);
      if (!child) {
        child = {
          name: part,
          path: fullPath,
          isDir: !isLast,
          children: !isLast ? [] : undefined,
          file: isLast ? file : undefined
        };
        cursor.children.push(child);
      }
      if (isLast) {
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

export function FileTree({ files, selectedPath, loading, onSelect }: FileTreeProps) {
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
      return (
        <div key={node.path}>
          <button
            type="button"
            onClick={() => toggleFolder(node.path)}
            className="w-full flex items-center gap-2 rounded-md px-2 py-1 text-left text-sm text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800"
            style={{ paddingLeft: `${8 + depth * 14}px` }}
          >
            <FiFolder className="h-4 w-4" />
            <span>{node.name}</span>
          </button>
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
        {file.mimeType?.toLowerCase().startsWith("image/") && file.width && file.height && (
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
