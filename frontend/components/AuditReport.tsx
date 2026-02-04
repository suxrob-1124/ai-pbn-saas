"use client";

import { useMemo, useState } from "react";
import { FiAlertTriangle, FiCheckCircle, FiInfo } from "react-icons/fi";

type AuditFinding = {
  rule_code?: string;
  severity?: string;
  message?: string;
  file_path?: string;
  details?: any;
};

type AuditReportData = {
  status?: string;
  summary?: {
    total?: number;
    errors?: number;
    warnings?: number;
  };
  findings?: AuditFinding[];
};

export function AuditReport({ report }: { report?: any }) {
  const [filter, setFilter] = useState<"all" | "error" | "warn">("all");

  const data = useMemo<AuditReportData | null>(() => {
    if (!report) return null;
    if (typeof report === "string") {
      try {
        return JSON.parse(report);
      } catch {
        return null;
      }
    }
    return report as AuditReportData;
  }, [report]);

  const findings = useMemo(() => {
    const list = Array.isArray(data?.findings) ? data?.findings : [];
    if (filter === "all") return list;
    return list.filter((f) => (f.severity || "").toLowerCase() === filter);
  }, [data?.findings, filter]);

  if (!data) return null;

  const status = (data.status || "ok").toLowerCase();
  const summary = data.summary || {};
  const statusLabel = status === "error" ? "Ошибки" : status === "warn" ? "Предупреждения" : "OK";
  const statusClass =
    status === "error"
      ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200"
      : status === "warn"
      ? "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
      : "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200";
  const StatusIcon = status === "error" ? FiAlertTriangle : status === "warn" ? FiInfo : FiCheckCircle;

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${statusClass}`}>
            <StatusIcon /> {statusLabel}
          </span>
          <span className="text-sm text-slate-600 dark:text-slate-300">
            Найдено: {summary.total ?? findings.length} · Ошибок: {summary.errors ?? 0} · Предупреждений: {summary.warnings ?? 0}
          </span>
        </div>
        <div className="flex items-center gap-2 text-xs">
          <FilterButton label="Все" active={filter === "all"} onClick={() => setFilter("all")} />
          <FilterButton label="Ошибки" active={filter === "error"} onClick={() => setFilter("error")} tone="error" />
          <FilterButton label="Warn" active={filter === "warn"} onClick={() => setFilter("warn")} tone="warn" />
        </div>
      </div>

      {findings.length === 0 ? (
        <div className="text-sm text-slate-500 dark:text-slate-400">Проблем не найдено.</div>
      ) : (
        <div className="space-y-2">
          {findings.map((f, idx) => (
            <div key={`${f.rule_code}-${idx}`} className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/70 dark:bg-slate-900/40 p-3">
              <div className="flex items-center gap-2">
                <span className={`text-xs font-semibold ${severityClass(f.severity)}`}>{(f.severity || "warn").toUpperCase()}</span>
                <span className="text-xs text-slate-500 dark:text-slate-400">{f.rule_code}</span>
              </div>
              <div className="text-sm text-slate-700 dark:text-slate-200 mt-1">{f.message}</div>
              {f.file_path && (
                <div className="text-xs text-slate-500 dark:text-slate-400 mt-1">Файл: {f.file_path}</div>
              )}
              {f.details && (
                <pre className="text-[11px] mt-2 bg-slate-100/70 dark:bg-slate-900/50 rounded-lg p-2 overflow-auto">
                  {JSON.stringify(f.details, null, 2)}
                </pre>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function severityClass(level?: string): string {
  const sev = (level || "").toLowerCase();
  if (sev === "error") return "text-red-600 dark:text-red-300";
  if (sev === "warn" || sev === "warning") return "text-amber-600 dark:text-amber-300";
  return "text-slate-600 dark:text-slate-300";
}

function FilterButton({
  label,
  active,
  onClick,
  tone
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  tone?: "error" | "warn";
}) {
  const toneClass =
    tone === "error"
      ? "border-red-200 text-red-700 dark:border-red-700 dark:text-red-200"
      : tone === "warn"
      ? "border-amber-200 text-amber-700 dark:border-amber-700 dark:text-amber-200"
      : "border-slate-200 text-slate-600 dark:border-slate-700 dark:text-slate-200";
  return (
    <button
      onClick={onClick}
      className={`rounded-full px-2 py-0.5 border text-[11px] font-semibold ${
        active ? "bg-indigo-600 border-indigo-600 text-white" : toneClass
      }`}
    >
      {label}
    </button>
  );
}
