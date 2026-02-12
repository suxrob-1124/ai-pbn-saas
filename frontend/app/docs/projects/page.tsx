export const metadata = {
  title: "Проекты",
  description: "Настройка проектов и общих параметров в SiteGen AI.",
};

export default function DocsProjectsPage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">Проекты</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Проект объединяет домены, расписания и общие параметры (страна, язык, таймзона).
        </p>
      </header>

      <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
        <p>
          При создании проекта укажите название, целевую страну и язык. Таймзона проекта
          используется для отображения времени и расчёта следующих запусков.
        </p>
        <p>
          API‑ключ берётся у владельца проекта. Если у владельца нет ключа, генерация
          не будет запущена.
        </p>
        <p>
          В проекте доступен вход в мониторинг индексации по проекту — это удобно,
          когда нужно оценить статус доменов сразу по всей группе.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Рекомендации</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Выбирайте таймзону проекта сразу и используйте её в расписаниях.</li>
          <li>Держите название проекта коротким — оно отображается в списках.</li>
        </ul>
      </section>
    </div>
  );
}
