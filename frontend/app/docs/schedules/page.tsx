export const metadata = {
  title: "Расписания",
  description: "Настройка расписаний генерации и ссылок.",
};

export default function DocsSchedulesPage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">Расписания</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          В проекте есть два типа расписаний: генерация сайтов и вставка ссылок.
        </p>
      </header>

      <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
        <p>
          Для каждого проекта поддерживается одно расписание генерации и одно расписание
          ссылок. Ручной запуск не блокирует автоматические запуски.
        </p>
        <p>
          Поле limit ограничивает количество доменов за один запуск. Время всегда
          интерпретируется в таймзоне проекта.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Примеры настроек</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Ежедневно, limit 2, время 09:30.</li>
          <li>Еженедельно, limit 10, день недели: суббота.</li>
        </ul>
      </section>
    </div>
  );
}
