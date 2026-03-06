import { Calendar } from 'lucide-react';

type FilterDateInputProps = {
  label?: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
  disabled?: boolean;
};

export function FilterDateInput({
  label,
  value,
  placeholder,
  onChange,
  disabled,
}: FilterDateInputProps) {
  return (
    <div className="relative inline-block w-full sm:w-auto">
      <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-slate-400">
        <Calendar className="w-4 h-4" />
      </div>
      <input
        type="date"
        className="w-full pl-9 pr-3 py-2 bg-white border border-slate-200 rounded-xl text-sm font-medium text-slate-700 hover:bg-slate-50 focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-200 dark:hover:bg-[#0a1020] transition-all disabled:opacity-50"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        aria-label={label}
        title={placeholder || label}
      />
    </div>
  );
}
