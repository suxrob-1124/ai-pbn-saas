type TableStateProps = {
  loading: boolean;
  error?: string | null;
  empty: boolean;
  loadingText?: string;
  emptyText?: string;
  className?: string;
};

export function TableState({
  loading,
  error,
  empty,
  loadingText = "Загрузка...",
  emptyText = "Нет данных",
  className = "text-sm"
}: TableStateProps) {
  if (loading) {
    return <div className={`${className} text-slate-500 dark:text-slate-400`}>{loadingText}</div>;
  }
  if (error) {
    return <div className={`${className} text-red-500`}>{error}</div>;
  }
  if (empty) {
    return <div className={`${className} text-slate-500 dark:text-slate-400`}>{emptyText}</div>;
  }
  return null;
}

type TableStateRowProps = TableStateProps & {
  colSpan: number;
};

export function TableStateRow({
  colSpan,
  loading,
  error,
  empty,
  loadingText,
  emptyText
}: TableStateRowProps) {
  const text = loading ? loadingText || "Загрузка..." : error || (empty ? emptyText || "Нет данных" : "");
  if (!text) {
    return null;
  }
  const tone = error ? "text-red-500" : "text-slate-500 dark:text-slate-400";
  return (
    <tr>
      <td colSpan={colSpan} className={`py-6 text-center ${tone}`}>
        {text}
      </td>
    </tr>
  );
}
