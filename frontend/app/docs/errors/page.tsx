export const metadata = {
  title: "Ошибки",
  description: "Как читать ошибки и восстанавливать задачи.",
};

export default function DocsErrorsPage() {
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold">Ошибки и повторы</h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Ошибки генерации и ссылок отображаются в логах задач и на странице проекта.
        </p>
      </header>

      <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
        <p>
          В случае сетевых ошибок (например, SERP timeout) система повторяет попытки
          автоматически. Максимум повторов настраивается в бэкенде.
        </p>
        <p>
          После устранения причины ошибки задачу можно перезапустить вручную через
          карточку домена или лог ссылки.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Где смотреть</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Страница проекта → вкладка «Ошибки»</li>
          <li>Карточка домена → «Логи ссылок»</li>
          <li>Swagger API → endpoints генераций и link tasks</li>
        </ul>
      </section>
    </div>
  );
}
