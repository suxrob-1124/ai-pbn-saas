"use client";

type LinkTaskFormValue = {
  anchorText: string;
  targetUrl: string;
  scheduledFor: string;
};

type LinkTaskFormProps = {
  value: LinkTaskFormValue;
  loading: boolean;
  error?: string | null;
  onChange: (value: LinkTaskFormValue) => void;
  onSubmit: () => void;
  onSubmitAndAddAnother: () => void;
};

const isValidUrl = (value: string) => {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
};

export function LinkTaskForm({
  value,
  loading,
  error,
  onChange,
  onSubmit,
  onSubmitAndAddAnother
}: LinkTaskFormProps) {
  const anchorOk = value.anchorText.trim().length > 0;
  const targetOk = value.targetUrl.trim().length > 0;
  const urlOk = !value.targetUrl.trim() || isValidUrl(value.targetUrl.trim());
  const formOk = anchorOk && targetOk && urlOk;

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <h3 className="font-semibold">Новая задача</h3>
      {error && <div className="text-sm text-red-500">{error}</div>}
      {!urlOk && (
        <div className="text-xs text-amber-600 dark:text-amber-400">
          URL должен начинаться с http:// или https://
        </div>
      )}
      <div className="grid gap-3 md:grid-cols-2">
        <input
          className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
          placeholder="Анкор"
          value={value.anchorText}
          onChange={(e) => onChange({ ...value, anchorText: e.target.value })}
        />
        <input
          type="url"
          className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
          placeholder="https://example.com"
          value={value.targetUrl}
          onChange={(e) => onChange({ ...value, targetUrl: e.target.value })}
        />
        <input
          type="datetime-local"
          className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 md:col-span-2"
          value={value.scheduledFor}
          onChange={(e) => onChange({ ...value, scheduledFor: e.target.value })}
        />
      </div>
      <div className="flex flex-wrap gap-2">
        <button
          onClick={onSubmit}
          disabled={loading || !formOk}
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
        >
          Сохранить
        </button>
        <button
          onClick={onSubmitAndAddAnother}
          disabled={loading || !formOk}
          className="inline-flex items-center gap-2 rounded-lg border border-indigo-200 bg-white px-4 py-2 text-sm font-semibold text-indigo-700 hover:bg-indigo-50 disabled:opacity-50 dark:border-indigo-700 dark:bg-slate-900 dark:text-indigo-200"
        >
          Сохранить и добавить ещё
        </button>
      </div>
    </div>
  );
}
