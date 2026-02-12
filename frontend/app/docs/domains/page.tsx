export const metadata = {
  title: "Домены",
  description: "Добавление доменов и настройка параметров домена.",
};

export default function DocsDomainsPage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">Домены</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Домены — это единицы генерации сайтов внутри проекта.
        </p>
      </header>

      <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
        <p>
          Для каждого домена укажите ключевое слово. Оно используется при SERP‑анализе и
          генерации контента.
        </p>
        <p>
          В настройках домена находятся поля для ссылок: анкор и акцептор. Это позволяет
          запускать задачи вставки ссылок без отдельной вкладки.
        </p>
        <p>
          В карточке домена доступна ссылка на мониторинг индексации, чтобы быстро
          проверить статус проверок по конкретному домену.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Полезно знать</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Ссылки вставляются только в body и не затрагивают заголовки.</li>
          <li>Если домен перегенерирован, ссылка будет добавлена повторно.</li>
        </ul>
      </section>
    </div>
  );
}
