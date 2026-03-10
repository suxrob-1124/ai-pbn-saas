"use client";

const SUGGESTED_PROMPTS = [
  "Прочитай index.html и кратко опиши структуру сайта",
  "Добавь SEO мета-теги (title, description, og:*) в index.html",
  "Создай страницу about.html в стиле главной страницы",
  "Оптимизируй CSS: убери дублирование, улучши читаемость",
  "Добавь кнопку «Наверх» с плавной прокруткой",
  "Проверь все ссылки на изображения и исправь битые пути",
];

type Props = {
  onSelect: (prompt: string) => void;
};

export function AgentSuggestedPrompts({ onSelect }: Props) {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-center text-sm text-slate-400 dark:text-slate-500">
        Напишите задачу или выберите подсказку:
      </p>
      <div className="grid grid-cols-1 gap-2">
        {SUGGESTED_PROMPTS.map((prompt) => (
          <button
            key={prompt}
            type="button"
            onClick={() => onSelect(prompt)}
            className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-left text-xs text-slate-600 transition-colors hover:border-indigo-300 hover:bg-indigo-50 hover:text-indigo-700 dark:border-slate-700 dark:bg-slate-800/50 dark:text-slate-300 dark:hover:border-indigo-700 dark:hover:bg-indigo-950/20 dark:hover:text-indigo-300"
          >
            {prompt}
          </button>
        ))}
      </div>
    </div>
  );
}
