import type { PeriodKey } from "../../../components/indexing/IndexStats";
import type { IndexCheckSort, IndexCheckSortKey } from "../../../components/indexing/IndexTable";
import type { IndexCheckStatus } from "../../../types/indexChecks";
import { normalizeIndexCheckStatusList } from "./statusMeta";

export const DEFAULT_SORT: IndexCheckSort = { key: "check_date", dir: "desc" };
export const DEFAULT_SORT_PARAM = sortToParam(DEFAULT_SORT);

const SORT_KEYS: IndexCheckSortKey[] = [
  "domain",
  "check_date",
  "status",
  "attempts",
  "is_indexed",
  "last_attempt_at",
  "next_retry_at",
];

export function formatDate(value?: string | null) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleDateString();
}

export function formatDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

export function parseStatusParam(raw: string | null): IndexCheckStatus[] {
  if (!raw) {
    return [];
  }
  return normalizeIndexCheckStatusList(raw.split(",") as IndexCheckStatus[]);
}

export function parseSortParam(raw: string | null): IndexCheckSort {
  if (!raw) {
    return DEFAULT_SORT;
  }
  const cleaned = raw.trim();
  if (!cleaned) {
    return DEFAULT_SORT;
  }
  const [keyRaw, dirRaw] = cleaned.split(":", 2);
  const key = keyRaw.trim();
  if (!isSortKey(key)) {
    return DEFAULT_SORT;
  }
  const dir = dirRaw && dirRaw.trim().toLowerCase() === "asc" ? "asc" : "desc";
  return { key, dir };
}

export function sortToParam(sort: IndexCheckSort): string {
  return `${sort.key}:${sort.dir}`;
}

export function sameSort(a: IndexCheckSort, b: IndexCheckSort): boolean {
  return a.key === b.key && a.dir === b.dir;
}

function isSortKey(value: string): value is IndexCheckSortKey {
  return SORT_KEYS.includes(value as IndexCheckSortKey);
}

export function sameStatusList(a: IndexCheckStatus[], b: IndexCheckStatus[]): boolean {
  if (a.length !== b.length) return false;
  const setA = new Set(a.map((item) => item.trim()));
  for (const item of b) {
    if (!setA.has(item.trim())) {
      return false;
    }
  }
  return true;
}

export function periodToDays(period: PeriodKey) {
  switch (period) {
    case "7d":
      return 7;
    case "90d":
      return 90;
    default:
      return 30;
  }
}

export function formatDateKey(date: Date) {
  return date.toISOString().slice(0, 10);
}

