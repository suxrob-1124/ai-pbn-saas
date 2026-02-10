export const metadata = {
  title: "Ссылки",
  description: "Правила вставки и обновления ссылок.",
};

export default function DocsLinksPage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">Ссылки</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Вставка ссылок происходит по анкору и акцептору, заданным в настройках домена.
        </p>
      </header>

      <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
        <p>
          Ссылка добавляется только в body. Заголовки и title исключаются из поиска.
          Если ссылка уже есть, система пытается заменить её по данным предыдущей задачи.
        </p>
        <p>
          В «Логах ссылок» можно увидеть статус и дифф вставки. Отдельная задача
          создаётся на каждый домен.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Статусы задач</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>pending — ожидает выполнения</li>
          <li>searching — поиск места вставки</li>
          <li>inserted — ссылка вставлена</li>
          <li>generated — вставлен сгенерированный контент</li>
          <li>failed — ошибка обработки</li>
        </ul>
      </section>
    </div>
  );
}
