"use client";

import { useEffect, useMemo, useState } from "react";
import { FiRefreshCw, FiSave, FiTrash2 } from "react-icons/fi";
import { authFetch } from "../lib/http";
import { showToast } from "../lib/toastStore";
import { Badge } from "./Badge";
import { PromptVariablesHelp } from "./PromptVariablesHelp";

type PromptOverrideDTO = {
  id: string;
  scope_type: string;
  scope_id: string;
  stage: string;
  body: string;
  model?: string;
};

type ResolvedPromptDTO = {
  stage: string;
  source: "domain" | "project" | "global" | string;
  prompt_id?: string;
  override_id?: string;
  body: string;
  model?: string;
};

type PromptResponseDTO = {
  overrides: PromptOverrideDTO[];
  resolved: ResolvedPromptDTO[];
};

type PromptOverridesPanelProps = {
  title: string;
  endpoint: string;
  canEdit: boolean;
  layout?: "list" | "single-stage";
};

const SOURCE_LABELS: Record<string, string> = {
  domain: "домен",
  project: "проект",
  global: "базовый",
};

const STAGE_LABELS: Record<string, string> = {
  competitor_analysis: "Анализ конкурентов",
  technical_spec: "Техническое задание",
  content_generation: "Генерация контента",
  design_architecture: "Дизайн-архитектура",
  logo_generation: "Генерация логотипа",
  html_generation: "Генерация HTML",
  css_generation: "Генерация CSS",
  js_generation: "Генерация JavaScript",
  image_prompt_generation: "Промпты для изображений",
  "404_page": "Генерация 404",
};

const MODEL_OPTIONS = [
  { value: "", label: `По умолчанию (${process.env.NEXT_PUBLIC_GEMINI_DEFAULT_MODEL || "gemini-2.5-pro"})` },
  { value: "gemini-3-pro-preview", label: "gemini-3-pro-preview" },
  { value: "gemini-2.5-pro", label: "gemini-2.5-pro" },
  { value: "gemini-2.5-flash", label: "gemini-2.5-flash" },
  { value: "gemini-2.5-flash-image", label: "gemini-2.5-flash-image" },
  { value: "gemini-1.5-pro", label: "gemini-1.5-pro" },
  { value: "gemini-1.5-flash", label: "gemini-1.5-flash" },
];

