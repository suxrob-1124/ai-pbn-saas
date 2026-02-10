export const metadata = {
  title: "Очередь",
  description: "Как работает очередь генерации.",
};

export default function DocsQueuePage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">Очередь</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Очередь показывает домены, запланированные к запуску.
        </p>
      </header>

      <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
        <p>
          Если подходящих доменов нет, очередь пустая. Время запуска отображается в
          таймзоне проекта.
        </p>
        <p>
          Очистка очереди доступна через кнопку «Очистить». Она удаляет устаревшие или
          неактуальные элементы.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Важные статусы</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>pending — ожидает постановки в обработку</li>
          <li>queued — уже поставлено в очередь воркера</li>
          <li>completed/failed — будут удалены при очистке</li>
        </ul>
      </section>
    </div>
  );
}
