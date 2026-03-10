import { Settings, Save, MapPin, Search, Activity } from 'lucide-react';
import { PromptOverridesPanel } from '../../../components/PromptOverridesPanel';
import { ProjectMembersSection } from './ProjectMembersSection';

type Member = { email: string; role: string; createdAt: string };

type ProjectSettingsSectionProps = {
  projectId: string;
  projectSettingsLoading: boolean;
  projectSettingsError: string | null;
  projectName: string;
  projectCountry: string;
  projectLanguage: string;
  timezoneQuery: string;
  resolvedProjectTimezone: string;
  projectTimezone: string;
  recentFiltered: string[];
  timezoneGroups: Array<[string, string[]]>;
  canEditPrompts: boolean;
  members: Member[];
  loading: boolean;
  newMemberEmail: string;
  newMemberRole: string;
  currentUserEmail?: string;
  indexCheckEnabled?: boolean;
  indexCheckLoading?: boolean;
  getTimezoneOffsetLabel: (timezone: string) => string;
  formatDateTime: (value?: string) => string;
  onSaveProjectSettings: () => void;
  onProjectNameChange: (value: string) => void;
  onProjectCountryChange: (value: string) => void;
  onProjectLanguageChange: (value: string) => void;
  onTimezoneQueryChange: (value: string) => void;
  onProjectTimezoneChange: (value: string) => void;
  onUseRecentTimezone: (value: string) => void;
  onNewMemberEmailChange: (value: string) => void;
  onNewMemberRoleChange: (value: string) => void;
  onAddMember: () => void;
  onUpdateMemberRole: (email: string, role: string) => void;
  onRemoveMember: (email: string) => void;
  onToggleIndexCheck?: (enabled: boolean) => void;
};

