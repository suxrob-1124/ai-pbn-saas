import type { AIPageSuggestionAsset } from "../../../lib/fileApi";
import type { EditorFileMeta } from "../../../types/editor";

export type AssetValidationIssueType = "missing" | "broken" | "invalid_mime";

export type AssetValidationIssue = {
  path: string;
  type: AssetValidationIssueType;
  message: string;
  source: "manifest" | "reference";
  expectedMimeType?: string;
  actualMimeType?: string;
  sizeBytes?: number;
};

const EXT_MIME: Record<string, string> = {
  webp: "image/webp",
  png: "image/png",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  gif: "image/gif",
  svg: "image/svg+xml",
};

const normalizeMime = (mime?: string) => {
  const value = (mime || "").trim().toLowerCase();
  if (!value) return "";
  if (value === "image/jpg") return "image/jpeg";
  return value;
};

const expectedMimeByPath = (pathValue: string) => {
  const ext = (pathValue.split(".").pop() || "").toLowerCase();
  return EXT_MIME[ext] || "";
};

const dedupeIssues = (issues: AssetValidationIssue[]) => {
  const keySet = new Set<string>();
  const out: AssetValidationIssue[] = [];
  for (const issue of issues) {
    const key = `${issue.path}:${issue.type}:${issue.source}`;
    if (keySet.has(key)) continue;
    keySet.add(key);
    out.push(issue);
  }
  return out;
};

export function validateEditorAssets(params: {
  manifestAssets: AIPageSuggestionAsset[];
  existingFilesMap: Map<string, EditorFileMeta>;
  missingPaths: string[];
  skippedPaths: string[];
  minImageBytes?: number;
}): AssetValidationIssue[] {
  const {
    manifestAssets,
    existingFilesMap,
    missingPaths,
    skippedPaths,
    minImageBytes = 64,
  } = params;
  const skipped = new Set(skippedPaths);
  const manifestPathSet = new Set(manifestAssets.map((item) => item.path));
  const issues: AssetValidationIssue[] = [];

  for (const pathValue of missingPaths) {
    if (skipped.has(pathValue)) continue;
    issues.push({
      path: pathValue,
      type: "missing",
      source: manifestPathSet.has(pathValue) ? "manifest" : "reference",
      message: manifestPathSet.has(pathValue)
        ? "Файл отсутствует в проекте."
        : "Ссылка на ассет найдена в HTML, но файл отсутствует и не описан в манифесте.",
    });
  }

  for (const asset of manifestAssets) {
    if (skipped.has(asset.path)) continue;
    const existing = existingFilesMap.get(asset.path);
    if (!existing) continue;

    const actualMime = normalizeMime(existing.mimeType);
    const expectedFromManifest = normalizeMime(asset.mime_type);
    const expectedFromExt = expectedMimeByPath(asset.path);
    const expectedMime = expectedFromManifest || expectedFromExt;

    if (!actualMime.startsWith("image/")) {
      issues.push({
        path: asset.path,
        type: "broken",
        source: "manifest",
        message: "Файл существует, но не является изображением.",
        expectedMimeType: expectedMime || undefined,
        actualMimeType: actualMime || undefined,
        sizeBytes: existing.size,
      });
      continue;
    }

    if (existing.size <= 0) {
      issues.push({
        path: asset.path,
        type: "broken",
        source: "manifest",
        message: "Файл изображения пустой.",
        expectedMimeType: expectedMime || undefined,
        actualMimeType: actualMime || undefined,
        sizeBytes: existing.size,
      });
      continue;
    }

    if (existing.size < minImageBytes) {
      issues.push({
        path: asset.path,
        type: "broken",
        source: "manifest",
        message: `Размер изображения слишком маленький (< ${minImageBytes} bytes).`,
        expectedMimeType: expectedMime || undefined,
        actualMimeType: actualMime || undefined,
        sizeBytes: existing.size,
      });
      continue;
    }

    if (expectedMime && actualMime && expectedMime !== actualMime) {
      issues.push({
        path: asset.path,
        type: "invalid_mime",
        source: "manifest",
        message: "MIME-тип изображения не соответствует ожидаемому.",
        expectedMimeType: expectedMime,
        actualMimeType: actualMime,
        sizeBytes: existing.size,
      });
    }
  }

  return dedupeIssues(issues);
}
