import { PromptOverridesPanel } from "../../../components/PromptOverridesPanel";
import { ProjectMembersSection } from "./ProjectMembersSection";

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
  onRemoveMember
}: ProjectSettingsSectionProps) {
  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h3 className="font-semibold">Настройки проекта</h3>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Основные параметры, таймзона и доступы команды.
          </p>
        </div>
        <button
          onClick={onSaveProjectSettings}
          disabled={projectSettingsLoading || !projectName.trim()}
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
        >
          {projectSettingsLoading ? "Сохраняем..." : "Сохранить настройки"}
        </button>
      </div>
      {projectSettingsError && (
        <div className="text-sm text-red-500">{projectSettingsError}</div>
      )}

      <div className="grid gap-4 lg:grid-cols-[1fr_1.2fr]">
        <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
          <div>
            <h4 className="font-semibold">Профиль проекта</h4>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Название и региональные параметры.
            </p>
          </div>
          <div className="grid gap-3">
            <div className="space-y-1">
              <div className="text-xs uppercase tracking-wide text-slate-400">Название</div>
              <input
                className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                value={projectName}
                onChange={(e) => onProjectNameChange(e.target.value)}
                placeholder="Название проекта"
              />
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <div className="text-xs uppercase tracking-wide text-slate-400">Страна</div>
                <input
                  className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  value={projectCountry}
                  onChange={(e) => onProjectCountryChange(e.target.value)}
                  placeholder="se"
                />
              </div>
              <div className="space-y-1">
                <div className="text-xs uppercase tracking-wide text-slate-400">Язык</div>
                <input
                  className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
                  value={projectLanguage}
                  onChange={(e) => onProjectLanguageChange(e.target.value)}
                  placeholder="sv"
                />
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
          <div>
            <h4 className="font-semibold">Таймзона проекта</h4>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Используется для расписаний и отображения времени.
            </p>
          </div>
          <div className="space-y-3">
            <input
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={timezoneQuery}
              onChange={(e) => onTimezoneQueryChange(e.target.value)}
              placeholder="Поиск таймзоны (например, Asia/Bangkok)"
            />
            <div className="rounded-lg border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-950 space-y-3">
              <div>
                <div className="text-[11px] uppercase tracking-wide text-slate-400">Выбранная зона</div>
                <div className="mt-1 flex items-center justify-between rounded-md border border-slate-200 px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:text-slate-100">
                  <span>{resolvedProjectTimezone || "UTC"}</span>
                  <span className="text-xs text-slate-500 dark:text-slate-400">
                    {getTimezoneOffsetLabel(resolvedProjectTimezone)}
                  </span>
                </div>
              </div>
              {recentFiltered.length > 0 && (
                <div>
                  <div className="text-[11px] uppercase tracking-wide text-slate-400">Недавние</div>
                  <div className="mt-1 flex flex-wrap gap-2">
                    {recentFiltered.map((tz) => (
                      <button
                        key={`recent-${tz}`}
                        type="button"
                        onClick={() => onUseRecentTimezone(tz)}
                        className="inline-flex items-center gap-2 rounded-full border border-slate-200 px-3 py-1 text-xs text-slate-700 hover:bg-slate-100 dark:border-slate-800 dark:text-slate-200 dark:hover:bg-slate-900"
                      >
                        <span>{tz}</span>
                        <span className="text-[10px] text-slate-500 dark:text-slate-400">
                          {getTimezoneOffsetLabel(tz)}
                        </span>
                      </button>
                    ))}
                  </div>
                </div>
              )}
              <div>
                <div className="text-[11px] uppercase tracking-wide text-slate-400">Все таймзоны</div>
                <div className="mt-2 max-h-64 space-y-3 overflow-auto pr-2">
                  {timezoneGroups.length === 0 && (
                    <div className="text-sm text-slate-500 dark:text-slate-400">Ничего не найдено</div>
                  )}
                  {timezoneGroups.map(([group, items]) => (
                    <div key={group} className="space-y-2">
                      <div className="text-xs font-semibold uppercase text-slate-500 dark:text-slate-400">
                        {group}
                      </div>
                      <div className="grid gap-2">
                        {items.map((tz) => (
                          <button
                            key={tz}
                            type="button"
                            onClick={() => onProjectTimezoneChange(tz)}
                            className={`flex items-center justify-between rounded-md border px-3 py-2 text-sm ${
                              tz === projectTimezone
                                ? "border-indigo-400 bg-indigo-50 text-indigo-700 dark:border-indigo-500/60 dark:bg-indigo-500/10 dark:text-indigo-200"
                                : "border-slate-200 text-slate-700 hover:bg-slate-100 dark:border-slate-800 dark:text-slate-200 dark:hover:bg-slate-900"
                            }`}
                          >
                            <span>{tz}</span>
                            <span className="text-xs text-slate-500 dark:text-slate-400">
                              {getTimezoneOffsetLabel(tz)}
                            </span>
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <PromptOverridesPanel
        title="Переопределения промптов (проект)"
        endpoint={`/api/projects/${projectId}/prompts`}
        canEdit={canEditPrompts}
      />

      <ProjectMembersSection
        members={members}
        loading={loading}
        newMemberEmail={newMemberEmail}
        newMemberRole={newMemberRole}
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
