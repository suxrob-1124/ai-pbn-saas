"use client";

import { useState } from "react";
import dynamic from "next/dynamic";
import "swagger-ui-react/swagger-ui.css";
import "./swagger-dark.css";
import { useAuthGuard } from "../../../lib/useAuth";

const SwaggerUI = dynamic(() => import("swagger-ui-react"), { ssr: false });

export default function ApiDocsPage() {
  const { me, loading } = useAuthGuard();
  const [copyStatus, setCopyStatus] = useState<string | null>(null);

  const handleCopyToken = async () => {
    setCopyStatus(null);
    const match = document.cookie.match(/(?:^|;\\s*)access_token=([^;]+)/);
    if (!match) {
      setCopyStatus(
        "Токен недоступен. Если cookie HttpOnly — копируйте через DevTools."
      );
      return;
    }
    const token = decodeURIComponent(match[1]);
    try {
      await navigator.clipboard.writeText(token);
      setCopyStatus("access_token скопирован");
    } catch {
      setCopyStatus("Не удалось скопировать. Скопируйте вручную из DevTools.");
    }
  };

  if (loading) {
    return (
      <div className="rounded-2xl border border-slate-200 bg-white/80 p-6 text-sm text-slate-600 shadow-sm dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-300">
        Загрузка документации...
      </div>
    );
  }

  if (!me) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="rounded-2xl border border-slate-200 bg-white/80 p-5 text-sm text-slate-700 shadow-sm dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200">
        <h2 className="text-base font-semibold">Как пользоваться Swagger UI</h2>
        <ol className="mt-2 list-decimal space-y-2 pl-5">
          <li>Выполните вход через интерфейс или вызовите <code>POST /api/login</code>.</li>
          <li>
            Скопируйте значение cookie <code>access_token</code> в DevTools →
            Application → Cookies.
          </li>
          <li>Нажмите <strong>Authorize</strong> и вставьте значение в поле.</li>
          <li>После авторизации выполняйте запросы как обычно.</li>
        </ol>
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={handleCopyToken}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 shadow-sm hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            Скопировать access_token
          </button>
          {copyStatus && (
            <span className="text-xs text-slate-500 dark:text-slate-300">
              {copyStatus}
            </span>
          )}
        </div>
      </div>
      <div className="rounded-2xl border border-slate-200 bg-white/80 p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900/70">
      <SwaggerUI
        url="/api/openapi"
        docExpansion="list"
        defaultModelsExpandDepth={-1}
        defaultModelExpandDepth={2}
      />
      </div>
    </div>
  );
}
