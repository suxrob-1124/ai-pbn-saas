import Link from 'next/link';
import type { Route } from 'next';
import {
  FolderGit2,
  Globe,
  Bot,
  BrainCircuit,
  Clock,
  ListChecks,
  Link as LinkIcon,
  ShieldAlert,
  Activity,
  LifeBuoy,
  FolderInput,
  FileCode,
  Zap,
  Info,
  ArrowRight,
} from 'lucide-react';

export const metadata = {
  title: 'Документация',
  description: 'Руководство по работе с SiteGen AI.',
};

export default function DocsPage() {
  const cards = [
    {
      href: '/docs/projects',
      title: 'Проекты',
      text: 'Создание проекта, роль владельца, таймзона и базовые параметры запуска.',
      icon: <FolderGit2 className="w-5 h-5 text-indigo-500" />,
    },
    {
      href: '/docs/domains',
      title: 'Домены',
      text: 'Подготовка доменов к генерации, ключевые слова и рабочий E2E-поток.',
      icon: <Globe className="w-5 h-5 text-emerald-500" />,
    },
    {
      href: '/docs/editor-ai-studio',
      title: 'Editor и AI Studio',
      text: 'Правки файлов, контекст AI, создание страниц, apply-план и работа с ассетами.',
      icon: <Bot className="w-5 h-5 text-purple-500" />,
    },
    {
      href: '/docs/ai-agent',
      title: 'AI Агент',
      text: 'Автономный агент для правки файлов: сессии, SSE-стриминг, откат и история.',
      icon: <BrainCircuit className="w-5 h-5 text-violet-500" />,
    },
    {
      href: '/docs/schedules',
      title: 'Расписания',
      text: 'Автоматические и ручные запуски генераций и задач по ссылкам.',
      icon: <Clock className="w-5 h-5 text-amber-500" />,
    },
    {
      href: '/docs/queue',
      title: 'Очередь',
      text: 'Контроль in-flight задач, статусы и разбор зависаний очереди.',
      icon: <ListChecks className="w-5 h-5 text-blue-500" />,
    },
    {
      href: '/docs/links',
      title: 'Ссылки',
      text: 'Link tasks: статусы, retry/delete guard-правила и рабочая диагностика.',
      icon: <LinkIcon className="w-5 h-5 text-pink-500" />,
    },
    {
      href: '/docs/errors',
      title: 'Ошибки',
      text: 'Где искать причину сбоя и какие данные собрать до эскалации.',
      icon: <ShieldAlert className="w-5 h-5 text-red-500" />,
    },
    {
      href: '/docs/indexing',
      title: 'Индексация',
      text: 'Проверки индексации, ретраи, история попыток и агрегаты.',
      icon: <Activity className="w-5 h-5 text-cyan-500" />,
    },
    {
      href: '/docs/legacy-import',
      title: 'Legacy-импорт',
      text: 'Импорт данных с существующих серверов: превью, запуск и мониторинг.',
      icon: <FolderInput className="w-5 h-5 text-teal-500" />,
    },
    {
      href: '/docs/troubleshooting',
      title: 'Диагностика и восстановление',
      text: 'Карта восстановления: симптом -> причина -> действие.',
      icon: <LifeBuoy className="w-5 h-5 text-orange-500" />,
    },
    {
      href: '/docs/api',
      title: 'API (Swagger)',
      text: 'OpenAPI-спецификация и интерактивная документация.',
      icon: <FileCode className="w-5 h-5 text-slate-500" />,
    },
  ];

  return (
    <div className="space-y-12">
      {/* HEADER */}
      <header className="border-b border-slate-100 dark:border-slate-800/60 pb-8">
        <a href='/' className="text-[11px] font-bold uppercase tracking-widest text-indigo-600 dark:text-indigo-400 mb-3">
          SiteGen AI Platform
        </a>
        <h1 className="text-3xl md:text-4xl font-extrabold tracking-tight text-slate-900 dark:text-white">
          Документация продукта
        </h1>
        <p className="mt-4 text-base text-slate-600 dark:text-slate-400 max-w-3xl leading-relaxed">
          Здесь собраны рабочие сценарии: от запуска проекта и массовой генерации страниц до
          диагностики ошибок и восстановления после сбоев. Выберите нужный раздел для старта.
        </p>
      </header>

      {/* КАРТОЧКИ РАЗДЕЛОВ */}
      <section className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {cards.map((card) => (
          <Link
            key={card.title}
            href={card.href as Route}
            className="group flex flex-col rounded-2xl border border-slate-200 bg-slate-50/50 p-6 transition-all hover:bg-white hover:shadow-lg hover:shadow-indigo-500/5 hover:border-indigo-300 dark:border-slate-800 dark:bg-[#0a1020] dark:hover:bg-[#0f1523] dark:hover:border-indigo-500/50">
            <div className="w-10 h-10 rounded-xl bg-white border border-slate-100 dark:bg-[#1a2235] dark:border-slate-700 flex items-center justify-center mb-4 shadow-sm group-hover:scale-110 transition-transform">
              {card.icon}
            </div>
            <h2 className="text-lg font-bold text-slate-900 dark:text-white mb-2 flex items-center justify-between">
              {card.title}
              <ArrowRight className="w-4 h-4 text-slate-300 dark:text-slate-600 group-hover:text-indigo-500 transition-colors opacity-0 group-hover:opacity-100 -translate-x-2 group-hover:translate-x-0" />
            </h2>
            <p className="text-sm text-slate-500 dark:text-slate-400 leading-relaxed flex-1">
              {card.text}
            </p>
          </Link>
        ))}
      </section>

      <div className="grid md:grid-cols-2 gap-8">
        {/* БЫСТРЫЙ СТАРТ */}
        <section className="rounded-2xl border border-indigo-100 bg-indigo-50/50 p-8 dark:border-indigo-900/30 dark:bg-indigo-900/10 relative overflow-hidden">
          <div className="absolute top-0 right-0 p-6 opacity-10">
            <Zap className="w-32 h-32 text-indigo-500" />
          </div>
          <div className="relative z-10">
            <h2 className="text-xl font-bold text-indigo-900 dark:text-indigo-300 flex items-center gap-2 mb-6">
              <Zap className="w-5 h-5 text-indigo-500" /> Быстрый старт
            </h2>
            <ol className="list-decimal list-outside ml-4 space-y-3 text-sm text-indigo-800/80 dark:text-indigo-200/70 marker:font-bold marker:text-indigo-400">
              <li>
                <strong className="text-indigo-900 dark:text-indigo-200">Создайте проект</strong> и
                проверьте API-ключ владельца.
              </li>
              <li>Добавьте домены и выполните первый запуск генерации.</li>
              <li>Откройте Editor и проверьте файлы перед применением изменений.</li>
              <li>
                При необходимости используйте{' '}
                <strong className="text-indigo-900 dark:text-indigo-200">AI Studio</strong> для
                правки файла или создания страницы.
              </li>
              <li>Проверьте apply-план, ассеты и обновление sitemap/navigation.</li>
              <li>Настройте расписания и контролируйте очередь.</li>
              <li>Отслеживайте индексацию и используйте troubleshooting при сбоях.</li>
            </ol>
          </div>
        </section>

        {/* ПРАВИЛА */}
        <section className="rounded-2xl border border-amber-100 bg-amber-50/50 p-8 dark:border-amber-900/30 dark:bg-amber-900/10 relative overflow-hidden">
          <div className="absolute top-0 right-0 p-6 opacity-10">
            <Info className="w-32 h-32 text-amber-500" />
          </div>
          <div className="relative z-10">
            <h2 className="text-xl font-bold text-amber-900 dark:text-amber-300 flex items-center gap-2 mb-6">
              <Info className="w-5 h-5 text-amber-500" /> Коротко о правилах
            </h2>
            <ul className="space-y-4 text-sm text-amber-800/80 dark:text-amber-200/70">
              <li className="flex gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-amber-400 mt-1.5 flex-shrink-0" />
                <span>
                  Перед массовым apply <strong>всегда проверяйте</strong> overwrite-файлы и
                  unresolved assets.
                </span>
              </li>
              <li className="flex gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-amber-400 mt-1.5 flex-shrink-0" />
                <span>Для режима manual в AI-контексте обязательно выбирайте файлы контекста.</span>
              </li>
              <li className="flex gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-amber-400 mt-1.5 flex-shrink-0" />
                <span>
                  Для изображений используйте <strong>только image-capable</strong> модель
                  (flash-image).
                </span>
              </li>
              <li className="flex gap-3">
                <div className="w-1.5 h-1.5 rounded-full bg-amber-400 mt-1.5 flex-shrink-0" />
                <span>Sitemap и внутренние ссылки после создания страницы проверяйте вручную.</span>
              </li>
            </ul>
          </div>
        </section>
      </div>
    </div>
  );
}
