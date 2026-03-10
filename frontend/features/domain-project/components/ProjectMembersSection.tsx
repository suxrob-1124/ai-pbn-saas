import { X, UserPlus, User, Star } from 'lucide-react';

type ProjectMember = { email: string; role: string; createdAt: string };

type ProjectMembersSectionProps = {
  members: ProjectMember[];
  loading: boolean;
  newMemberEmail: string;
  newMemberRole: string;
  currentUserEmail?: string;
  onNewMemberEmailChange: (value: string) => void;
  onNewMemberRoleChange: (value: string) => void;
  onAddMember: () => void;
  onUpdateMemberRole: (email: string, role: string) => void;
  onRemoveMember: (email: string) => void;
  formatDateTime: (value?: string) => string;
};

export function ProjectMembersSection({
  members,
  loading,
  newMemberEmail,
  newMemberRole,
  currentUserEmail,
  onNewMemberEmailChange,
  onNewMemberRoleChange,
  onAddMember,
  onUpdateMemberRole,
  onRemoveMember,
  formatDateTime,
}: ProjectMembersSectionProps) {
  const getRoleIcon = (role: string) => {
    if (role === 'owner') return <Star className="w-4 h-4 text-amber-500" />;
    if (role === 'editor') return <User className="w-4 h-4 text-blue-500" />;
    return <User className="w-4 h-4 text-slate-500" />;
  };

  return (
    <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm overflow-hidden animate-in fade-in">
      {/* HEADER */}
      <div className="p-6 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-[#0a1020]">
        <h3 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
          <UserPlus className="w-5 h-5 text-indigo-500" /> Команда проекта
        </h3>
        <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
          Управление доступом и ролями участников.
        </p>
      </div>

      <div className="p-6 space-y-8">
        {/* Форма добавления (В одну строку) */}
        <div>
          <label className="block text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-2">
            Пригласить участника
          </label>
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
            <input
              className="flex-1 bg-white dark:bg-[#060d18] border border-slate-300 dark:border-slate-700 px-4 py-2.5 text-sm rounded-xl outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 dark:text-white placeholder:text-slate-400 transition-all shadow-sm"
              placeholder="email@example.com"
              value={newMemberEmail}
              onChange={(e) => onNewMemberEmailChange(e.target.value)}
            />
            <select
              className="sm:w-48 bg-white dark:bg-[#060d18] border border-slate-300 dark:border-slate-700 px-4 py-2.5 text-sm rounded-xl outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 dark:text-white transition-all shadow-sm"
              value={newMemberRole}
              onChange={(e) => onNewMemberRoleChange(e.target.value)}>
              <option value="owner">Владелец</option>
              <option value="viewer">Наблюдатель</option>
              <option value="editor">Редактор (Копирайтер)</option>
            </select>
            <button
              onClick={onAddMember}
              disabled={loading || !newMemberEmail.trim()}
              className="px-6 py-2.5 text-sm font-semibold text-white bg-slate-900 dark:bg-white dark:text-slate-900 rounded-xl hover:bg-slate-800 dark:hover:bg-slate-200 disabled:opacity-50 transition-all shadow-sm">
              Пригласить
            </button>
          </div>
        </div>

        {/* Список команды */}
        <div>
          <label className="block text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-3 flex justify-between items-center">
            <span>Список участников</span>
            <span className="bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded text-slate-500 normal-case">
              {members.length} чел.
            </span>
          </label>

          <div className="border border-slate-200 dark:border-slate-700 rounded-xl overflow-hidden divide-y divide-slate-100 dark:divide-slate-800/60 bg-white dark:bg-[#060d18]">
            {members.map((member) => (
              <div
                key={member.email}
                className="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4 hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                <div className="flex items-center gap-3 min-w-0">
                  <div className="w-10 h-10 rounded-full bg-slate-100 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 flex items-center justify-center flex-shrink-0">
                    {getRoleIcon(member.role)}
                  </div>
                  <div className="min-w-0">
                    <p className="font-semibold text-slate-900 dark:text-white text-sm truncate">
                      {member.email}
                    </p>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Добавлен: {formatDateTime(member.createdAt)}
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-3">
                  <div className="flex items-center gap-2">
                    {currentUserEmail && member.email === currentUserEmail ? (
                      <span className="px-3 py-1.5 rounded-lg text-xs font-medium border border-slate-200 dark:border-slate-700 text-slate-400 dark:text-slate-500 bg-slate-50 dark:bg-slate-800/50 select-none" title="Нельзя изменить свою роль">
                        {member.role === 'owner' ? 'Владелец' : member.role === 'editor' ? 'Редактор' : 'Наблюдатель'}
                      </span>
                    ) : (
                      <select
                        className="bg-transparent border border-slate-200 dark:border-slate-700 px-3 py-1.5 rounded-lg text-xs font-medium outline-none focus:border-indigo-500 dark:text-slate-200"
                        value={member.role}
                        onChange={(e) => onUpdateMemberRole(member.email, e.target.value)}>
                        <option value="owner">Владелец</option>
                        <option value="viewer">Наблюдатель</option>
                        <option value="editor">Редактор</option>
                      </select>
                    )}
                    <button
                      onClick={() => onRemoveMember(member.email)}
                      disabled={loading || (!!currentUserEmail && member.email === currentUserEmail)}
                      className="p-1.5 text-slate-400 hover:text-red-500 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors disabled:opacity-30 disabled:cursor-not-allowed">
                      <X className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))}
            {members.length === 0 && (
              <div className="p-6 text-center text-sm text-slate-500">
                В проекте пока нет приглашенных участников.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