export function PromptOverridesPanel({ title, endpoint, canEdit, layout = "list" }: PromptOverridesPanelProps) {
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<PromptResponseDTO>({ overrides: [], resolved: [] });
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [modelDrafts, setModelDrafts] = useState<Record<string, string>>({});
  const [selectedStage, setSelectedStage] = useState("");

  const overrideByStage = useMemo(() => {
    const map: Record<string, PromptOverrideDTO> = {};
    response.overrides.forEach((item) => {
      map[item.stage] = item;
    });
    return map;
  }, [response.overrides]);

  const load = async () => {
    if (!endpoint) return;
    setLoading(true);
    try {
      const data = await authFetch<PromptResponseDTO>(endpoint);
      setResponse({
        overrides: Array.isArray(data?.overrides) ? data.overrides : [],
        resolved: Array.isArray(data?.resolved) ? data.resolved : [],
      });
      setDrafts((prev) => {
        const next = { ...prev };
        (Array.isArray(data?.resolved) ? data.resolved : []).forEach((item) => {
          const fromOverride = (Array.isArray(data?.overrides) ? data.overrides : []).find((ov) => ov.stage === item.stage);
          if (!next[item.stage]) {
            next[item.stage] = (fromOverride?.body || item.body || "").trim();
          }
        });
        return next;
      });
      setModelDrafts((prev) => {
        const next = { ...prev };
        (Array.isArray(data?.resolved) ? data.resolved : []).forEach((item) => {
          const fromOverride = (Array.isArray(data?.overrides) ? data.overrides : []).find((ov) => ov.stage === item.stage);
          if (typeof next[item.stage] === "undefined") {
            next[item.stage] = (fromOverride?.model || item.model || "").trim();
          }
        });
        return next;
      });
    } catch (err: any) {
      showToast({
        type: "error",
        title: "Ошибка загрузки промптов",
        message: err?.message || "Не удалось загрузить переопределения промптов",
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, [endpoint]);

  useEffect(() => {
    if (layout !== "single-stage") {
      return;
    }
    if (!response.resolved.length) {
      setSelectedStage("");
      return;
    }
    const hasCurrent = response.resolved.some((item) => item.stage === selectedStage);
    if (!hasCurrent) {
      const preferred = response.overrides[0]?.stage || response.resolved[0].stage;
      setSelectedStage(preferred);
    }
  }, [layout, response.overrides, response.resolved, selectedStage]);

  const onSave = async (stage: string) => {
    const body = (drafts[stage] || "").trim();
    const model = (modelDrafts[stage] || "").trim();
    if (!body) {
      showToast({ type: "error", title: "Пустой оверрайд", message: "Введите текст промпта перед сохранением." });
      return;
    }
    setLoading(true);
    try {
      await authFetch(`${endpoint}/${stage}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body, model }),
      });
      showToast({ type: "success", title: "Оверрайд сохранен", message: formatStageLabel(stage) });
      await load();
    } catch (err: any) {
      showToast({ type: "error", title: "Ошибка сохранения", message: err?.message || stage });
    } finally {
      setLoading(false);
    }
  };

  const onReset = async (stage: string) => {
    setLoading(true);
    try {
      await authFetch(`${endpoint}/${stage}`, { method: "DELETE" });
      setDrafts((prev) => ({ ...prev, [stage]: "" }));
      showToast({ type: "success", title: "Оверрайд удален", message: formatStageLabel(stage) });
      await load();
    } catch (err: any) {
      showToast({ type: "error", title: "Ошибка удаления", message: err?.message || stage });
    } finally {
      setLoading(false);
    }
  };

  const sourceTone = (source: string): "green" | "blue" | "amber" | "slate" => {
    if (source === "domain") return "green";
    if (source === "project") return "blue";
    if (source === "global") return "amber";
    return "slate";
  };

  const formatSourceLabel = (source: string) => SOURCE_LABELS[source] || source;

  const formatStageLabel = (stage: string) => STAGE_LABELS[stage] || stage;
  const visibleResolved = useMemo(() => {
    if (layout !== "single-stage") {
      return response.resolved;
    }
    if (!selectedStage) {
      return [];
    }
    return response.resolved.filter((item) => item.stage === selectedStage);
  }, [layout, response.resolved, selectedStage]);

  return (
    <div className="rounded-xl border border-slate-200 bg-white/80 p-4 shadow dark:border-slate-800 dark:bg-slate-900/60 space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">{title}</h3>
        <button
          type="button"
          onClick={() => void load()}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiRefreshCw /> Обновить
        </button>
      </div>

      {response.resolved.length === 0 ? (
        <div className="text-sm text-slate-500 dark:text-slate-400">Промпты еще не настроены.</div>
      ) : (
        <div className="space-y-3">
          {layout === "single-stage" && (
            <div className="rounded-lg border border-slate-200 bg-slate-50/70 p-3 dark:border-slate-800 dark:bg-slate-900/60 space-y-2">
              <div className="text-xs text-slate-500 dark:text-slate-400">Выберите этап, который хотите переопределить</div>
              <select
                value={selectedStage}
                onChange={(event) => setSelectedStage(event.target.value)}
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              >
                {response.resolved.map((item) => {
                  const hasOverride = Boolean(overrideByStage[item.stage]);
                  const source = formatSourceLabel(item.source);
                  const suffix = hasOverride ? " • есть оверрайд" : "";
                  return (
                    <option key={item.stage} value={item.stage}>
                      {formatStageLabel(item.stage)} ({source}){suffix}
                    </option>
                  );
                })}
              </select>
            </div>
          )}
          {visibleResolved.map((item) => {
            const override = overrideByStage[item.stage];
            const draft = drafts[item.stage] ?? (override?.body || "");
            const modelDraft = modelDrafts[item.stage] ?? (override?.model || item.model || "");
            return (
              <div key={item.stage} className="rounded-lg border border-slate-200 bg-slate-50/70 p-3 dark:border-slate-800 dark:bg-slate-900/60 space-y-2">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="text-sm font-semibold">{formatStageLabel(item.stage)}</div>
                  <code className="rounded bg-slate-200/80 px-1.5 py-0.5 text-[10px] text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                    {item.stage}
                  </code>
                  <Badge label={`Источник: ${formatSourceLabel(item.source)}`} tone={sourceTone(item.source)} className="text-[11px]" />
                  {item.model && (
                    <span className="text-[11px] text-slate-500 dark:text-slate-400">Модель: {item.model}</span>
                  )}
                </div>
                <details className="rounded-lg border border-slate-200 bg-white/70 p-2 dark:border-slate-700 dark:bg-slate-900/40">
                  <summary className="cursor-pointer text-xs font-semibold text-slate-600 dark:text-slate-300">
                    Итоговый промпт (источник: {formatSourceLabel(item.source)})
                  </summary>
                  <pre className="mt-2 max-h-56 overflow-auto rounded-lg bg-slate-100/80 p-2 text-xs text-slate-700 dark:bg-slate-900/70 dark:text-slate-200 whitespace-pre-wrap">
                    {item.body || "(пусто)"}
                  </pre>
                </details>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-slate-500 dark:text-slate-400">Текст оверрайда</span>
                    <PromptVariablesHelp />
                  </div>
                  <textarea
                    value={canEdit ? draft : item.body}
                    onChange={(event) => setDrafts((prev) => ({ ...prev, [item.stage]: event.target.value }))}
                    readOnly={!canEdit}
                    rows={6}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  />
                </div>
                <div className="space-y-1">
                  <span className="text-xs text-slate-500 dark:text-slate-400">Модель</span>
                  <select
                    value={modelDraft}
                    disabled={!canEdit}
                    onChange={(event) => setModelDrafts((prev) => ({ ...prev, [item.stage]: event.target.value }))}
                    className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 disabled:opacity-70"
                  >
                    {MODEL_OPTIONS.map((option) => (
                      <option key={option.value || "__default"} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </div>
                {canEdit && (
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => void onSave(item.stage)}
                      disabled={loading}
                      className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-1 text-xs font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
                    >
                      <FiSave /> Сохранить оверрайд
                    </button>
                    <button
                      type="button"
                      onClick={() => void onReset(item.stage)}
                      disabled={loading}
                      className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                    >
                      <FiTrash2 /> Сбросить оверрайд
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
