"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { authFetch, post, patch, del } from "../../../lib/http";
import { useAuthGuard } from "../../../lib/useAuth";
import { FiPlay, FiRefreshCw, FiList, FiClock, FiPauseCircle, FiCheck, FiTrash2, FiUsers, FiX, FiKey, FiAlertCircle } from "react-icons/fi";
import { useRouter } from "next/navigation";
import Link from "next/link";

type Project = {
  id: string;
  name: string;
  target_country?: string;
  target_language?: string;
  status?: string;
  ownerHasApiKey?: boolean;
};

type Domain = {
  id: string;
  project_id: string;
  url: string;
  main_keyword?: string;
  target_country?: string;
  target_language?: string;
  exclude_domains?: string;
  status: string;
  last_generation_id?: string;
  updated_at?: string;
};

type Generation = {
  id: string;
  status: string;
  progress: number;
  error?: string;
  created_at?: string;
  finished_at?: string;
};

export default function ProjectDetailPage() {
  const { me } = useAuthGuard();
  const params = useParams();
  const router = useRouter();
  const projectId = params?.id as string;
  const [project, setProject] = useState<Project | null>(null);
  const [domains, setDomains] = useState<Domain[]>([]);
  const [gens, setGens] = useState<Record<string, Generation[]>>({});
  const [openRuns, setOpenRuns] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [url, setUrl] = useState("");
  const [keyword, setKeyword] = useState("");
  const [country, setCountry] = useState("");
  const [language, setLanguage] = useState("");
  const [exclude, setExclude] = useState("");
  const [importText, setImportText] = useState("");
  const [keywordEdits, setKeywordEdits] = useState<Record<string, string>>({});
  const [activeTab, setActiveTab] = useState<"domains" | "members">("domains");
  const [members, setMembers] = useState<Array<{ email: string; role: string; createdAt: string }>>([]);
  const [newMemberEmail, setNewMemberEmail] = useState("");
  const [newMemberRole, setNewMemberRole] = useState("editor");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const p = await authFetch<Project>(`/api/projects/${projectId}`);
      setProject(p);
      const d = await authFetch<Domain[]>(`/api/projects/${projectId}/domains`);
      setDomains(Array.isArray(d) ? d : []);
      const edits: Record<string, string> = {};
      (Array.isArray(d) ? d : []).forEach((item) => {
        edits[item.id] = item.main_keyword || "";
      });
      setKeywordEdits(edits);
      setCountry(p?.target_country || "");
      setLanguage(p?.target_language || "");
      
      // Загружаем участников
      try {
        const m = await authFetch<Array<{ email: string; role: string; createdAt: string }>>(`/api/projects/${projectId}/members`);
        setMembers(Array.isArray(m) ? m : []);
      } catch {
        // Игнорируем ошибки загрузки участников (может не быть прав)
      }
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить проект");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (projectId) {
      load();
    }
  }, [projectId]);

  const addDomain = async () => {
    if (!url.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains`, { url, keyword, country, language, exclude_domains: exclude });
      setUrl("");
      setKeyword("");
      setExclude("");
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось добавить домен");
    } finally {
      setLoading(false);
    }
  };

  const importDomains = async () => {
    if (!importText.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/domains/import`, { text: importText });
      setImportText("");
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось импортировать");
    } finally {
      setLoading(false);
    }
  };

  const runGeneration = async (id: string) => {
    const domain = domains.find((d) => d.id === id);
    if (!(keywordEdits[id] || "").trim() && !(domain?.main_keyword || "").trim()) {
      setError("Сначала задайте keyword");
      return;
    }
    if (domain?.status === "processing" || domain?.status === "pending") {
      setError("У этого домена уже есть запущенная генерация");
      return;
    }
    // Проверяем наличие API ключа у владельца проекта
    if (project && project.ownerHasApiKey === false) {
      setError("API ключ не настроен у владельца проекта. Настройте ключ в профиле для запуска генерации.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await post(`/api/domains/${id}/generate`);
      await load();
    } catch (err: any) {
      const errMsg = err?.message || "Не удалось запустить генерацию";
      // Улучшаем сообщение об ошибке, если это связано с API ключом
      if (errMsg.includes("API key") || errMsg.includes("api key")) {
        setError(`${errMsg} Настройте API ключ в профиле.`);
      } else {
        setError(errMsg);
      }
    } finally {
      setLoading(false);
    }
  };

  const updateKeyword = async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/domains/${id}`, { keyword: keywordEdits[id] || "" });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось обновить keyword");
    } finally {
      setLoading(false);
    }
  };

  const deleteDomain = async (id: string) => {
    if (!confirm("Удалить домен?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/domains/${id}`);
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить домен");
    } finally {
      setLoading(false);
    }
  };

  const deleteProject = async () => {
    if (!confirm("Удалить проект и все его домены?")) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/projects/${projectId}`);
      router.push("/projects");
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить проект");
    } finally {
      setLoading(false);
    }
  };

  const addMember = async () => {
    if (!newMemberEmail.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post(`/api/projects/${projectId}/members`, { email: newMemberEmail.trim(), role: newMemberRole });
      setNewMemberEmail("");
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось добавить участника");
    } finally {
      setLoading(false);
    }
  };

  const updateMemberRole = async (email: string, role: string) => {
    setLoading(true);
    setError(null);
    try {
      await patch(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`, { role });
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось обновить роль");
    } finally {
      setLoading(false);
    }
  };

  const removeMember = async (email: string) => {
    if (!confirm(`Удалить участника ${email}?`)) return;
    setLoading(true);
    setError(null);
    try {
      await del(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`);
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить участника");
    } finally {
      setLoading(false);
    }
  };

  const loadGens = async (id: string) => {
    try {
      const list = await authFetch<Generation[]>(`/api/domains/${id}/generations`);
      setGens((prev) => ({ ...prev, [id]: Array.isArray(list) ? list : [] }));
      // Переключаем состояние открытия/закрытия
      setOpenRuns((prev) => ({ ...prev, [id]: !prev[id] }));
    } catch {
      /* ignore */
    }
  };


  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold">{project?.name || "Проект"}</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Страна: {project?.target_country || "—"} · Язык: {project?.target_language || "—"}
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            <button
              onClick={deleteProject}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-white px-3 py-2 text-sm font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
            >
              <FiTrash2 /> Удалить
            </button>
          </div>
        </div>
        {error && <div className="text-red-500 text-sm mt-2">{error}</div>}
        
        {/* Индикатор API ключа */}
        {project && (
          <div className="mt-4">
            {project.ownerHasApiKey === false ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/20 p-3">
                <div className="flex items-start gap-2">
                  <FiAlertCircle className="text-amber-600 dark:text-amber-400 mt-0.5" />
                  <div className="flex-1">
                    <div className="text-sm font-semibold text-amber-800 dark:text-amber-200">
                      ⚠️ API ключ не настроен
                    </div>
                    <div className="text-xs text-amber-700 dark:text-amber-300 mt-1">
                      Генерация не будет работать без API ключа владельца проекта.{" "}
                      <a href="/me" className="underline hover:no-underline">
                        Настроить в профиле →
                      </a>
                    </div>
                  </div>
                </div>
              </div>
            ) : project.ownerHasApiKey === true ? (
              <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/40 p-3">
                <div className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-400">
                  <FiKey className="text-emerald-600 dark:text-emerald-400" />
                  <span>API ключ настроен. Генерация будет использовать ключ владельца проекта.</span>
                </div>
              </div>
            ) : null}
          </div>
        )}
      </div>

      {/* Tabs */}
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <div className="flex gap-2 border-b border-slate-200 dark:border-slate-800 mb-4">
          <button
            onClick={() => setActiveTab("domains")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "domains"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            Домены
          </button>
          <button
            onClick={() => setActiveTab("members")}
            className={`px-4 py-2 text-sm font-semibold border-b-2 transition-colors ${
              activeTab === "members"
                ? "border-indigo-600 text-indigo-600 dark:border-indigo-400 dark:text-indigo-400"
                : "border-transparent text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            <FiUsers className="inline mr-1" /> Участники
          </button>
        </div>

        {activeTab === "domains" && (
          <>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <h3 className="font-semibold">Добавить домен</h3>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="example.com"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Keyword"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
          <div className="flex gap-2">
            <button
              onClick={addDomain}
              disabled={loading || !url.trim()}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
            >
              Добавить
            </button>
          </div>
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Страна (по умолчанию из проекта)"
            value={country}
            onChange={(e) => setCountry(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Язык (по умолчанию из проекта)"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Исключить домены (через запятую)"
            value={exclude}
            onChange={(e) => setExclude(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Импорт списком (url[,keyword] на строку). Пример: <code>example.com,casino</code>
          </p>
          <textarea
            className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            rows={4}
            placeholder="example.com,keyword&#10;example.org"
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
          />
          <button
            onClick={importDomains}
            disabled={loading || !importText.trim()}
            className="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-4 py-2 text-sm font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
          >
            Импортировать
          </button>
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Домены</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {domains.length}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                <th className="py-2 pr-4">URL</th>
                <th className="py-2 pr-4">Keyword</th>
                <th className="py-2 pr-4">Статус</th>
                <th className="py-2 pr-4">Обновлено</th>
                <th className="py-2 pr-4 text-right">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
              {domains.map((d) => (
                <tr key={d.id}>
                  <td className="py-3 pr-4 font-medium">
                    <Link href={`/domains/${d.id}`} className="text-indigo-600 hover:underline">
                      {d.url}
                    </Link>
                  </td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-2">
                      <input
                        className="w-full rounded border border-slate-200 bg-white px-2 py-1 text-xs text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                        value={keywordEdits[d.id] ?? ""}
                        onChange={(e) => setKeywordEdits((prev) => ({ ...prev, [d.id]: e.target.value }))}
                        placeholder="Keyword"
                      />
                      <button
                        onClick={() => updateKeyword(d.id)}
                        disabled={loading}
                        className="inline-flex items-center gap-1 rounded-lg bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-800 hover:bg-slate-300 dark:bg-slate-700 dark:text-slate-100 dark:hover:bg-slate-600"
                      >
                        Сохранить
                      </button>
                    </div>
                  </td>
                  <td className="py-3 pr-4">
                    <StatusBadge status={d.status} />
                  </td>
                  <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                    {d.updated_at ? new Date(d.updated_at).toLocaleString() : "—"}
                    <div className="text-xs text-slate-400">Страна: {d.target_country || "—"} · Язык: {d.target_language || "—"}</div>
                    {d.exclude_domains && <div className="text-xs text-slate-400">Исключить: {d.exclude_domains}</div>}
                  </td>
                  <td className="py-3 pr-4 text-right space-x-2">
                    <button
                      onClick={() => loadGens(d.id)}
                      className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    >
                      <FiList /> Запуски {openRuns[d.id] && gens[d.id] && `(${gens[d.id].length})`}
                    </button>
                    <button
                      onClick={() => deleteDomain(d.id)}
                      disabled={loading}
                      className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                    >
                      Удалить
                    </button>
                    {openRuns[d.id] && gens[d.id] && <RunsList runs={gens[d.id]} />}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
          </>
        )}

        {activeTab === "members" && (
          <div className="space-y-4">
            <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
              <h3 className="font-semibold">Добавить участника</h3>
              <div className="grid gap-3 md:grid-cols-3">
                <input
                  className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  placeholder="email@example.com"
                  value={newMemberEmail}
                  onChange={(e) => setNewMemberEmail(e.target.value)}
                />
                <select
                  className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  value={newMemberRole}
                  onChange={(e) => setNewMemberRole(e.target.value)}
                >
                  <option value="viewer">Viewer</option>
                  <option value="editor">Editor</option>
                  <option value="owner">Owner</option>
                </select>
                <button
                  onClick={addMember}
                  disabled={loading || !newMemberEmail.trim()}
                  className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
                >
                  Добавить
                </button>
              </div>
            </div>

            <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="font-semibold">Участники проекта</h3>
                <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {members.length}</span>
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                      <th className="py-2 pr-4">Email</th>
                      <th className="py-2 pr-4">Роль</th>
                      <th className="py-2 pr-4">Добавлен</th>
                      <th className="py-2 pr-4 text-right">Действия</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                    {members.map((m) => (
                      <tr key={m.email}>
                        <td className="py-3 pr-4 font-medium">{m.email}</td>
                        <td className="py-3 pr-4">
                          {m.role === "owner" ? (
                            <span className="text-sm text-slate-600 dark:text-slate-400">Owner (владелец)</span>
                          ) : (
                            <select
                              className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                              value={m.role}
                              onChange={(e) => updateMemberRole(m.email, e.target.value)}
                            >
                              <option value="viewer">Viewer</option>
                              <option value="editor">Editor</option>
                              <option value="owner">Owner</option>
                            </select>
                          )}
                        </td>
                        <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                          {m.createdAt ? new Date(m.createdAt).toLocaleString() : "—"}
                        </td>
                        <td className="py-3 pr-4 text-right">
                          {m.role !== "owner" && (
                            <button
                              onClick={() => removeMember(m.email)}
                              disabled={loading}
                              className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                            >
                              <FiX /> Удалить
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {members.length === 0 && <div className="text-sm text-slate-500 mt-2">Участников пока нет.</div>}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function RunsList({ runs }: { runs: Generation[] }) {
  if (!Array.isArray(runs) || runs.length === 0) return null;
  // Показываем только последние 4 запуска
  const displayRuns = runs.slice(0, 4);
  return (
    <div className="mt-2 text-left text-xs bg-slate-50 dark:bg-slate-800/60 border border-slate-200 dark:border-slate-700 rounded-lg p-2 space-y-1">
      {displayRuns.map((r) => (
        <Link
          key={r.id}
          href={`/queue/${r.id}`}
          className="flex items-center justify-between rounded-lg px-2 py-1 hover:bg-slate-100 dark:hover:bg-slate-700/60"
        >
          <span className="font-semibold">{r.id.slice(0, 8)}</span>
          <div className="flex items-center gap-2">
            <StatusBadge status={r.status} />
            <span className="text-slate-500 dark:text-slate-400">{r.progress}%</span>
            {r.error && <span className="text-red-500">err</span>}
          </div>
        </Link>
      ))}
      {runs.length > 4 && (
        <div className="text-xs text-slate-500 dark:text-slate-400 px-2 py-1">
          ... и еще {runs.length - 4} запусков
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; color: string; icon: React.ReactNode }> = {
    waiting: { text: "Ожидает", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiClock /> },
    processing: { text: "В работе", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPlay /> },
    published: { text: "Опубликован", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiPlay /> },
    draft: { text: "Черновик", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> },
    active: { text: "Активен", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiPlay /> },
    paused: { text: "Приостановлено", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> },
    pause_requested: { text: "Пауза запрошена", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-200", icon: <FiPauseCircle /> },
    cancelling: { text: "Отмена...", color: "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-200", icon: <FiPauseCircle /> },
    cancelled: { text: "Отменено", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiPauseCircle /> },
    pending: { text: "В очереди", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiClock /> },
    success: { text: "Готово", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiCheck /> },
    error: { text: "Ошибка", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiPauseCircle /> },
    running: { text: "В работе", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPlay /> }
  };
  const cfg = map[status] || { text: status, color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${cfg.color}`}>
      {cfg.icon} {cfg.text}
    </span>
  );
}
