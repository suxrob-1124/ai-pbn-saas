"use client";

import { useState } from "react";
import { apiBase } from "@/lib/http";

const IMAGE_EXTS = new Set([".webp", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".avif"]);

function isImagePath(p: string) {
  const ext = p.slice(p.lastIndexOf(".")).toLowerCase();
  return IMAGE_EXTS.has(ext);
}

type Props = {
  domainId: string;
  filePath: string;
};

export function AgentImagePreview({ domainId, filePath }: Props) {
  const [error, setError] = useState(false);

  if (!isImagePath(filePath) || error) return null;

  const encodedPath = filePath.split("/").map(encodeURIComponent).join("/");
  const src = `${apiBase()}/api/domains/${domainId}/files/${encodedPath}?raw=1`;

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 dark:border-slate-700 max-w-[240px]">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={src}
        alt={filePath}
        className="h-auto w-full object-cover"
        onError={() => setError(true)}
      />
      <div className="bg-slate-50 px-2 py-1 text-[10px] text-slate-500 dark:bg-slate-800 dark:text-slate-400 truncate">
        {filePath}
      </div>
    </div>
  );
}

export { isImagePath };
