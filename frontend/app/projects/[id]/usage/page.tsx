"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { FiDollarSign, FiRefreshCw } from "react-icons/fi";
import { listProjectLLMUsageEvents, listProjectLLMUsageStats } from "../../../../lib/llmUsageApi";
import { authFetchCached } from "../../../../lib/http";
import { useAuthGuard } from "../../../../lib/useAuth";
import type { LLMUsageEventDTO, LLMUsageFilters, LLMUsageStatsDTO } from "../../../../types/llmUsage";
import { UsageCostValue } from "../../../../features/llm-usage/components/UsageCostValue";
import { UsageTokenSourceBadge } from "../../../../features/llm-usage/components/UsageTokenSourceBadge";

const DEFAULT_LIMIT = 50;

type ProjectSummaryResponse = {
  project?: {
    id?: string;
    name?: string;
  };
  domains?: Array<{
    id?: string;
    url?: string;
  }>;
};

export default function ProjectLLMUsagePage() {
  useAuthGuard();
  const params = useParams();
  const projectId = String(params?.id || "").trim();

  const [projectName, setProjectName] = useState("");
  const [projectDomains, setProjectDomains] = useState<Array<{ id: string; url: string }>>([]);
  const [items, setItems] = useState<LLMUsageEventDTO[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<LLMUsageStatsDTO | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [page, setPage] = useState(1);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [model, setModel] = useState("");
  const [operation, setOperation] = useState("");
  const [status, setStatus] = useState("");
  const [userEmail, setUserEmail] = useState("");
  const [domainQuery, setDomainQuery] = useState("");

  const totalPages = Math.max(1, Math.ceil(total / DEFAULT_LIMIT));

  const domainLookup = useMemo(() => {
    const map = new Map<string, string>();
    for (const domain of projectDomains) {
      map.set(domain.id, domain.url);
    }
    return map;
  }, [projectDomains]);

  const resolvedDomainId = useMemo(() => {
    const query = domainQuery.trim().toLowerCase();
    if (!query) {
      return "";
    }
    const exact = projectDomains.find((item) => item.url.toLowerCase() === query);
    return exact?.id || "";
  }, [domainQuery, projectDomains]);

  const modelOptions = useMemo(() => {
    const values = new Set<string>(["gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.5-flash-image"]);
    for (const item of items) {
      const value = item.model?.trim();
      if (value) values.add(value);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [items]);

  const operationOptions = useMemo(() => {
    const values = new Set<string>([
      "generation_step/competitor_analysis",
      "generation_step/technical_spec",
      "generation_step/content_generation",
      "generation_step/design_architecture",
      "generation_step/logo_generation",
      "generation_step/html_generation",
      "generation_step/css_generation",
      "generation_step/js_generation",
      "generation_step/image_generation",
      "generation_step/image_prompt_generation",
      "generation_step/404_page",
      "editor_ai_suggest",
      "editor_ai_create_page",
      "editor_ai_regenerate_asset",
      "link_ai_generate",
    ]);
    for (const item of items) {
      const value = item.operation?.trim();
      if (value) values.add(value);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [items]);

  const statusOptions = useMemo(() => {
    const values = new Set<string>(["success", "error"]);
    for (const item of items) {
      const value = item.status?.trim();
      if (value) values.add(value);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [items]);

  const visibleItems = useMemo(() => {
    const query = domainQuery.trim().toLowerCase();
    if (!query) {
      return items;
    }
    return items.filter((item) => {
      const label = item.domain_id ? (domainLookup.get(item.domain_id) || item.domain_id) : "";
      return label.toLowerCase().includes(query);
    });
  }, [domainLookup, domainQuery, items]);

  const filters = useMemo<LLMUsageFilters>(() => {
    const value: LLMUsageFilters = { page, limit: DEFAULT_LIMIT };
    if (from.trim()) value.from = from.trim();
    if (to.trim()) value.to = to.trim();
    if (model.trim()) value.model = model.trim();
    if (operation.trim()) value.operation = operation.trim();
    if (status.trim()) value.status = status.trim();
    if (userEmail.trim()) value.userEmail = userEmail.trim();
    if (resolvedDomainId) value.domainId = resolvedDomainId;
    return value;
  }, [from, model, operation, page, resolvedDomainId, status, to, userEmail]);

  const load = useCallback(async () => {
    if (!projectId) return;
    setLoading(true);
    setError(null);
    try {
      const [eventsRes, statsRes] = await Promise.all([
        listProjectLLMUsageEvents(projectId, filters),
        listProjectLLMUsageStats(projectId, filters),
      ]);
      setItems(Array.isArray(eventsRes.items) ? eventsRes.items : []);
      setTotal(eventsRes.total || 0);
      setStats(statsRes);
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить usage проекта");
    } finally {
      setLoading(false);
    }
  }, [filters, projectId]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    let active = true;
    if (!projectId) {
      setProjectName("");
      return () => {
        active = false;
      };
    }
    authFetchCached<ProjectSummaryResponse>(`/api/projects/${encodeURIComponent(projectId)}/summary`)
      .then((summary) => {
        if (!active) return;
        setProjectName((summary?.project?.name || "").trim());
        const domains = Array.isArray(summary?.domains)
          ? summary.domains
              .map((item) => ({
                id: String(item?.id || "").trim(),
                url: String(item?.url || "").trim(),
              }))
              .filter((item) => item.id && item.url)
          : [];
        setProjectDomains(domains);
      })
      .catch(() => {
        if (!active) return;
        setProjectName("");
        setProjectDomains([]);
      });
    return () => {
      active = false;
    };
  }, [projectId]);

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-slate-200 bg-white/80 p-4 dark:border-slate-800 dark:bg-slate-900/60">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-xl font-semibold flex items-center gap-2">
              <FiDollarSign /> LLM Usage проекта
            </h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Проект: <span className="font-medium">{projectName || "—"}</span>
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Link
              href={`/projects/${projectId}`}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              К проекту
            </Link>
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <StatCard label="Запросы" value={stats?.total_requests ?? 0} />
        <StatCard label="Токены" value={stats?.total_tokens ?? 0} />
        <StatCard label="Оценочная стоимость (USD)" value={(stats?.total_cost_usd ?? 0).toFixed(6)} />
      </div>

      <div className="rounded-xl border border-slate-200 bg-white/80 p-4 dark:border-slate-800 dark:bg-slate-900/60">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <input
            type="date"
            value={from}
            max={to || undefined}
            onChange={(e) => {
              setPage(1);
              setFrom(e.target.value);
            }}
            className="rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm"
          />
          <input
            type="date"
            value={to}
            min={from || undefined}
            onChange={(e) => {
              setPage(1);
              setTo(e.target.value);
            }}
            className="rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm"
          />
          <select
            value={model}
            onChange={(e) => {
              setPage(1);
              setModel(e.target.value);
            }}
            className="rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm"
          >
            <option value="">Все модели</option>
            {modelOptions.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
          <select
            value={operation}
            onChange={(e) => {
              setPage(1);
              setOperation(e.target.value);
            }}
            className="rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm"
          >
            <option value="">Все операции</option>
            {operationOptions.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
          <select
            value={status}
            onChange={(e) => {
              setPage(1);
              setStatus(e.target.value);
            }}
            className="rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm"
          >
            <option value="">Все статусы</option>
            {statusOptions.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
          <input
            value={userEmail}
            onChange={(e) => {
              setPage(1);
              setUserEmail(e.target.value);
            }}
            placeholder="Email пользователя"
            className="rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm"
          />
          <input
            list="project-domain-options"
            value={domainQuery}
            onChange={(e) => {
              setPage(1);
              setDomainQuery(e.target.value);
            }}
            placeholder="Поиск по домену"
            className="rounded-lg border border-slate-300 bg-transparent px-3 py-2 text-sm"
          />
          <datalist id="project-domain-options">
            {projectDomains.map((item) => (
              <option key={item.id} value={item.url} />
            ))}
          </datalist>
        </div>
      </div>

      <div className="rounded-xl border border-slate-200 bg-white/80 p-4 dark:border-slate-800 dark:bg-slate-900/60">
        {error && <div className="mb-3 text-sm text-red-500">{error}</div>}
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500">
                <th className="py-2 pr-4">Когда</th>
                <th className="py-2 pr-4">Операция</th>
                <th className="py-2 pr-4">Этап</th>
                <th className="py-2 pr-4">Модель</th>
                <th className="py-2 pr-4">Домен</th>
                <th className="py-2 pr-4">Кто</th>
                <th className="py-2 pr-4">Статус</th>
                <th className="py-2 pr-4">Токены</th>
                <th className="py-2 pr-4">Cost USD</th>
                <th className="py-2 pr-4">Источник</th>
              </tr>
            </thead>
            <tbody>
              {visibleItems.map((item) => (
                <tr key={item.id} className="border-t border-slate-200 dark:border-slate-800">
                  <td className="py-2 pr-4 whitespace-nowrap">{new Date(item.created_at).toLocaleString()}</td>
                  <td className="py-2 pr-4">{item.operation}</td>
                  <td className="py-2 pr-4">{item.stage || "—"}</td>
                  <td className="py-2 pr-4">{item.model}</td>
                  <td className="py-2 pr-4">
                    {item.domain_id ? (domainLookup.get(item.domain_id) || item.domain_id) : "—"}
                  </td>
                  <td className="py-2 pr-4">{item.requester_email}</td>
                  <td className="py-2 pr-4">
                    <span className={`rounded px-2 py-1 text-xs ${item.status === "error" ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300" : "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"}`}>
                      {item.status}
                    </span>
                  </td>
                  <td className="py-2 pr-4">{item.total_tokens ?? "n/a"}</td>
                  <td className="py-2 pr-4"><UsageCostValue value={item.estimated_cost_usd} /></td>
                  <td className="py-2 pr-4"><UsageTokenSourceBadge tokenSource={item.token_source} /></td>
                </tr>
              ))}
              {visibleItems.length === 0 && (
                <tr>
                  <td colSpan={10} className="py-6 text-center text-slate-500">Нет данных</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        <div className="mt-4 flex items-center justify-end gap-2 text-sm">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1 || loading}
            className="rounded border border-slate-300 px-3 py-1 disabled:opacity-50"
          >
            Назад
          </button>
          <span>{page} / {totalPages}</span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages || loading}
            className="rounded border border-slate-300 px-3 py-1 disabled:opacity-50"
          >
            Вперёд
          </button>
        </div>
      </div>
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white/80 p-4 dark:border-slate-800 dark:bg-slate-900/60">
      <div className="text-xs text-slate-500 dark:text-slate-400">{label}</div>
      <div className="mt-1 text-xl font-semibold">{value}</div>
    </div>
  );
}
