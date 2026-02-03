"use client";

import { useAuthGuard } from "../../lib/useAuth";
import { FiActivity, FiCheckCircle, FiClock, FiAlertTriangle } from "react-icons/fi";

export default function MonitoringPage() {
  useAuthGuard();
  const checks = [
    { name: "API /healthz", status: "OK", detail: "Ответ 200" },
    { name: "БД", status: "OK", detail: "Подключено" },
    { name: "Очереди", status: "WARN", detail: "Запланировано подключить" }
  ];

  const latency = [
    { label: "P50", value: 120 },
    { label: "P95", value: 260 },
    { label: "P99", value: 410 }
  ];

  const uptime = [
    { label: "API", value: 99.9 },
    { label: "БД", value: 99.7 },
    { label: "Очереди", value: 98.4 }
  ];

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <h2 className="text-xl font-semibold mb-2">Мониторинг</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">Плейсхолдер под статус сервисов и метрики.</p>
      </div>
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <div className="space-y-3">
          {checks.map((c) => (
            <div key={c.name} className="flex items-center justify-between border-b border-slate-800 pb-2 last:border-0">
              <div>
                <div className="font-semibold flex items-center gap-2">
                  <FiActivity /> {c.name}
                </div>
                <div className="text-sm text-slate-500 dark:text-slate-400">{c.detail}</div>
              </div>
              <Status status={c.status} />
            </div>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
          <h3 className="font-semibold mb-3">Latency (ms)</h3>
          <div className="space-y-2">
            {latency.map((l) => (
              <div key={l.label}>
                <div className="flex justify-between text-sm text-slate-500 dark:text-slate-400">
                  <span>{l.label}</span>
                  <span>{l.value} ms</span>
                </div>
                <div className="h-2 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                  <div className="h-2 bg-indigo-500" style={{ width: `${Math.min(l.value / 5, 100)}%` }}></div>
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
          <h3 className="font-semibold mb-3">Uptime (%)</h3>
          <div className="space-y-2">
            {uptime.map((u) => (
              <div key={u.label} className="flex items-center justify-between">
                <span className="text-sm text-slate-500 dark:text-slate-400">{u.label}</span>
                <div className="flex items-center gap-2">
                  <div className="w-28 h-2 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                    <div className="h-2 bg-emerald-500" style={{ width: `${Math.min(u.value, 100)}%` }}></div>
                  </div>
                  <span className="text-sm font-semibold">{u.value}%</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function Status({ status }: { status: string }) {
  if (status === "OK") {
    return (
      <span className="text-xs text-green-400 flex items-center gap-1">
        <FiCheckCircle /> OK
      </span>
    );
  }
  return (
    <span className="text-xs text-amber-400 flex items-center gap-1">
      <FiAlertTriangle /> {status}
    </span>
  );
}
