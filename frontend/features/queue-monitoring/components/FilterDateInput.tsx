type FilterDateInputProps = {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
};

export function FilterDateInput({ label, value, onChange, disabled }: FilterDateInputProps) {
  return (
    <label className="text-sm space-y-1">
      <span className="text-slate-600 dark:text-slate-300">{label}</span>
      <input
        type="date"
        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
      />
    </label>
  );
}
