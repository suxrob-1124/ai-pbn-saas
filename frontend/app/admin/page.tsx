"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  FiUsers,
  FiCheckCircle,
  FiXCircle,
  FiRefreshCw,
  FiShield,
  FiTrash2
} from "react-icons/fi";
import { useAuthGuard } from "../../lib/useAuth";
import { authFetch, patch, post, del } from "../../lib/http";
import { PromptVariablesHelp } from "../../components/PromptVariablesHelp";
import { GENERATION_STAGES, getStageLabel } from "../../lib/promptStages";

type AdminUser = {
  email: string;
  name?: string;
  role: string;
  isApproved: boolean;
  verified: boolean;
  createdAt: string;
  apiKeyUpdatedAt?: string;
};

type AdminPrompt = {
  id: string;
  name: string;
  description?: string | null;
  body: string;
  stage?: string | null;
  model?: string | null;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
};

type AdminAuditRule = {
  code: string;
  title: string;
  description?: string | null;
  severity: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
};

type ProjectDTO = {
  id: string;
  name: string;
  status: string;
  target_country?: string;
  target_language?: string;
  created_at: string;
  updated_at: string;
};

type PromptPatch = Partial<{
  name: string;
  description: string | null;
  body: string;
  stage: string | null;
  model: string | null;
  isActive: boolean;
}>;

type AuditRulePatch = Partial<{
  title: string;
  description: string | null;
  severity: string;
  isActive: boolean;
}>;

