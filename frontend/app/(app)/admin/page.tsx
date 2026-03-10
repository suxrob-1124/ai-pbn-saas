'use client';

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Users,
  ShieldAlert,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Trash2,
  Save,
  ChevronRight,
  ChevronDown,
  Code2,
  ShieldCheck,
  Plus,
  FolderGit2,
  Info,
  UserPlus,
  UserMinus,
  Loader2,
  RotateCcw,
  Archive,
} from 'lucide-react';
import Editor from '@monaco-editor/react';
import { showToast } from '@/lib/toastStore';

import { useAuthGuard } from '@/lib/useAuth';
import { authFetch, patch, post, del } from '@/lib/http';
import { useTheme } from '@/lib/useTheme';
import { GENERATION_STAGES, getStageLabel } from '@/lib/promptStages';
import { Badge } from '@/components/Badge';

// --- ТИПЫ ---
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
  owner_email?: string;
  name: string;
  status: string;
  target_country?: string;
  target_language?: string;
  created_at: string;
  updated_at: string;
};
type ProjectMemberDTO = {
  email: string;
  role: string;
  created_at: string;
};
type TrashProject = {
  id: string;
  name: string;
  userEmail: string;
  deletedAt: string;
  deletedBy: string;
};
type TrashDomain = {
  id: string;
  url: string;
  projectId: string;
  deletedAt: string;
  deletedBy: string;
};
type TrashList = {
  projects: TrashProject[];
  domains: TrashDomain[];
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

// Шпаргалка переменных для Prompt Studio
const PROMPT_VARIABLES = [
  { name: '{{ keyword }}', desc: 'Главное ключевое слово сайта' },
  { name: '{{ country }}', desc: 'Страна (гео), напр.: US, SE' },
  { name: '{{ language }}', desc: 'Язык текста, напр.: en-US, sv-SE' },
  { name: '{{ analysis_data }}', desc: 'Анализ конкурентов (JSON)' },
  { name: '{{ tech_spec }}', desc: 'Готовое ТЗ для копирайтера' },
  { name: '{{ contents_data }}', desc: 'Структура статьи (H2-H3 заголовки)' },
  { name: '{{ html_content }}', desc: 'Контент страницы в HTML' },
];

export default function AdminPage() {
  const router = useRouter();
  const { me, loading } = useAuthGuard();
  const { theme } = useTheme(); // Для синхронизации Monaco Editor с темой
  const isAdmin = useMemo(() => (me?.role || '').toLowerCase() === 'admin', [me]);

  const [uiView, setUiView] = useState<'users' | 'prompts' | 'audit' | 'trash'>('users');

  // --- СТЕЙТЫ ПОЛЬЗОВАТЕЛЕЙ ---
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [usersError, setUsersError] = useState<string | null>(null);
  const [usersLoading, setUsersLoading] = useState(false);
  const [selectedProjectsEmail, setSelectedProjectsEmail] = useState<string | null>(null);
  const [selectedProjects, setSelectedProjects] = useState<ProjectDTO[]>([]);
  const [projectsLoading, setProjectsLoading] = useState(false);
  // Управление участниками проектов
  const [projectMembers, setProjectMembers] = useState<Record<string, ProjectMemberDTO[]>>({});
  const [membersLoading, setMembersLoading] = useState<string | null>(null);
  const [addMemberProject, setAddMemberProject] = useState<string | null>(null);
  const [addMemberEmail, setAddMemberEmail] = useState('');
  const [addMemberRole, setAddMemberRole] = useState('editor');
  const [addingMember, setAddingMember] = useState(false);
  // Удаление пользователя
  const [deleteConfirmEmail, setDeleteConfirmEmail] = useState<string | null>(null);
  const [deletingUser, setDeletingUser] = useState(false);

  // --- СТЕЙТЫ ПРОМПТОВ (PROMPT STUDIO) ---
  const [prompts, setPrompts] = useState<AdminPrompt[]>([]);
  const [promptsError, setPromptsError] = useState<string | null>(null);
  const [promptsLoading, setPromptsLoading] = useState(false);
  const [selectedPromptId, setSelectedPromptId] = useState<string | 'new' | null>(null);
  // Драфт для редактора
  const [promptDraft, setPromptDraft] = useState({
    name: '',
    description: '',
    body: '',
    stage: '',
    model: '',
    isActive: true,
  });
  const [savingPrompt, setSavingPrompt] = useState(false);

  // --- СТЕЙТЫ ПРАВИЛ АУДИТА ---
  const [auditRules, setAuditRules] = useState<AdminAuditRule[]>([]);
  const [auditRulesError, setAuditRulesError] = useState<string | null>(null);
  const [auditRulesLoading, setAuditRulesLoading] = useState(false);
  const [newRule, setNewRule] = useState({
    code: '',
    title: '',
    description: '',
    severity: 'warn',
    isActive: true,
  });
  const [creatingRule, setCreatingRule] = useState(false);

  // --- СТЕЙТЫ КОРЗИНЫ ---
  const [trash, setTrash] = useState<TrashList>({ projects: [], domains: [] });
  const [trashLoading, setTrashLoading] = useState(false);
  const [trashError, setTrashError] = useState<string | null>(null);
  const [trashActionLoading, setTrashActionLoading] = useState<string | null>(null);

  // --- ЗАГРУЗКА ---
  const loadUsers = useCallback(async () => {
    setUsersLoading(true);
    setUsersError(null);
    try {
      setUsers(await authFetch<AdminUser[]>('/api/admin/users'));
    } catch (err: any) {
      setUsersError(err?.message || 'Ошибка загрузки');
    } finally {
      setUsersLoading(false);
    }
  }, []);

  const loadPrompts = useCallback(async () => {
    setPromptsLoading(true);
    setPromptsError(null);
    try {
      const list = await authFetch<AdminPrompt[]>('/api/admin/prompts');
      setPrompts(list);
      // Автовыбор первого промпта при загрузке
      if (list.length > 0 && !selectedPromptId) setSelectedPromptId(list[0].id);
    } catch (err: any) {
      setPromptsError(err?.message || 'Ошибка загрузки');
    } finally {
      setPromptsLoading(false);
    }
  }, [selectedPromptId]);

  const loadAuditRules = useCallback(async () => {
    setAuditRulesLoading(true);
    setAuditRulesError(null);
    try {
      setAuditRules(await authFetch<AdminAuditRule[]>('/api/admin/audit-rules'));
    } catch (err: any) {
      setAuditRulesError(err?.message || 'Ошибка загрузки');
    } finally {
      setAuditRulesLoading(false);
    }
  }, []);

  const loadTrash = useCallback(async () => {
    setTrashLoading(true);
    setTrashError(null);
    try {
      setTrash(await authFetch<TrashList>('/api/admin/trash'));
    } catch (err: any) {
      setTrashError(err?.message || 'Ошибка загрузки');
    } finally {
      setTrashLoading(false);
    }
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!isAdmin) {
      router.replace('/projects');
      return;
    }
    if (uiView === 'users') loadUsers();
    if (uiView === 'prompts') loadPrompts();
    if (uiView === 'audit') loadAuditRules();
    if (uiView === 'trash') loadTrash();
  }, [isAdmin, loading, router, loadUsers, loadPrompts, loadAuditRules, loadTrash, uiView]);

  // Подстановка данных в черновик при выборе промпта
  useEffect(() => {
    if (selectedPromptId === 'new') {
      setPromptDraft({ name: '', description: '', body: '', stage: '', model: '', isActive: true });
    } else if (selectedPromptId) {
      const p = prompts.find((x) => x.id === selectedPromptId);
      if (p)
        setPromptDraft({
          name: p.name,
          description: p.description || '',
          body: p.body,
          stage: p.stage || '',
          model: p.model || '',
          isActive: p.isActive,
        });
    }
  }, [selectedPromptId, prompts]);

  // --- ЭКШЕНЫ ПОЛЬЗОВАТЕЛЕЙ ---
  const handleRoleChange = async (email: string, role: string) => {
    await patch(`/api/admin/users/${encodeURIComponent(email)}`, { role });
    await loadUsers();
  };
  const handleApprovalToggle = async (email: string, approved: boolean) => {
    await patch(`/api/admin/users/${encodeURIComponent(email)}`, { isApproved: approved });
    await loadUsers();
  };
  const handleLoadProjects = async (email: string) => {
    if (selectedProjectsEmail === email) {
      setSelectedProjectsEmail(null);
      return;
    }
    setSelectedProjectsEmail(email);
    setProjectsLoading(true);
    setProjectMembers({});
    setAddMemberProject(null);
    try {
      setSelectedProjects(
        await authFetch<ProjectDTO[]>(`/api/admin/users/${encodeURIComponent(email)}/projects`),
      );
    } catch {
      setSelectedProjects([]);
    } finally {
      setProjectsLoading(false);
    }
  };

  const loadProjectMembers = async (projectId: string) => {
    setMembersLoading(projectId);
    try {
      const members = await authFetch<ProjectMemberDTO[]>(`/api/projects/${projectId}/members`);
      setProjectMembers((prev) => ({ ...prev, [projectId]: members }));
    } catch {
      setProjectMembers((prev) => ({ ...prev, [projectId]: [] }));
    } finally {
      setMembersLoading(null);
    }
  };

  const handleAddMember = async (projectId: string) => {
    const email = addMemberEmail.trim().toLowerCase();
    if (!email) return;
    setAddingMember(true);
    try {
      await post(`/api/projects/${projectId}/members`, { email, role: addMemberRole });
      showToast({ type: 'success', title: 'Участник добавлен' });
      setAddMemberEmail('');
      setAddMemberProject(null);
      await loadProjectMembers(projectId);
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    } finally {
      setAddingMember(false);
    }
  };

  const handleRemoveMember = async (projectId: string, email: string) => {
    try {
      await del(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`);
      showToast({ type: 'success', title: 'Доступ снят' });
      await loadProjectMembers(projectId);
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    }
  };

  const handleUpdateMemberRole = async (projectId: string, email: string, role: string) => {
    try {
      await patch(`/api/projects/${projectId}/members/${encodeURIComponent(email)}`, { role });
      showToast({ type: 'success', title: 'Роль обновлена' });
      await loadProjectMembers(projectId);
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    }
  };

  const handleDeleteUser = async (email: string) => {
    setDeletingUser(true);
    try {
      await del(`/api/admin/users/${encodeURIComponent(email)}`);
      showToast({ type: 'success', title: 'Пользователь удалён' });
      setDeleteConfirmEmail(null);
      await loadUsers();
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка удаления', message: err?.message });
    } finally {
      setDeletingUser(false);
    }
  };

  // --- ЭКШЕНЫ ПРОМПТОВ ---
  const handleSavePrompt = async () => {
    if (!promptDraft.name.trim() || !promptDraft.body.trim())
      return showToast({ type: 'error', title: 'Заполните Название и Текст' });
    setSavingPrompt(true);
    try {
      const payload = {
        name: promptDraft.name.trim(),
        description: promptDraft.description.trim() || undefined,
        body: promptDraft.body,
        stage: promptDraft.stage.trim() || undefined,
        model: promptDraft.model.trim() || undefined,
        isActive: promptDraft.isActive,
      };
      if (selectedPromptId === 'new') {
        await post('/api/admin/prompts', payload);
        showToast({ type: 'success', title: 'Промпт создан' });
        setSelectedPromptId(null); // Сбросим, чтобы loadPrompts выбрал новый
      } else {
        await patch(`/api/admin/prompts/${selectedPromptId}`, payload);
        showToast({ type: 'success', title: 'Промпт обновлен' });
      }
      await loadPrompts();
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    } finally {
      setSavingPrompt(false);
    }
  };

  const handleDeletePrompt = async () => {
    if (!confirm('Точно удалить системный промпт?')) return;
    setSavingPrompt(true);
    try {
      await del(`/api/admin/prompts/${selectedPromptId}`);
      showToast({ type: 'success', title: 'Удалено' });
      setSelectedPromptId(null);
      await loadPrompts();
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка удаления', message: err?.message });
    } finally {
      setSavingPrompt(false);
    }
  };

  if (!isAdmin) return null;

  const inputClass =
    'w-full bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-700 rounded-xl px-3 py-2 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none dark:text-white transition-all shadow-sm';
  const cardClass =
    'bg-white dark:bg-slate-900/80 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden';
  const tableHeaderClass =
    'text-left text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-800/40 border-b border-slate-200 dark:border-slate-800/60';

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-5">
      {/* ── Header ─────────────────────────────────────────────────── */}
      <div className="bg-white dark:bg-slate-900/80 border border-slate-200 dark:border-slate-800 rounded-2xl p-6 shadow-sm">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="flex items-center gap-1 text-xs font-medium text-slate-400 mb-1.5">
              <ShieldAlert className="w-3.5 h-3.5" />
              <span>Система</span>
              <ChevronRight className="w-3.5 h-3.5 opacity-40" />
              <span className="text-slate-600 dark:text-slate-300">Админ-панель</span>
            </div>
            <h1 className="text-xl font-bold text-slate-900 dark:text-white">
              Управление платформой
            </h1>
            <p className="text-sm text-slate-500 dark:text-slate-400 mt-0.5">
              Пользователи, промпты и правила аудита
            </p>
          </div>
          <div className="flex items-center gap-1 bg-slate-100 p-1 rounded-xl dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700/60 flex-shrink-0">
            <TabBtn
              active={uiView === 'users'}
              onClick={() => setUiView('users')}
              icon={<Users className="w-4 h-4" />}
              label="Пользователи"
            />
            <TabBtn
              active={uiView === 'prompts'}
              onClick={() => setUiView('prompts')}
              icon={<Code2 className="w-4 h-4" />}
              label="Промпты"
            />
            <TabBtn
              active={uiView === 'trash'}
              onClick={() => setUiView('trash')}
              icon={<Archive className="w-4 h-4" />}
              label="Корзина"
            />
          </div>
        </div>
      </div>
          {/* ========================================================= */}
          {/* TAB 1: ПОЛЬЗОВАТЕЛИ (Без изменений) */}
          {/* ========================================================= */}
          {uiView === 'users' && (
            <div className={cardClass}>
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-indigo-50 dark:bg-indigo-900/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                    <Users className="w-4 h-4" />
                  </div>
                  <h3 className="font-bold text-slate-900 dark:text-white">База пользователей</h3>
                </div>
                <button
                  onClick={loadUsers}
                  disabled={usersLoading}
                  className="inline-flex items-center gap-2 p-2 rounded-xl bg-white border border-slate-200 text-slate-700 hover:bg-slate-50 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-300 dark:hover:bg-[#0a1020] transition-colors">
                  <RefreshCw className={`w-4 h-4 ${usersLoading ? 'animate-spin' : ''}`} />
                </button>
              </div>

              {usersError && <div className="p-4 text-sm text-red-500">{usersError}</div>}

              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className={tableHeaderClass}>
                      <th className="py-3 px-5">Пользователь</th>
                      <th className="py-3 px-5">Роль</th>
                      <th className="py-3 px-5">Доступы</th>
                      <th className="py-3 px-5 text-right">Действия</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-800/40">
                    {users.map((user) => {
                      const isExpanded = selectedProjectsEmail === user.email;
                      return (
                      <React.Fragment key={user.email}>
                      <tr
                        className={`border-b border-slate-100 dark:border-slate-800/40 hover:bg-slate-50/50 dark:hover:bg-white/[0.02] transition-colors ${isExpanded ? 'bg-slate-50 dark:bg-white/[0.02]' : ''}`}>
                        <td className="py-3 px-5">
                          <div className="font-medium text-slate-900 dark:text-white">
                            {user.email}
                          </div>
                          {user.name && (
                            <div className="text-xs text-slate-500 mt-0.5">{user.name}</div>
                          )}
                        </td>
                        <td className="py-3 px-5">
                          {user.email === me?.email ? (
                            <Badge label="Вы (Админ)" tone="indigo" />
                          ) : user.role === 'admin' ? (
                            <Badge label="Админ" tone="indigo" />
                          ) : (
                            <select
                              className="bg-white dark:bg-[#060d18] border border-slate-200 dark:border-slate-700 rounded-lg text-xs px-2 py-1 outline-none focus:border-indigo-500 font-medium text-slate-700 dark:text-slate-300"
                              value={user.role}
                              onChange={(e) => handleRoleChange(user.email, e.target.value)}>
                              <option value="user">Пользователь</option>
                              <option value="manager">Менеджер</option>
                              <option value="admin">Админ</option>
                            </select>
                          )}
                        </td>
                        <td className="py-3 px-5">
                          <div className="flex flex-col gap-1.5">
                            {user.verified ? (
                              <span className="inline-flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400 text-xs font-medium">
                                <CheckCircle2 className="w-3.5 h-3.5" /> Email подтвержден
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1.5 text-slate-400 text-xs font-medium">
                                <XCircle className="w-3.5 h-3.5" /> Ожидает email
                              </span>
                            )}
                            {user.isApproved ? (
                              <span className="inline-flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400 text-xs font-medium">
                                <CheckCircle2 className="w-3.5 h-3.5" /> Активен
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1.5 text-amber-500 dark:text-amber-400 text-xs font-medium">
                                <XCircle className="w-3.5 h-3.5" /> Заблокирован
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="py-3 px-5 text-right">
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => handleLoadProjects(user.email)}
                              className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${isExpanded ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/50 dark:text-indigo-300' : 'bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700'}`}>
                              {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
                              Проекты
                            </button>
                            {user.email !== me?.email && (
                              <>
                                <button
                                  onClick={() => handleApprovalToggle(user.email, !user.isApproved)}
                                  className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors border ${user.isApproved ? 'border-amber-200 text-amber-700 hover:bg-amber-50 dark:border-amber-900/50 dark:text-amber-400 dark:hover:bg-amber-900/20' : 'border-emerald-200 text-emerald-700 hover:bg-emerald-50 dark:border-emerald-900/50 dark:text-emerald-400 dark:hover:bg-emerald-900/20'}`}>
                                  {user.isApproved ? 'Блок' : 'Активировать'}
                                </button>
                                {user.role !== 'admin' && (
                                  <button
                                    onClick={() => setDeleteConfirmEmail(user.email)}
                                    className="p-1.5 text-xs rounded-lg text-red-500 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 transition-colors border border-transparent hover:border-red-200 dark:hover:border-red-900/50"
                                    title="Удалить пользователя">
                                    <Trash2 className="w-3.5 h-3.5" />
                                  </button>
                                )}
                              </>
                            )}
                          </div>
                        </td>
                      </tr>
                      {/* Expandable projects row */}
                      {isExpanded && (
                        <tr>
                          <td colSpan={4} className="p-0">
                            <div className="bg-slate-50/80 dark:bg-slate-800/20 border-b border-slate-200 dark:border-slate-800/60 px-5 py-4">
                              {projectsLoading ? (
                                <div className="flex items-center gap-2 text-sm text-slate-500">
                                  <Loader2 className="w-4 h-4 animate-spin" /> Загрузка проектов…
                                </div>
                              ) : selectedProjects.length === 0 ? (
                                <div className="text-sm text-slate-500">У пользователя нет проектов</div>
                              ) : (
                                <div className="space-y-3">
                                  <div className="text-[10px] font-bold uppercase tracking-wider text-slate-400">
                                    Проекты ({selectedProjects.length})
                                  </div>
                                  {selectedProjects.map((proj) => {
                                    const members = projectMembers[proj.id];
                                    const isLoadingMembers = membersLoading === proj.id;
                                    const isAddingTo = addMemberProject === proj.id;
                                    return (
                                      <div key={proj.id} className="bg-white dark:bg-slate-900/60 border border-slate-200 dark:border-slate-700/60 rounded-xl p-3">
                                        <div className="flex items-center justify-between">
                                          <div>
                                            <div className="font-semibold text-sm text-slate-900 dark:text-white">{proj.name}</div>
                                            <div className="text-[11px] text-slate-500 mt-0.5 flex items-center gap-2">
                                              <Badge label={proj.status} tone={proj.status === 'active' ? 'green' : 'slate'} />
                                              {proj.target_country && <span>{proj.target_country}/{proj.target_language}</span>}
                                            </div>
                                          </div>
                                          <button
                                            onClick={() => {
                                              if (members) {
                                                setProjectMembers((prev) => {
                                                  const next = { ...prev };
                                                  delete next[proj.id];
                                                  return next;
                                                });
                                              } else {
                                                loadProjectMembers(proj.id);
                                              }
                                            }}
                                            className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 transition-colors">
                                            <Users className="w-3.5 h-3.5" />
                                            Участники
                                            {members ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                                          </button>
                                        </div>

                                        {/* Members section */}
                                        {isLoadingMembers && (
                                          <div className="mt-3 flex items-center gap-2 text-xs text-slate-500">
                                            <Loader2 className="w-3.5 h-3.5 animate-spin" /> Загрузка участников…
                                          </div>
                                        )}
                                        {members && !isLoadingMembers && (
                                          <div className="mt-3 border-t border-slate-100 dark:border-slate-700/40 pt-3 space-y-2">
                                            {members.map((m) => {
                                              const isProjectOwner = m.email === (proj.owner_email || selectedProjectsEmail);
                                              return (
                                              <div key={m.email} className="flex items-center justify-between text-xs">
                                                <span className="font-medium text-slate-700 dark:text-slate-300">{m.email}</span>
                                                <div className="flex items-center gap-2">
                                                  {isProjectOwner ? (
                                                    <Badge label="Создатель" tone="amber" />
                                                  ) : (
                                                    <select
                                                      className="bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-700 rounded-lg px-2 py-1 text-xs outline-none focus:border-indigo-500 dark:text-white"
                                                      value={m.role}
                                                      onChange={(e) => handleUpdateMemberRole(proj.id, m.email, e.target.value)}>
                                                      <option value="manager">Менеджер</option>
                                                      <option value="editor">Редактор</option>
                                                      <option value="viewer">Наблюдатель</option>
                                                    </select>
                                                  )}
                                                  {!isProjectOwner && (
                                                    <button
                                                      onClick={() => handleRemoveMember(proj.id, m.email)}
                                                      className="inline-flex items-center gap-1 text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 transition-colors"
                                                      title="Снять доступ">
                                                      <UserMinus className="w-3.5 h-3.5" />
                                                    </button>
                                                  )}
                                                </div>
                                              </div>
                                              );
                                            })}

                                            {/* Add member form */}
                                            {isAddingTo ? (
                                              <div className="flex items-center gap-2 mt-2">
                                                <input
                                                  type="email"
                                                  placeholder="email@example.com"
                                                  value={addMemberEmail}
                                                  onChange={(e) => setAddMemberEmail(e.target.value)}
                                                  className="flex-1 bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-700 rounded-lg px-2.5 py-1.5 text-xs outline-none focus:border-indigo-500 dark:text-white"
                                                />
                                                <select
                                                  value={addMemberRole}
                                                  onChange={(e) => setAddMemberRole(e.target.value)}
                                                  className="bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-700 rounded-lg px-2 py-1.5 text-xs outline-none focus:border-indigo-500 dark:text-white">
                                                  <option value="manager">Менеджер</option>
                                                  <option value="editor">Редактор</option>
                                                  <option value="viewer">Наблюдатель</option>
                                                </select>
                                                <button
                                                  onClick={() => handleAddMember(proj.id)}
                                                  disabled={addingMember || !addMemberEmail.trim()}
                                                  className="px-3 py-1.5 text-xs font-medium rounded-lg bg-indigo-600 text-white hover:bg-indigo-500 disabled:opacity-50 transition-colors">
                                                  {addingMember ? '…' : 'Добавить'}
                                                </button>
                                                <button
                                                  onClick={() => { setAddMemberProject(null); setAddMemberEmail(''); }}
                                                  className="px-2 py-1.5 text-xs text-slate-500 hover:text-slate-700 dark:hover:text-slate-300">
                                                  Отмена
                                                </button>
                                              </div>
                                            ) : (
                                              <button
                                                onClick={() => { setAddMemberProject(proj.id); setAddMemberEmail(''); setAddMemberRole('editor'); }}
                                                className="inline-flex items-center gap-1.5 text-xs text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 font-medium mt-1 transition-colors">
                                                <UserPlus className="w-3.5 h-3.5" /> Добавить участника
                                              </button>
                                            )}
                                          </div>
                                        )}
                                      </div>
                                    );
                                  })}
                                </div>
                              )}
                            </div>
                          </td>
                        </tr>
                      )}
                      </React.Fragment>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* Confirmation modal for user deletion */}
          {deleteConfirmEmail && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
              <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-2xl shadow-xl max-w-md w-full mx-4 p-6">
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-10 h-10 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
                    <Trash2 className="w-5 h-5 text-red-600 dark:text-red-400" />
                  </div>
                  <div>
                    <h3 className="font-bold text-slate-900 dark:text-white">Удалить пользователя?</h3>
                    <p className="text-sm text-slate-500 dark:text-slate-400 mt-0.5">Это действие нельзя отменить</p>
                  </div>
                </div>
                <p className="text-sm text-slate-700 dark:text-slate-300 mb-6">
                  Пользователь <span className="font-semibold">{deleteConfirmEmail}</span> будет удалён вместе со всеми сессиями и участием в проектах.
                </p>
                <div className="flex items-center justify-end gap-3">
                  <button
                    onClick={() => setDeleteConfirmEmail(null)}
                    disabled={deletingUser}
                    className="px-4 py-2 text-sm font-medium rounded-xl bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 transition-colors">
                    Отмена
                  </button>
                  <button
                    onClick={() => handleDeleteUser(deleteConfirmEmail)}
                    disabled={deletingUser}
                    className="px-4 py-2 text-sm font-semibold rounded-xl bg-red-600 text-white hover:bg-red-500 disabled:opacity-50 transition-colors flex items-center gap-2">
                    {deletingUser ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4" />}
                    {deletingUser ? 'Удаление…' : 'Удалить'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* ========================================================= */}
          {/* TAB 2: PROMPT STUDIO (Новый UX с Monaco Editor)           */}
          {/* ========================================================= */}
          {uiView === 'prompts' && (
            <div className={`${cardClass} flex flex-col md:flex-row min-h-[700px]`}>
              {/* ЛЕВАЯ КОЛОНКА: СПИСОК */}
              <div className="w-full md:w-72 border-r border-slate-200 dark:border-slate-700 bg-slate-50/50 dark:bg-[#0a1020] flex flex-col flex-shrink-0">
                <div className="p-4 border-b border-slate-200 dark:border-slate-700 flex items-center justify-between">
                  <button
                    onClick={() => setSelectedPromptId('new')}
                    className="w-full flex items-center justify-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white py-2 rounded-xl text-sm font-semibold transition-colors shadow-sm">
                    <Plus className="w-4 h-4" /> Новый промпт
                  </button>
                </div>

                <div className="p-3 flex-1 overflow-y-auto space-y-1">
                  <div className="text-[10px] font-bold uppercase tracking-wider text-slate-400 px-2 pb-2">
                    Системные промпты
                  </div>
                  {prompts.map((p) => (
                    <button
                      key={p.id}
                      onClick={() => setSelectedPromptId(p.id)}
                      className={`w-full text-left px-3 py-2.5 rounded-xl text-sm transition-all flex flex-col gap-1 ${
                        selectedPromptId === p.id
                          ? 'bg-white dark:bg-[#1a2235] shadow-sm border border-slate-200 dark:border-slate-700'
                          : 'border border-transparent hover:bg-slate-200/50 dark:hover:bg-slate-800/50'
                      }`}>
                      <div className="flex items-center justify-between">
                        <span
                          className={`font-semibold truncate ${selectedPromptId === p.id ? 'text-indigo-600 dark:text-indigo-400' : 'text-slate-700 dark:text-slate-300'}`}>
                          {p.name}
                        </span>
                        {p.isActive && (
                          <div className="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)] flex-shrink-0" />
                        )}
                      </div>
                      <span className="text-[10px] text-slate-500 font-mono truncate">
                        {p.stage || 'Универсальный'}
                      </span>
                    </button>
                  ))}
                  {prompts.length === 0 && !promptsLoading && (
                    <div className="text-xs text-slate-500 px-2">Список пуст</div>
                  )}
                </div>
              </div>

              {/* ПРАВАЯ КОЛОНКА: РЕДАКТОР И МЕТАДАННЫЕ */}
              <div className="flex-1 flex flex-col bg-white dark:bg-[#0f1523]">
                {selectedPromptId ? (
                  <div className="flex flex-col h-full animate-in fade-in">
                    {/* Метаданные (Шапка) */}
                    <div className="p-6 border-b border-slate-100 dark:border-slate-800/60 grid grid-cols-1 xl:grid-cols-2 gap-4">
                      <div className="space-y-3">
                        <input
                          className="w-full text-lg font-bold bg-transparent outline-none placeholder:text-slate-300 dark:placeholder:text-slate-700 text-slate-900 dark:text-white"
                          placeholder="Название промпта..."
                          value={promptDraft.name}
                          onChange={(e) => setPromptDraft((p) => ({ ...p, name: e.target.value }))}
                        />
                        <input
                          className="w-full text-sm bg-transparent outline-none placeholder:text-slate-400 text-slate-600 dark:text-slate-400"
                          placeholder="Описание (зачем нужен)..."
                          value={promptDraft.description}
                          onChange={(e) =>
                            setPromptDraft((p) => ({ ...p, description: e.target.value }))
                          }
                        />
                      </div>

                      <div className="flex flex-col justify-end gap-3 xl:items-end">
                        <div className="flex flex-wrap items-center gap-2">
                          <select
                            className="bg-slate-50 dark:bg-[#060d18] border border-slate-200 dark:border-slate-700 rounded-lg text-xs px-3 py-2 outline-none focus:border-indigo-500 font-medium text-slate-700 dark:text-slate-300"
                            value={promptDraft.stage}
                            onChange={(e) =>
                              setPromptDraft((p) => ({ ...p, stage: e.target.value }))
                            }>
                            <option value="">— Универсальный этап —</option>
                            {GENERATION_STAGES.map((s) => (
                              <option key={s.value} value={s.value}>
                                {s.label}
                              </option>
                            ))}
                          </select>
                          <select
                            className="bg-slate-50 dark:bg-[#060d18] border border-slate-200 dark:border-slate-700 rounded-lg text-xs px-3 py-2 outline-none focus:border-indigo-500 font-mono text-slate-700 dark:text-slate-300 w-36"
                            value={promptDraft.model}
                            onChange={(e) =>
                              setPromptDraft((p) => ({ ...p, model: e.target.value }))
                            }>
                            <option value="">Default Model</option>
                            <option value="gemini-2.5-pro">gemini-2.5-pro</option>
                            <option value="gemini-2.5-flash">gemini-2.5-flash</option>
                            <option value="gemini-2.5-flash-image">flash-image</option>
                          </select>
                        </div>
                        <label className="flex items-center gap-2 cursor-pointer group">
                          <div className="relative flex items-center">
                            <input
                              type="checkbox"
                              checked={promptDraft.isActive}
                              onChange={(e) =>
                                setPromptDraft((p) => ({ ...p, isActive: e.target.checked }))
                              }
                              className="sr-only peer"
                            />
                            <div className="w-8 h-4 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-3 after:w-3 after:transition-all peer-checked:bg-emerald-500"></div>
                          </div>
                          <span className="text-xs font-medium text-slate-500 dark:text-slate-400 group-hover:text-slate-900 dark:group-hover:text-white transition-colors">
                            Активен в системе
                          </span>
                        </label>
                      </div>
                    </div>

                    {/* Рабочая зона: Monaco + Шпаргалка */}
                    <div className="flex-1 flex flex-col xl:flex-row min-h-0">
                      <div className="flex-1 flex flex-col bg-[#1e1e1e] dark:bg-[#060d18] relative">
                        <div className="p-2 border-b border-[#2d2d2d] dark:border-slate-800 flex justify-between items-center text-xs font-mono text-[#858585]">
                          <span>system_prompt.txt</span>
                          <span className="bg-[#2d2d2d] px-2 py-0.5 rounded">Handlebars</span>
                        </div>
                        <div className="flex-1 relative">
                          <Editor
                            height="100%"
                            language="handlebars"
                            theme={theme === 'dark' ? 'vs-dark' : 'vs-dark'} // Monaco круто выглядит темным всегда для кода
                            value={promptDraft.body}
                            onChange={(val) => setPromptDraft((p) => ({ ...p, body: val || '' }))}
                            options={{
                              minimap: { enabled: false },
                              wordWrap: 'on',
                              fontSize: 13,
                              padding: { top: 16 },
                              fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
                            }}
                          />
                        </div>
                      </div>

                      {/* Шпаргалка */}
                      <div className="w-full xl:w-64 border-l border-slate-100 dark:border-slate-800/80 bg-slate-50/50 dark:bg-[#0a1020] p-4 overflow-y-auto">
                        <h4 className="text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-3 flex items-center gap-1.5">
                          <Info className="w-3.5 h-3.5" /> Доступные переменные
                        </h4>
                        <div className="space-y-3">
                          {PROMPT_VARIABLES.map((v) => (
                            <div key={v.name} className="group">
                              <code className="text-xs font-bold text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-900/30 px-1.5 py-0.5 rounded cursor-pointer select-all">
                                {v.name}
                              </code>
                              <p className="text-[11px] text-slate-500 mt-1 leading-snug group-hover:text-slate-700 dark:group-hover:text-slate-300 transition-colors">
                                {v.desc}
                              </p>
                            </div>
                          ))}
                        </div>
                      </div>
                    </div>

                    {/* Подвал с кнопками сохранения */}
                    <div className="p-4 border-t border-slate-100 dark:border-slate-800/80 bg-white dark:bg-[#0f1523] flex items-center justify-end gap-3">
                      {selectedPromptId !== 'new' && (
                        <button
                          onClick={handleDeletePrompt}
                          disabled={savingPrompt}
                          className="px-4 py-2.5 rounded-xl text-sm font-medium text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20 transition-colors flex items-center gap-2 mr-auto">
                          <Trash2 className="w-4 h-4" /> Удалить
                        </button>
                      )}
                      <button
                        onClick={handleSavePrompt}
                        disabled={savingPrompt}
                        className="px-6 py-2.5 rounded-xl text-sm font-semibold bg-indigo-600 text-white hover:bg-indigo-500 transition-all shadow-sm active:scale-95 flex items-center gap-2">
                        <Save className="w-4 h-4" />{' '}
                        {savingPrompt ? 'Сохранение...' : 'Сохранить промпт'}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex-1 flex flex-col items-center justify-center text-slate-400 p-10 bg-slate-50/50 dark:bg-transparent">
                    <Code2 className="w-16 h-16 mb-4 opacity-20" />
                    <h3 className="text-lg font-medium text-slate-900 dark:text-white mb-1">
                      Редактор промптов
                    </h3>
                    <p className="text-sm max-w-sm text-center">
                      Выберите системный промпт слева для настройки или создайте новый.
                    </p>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ========================================================= */}
          {/* TAB 3: КОРЗИНА */}
          {/* ========================================================= */}
          {uiView === 'trash' && (
            <div className={cardClass}>
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-orange-50 dark:bg-orange-900/30 flex items-center justify-center text-orange-600 dark:text-orange-400">
                    <Archive className="w-4 h-4" />
                  </div>
                  <h3 className="font-bold text-slate-900 dark:text-white">Корзина</h3>
                </div>
                <button
                  onClick={loadTrash}
                  disabled={trashLoading}
                  className="inline-flex items-center gap-2 p-2 rounded-xl bg-white border border-slate-200 text-slate-700 hover:bg-slate-50 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-300 dark:hover:bg-[#0a1020] transition-colors">
                  <RefreshCw className={`w-4 h-4 ${trashLoading ? 'animate-spin' : ''}`} />
                </button>
              </div>

              {trashError && (
                <div className="p-4 bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 text-sm">
                  {trashError}
                </div>
              )}

              <div className="divide-y divide-slate-100 dark:divide-slate-800/60">
                {/* Удалённые проекты */}
                {trash.projects.length > 0 && (
                  <div className="p-5">
                    <h4 className="text-sm font-semibold text-slate-500 dark:text-slate-400 mb-3 flex items-center gap-2">
                      <FolderGit2 className="w-4 h-4" /> Проекты ({trash.projects.length})
                    </h4>
                    <div className="space-y-2">
                      {trash.projects.map((p) => {
                        const deletedDate = new Date(p.deletedAt);
                        const daysAgo = Math.floor((Date.now() - deletedDate.getTime()) / 86400000);
                        return (
                          <div
                            key={p.id}
                            className="flex items-center justify-between p-3 rounded-xl bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-700/40"
                          >
                            <div>
                              <div className="font-medium text-slate-900 dark:text-white text-sm">{p.name}</div>
                              <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                                Удалено {daysAgo === 0 ? 'сегодня' : `${daysAgo} дн. назад`} · {p.deletedBy}
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                onClick={async () => {
                                  setTrashActionLoading(p.id);
                                  try {
                                    await post(`/api/admin/trash/projects/${p.id}/restore`, {});
                                    showToast({ title: `Проект "${p.name}" восстановлен`, type: 'success' });
                                    loadTrash();
                                  } catch (err: any) {
                                    showToast({ title: err?.message || 'Ошибка восстановления', type: 'error' });
                                  } finally {
                                    setTrashActionLoading(null);
                                  }
                                }}
                                disabled={trashActionLoading === p.id}
                                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-green-50 text-green-700 hover:bg-green-100 dark:bg-green-900/20 dark:text-green-400 dark:hover:bg-green-900/40 transition-colors"
                              >
                                <RotateCcw className="w-3.5 h-3.5" /> Восстановить
                              </button>
                              <button
                                onClick={async () => {
                                  if (!confirm('Удалить проект навсегда? Это действие необратимо.')) return;
                                  setTrashActionLoading(p.id);
                                  try {
                                    await del(`/api/admin/trash/projects/${p.id}`);
                                    showToast({ title: 'Проект удалён навсегда', type: 'success' });
                                    loadTrash();
                                  } catch (err: any) {
                                    showToast({ title: err?.message || 'Ошибка удаления', type: 'error' });
                                  } finally {
                                    setTrashActionLoading(null);
                                  }
                                }}
                                disabled={trashActionLoading === p.id}
                                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-red-50 text-red-600 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400 dark:hover:bg-red-900/40 transition-colors"
                              >
                                <Trash2 className="w-3.5 h-3.5" /> Удалить
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                {/* Удалённые домены */}
                {trash.domains.length > 0 && (
                  <div className="p-5">
                    <h4 className="text-sm font-semibold text-slate-500 dark:text-slate-400 mb-3 flex items-center gap-2">
                      <ShieldCheck className="w-4 h-4" /> Домены ({trash.domains.length})
                    </h4>
                    <div className="space-y-2">
                      {trash.domains.map((d) => {
                        const deletedDate = new Date(d.deletedAt);
                        const daysAgo = Math.floor((Date.now() - deletedDate.getTime()) / 86400000);
                        return (
                          <div
                            key={d.id}
                            className="flex items-center justify-between p-3 rounded-xl bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-700/40"
                          >
                            <div>
                              <div className="font-medium text-slate-900 dark:text-white text-sm">{d.url}</div>
                              <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                                Удалено {daysAgo === 0 ? 'сегодня' : `${daysAgo} дн. назад`} · {d.deletedBy}
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                onClick={async () => {
                                  setTrashActionLoading(d.id);
                                  try {
                                    await post(`/api/admin/trash/domains/${d.id}/restore`, {});
                                    showToast({ title: `Домен "${d.url}" восстановлен`, type: 'success' });
                                    loadTrash();
                                  } catch (err: any) {
                                    showToast({ title: err?.message || 'Ошибка восстановления', type: 'error' });
                                  } finally {
                                    setTrashActionLoading(null);
                                  }
                                }}
                                disabled={trashActionLoading === d.id}
                                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-green-50 text-green-700 hover:bg-green-100 dark:bg-green-900/20 dark:text-green-400 dark:hover:bg-green-900/40 transition-colors"
                              >
                                <RotateCcw className="w-3.5 h-3.5" /> Восстановить
                              </button>
                              <button
                                onClick={async () => {
                                  if (!confirm('Удалить домен навсегда? Это действие необратимо.')) return;
                                  setTrashActionLoading(d.id);
                                  try {
                                    await del(`/api/admin/trash/domains/${d.id}`);
                                    showToast({ title: 'Домен удалён навсегда', type: 'success' });
                                    loadTrash();
                                  } catch (err: any) {
                                    showToast({ title: err?.message || 'Ошибка удаления', type: 'error' });
                                  } finally {
                                    setTrashActionLoading(null);
                                  }
                                }}
                                disabled={trashActionLoading === d.id}
                                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-red-50 text-red-600 hover:bg-red-100 dark:bg-red-900/20 dark:text-red-400 dark:hover:bg-red-900/40 transition-colors"
                              >
                                <Trash2 className="w-3.5 h-3.5" /> Удалить
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                {/* Пустое состояние */}
                {!trashLoading && trash.projects.length === 0 && trash.domains.length === 0 && (
                  <div className="p-10 flex flex-col items-center justify-center text-slate-400">
                    <Archive className="w-12 h-12 mb-3 opacity-20" />
                    <p className="text-sm">Корзина пуста</p>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

// Вспомогательная кнопка-вкладка
function TabBtn({ active, icon, label, onClick }: any) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-2 px-4 py-2 text-sm font-semibold rounded-xl transition-all ${
        active
          ? 'bg-white text-slate-900 shadow-sm dark:bg-slate-700/60 dark:text-white'
          : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'
      }`}
    >
      {icon}
      <span className="hidden sm:inline">{label}</span>
    </button>
  );
}