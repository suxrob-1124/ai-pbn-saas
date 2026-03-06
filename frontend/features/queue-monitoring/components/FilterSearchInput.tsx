import { Search, X } from 'lucide-react';

type FilterSearchInputProps = {
  label?: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  list?: string; // <-- ДОБАВИЛИ ЭТО
};

export function FilterSearchInput({
  label,
  value,
  placeholder,
  onChange,
  disabled,
  list, // <-- ДОБАВИЛИ ЭТО
}: FilterSearchInputProps) {
  return (
    <div className="relative inline-block w-full sm:w-auto min-w-[200px]">
      <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-slate-400">
        <Search className="w-4 h-4" />
      </div>
      <input
        type="text"
        list={list} // <-- ДОБАВИЛИ ЭТО
        className="w-full pl-9 pr-8 py-2 bg-white border border-slate-200 rounded-xl text-sm font-medium text-slate-900 placeholder:text-slate-400 focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none dark:bg-[#060d18] dark:border-slate-700 dark:text-white transition-all disabled:opacity-50"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder || label}
        disabled={disabled}
        aria-label={label}
      />
      {value && !disabled && (
        <button
          onClick={() => onChange('')}
          className="absolute inset-y-0 right-0 flex items-center pr-2.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors">
          <X className="w-4 h-4" />
        </button>
      )}
    </div>
  );
}
