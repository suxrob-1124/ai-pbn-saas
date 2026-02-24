import { FiX } from "react-icons/fi";

type ProjectMember = {
  email: string;
  role: string;
  createdAt: string;
};

type ProjectMembersSectionProps = {
  members: ProjectMember[];
  loading: boolean;
  newMemberEmail: string;
  newMemberRole: string;
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
  onNewMemberEmailChange,
  onNewMemberRoleChange,
  onAddMember,
  onUpdateMemberRole,
  onRemoveMember,
  formatDateTime
}: ProjectMembersSectionProps) {
  return (
    <div className="border-t border-slate-200 dark:border-slate-800 pt-4 space-y-4">
      <h3 className="font-semibold">Участники проекта</h3>
      <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <h4 className="font-semibold">Добавить участника</h4>
        <div className="grid gap-3 md:grid-cols-3">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="email@example.com"
            value={newMemberEmail}
            onChange={(e) => onNewMemberEmailChange(e.target.value)}
          />
          <select
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            value={newMemberRole}
            onChange={(e) => onNewMemberRoleChange(e.target.value)}
          >
            <option value="viewer">Наблюдатель</option>
            <option value="editor">Редактор</option>
            <option value="owner">Владелец</option>
          </select>
          <button
            onClick={onAddMember}
            disabled={loading || !newMemberEmail.trim()}
            className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
          >
            Добавить
          </button>
        </div>
      </div>

      <div className="bg-white/60 dark:bg-slate-900/40 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h4 className="font-semibold">Список участников</h4>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {members.length}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                <th className="py-2 pr-4">Почта</th>
                <th className="py-2 pr-4">Роль</th>
                <th className="py-2 pr-4">Добавлен</th>
                <th className="py-2 pr-4 text-right">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
              {members.map((member) => (
                <tr key={member.email}>
                  <td className="py-3 pr-4 font-medium">{member.email}</td>
                  <td className="py-3 pr-4">
                    {member.role === "owner" ? (
                      <span className="text-sm text-slate-600 dark:text-slate-400">Владелец</span>
                    ) : (
                      <select
                        className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-sm text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
                        value={member.role}
                        onChange={(e) => onUpdateMemberRole(member.email, e.target.value)}
                      >
                        <option value="viewer">Наблюдатель</option>
                        <option value="editor">Редактор</option>
                        <option value="owner">Владелец</option>
                      </select>
                    )}
                  </td>
                  <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">{formatDateTime(member.createdAt)}</td>
                  <td className="py-3 pr-4 text-right">
                    {member.role !== "owner" && (
                      <button
                        onClick={() => onRemoveMember(member.email)}
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
  );
}