export default function AdminPage() {
  const router = useRouter();
  const { me, loading } = useAuthGuard();
  const isAdmin = useMemo(() => (me?.role || "").toLowerCase() === "admin", [me]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [usersError, setUsersError] = useState<string | null>(null);
  const [usersLoading, setUsersLoading] = useState(false);
  const [prompts, setPrompts] = useState<AdminPrompt[]>([]);
  const [promptsError, setPromptsError] = useState<string | null>(null);
  const [promptsLoading, setPromptsLoading] = useState(false);
  const [auditRules, setAuditRules] = useState<AdminAuditRule[]>([]);
  const [auditRulesError, setAuditRulesError] = useState<string | null>(null);
  const [auditRulesLoading, setAuditRulesLoading] = useState(false);
  const [selectedProjectsEmail, setSelectedProjectsEmail] = useState<string | null>(null);
  const [selectedProjects, setSelectedProjects] = useState<ProjectDTO[]>([]);
  const [projectsLoading, setProjectsLoading] = useState(false);
  const [projectsError, setProjectsError] = useState<string | null>(null);

  const [newPrompt, setNewPrompt] = useState({
    name: "",
    description: "",
    body: "",
    stage: "",
    isActive: true
  });
  const [creatingPrompt, setCreatingPrompt] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const [newRule, setNewRule] = useState({
    code: "",
    title: "",
    description: "",
    severity: "warn",
    isActive: true
  });
  const [creatingRule, setCreatingRule] = useState(false);
  const [createRuleError, setCreateRuleError] = useState<string | null>(null);

  const loadUsers = useCallback(async () => {
    setUsersLoading(true);
    setUsersError(null);
    try {
      const list = await authFetch<AdminUser[]>("/api/admin/users");
      setUsers(list);
    } catch (err: any) {
      setUsersError(err?.message || "Не удалось загрузить пользователей");
    } finally {
      setUsersLoading(false);
    }
  }, []);

  const loadPrompts = useCallback(async () => {
    setPromptsLoading(true);
    setPromptsError(null);
    try {
      const list = await authFetch<AdminPrompt[]>("/api/admin/prompts");
      setPrompts(list);
    } catch (err: any) {
      setPromptsError(err?.message || "Не удалось загрузить промпты");
    } finally {
      setPromptsLoading(false);
    }
  }, []);

  const loadAuditRules = useCallback(async () => {
    setAuditRulesLoading(true);
    setAuditRulesError(null);
    try {
      const list = await authFetch<AdminAuditRule[]>("/api/admin/audit-rules");
      setAuditRules(list);
    } catch (err: any) {
      setAuditRulesError(err?.message || "Не удалось загрузить правила аудита");
    } finally {
      setAuditRulesLoading(false);
    }
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!isAdmin) {
      router.replace("/projects");
      return;
    }
    loadUsers();
    loadPrompts();
    loadAuditRules();
  }, [isAdmin, loading, router, loadUsers, loadPrompts, loadAuditRules]);

  const handleRoleChange = async (email: string, role: string) => {
    await patch(`/api/admin/users/${encodeURIComponent(email)}`, { role });
    await loadUsers();
  };

  const handleApprovalToggle = async (email: string, approved: boolean) => {
    await patch(`/api/admin/users/${encodeURIComponent(email)}`, { isApproved: approved });
    await loadUsers();
  };

  const handleLoadProjects = async (email: string) => {
    setSelectedProjectsEmail(email);
    setProjectsLoading(true);
    setProjectsError(null);
    try {
      const list = await authFetch<ProjectDTO[]>(`/api/admin/users/${encodeURIComponent(email)}/projects`);
      setSelectedProjects(list);
    } catch (err: any) {
      setProjectsError(err?.message || "Не удалось загрузить проекты пользователя");
      setSelectedProjects([]);
    } finally {
      setProjectsLoading(false);
    }
  };

  const createPrompt = async () => {
    if (!newPrompt.name.trim() || !newPrompt.body.trim()) {
      setCreateError("Название и текст промпта обязательны");
      return;
    }
    setCreateError(null);
    setCreatingPrompt(true);
    try {
      await post("/api/admin/prompts", {
        name: newPrompt.name.trim(),
        description: newPrompt.description.trim() ? newPrompt.description.trim() : undefined,
        body: newPrompt.body.trim(),
        stage: newPrompt.stage.trim() ? newPrompt.stage.trim() : undefined,
        isActive: newPrompt.isActive
      });
      setNewPrompt({ name: "", description: "", body: "", stage: "", isActive: true });
      await loadPrompts();
    } catch (err: any) {
      setCreateError(err?.message || "Не удалось создать промпт");
    } finally {
      setCreatingPrompt(false);
    }
  };

  const savePrompt = async (id: string, changes: PromptPatch) => {
    await patch(`/api/admin/prompts/${id}`, changes);
    await loadPrompts();
  };

  const deletePrompt = async (id: string) => {
    await del(`/api/admin/prompts/${id}`);
    await loadPrompts();
  };

  const createRule = async () => {
    if (!newRule.code.trim() || !newRule.title.trim()) {
      setCreateRuleError("Код и название обязательны");
      return;
    }
    setCreateRuleError(null);
    setCreatingRule(true);
    try {
      await post("/api/admin/audit-rules", {
        code: newRule.code.trim(),
        title: newRule.title.trim(),
        description: newRule.description.trim() ? newRule.description.trim() : undefined,
        severity: newRule.severity,
        isActive: newRule.isActive
      });
      setNewRule({ code: "", title: "", description: "", severity: "warn", isActive: true });
      await loadAuditRules();
    } catch (err: any) {
      setCreateRuleError(err?.message || "Не удалось создать правило");
    } finally {
      setCreatingRule(false);
    }
  };

  const saveRule = async (code: string, changes: AuditRulePatch) => {
    await patch(`/api/admin/audit-rules/${encodeURIComponent(code)}`, changes);
    await loadAuditRules();
  };

  const deleteRule = async (code: string) => {
    await del(`/api/admin/audit-rules/${encodeURIComponent(code)}`);
    await loadAuditRules();
  };

  if (!isAdmin) {
    return null;
  }

  return (
    <div className="space-y-6">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center gap-2 text-lg font-semibold">
          <FiShield /> Админ-панель
        </div>
        <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
          Управление пользователями, доступами и системными промптами генератора.
        </p>
      </div>

      <section className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <FiUsers /> Пользователи
          </h2>
          <button
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            onClick={loadUsers}
            disabled={usersLoading}
          >
            <FiRefreshCw /> Обновить
          </button>
        </div>
        {usersError && <div className="text-red-500 text-sm mb-3">{usersError}</div>}
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-xs uppercase text-slate-500 dark:text-slate-400">
                <th className="py-2">Почта</th>
                <th className="py-2">Роль</th>
                <th className="py-2">Подтверждён</th>
                <th className="py-2">Статус</th>
                <th className="py-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.email} className="border-t border-slate-100 dark:border-slate-800">
                  <td className="py-2">
                    <div className="font-medium text-slate-800 dark:text-slate-100">{user.email}</div>
                    {user.name && <div className="text-xs text-slate-500">{user.name}</div>}
                  </td>
                  <td className="py-2">
                    {user.email === me?.email ? (
                      <span className="text-sm text-slate-600 dark:text-slate-400">
                        {user.role === "admin" ? "Админ" : "Менеджер"}
                        <span className="ml-2 text-xs text-slate-400">(вы)</span>
                      </span>
                    ) : user.role === "admin" ? (
                      <span className="text-sm text-slate-600 dark:text-slate-400">
                        Админ
                        <span className="ml-2 text-xs text-slate-400">(нельзя изменить)</span>
                      </span>
                    ) : (
                    <select
                      className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                      value={user.role}
                      onChange={(e) => handleRoleChange(user.email, e.target.value)}
                    >
                      <option value="manager">Менеджер</option>
                      <option value="admin">Админ</option>
                    </select>
                    )}
                  </td>
                  <td className="py-2">
                    {user.verified ? (
                      <span className="inline-flex items-center gap-1 text-emerald-600 text-xs">
                        <FiCheckCircle /> почта подтверждена
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-slate-500 text-xs">
                        <FiXCircle /> не подтверждён
                      </span>
                    )}
                  </td>
                  <td className="py-2">
                    {user.isApproved ? (
                      <span className="inline-flex items-center gap-1 text-emerald-600 text-xs">
                        <FiCheckCircle /> активен
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-amber-500 text-xs">
                        <FiXCircle /> ожидает
                      </span>
                    )}
                  </td>
                  <td className="py-2">
                    <div className="flex flex-wrap gap-2">
                      <button
                        className="px-3 py-1 text-xs rounded-lg border border-slate-200 hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                        onClick={() => handleLoadProjects(user.email)}
                      >
                        Проекты
                      </button>
                      <button
                        className="px-3 py-1 text-xs rounded-lg border border-slate-200 hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
                        onClick={() => handleApprovalToggle(user.email, !user.isApproved)}
                      >
                        {user.isApproved ? "Заблокировать" : "Активировать"}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {users.length === 0 && !usersLoading && <div className="text-sm text-slate-500 mt-2">Пользователей пока нет.</div>}
        </div>
        {selectedProjectsEmail && (
          <div className="mt-6">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold">
                Проекты пользователя <span className="text-indigo-600">{selectedProjectsEmail}</span>
              </h3>
              {projectsLoading && <span className="text-xs text-slate-400">Загрузка...</span>}
            </div>
            {projectsError && <div className="text-red-500 text-sm">{projectsError}</div>}
            <div className="grid md:grid-cols-2 gap-3 mt-3">
              {selectedProjects.map((p) => (
                <div key={p.id} className="border border-slate-200 dark:border-slate-800 rounded-lg p-3 bg-white dark:bg-slate-900">
                  <div className="font-semibold">{p.name}</div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">
                    Статус: {p.status || "—"} · Страна: {p.target_country || "—"} · Язык: {p.target_language || "—"}
                  </div>
                </div>
              ))}
              {!projectsLoading && selectedProjects.length === 0 && (
                <div className="text-sm text-slate-500">Проектов нет.</div>
              )}
            </div>
          </div>
        )}
      </section>

      <section className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <FiShield /> Системные промпты
          </h2>
          <button
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            onClick={loadPrompts}
            disabled={promptsLoading}
          >
            <FiRefreshCw /> Обновить
          </button>
        </div>
        {promptsError && <div className="text-red-500 text-sm mb-3">{promptsError}</div>}

        <div className="grid md:grid-cols-2 gap-4">
          <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
            <div className="flex items-center justify-between">
            <div className="text-sm font-semibold">Создать промпт</div>
              <PromptVariablesHelp />
            </div>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              placeholder="Название"
              value={newPrompt.name}
              onChange={(e) => setNewPrompt((p) => ({ ...p, name: e.target.value }))}
            />
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              placeholder="Описание"
              value={newPrompt.description}
              onChange={(e) => setNewPrompt((p) => ({ ...p, description: e.target.value }))}
            />
            <div className="space-y-1">
              <label className="text-xs text-slate-600 dark:text-slate-400">Этап генерации</label>
              <select
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                value={newPrompt.stage}
                onChange={(e) => setNewPrompt((p) => ({ ...p, stage: e.target.value }))}
              >
                <option value="">— Не указан (универсальный промпт) —</option>
                {GENERATION_STAGES.map((stage) => (
                  <option key={stage.value} value={stage.value}>
                    {stage.label} — {stage.description}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <label className="text-xs text-slate-600 dark:text-slate-400">Текст промпта</label>
                <PromptVariablesHelp />
              </div>
            <textarea
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 min-h-[140px] font-mono text-xs"
                placeholder='Используйте переменные: {{ keyword }}, {{ contents_data }}, {{ analysis_data }}, {{ country }}, {{ language }}'
              value={newPrompt.body}
              onChange={(e) => setNewPrompt((p) => ({ ...p, body: e.target.value }))}
            />
            </div>
            <label className="inline-flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
              <input
                type="checkbox"
                checked={newPrompt.isActive}
                onChange={(e) => setNewPrompt((p) => ({ ...p, isActive: e.target.checked }))}
              />
              Активировать сразу
            </label>
            {createError && <div className="text-red-500 text-sm">{createError}</div>}
            <button
              className="inline-flex items-center justify-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
              onClick={createPrompt}
              disabled={creatingPrompt}
            >
              {creatingPrompt ? "Сохраняем..." : "Добавить промпт"}
            </button>
          </div>

          <div className="space-y-3">
            {prompts.map((prompt) => (
              <PromptCard
                key={prompt.id}
                prompt={prompt}
                onSave={(changes) => savePrompt(prompt.id, changes)}
                onDelete={() => deletePrompt(prompt.id)}
              />
            ))}
            {!promptsLoading && prompts.length === 0 && (
              <div className="text-sm text-slate-500">Промптов пока нет.</div>
            )}
          </div>
        </div>
      </section>

      <section className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <FiShield /> Правила аудита
          </h2>
          <button
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            onClick={loadAuditRules}
            disabled={auditRulesLoading}
          >
            <FiRefreshCw /> Обновить
          </button>
        </div>
        {auditRulesError && <div className="text-red-500 text-sm mb-3">{auditRulesError}</div>}

        <div className="grid md:grid-cols-2 gap-4">
          <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
            <div className="text-sm font-semibold">Добавить правило</div>
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              placeholder="код (уникальный)"
              value={newRule.code}
              onChange={(e) => setNewRule((r) => ({ ...r, code: e.target.value }))}
            />
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              placeholder="Название"
              value={newRule.title}
              onChange={(e) => setNewRule((r) => ({ ...r, title: e.target.value }))}
            />
            <textarea
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 min-h-[120px]"
              placeholder="Описание"
              value={newRule.description}
              onChange={(e) => setNewRule((r) => ({ ...r, description: e.target.value }))}
            />
            <div className="space-y-1">
              <label className="text-xs text-slate-600 dark:text-slate-400">Критичность</label>
              <select
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                value={newRule.severity}
                onChange={(e) => setNewRule((r) => ({ ...r, severity: e.target.value }))}
              >
                <option value="warn">предупреждение</option>
                <option value="error">ошибка</option>
                <option value="info">инфо</option>
              </select>
            </div>
            <label className="inline-flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
              <input
                type="checkbox"
                checked={newRule.isActive}
                onChange={(e) => setNewRule((r) => ({ ...r, isActive: e.target.checked }))}
              />
              Активно
            </label>
            {createRuleError && <div className="text-red-500 text-sm">{createRuleError}</div>}
            <button
              className="inline-flex items-center justify-center rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
              onClick={createRule}
              disabled={creatingRule}
            >
              {creatingRule ? "Сохраняем..." : "Добавить правило"}
            </button>
          </div>

          <div className="space-y-3">
            {auditRules.map((rule) => (
              <AuditRuleCard
                key={rule.code}
                rule={rule}
                onSave={(changes) => saveRule(rule.code, changes)}
                onDelete={() => deleteRule(rule.code)}
              />
            ))}
            {!auditRulesLoading && auditRules.length === 0 && (
              <div className="text-sm text-slate-500">Правил пока нет.</div>
            )}
          </div>
        </div>
      </section>
    </div>
  );
}

function PromptCard({
  prompt,
  onSave,
  onDelete
}: {
  prompt: AdminPrompt;
  onSave: (changes: PromptPatch) => Promise<void>;
  onDelete: () => Promise<void>;
}) {
  const [name, setName] = useState(prompt.name);
  const [description, setDescription] = useState(prompt.description || "");
  const [body, setBody] = useState(prompt.body);
  const [stage, setStage] = useState(
    prompt.stage ||
      (prompt.name?.toLowerCase().includes("design architect") ? "design_architecture" : "")
  );
  const [model, setModel] = useState(prompt.model || "");
  const [active, setActive] = useState(prompt.isActive);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    setName(prompt.name);
    setDescription(prompt.description || "");
    setBody(prompt.body);
    setStage(
      prompt.stage ||
        (prompt.name?.toLowerCase().includes("design architect") ? "design_architecture" : "")
    );
    setModel(prompt.model || "");
    setActive(prompt.isActive);
  }, [prompt]);

  const save = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      await onSave({
        name: name.trim(),
        description: description.trim() ? description.trim() : null,
        body,
        stage: stage.trim() ? stage.trim() : null,
        model: model.trim() ? model.trim() : null,
        isActive: active
      });
      setSuccess("Сохранено");
    } catch (err: any) {
      setError(err?.message || "Не удалось сохранить");
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!confirm("Удалить промпт?")) return;
    setDeleting(true);
    setError(null);
    try {
      await onDelete();
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить");
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 bg-white dark:bg-slate-900 space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold truncate">{prompt.id}</div>
          {stage && (
            <div className="text-xs text-slate-500 dark:text-slate-400">
              Этап: {getStageLabel(stage)}
            </div>
          )}
          {model && (
            <div className="text-xs text-slate-500 dark:text-slate-400">
              Модель: {model}
            </div>
          )}
        </div>
        <span
          className={`text-xs px-2 py-1 rounded-full ${active ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200" : "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200"}`}
        >
          {active ? "Активен" : "Неактивен"}
        </span>
      </div>
      <input
        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <input
        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="Описание"
      />
      <div className="space-y-1">
        <label className="text-xs text-slate-600 dark:text-slate-400">Этап генерации</label>
        <select
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
          value={stage}
          onChange={(e) => setStage(e.target.value)}
        >
          <option value="">— Не указан (универсальный промпт) —</option>
          {GENERATION_STAGES.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label} — {s.description}
            </option>
          ))}
        </select>
      </div>
      <div className="space-y-1">
        <label className="text-xs text-slate-600 dark:text-slate-400">Модель LLM</label>
        <select
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
          value={model}
          onChange={(e) => setModel(e.target.value)}
        >
          <option value="">— По умолчанию ({process.env.NEXT_PUBLIC_GEMINI_DEFAULT_MODEL || "gemini-2.5-pro"}) —</option>
          <option value="gemini-3-pro-preview">gemini-3-pro-preview (Новейшая версия, preview)</option>
          <option value="gemini-2.5-pro">gemini-2.5-pro (Рекомендуется для сложных задач)</option>
          <option value="gemini-2.5-flash">gemini-2.5-flash (Быстрая, для простых задач)</option>
          <option value="gemini-2.5-flash-image">gemini-2.5-flash-image (Генерация изображений)</option>
          <option value="gemini-1.5-pro">gemini-1.5-pro (Предыдущая версия Pro)</option>
          <option value="gemini-1.5-flash">gemini-1.5-flash (Предыдущая версия Flash)</option>
        </select>
      </div>
      <div className="space-y-1">
        <div className="flex items-center justify-between">
          <label className="text-xs text-slate-600 dark:text-slate-400">Текст промпта</label>
          <PromptVariablesHelp />
        </div>
      <textarea
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 min-h-[140px] font-mono text-xs"
          placeholder='Используйте переменные: {{ keyword }}, {{ contents_data }}, {{ analysis_data }}, {{ country }}, {{ language }}'
        value={body}
        onChange={(e) => setBody(e.target.value)}
      />
      </div>
      <label className="inline-flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
        <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} /> Активный промпт
      </label>
      {error && <div className="text-red-500 text-xs">{error}</div>}
      {success && <div className="text-emerald-500 text-xs">{success}</div>}
      <div className="flex gap-2">
        <button
          className="inline-flex items-center gap-1 rounded-lg bg-indigo-600 px-3 py-1 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
          onClick={save}
          disabled={saving}
        >
          {saving ? "Сохраняем..." : "Сохранить"}
        </button>
        <button
          className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-red-50 px-3 py-1 text-sm font-semibold text-red-700 hover:bg-red-100 dark:border-red-800 dark:bg-red-900/30 dark:text-red-100 disabled:opacity-50"
          onClick={remove}
          disabled={deleting}
        >
          <FiTrash2 /> Удалить
        </button>
      </div>
    </div>
  );
}

