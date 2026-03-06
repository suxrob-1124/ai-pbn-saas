import Link from 'next/link';
import { ArrowRight, Bot, Clock, Globe, Zap, ShieldCheck, Database, LayoutDashboard } from 'lucide-react';

export default function Home() {
  return (
    <div className="flex flex-col items-center w-full">
      {/* HERO SECTION */}
      <section className="w-full py-20 md:py-32 flex flex-col items-center text-center space-y-8">
        <div className="inline-flex items-center gap-2 rounded-full border border-indigo-200 bg-indigo-50 px-4 py-1.5 text-sm font-medium text-indigo-600 dark:border-indigo-800 dark:bg-indigo-950/50 dark:text-indigo-300">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
          </span>
          SiteGen AI · Beta Access
        </div>

        <h1 className="text-5xl md:text-7xl font-extrabold tracking-tight text-slate-900 dark:text-white max-w-4xl">
          Массовая генерация сайтов с{' '}
          <span className="text-transparent bg-clip-text bg-gradient-to-r from-indigo-500 to-purple-600">
            силой AI
          </span>
        </h1>

        <p className="text-lg md:text-xl text-slate-600 dark:text-slate-400 max-w-2xl">
          Единая платформа для управления PBN-сетями. Планируйте генерации, управляйте ссылками,
          отслеживайте индексацию и редактируйте контент в одном окне.
        </p>

        <div className="flex flex-col sm:flex-row items-center gap-4 pt-4">
          <Link
            href="/login"
            className="group inline-flex items-center gap-2 rounded-full bg-indigo-600 px-8 py-3.5 text-base font-semibold text-white hover:bg-indigo-500 transition-all shadow-lg hover:shadow-indigo-500/25">
            Начать работу
            <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
          </Link>
          <Link
            href="/docs"
            className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-8 py-3.5 text-base font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800 transition-all">
            Документация
          </Link>
        </div>
      </section>

      {/* АБСТРАКТНЫЙ MOCKUP ИНТЕРФЕЙСА (Добавляет "дороговизны") */}
      <div className="w-full max-w-5xl rounded-2xl border border-slate-200 bg-white/50 p-2 shadow-2xl dark:border-slate-800 dark:bg-slate-900/50 backdrop-blur-sm -mt-8 mb-24">
        <div className="rounded-xl border border-slate-100 bg-slate-50 overflow-hidden dark:border-slate-800 dark:bg-slate-950 aspect-video relative flex items-center justify-center">
          {/* Декоративные элементы интерфейса */}
          <div className="absolute top-0 w-full h-12 border-b border-slate-200 dark:border-slate-800 flex items-center px-4 gap-2">
            <div className="w-3 h-3 rounded-full bg-red-400"></div>
            <div className="w-3 h-3 rounded-full bg-amber-400"></div>
            <div className="w-3 h-3 rounded-full bg-emerald-400"></div>
          </div>
          <div className="text-slate-400 flex flex-col items-center gap-4">
            <LayoutDashboard className="w-12 h-12 opacity-50" />
            <p className="font-medium tracking-widest uppercase text-sm opacity-50">
              Рабочее пространство
            </p>
          </div>
        </div>
      </div>

      {/* FEATURES GRID */}
      <div className="w-full max-w-6xl grid gap-6 md:grid-cols-3 mb-20">
        {[
          {
            icon: <Bot className="w-6 h-6 text-indigo-500" />,
            title: 'AI Редактор контента',
            text: 'Не пишите HTML руками. Наш Notion-like редактор и AI сверстают страницы в дизайне вашего сайта за вас.',
          },
          {
            icon: <Clock className="w-6 h-6 text-amber-500" />,
            title: 'Умные расписания',
            text: 'Запускайте генерации контента и вставку ссылок по cron-расписанию с учетом лимитов и часовых поясов.',
          },
          {
            icon: <Globe className="w-6 h-6 text-emerald-500" />,
            title: 'Управление ссылками',
            text: 'Автоматизированный Link-flow: вставка, удаление, повтор при ошибках и мониторинг доноров.',
          },
          {
            icon: <Zap className="w-6 h-6 text-yellow-500" />,
            title: 'Мониторинг индексации',
            text: 'Встроенные циклы проверок. Трекинг попыток, статистика и календарь индексации ваших сетей.',
          },
          {
            icon: <Database className="w-6 h-6 text-blue-500" />,
            title: 'Центр очередей (Queue)',
            text: 'Полный контроль над in-flight задачами. Приоритеты, логи воркеров и массовые перезапуски.',
          },
          {
            icon: <ShieldCheck className="w-6 h-6 text-rose-500" />,
            title: 'Аудит и LLM Биллинг',
            text: 'Прозрачный учет токенов и стоимости запросов к Gemini. Разделение биллинга по проектам и ролям.',
          },
        ].map((card, i) => (
          <div
            key={i}
            className="group rounded-2xl border border-slate-200 bg-white p-8 hover:shadow-xl hover:border-indigo-200 transition-all dark:border-slate-800 dark:bg-slate-900 dark:hover:border-indigo-900/50">
            <div className="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-xl bg-slate-50 dark:bg-slate-800 group-hover:scale-110 transition-transform">
              {card.icon}
            </div>
            <h3 className="text-xl font-bold text-slate-900 dark:text-white mb-2">{card.title}</h3>
            <p className="text-slate-600 dark:text-slate-400 leading-relaxed">{card.text}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
