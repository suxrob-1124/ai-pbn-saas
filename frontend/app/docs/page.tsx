export const metadata = {
  title: "Документация",
  description: "Руководство по работе с SiteGen AI.",
};

export default function DocsPage() {
  return (
    <div className="space-y-8">
      <header>
        <p className="text-xs uppercase tracking-[0.25em] text-slate-400">SiteGen AI</p>
        <h1 className="mt-3 text-3xl font-bold">Документация продукта</h1>
        <p className="mt-2 text-slate-600 dark:text-slate-300">
          Здесь собраны основные сценарии: как запускать генерацию, управлять доменами,
          расписаниями и ссылками.
        </p>
      </header>

      <section className="grid gap-4 md:grid-cols-2">
        {[
          {
            title: "Проекты",
            text: "Создавайте проекты, настраивайте страну, язык и таймзону. Таймзона влияет на расписания.",
          },
          {
            title: "Домены",
            text: "Добавляйте домены, задавайте ключевое слово, анкор и акцептор для ссылок.",
          },
          {
            title: "Расписания",
            text: "Планируйте генерации и вставку ссылок. Ручные запуски не блокируют авто‑расписание.",
          },
          {
            title: "Очередь",
            text: "В очереди отображаются только домены, которые реально ожидают запуск.",
          },
        ].map((card) => (
          <div
            key={card.title}
            className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60"
          >
            <h2 className="text-base font-semibold">{card.title}</h2>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">{card.text}</p>
          </div>
        ))}
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-6 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Быстрый старт</h2>
        <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Создайте проект и задайте страну, язык и таймзону.</li>
          <li>Добавьте домены и ключевые слова.</li>
          <li>Заполните анкор и акцептор для ссылок.</li>
          <li>Настройте расписания генерации и ссылок.</li>
          <li>Запустите генерацию и следите за очередью и логами.</li>
        </ol>
        <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-200">
          <p className="text-xs uppercase tracking-[0.2em] text-slate-400">Пример</p>
          <p className="mt-2">
            Проект: <strong>surstrem</strong> → Домены: <strong>kundservice.net</strong>,
            <strong> elinloe.se</strong> → Анкор: <strong>casino utan svensk licens</strong> →
            Акцептор: <strong>https://example.com</strong>.
          </p>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-6 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Коротко о правилах</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>API‑ключ используется владельца проекта.</li>
          <li>Ссылки вставляются только в body, заголовки исключаются.</li>
          <li>Ошибки доступны в карточках задач и логах.</li>
        </ul>
      </section>
    </div>
  );
}
