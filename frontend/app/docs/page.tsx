import Link from "next/link";
import type { Route } from "next";

export const metadata = {
  title: "Документация",
  description: "Руководство по работе с SiteGen AI.",
};

export default function DocsPage() {
  const cards = [
    {
      href: "/docs/projects",
      title: "Проекты",
      text: "Создание проекта, роль владельца, таймзона и базовые параметры запуска.",
    },
    {
      href: "/docs/domains",
      title: "Домены",
      text: "Подготовка доменов к генерации, ключевые слова и рабочий E2E-поток.",
    },
    {
      href: "/docs/editor-ai-studio",
      title: "Editor и AI Studio",
      text: "Правки файлов, контекст AI, создание страниц, apply-план и работа с ассетами.",
    },
    {
      href: "/docs/schedules",
      title: "Расписания",
      text: "Автоматические и ручные запуски генераций и задач по ссылкам.",
    },
    {
      href: "/docs/queue",
      title: "Очередь",
      text: "Контроль in-flight задач, статусы и разбор зависаний очереди.",
    },
    {
      href: "/docs/links",
      title: "Ссылки",
      text: "Link tasks: статусы, retry/delete guard-правила и рабочая диагностика.",
    },
    {
      href: "/docs/errors",
      title: "Ошибки",
      text: "Где искать причину сбоя и какие данные собрать до эскалации.",
    },
    {
      href: "/docs/indexing",
      title: "Индексация",
      text: "Проверки индексации, ретраи, история попыток и агрегаты.",
    },
    {
      href: "/docs/troubleshooting",
      title: "Диагностика и восстановление",
      text: "Карта восстановления: симптом -> причина -> действие.",
    },
  ];

  return (
    <div className="space-y-8">
      <header>
        <p className="text-xs uppercase tracking-[0.25em] text-slate-400">SiteGen AI</p>
        <h1 className="mt-3 text-3xl font-bold">Документация продукта</h1>
        <p className="mt-2 text-slate-600 dark:text-slate-300">
          Здесь собраны рабочие сценарии: от запуска проекта и генерации страниц до
          диагностики ошибок и восстановления после сбоев.
        </p>
      </header>

      <section className="grid gap-4 md:grid-cols-2">
        {cards.map((card) => (
          <Link
            key={card.title}
            href={card.href as Route}
            className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60"
          >
            <h2 className="text-base font-semibold text-slate-900 dark:text-slate-100">{card.title}</h2>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">{card.text}</p>
          </Link>
        ))}
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-6 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Быстрый старт</h2>
        <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Создайте проект и проверьте API-ключ владельца.</li>
          <li>Добавьте домены и выполните первый запуск генерации.</li>
          <li>Откройте Editor и проверьте файлы перед применением изменений.</li>
          <li>При необходимости используйте AI Studio для правки файла или создания страницы.</li>
          <li>Проверьте apply-план, ассеты и обновление sitemap/navigation.</li>
          <li>Настройте расписания и контролируйте очередь.</li>
          <li>Отслеживайте индексацию и используйте troubleshooting при сбоях.</li>
        </ol>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-6 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Коротко о правилах</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Перед массовым apply всегда проверяйте overwrite-файлы и unresolved assets.</li>
          <li>Для режима manual в AI-контексте обязательно выбирайте файлы контекста.</li>
          <li>Для изображений используйте только image-capable модель.</li>
          <li>Sitemap и внутренние ссылки после создания страницы проверяйте вручную.</li>
        </ul>
      </section>
    </div>
  );
}
