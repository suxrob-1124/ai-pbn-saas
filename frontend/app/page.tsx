import Link from "next/link";

export default function Home() {
  return (
    <div className="space-y-8">
      <section className="rounded-2xl border border-slate-200 bg-white/80 p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900/70">
        <div className="space-y-4">
          <div className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-semibold text-slate-700 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-200">
            SiteGen AI · beta
          </div>
          <h2 className="text-3xl font-semibold leading-tight">
            Генерация и управление сайтами с ИИ — в одном месте
          </h2>
          <p className="text-sm text-slate-600 dark:text-slate-300">
            Планируйте генерации, управляйте ссылками, отслеживайте ошибки и
            публикуйте сайты по расписанию. Всё с прозрачными логами и контролем качества.
          </p>
          <div className="flex flex-wrap gap-3">
            <Link
              href="/login"
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
            >
              Войти
            </Link>
            <Link
              href="/docs"
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            >
              Документация
            </Link>
            <Link
              href="/docs/api"
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            >
              API
            </Link>
          </div>
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-3">
        {[
          {
            title: "Расписания",
            text: "Запускайте генерации и вставку ссылок по времени и лимитам."
          },
          {
            title: "Контроль качества",
            text: "Смотрите логи шагов и диффы для вставок ссылок."
          },
          {
            title: "Очереди и ошибки",
            text: "Следите за очередью, статусами и быстрыми повторами."
          }
        ].map((card) => (
          <div
            key={card.title}
            className="rounded-2xl border border-slate-200 bg-white/80 p-5 text-sm text-slate-700 shadow-sm dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200"
          >
            <h3 className="text-base font-semibold text-slate-900 dark:text-slate-100">
              {card.title}
            </h3>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">{card.text}</p>
          </div>
        ))}
      </section>
    </div>
  );
}
