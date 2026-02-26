type FilterSearchInputProps = {
  label?: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
  disabled?: boolean;
};

export function FilterSearchInput({
  label = "Поиск",
  value,
  placeholder,
  onChange,
  disabled
}: FilterSearchInputProps) {
  return (
    <label className="text-sm space-y-1">
      <span className="text-slate-600 dark:text-slate-300">{label}</span>
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 disabled:opacity-60 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
      />
    </label>
  );
}
