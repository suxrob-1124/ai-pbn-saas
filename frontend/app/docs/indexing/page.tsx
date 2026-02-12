export const metadata = {
  title: "Мониторинг индексации",
  description: "Мониторинг индексации доменов и аналитика по проверкам.",
};

export default function DocsIndexingPage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">Мониторинг · Индексация</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Отслеживание индексации доменов: ежедневные проверки, история попыток,
          статистика и календарь.
        </p>
      </header>

      <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
        <p>
          Для каждого домена создаётся ежедневная проверка. Результат фиксируется как
          <strong> в индексе </strong> или <strong> не в индексе </strong>, а ошибки
          приводят к ретраям по расписанию.
        </p>
        <p>
          Страница мониторинга доступна по адресу{" "}
          <code className="rounded bg-slate-100 px-1 dark:bg-slate-800">/monitoring/indexing</code>.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Статусы</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li><strong>pending</strong> — создана, ожидает запуска.</li>
          <li><strong>checking</strong> — идёт проверка или запланирован повтор.</li>
          <li><strong>success</strong> — получен финальный ответ (в индексе/не в индексе).</li>
          <li><strong>failed_investigation</strong> — превышен лимит попыток.</li>
        </ul>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Фильтры и сортировка</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Статусы (multi‑select).</li>
          <li>Период <code>from/to</code>.</li>
          <li>Фильтр <code>isIndexed</code> (true/false).</li>
          <li>Поиск по <code>domain_id</code> или URL.</li>
          <li>Сортировка по дате, домену, статусу и другим колонкам.</li>
        </ul>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Ручной запуск</h2>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">
          Ручной запуск доступен на уровне домена, проекта и для администратора
          в глобальном списке. После запуска статус сбрасывается в <code>pending</code>
          и выполняется проверка по расписанию.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Админ‑возможности</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Глобальный список всех проверок по всем доменам.</li>
          <li>Фильтр по домену и поиск по URL/ID.</li>
          <li>Запуск проверки вручную для любого домена.</li>
          <li>Отдельный список проблемных проверок (failed_investigation).</li>
        </ul>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">
          Раздел доступен только пользователям с ролью администратора.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Статистика и календарь</h2>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">
          Отдельные агрегаты дают корректные метрики независимо от размера списка:
          процент индексации, среднее число попыток до успеха,{" "}
          <code>failed_investigation</code> за неделю и динамику по дням.
        </p>
      </section>
    </div>
  );
}
