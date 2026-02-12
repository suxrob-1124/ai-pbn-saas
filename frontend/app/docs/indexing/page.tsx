import { AdminOnlySection } from "../../../components/AdminOnlySection";

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
          Обычным пользователям мониторинг доступен из карточки проекта: откройте проект и в правой
          части шапки нажмите кнопку <strong>«Индексация»</strong> (рядом с «Очередь проекта»). Это
          откроет страницу мониторинга в контексте выбранного проекта. Администраторы дополнительно
          могут открыть глобальный мониторинг через меню <strong>Monitoring → Indexing</strong>.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Как работает проверка</h2>
        <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>
            Сервис indexchecker запускается по cron{" "}
            <code>@every 1h</code> и выполняет «тик».
          </li>
          <li>
            Для каждого опубликованного домена создаётся проверка на текущую дату
            (если её ещё нет). Уникальность обеспечивается парой{" "}
            <code>(domain_id, check_date)</code>.
          </li>
          <li>
            В работу берутся проверки со статусом <code>pending</code> и просроченным
            <code> next_retry_at</code>, а также проверки <code>checking</code>,
            у которых время ретрая уже наступило.
          </li>
          <li>
            Для каждой проверки вызывается SERP‑API с запросом{" "}
            <code>site:&lt;domain&gt;</code> (домен нормализуется в punycode).
          </li>
          <li>
            Если ответ получен — статус становится <code>success</code>, а поле{" "}
            <code>is_indexed</code> устанавливается в <code>true</code> или{" "}
            <code>false</code>.
          </li>
          <li>
            Если произошла ошибка — выполняется ретрай по расписанию, либо статус
            переходит в <code>failed_investigation</code>.
          </li>
        </ol>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Статусы</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li><strong>pending</strong> — создана, ожидает запуска.</li>
          <li><strong>checking</strong> — идёт проверка или запланирован повтор.</li>
          <li><strong>success</strong> — получен финальный ответ (в индексе/не в индексе).</li>
          <li><strong>failed_investigation</strong> — превышен лимит попыток.</li>
        </ul>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">
          Важно: «не в индексе» — это тоже корректный результат и считается
          успешной проверкой.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Какие домены участвуют</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Только опубликованные домены: статус <code>published</code> или заполнен <code>published_at</code>.</li>
          <li>URL домена должен быть задан.</li>
        </ul>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">
          Для непубликованных доменов проверка не запускается и ручной запуск
          блокируется.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Алгоритм запроса (SERP)</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Запрос: <code>site:&lt;domain&gt;</code> (punycode).</li>
          <li>Гео берётся из домена, иначе из проекта, иначе fallback <code>se</code>.</li>
          <li>Таймауты и ретраи SERP используют общие настройки (как в анализаторе).</li>
          <li>Файлы с диска не читаются: используется только URL и SERP‑ответ.</li>
        </ul>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">Ретраи и расписание</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-sm text-slate-600 dark:text-slate-300">
          <li>Attempt 1: 30 минут</li>
          <li>Attempt 2: 1 час</li>
          <li>Attempt 3: 2 часа</li>
          <li>Attempt 4: 4 часа</li>
          <li>Максимум: 8 попыток в сутки</li>
        </ul>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">
          Если прошло 24 часа с момента создания проверки или превышен лимит
          попыток — статус становится <code>failed_investigation</code>.
        </p>
      </section>

      <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
        <h2 className="text-base font-semibold">История попыток</h2>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-300">
          Каждая попытка логируется в <code>index_check_history</code>:
          результат (<code>success/error/timeout</code>), длительность, данные ответа
          или сообщение об ошибке. Это используется для истории в UI и для аналитики.
        </p>
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
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Ручной запуск возможен только для опубликованных доменов.
        </p>
      </section>

      <AdminOnlySection>
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
      </AdminOnlySection>

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
