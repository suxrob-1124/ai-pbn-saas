import { ChevronDown } from 'lucide-react';

type FilterOption = {
  value: string;
  label: string;
};

type FilterSelectProps = {
  label?: string;
  value: string;
  options: FilterOption[];
  onChange: (value: string) => void;
  disabled?: boolean;
};

export function FilterSelect({ label, value, options, onChange, disabled }: FilterSelectProps) {
  return (
    <div className="relative inline-block w-full sm:w-auto min-w-[160px]">
      <select
        className="w-full appearance-none pl-3 pr-8 py-2 bg-white border border-slate-200 rounded-xl text-sm font-medium text-slate-700 hover:bg-slate-50 focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-200 dark:hover:bg-[#0a1020] transition-all cursor-pointer disabled:opacity-50"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        aria-label={label}
        title={label}>
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
      <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2.5 text-slate-400">
        <ChevronDown className="w-4 h-4" />
      </div>
    </div>
  );
}