function AuditRuleCard({
  rule,
  onSave,
  onDelete
}: {
  rule: AdminAuditRule;
  onSave: (changes: AuditRulePatch) => Promise<void>;
  onDelete: () => Promise<void>;
}) {
  const [title, setTitle] = useState(rule.title);
  const [description, setDescription] = useState(rule.description || "");
  const [severity, setSeverity] = useState(rule.severity || "warn");
  const [active, setActive] = useState(rule.isActive);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    setTitle(rule.title);
    setDescription(rule.description || "");
    setSeverity(rule.severity || "warn");
    setActive(rule.isActive);
  }, [rule]);

  const save = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      await onSave({
        title: title.trim(),
        description: description.trim() ? description.trim() : null,
        severity,
        isActive: active
      });
      setSuccess("Сохранено");
    } catch (err: any) {
      setError(err?.message || "Не удалось сохранить");
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!confirm("Удалить правило аудита?")) return;
    setDeleting(true);
    setError(null);
    try {
      await onDelete();
    } catch (err: any) {
      setError(err?.message || "Не удалось удалить");
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 bg-white dark:bg-slate-900 space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold truncate">{rule.code}</div>
        </div>
        <span
          className={`text-xs px-2 py-1 rounded-full ${
            active
              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
              : "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
          }`}
        >
          {active ? "Активно" : "Неактивно"}
        </span>
      </div>
      <input
        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <textarea
        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 min-h-[90px]"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
        placeholder="Описание"
      />
      <div className="grid grid-cols-2 gap-2">
        <div className="space-y-1">
          <label className="text-xs text-slate-600 dark:text-slate-400">Критичность</label>
          <select
            className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            value={severity}
            onChange={(e) => setSeverity(e.target.value)}
          >
            <option value="warn">предупреждение</option>
            <option value="error">ошибка</option>
            <option value="info">инфо</option>
          </select>
        </div>
        <label className="inline-flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300 mt-6">
          <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
          Активно
        </label>
      </div>
      {error && <div className="text-red-500 text-sm">{error}</div>}
      {success && <div className="text-emerald-600 text-sm">{success}</div>}
      <div className="flex items-center gap-2">
        <button
          className="inline-flex items-center justify-center rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
          onClick={save}
          disabled={saving}
        >
          {saving ? "Сохраняем..." : "Сохранить"}
        </button>
        <button
          className="inline-flex items-center justify-center rounded-lg border border-red-200 px-3 py-1.5 text-xs font-semibold text-red-600 hover:bg-red-50 disabled:opacity-50 dark:border-red-700 dark:text-red-300 dark:hover:bg-red-900/30"
          onClick={remove}
          disabled={deleting}
        >
          <FiTrash2 /> Удалить
        </button>
      </div>
    </div>
  );
}