export function ProjectSettingsSection({
  projectId,
  projectSettingsLoading,
  projectSettingsError,
  projectName,
  projectCountry,
  projectLanguage,
  timezoneQuery,
  resolvedProjectTimezone,
  projectTimezone,
  recentFiltered,
  timezoneGroups,
  canEditPrompts,
  members,
  loading,
  newMemberEmail,
  newMemberRole,
  currentUserEmail,
  getTimezoneOffsetLabel,
  formatDateTime,
  onSaveProjectSettings,
  onProjectNameChange,
  onProjectCountryChange,
  onProjectLanguageChange,
  onTimezoneQueryChange,
  onProjectTimezoneChange,
  onUseRecentTimezone,
  onNewMemberEmailChange,
  onNewMemberRoleChange,
  onAddMember,
  onUpdateMemberRole,
  onRemoveMember,
  indexCheckEnabled,
  indexCheckLoading,
  onToggleIndexCheck,
}: ProjectSettingsSectionProps) {
  const inputBaseClass =
    'w-full bg-white dark:bg-[#060d18] border border-slate-300 dark:border-slate-700 px-4 py-2.5 text-sm rounded-xl outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 dark:text-slate-100 placeholder:text-slate-400 transition-all';

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* ГЛАВНЫЙ БЛОК НАСТРОЕК */}
      <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm overflow-hidden">
        <div className="p-6 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-[#0a1020] flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h3 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
              <Settings className="w-5 h-5 text-indigo-500" /> Основные настройки
            </h3>
            <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
              Профиль проекта и часовой пояс.
            </p>
          </div>
          <button
            onClick={onSaveProjectSettings}
            disabled={projectSettingsLoading || !projectName.trim()}
            className="inline-flex items-center gap-2 bg-indigo-600 text-white px-5 py-2.5 rounded-xl text-sm font-semibold hover:bg-indigo-500 transition-all shadow-sm disabled:opacity-50 active:scale-95">
            <Save className="w-4 h-4" />
            {projectSettingsLoading ? 'Сохранение...' : 'Сохранить изменения'}
          </button>
        </div>

        {projectSettingsError && (
          <div className="mx-6 mt-6 p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50">
            {projectSettingsError}
          </div>
        )}

        <div className="p-6 grid gap-8 lg:grid-cols-2">
          {/* Левая колонка: Профиль */}
          <div className="space-y-5">
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-2">
                Название проекта <span className="text-red-500">*</span>
              </label>
              <input
                className={inputBaseClass}
                value={projectName}
                onChange={(e) => onProjectNameChange(e.target.value)}
                placeholder="Например: PBN Health"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-2">
                  Страна (Гео)
                </label>
                <input
                  className={inputBaseClass}
                  value={projectCountry}
                  onChange={(e) => onProjectCountryChange(e.target.value)}
                  placeholder="US, DE, RU..."
                />
              </div>
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-2">
                  Язык
                </label>
                <input
                  className={inputBaseClass}
                  value={projectLanguage}
                  onChange={(e) => onProjectLanguageChange(e.target.value)}
                  placeholder="en-US, de-DE..."
                />
              </div>
            </div>
          </div>

          {/* Правая колонка: Таймзона */}
          <div>
            <label className="block text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-2 flex items-center gap-1.5">
              <MapPin className="w-3.5 h-3.5" /> Часовой пояс
            </label>
            <div className="bg-slate-50 dark:bg-[#0a1020] border border-slate-200 dark:border-slate-700/60 rounded-xl overflow-hidden">
              <div className="p-3 border-b border-slate-200 dark:border-slate-700/60 relative">
                <Search className="absolute left-6 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
                <input
                  className="w-full pl-10 pr-4 py-2 bg-white dark:bg-[#060d18] border border-slate-200 dark:border-slate-700 rounded-lg text-sm focus:ring-2 focus:ring-indigo-500/20 outline-none dark:text-white transition-all"
                  value={timezoneQuery}
                  onChange={(e) => onTimezoneQueryChange(e.target.value)}
                  placeholder="Поиск (Europe/Berlin)"
                />
              </div>

              <div className="p-4 space-y-4 max-h-[250px] overflow-y-auto">
                {/* Текущая */}
                <div>
                  <div className="text-[10px] uppercase tracking-widest text-slate-400 mb-2 font-bold">
                    Выбранная зона
                  </div>
                  <div className="flex items-center justify-between bg-indigo-50 dark:bg-indigo-500/10 border border-indigo-200 dark:border-indigo-500/30 px-3 py-2.5 rounded-lg text-sm text-indigo-900 dark:text-indigo-200 font-medium">
                    <span>{resolvedProjectTimezone || 'UTC'}</span>
                    <span className="text-xs opacity-70">
                      {getTimezoneOffsetLabel(resolvedProjectTimezone)}
                    </span>
                  </div>
                </div>

                {/* Недавние */}
                {recentFiltered.length > 0 && (
                  <div>
                    <div className="text-[10px] uppercase tracking-widest text-slate-400 mb-2 font-bold">
                      Недавние
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {recentFiltered.map((tz) => (
                        <button
                          key={`recent-${tz}`}
                          type="button"
                          onClick={() => onUseRecentTimezone(tz)}
                          className="px-3 py-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-xs font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors flex items-center gap-2">
                          {tz}{' '}
                          <span className="opacity-50 text-[10px]">
                            {getTimezoneOffsetLabel(tz)}
                          </span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {/* Все */}
                <div>
                  <div className="text-[10px] uppercase tracking-widest text-slate-400 mb-2 font-bold">
                    Все доступные
                  </div>
                  <div className="space-y-3">
                    {timezoneGroups.map(([group, items]) => (
                      <div key={group} className="space-y-1.5">
                        <div className="text-[11px] font-semibold text-slate-400 px-1">{group}</div>
                        {items.map((tz) => (
                          <button
                            key={tz}
                            type="button"
                            onClick={() => onProjectTimezoneChange(tz)}
                            className="w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm text-left hover:bg-slate-100 dark:hover:bg-slate-800/50 text-slate-700 dark:text-slate-300 transition-colors">
                            <span>{tz}</span>
                            <span className="text-xs opacity-50">{getTimezoneOffsetLabel(tz)}</span>
                          </button>
                        ))}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ПРОВЕРКА ИНДЕКСАЦИИ */}
      {onToggleIndexCheck && (
        <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm overflow-hidden">
          <div className="p-6 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className={`p-2.5 rounded-xl ${indexCheckEnabled ? 'bg-emerald-50 dark:bg-emerald-500/10' : 'bg-slate-100 dark:bg-slate-800'}`}>
                <Activity className={`w-5 h-5 ${indexCheckEnabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-400'}`} />
              </div>
              <div>
                <h4 className="text-sm font-bold text-slate-900 dark:text-white">
                  Автоматическая проверка индексации
                </h4>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                  Ежедневная проверка индексации для всех доменов проекта
                </p>
              </div>
            </div>
            <button
              onClick={() => onToggleIndexCheck(!indexCheckEnabled)}
              disabled={indexCheckLoading}
              className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500/20 disabled:opacity-50 ${
                indexCheckEnabled ? 'bg-emerald-500' : 'bg-slate-300 dark:bg-slate-600'
              }`}>
              <span className={`inline-block h-5 w-5 rounded-full bg-white shadow-sm transform transition-transform ${
                indexCheckEnabled ? 'translate-x-6' : 'translate-x-1'
              }`} />
            </button>
          </div>
        </div>
      )}

      {/* ПРОМПТЫ */}
      <PromptOverridesPanel
        title="Переопределения промптов (проект)"
        endpoint={`/api/projects/${projectId}/prompts`}
        canEdit={canEditPrompts}
      />

      {/* УЧАСТНИКИ КОМАНДЫ */}
      <ProjectMembersSection
        members={members}
        loading={loading}
        newMemberEmail={newMemberEmail}
        newMemberRole={newMemberRole}
        currentUserEmail={currentUserEmail}
        onNewMemberEmailChange={onNewMemberEmailChange}
        onNewMemberRoleChange={onNewMemberRoleChange}
        onAddMember={onAddMember}
        onUpdateMemberRole={onUpdateMemberRole}
        onRemoveMember={onRemoveMember}
        formatDateTime={formatDateTime}
      />
    </div>
  );
}
