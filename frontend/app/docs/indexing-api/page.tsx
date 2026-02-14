import { AdminOnlyDocs } from "../../../components/AdminOnlyDocs";

const code = (value: string) => (
  <pre className="mt-2 overflow-x-auto rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-700 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-200">
    <code>{value}</code>
  </pre>
);

export const metadata = {
  title: "API проверок индексации",
  description: "Примеры запросов для мониторинга индексации.",
};

export default function DocsIndexingApiPage() {
  return (
    <AdminOnlyDocs title="API проверок индексации для администраторов">
      <div className="space-y-6">
        <header>
          <h1 className="text-2xl font-bold">API проверок индексации</h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
            Примеры запросов для мониторинга индексации. Используйте их в дополнение
            к Swagger UI.
          </p>
        </header>

        <section className="space-y-3 text-sm text-slate-600 dark:text-slate-300">
          <p>
            Все запросы требуют авторизации через cookie <code>access_token</code>.
            Для curl укажите cookie вручную (значение можно взять из DevTools).
          </p>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
          <h2 className="text-base font-semibold">Список проверок по домену</h2>
          {code(`curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks?status=success,checking&is_indexed=true&from=2026-02-01&to=2026-02-12&search=example.com&sort=check_date:desc&limit=20&page=1"`)}
          <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
            Ответ: <code>{`{ items: IndexCheck[], total: number }`}</code>
          </p>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
          <h2 className="text-base font-semibold">История попыток</h2>
          {code(`curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks/{checkId}/history?limit=50"`)}
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
          <h2 className="text-base font-semibold">Статистика и календарь</h2>
          {code(`curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks/stats?from=2026-02-01&to=2026-02-12"`)}
          {code(`curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks/calendar?month=2026-02"`)}
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
          <h2 className="text-base font-semibold">Запуск вручную (домен)</h2>
          {code(`curl -s -X POST \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/domains/{domainId}/index-checks"`)}
          <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
            В ответе доступны поля <code>run_now_enqueued</code> и <code>run_now_error</code>.
          </p>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
          <h2 className="text-base font-semibold">Проектные проверки</h2>
          {code(`curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/projects/{projectId}/index-checks?status=success&limit=20&page=1"`)}
          {code(`curl -s -X POST \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/projects/{projectId}/index-checks"`)}
          <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
            Ответ содержит counters: <code>created</code>, <code>updated</code>, <code>skipped</code>, <code>enqueued</code>, <code>enqueue_failed</code>.
          </p>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white/80 p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950/60">
          <h2 className="text-base font-semibold">Admin‑эндпоинты</h2>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
            Доступны только администратору.
          </p>
          {code(`curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/admin/index-checks?domain_id={domainId}&limit=20&page=1"`)}
          {code(`curl -s -X POST \\
  -H "Content-Type: application/json" \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  -d '{ "domain_id": "{domainId}" }' \\
  "http://localhost:8080/api/admin/index-checks/run"`)}
          {code(`curl -s \\
  -H "Cookie: access_token=YOUR_TOKEN" \\
  "http://localhost:8080/api/admin/index-checks/failed?limit=20&page=1"`)}
        </section>
      </div>
    </AdminOnlyDocs>
  );
}
